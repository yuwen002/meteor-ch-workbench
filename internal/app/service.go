// Package app 提供与具体 GUI 框架无关的应用服务。
//
// 本包只依赖领域服务与配置管理，不依赖 Wails、Gio 或任何前端运行时。
// GUI 适配层应把用户操作转换为本包的方法调用，再把结果映射到对应界面。
package app

import (
	"strings"
	"time"

	"github.com/yuwen002/meteor-ch-workbench/internal/clickhouse"
	"github.com/yuwen002/meteor-ch-workbench/internal/config"
)

// Service 编排配置、查询历史、用户设置与 ClickHouse 服务。
//
// 依赖通过构造函数注入，便于后续 Gio、Wails 等不同界面复用同一业务入口。
type Service struct {
	conf  *config.Manager
	hist  *config.HistoryManager
	ch    *clickhouse.Service
	prefs *config.SettingsManager
}

// NewService 创建与 GUI 无关的应用服务。传入的依赖由应用入口统一创建和管理。
func NewService(
	conf *config.Manager,
	hist *config.HistoryManager,
	ch *clickhouse.Service,
	prefs *config.SettingsManager,
) *Service {
	return &Service{conf: conf, hist: hist, ch: ch, prefs: prefs}
}

// SaveConnection 保存连接配置，并使旧连接池失效。
func (s *Service) SaveConnection(conn config.Connection) error {
	if err := s.conf.SaveConnection(conn); err != nil {
		return err
	}
	s.ch.Invalidate(conn.ID)
	return nil
}

// DeleteConnection 删除连接配置，并释放对应连接池。
func (s *Service) DeleteConnection(id string) error {
	if err := s.conf.DeleteConnection(id); err != nil {
		return err
	}
	s.ch.Invalidate(id)
	return nil
}

// TestConnection 测试连接参数，不要求连接已保存。
func (s *Service) TestConnection(conn config.Connection) error {
	return s.ch.TestConnection(conn)
}

// Disconnect 断开指定连接。
func (s *Service) Disconnect(connectionID string) {
	s.ch.Invalidate(connectionID)
}

// Query 执行 SQL，并在查询完成后记录历史。SQL 执行错误仍由 QueryResult.Error 承载。
func (s *Service) Query(connectionID, sql, queryID string) (*clickhouse.QueryResult, error) {
	conn, err := s.conf.GetConnection(connectionID)
	if err != nil {
		return nil, err
	}
	res, err := s.ch.QueryWithLimit(*conn, sql, queryID, s.prefs.RowLimit())
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(sql) != "" {
		_ = s.hist.Add(config.HistoryEntry{
			SQL:          sql,
			ConnectionID: connectionID,
			Timestamp:    time.Now(),
			DurationMs:   res.ElapsedMs,
			Success:      res.Error == "",
		})
	}
	return res, nil
}

// CancelQuery 取消指定 ID 的在途查询。
func (s *Service) CancelQuery(queryID string) bool {
	return s.ch.CancelQuery(queryID)
}

// Execute 执行一条 DDL/DML 语句。
func (s *Service) Execute(connectionID, sql string) (*clickhouse.ExecResult, error) {
	conn, err := s.conf.GetConnection(connectionID)
	if err != nil {
		return nil, err
	}
	return s.ch.Exec(*conn, sql)
}

// ExecuteBatch 顺序执行多条 DDL/DML 语句，单条失败不会中断整批执行。
func (s *Service) ExecuteBatch(connectionID string, statements []string) (*clickhouse.ExecBatchResult, error) {
	conn, err := s.conf.GetConnection(connectionID)
	if err != nil {
		return nil, err
	}
	return s.ch.ExecBatch(*conn, statements, true)
}

// GetDatabases 获取连接下的数据库列表。
func (s *Service) GetDatabases(connectionID string) ([]string, error) {
	conn, err := s.conf.GetConnection(connectionID)
	if err != nil {
		return nil, err
	}
	return s.ch.GetDatabases(*conn)
}

// GetTables 获取指定数据库中的表。
func (s *Service) GetTables(connectionID, database string) ([]clickhouse.TableInfo, error) {
	conn, err := s.conf.GetConnection(connectionID)
	if err != nil {
		return nil, err
	}
	return s.ch.GetTables(*conn, database)
}

// GetViews 获取指定数据库中的视图。
func (s *Service) GetViews(connectionID, database string) ([]clickhouse.TableInfo, error) {
	conn, err := s.conf.GetConnection(connectionID)
	if err != nil {
		return nil, err
	}
	return s.ch.GetViews(*conn, database)
}

// GetTableSchema 获取指定表的列结构。
func (s *Service) GetTableSchema(connectionID, database, table string) ([]clickhouse.ColumnInfo, error) {
	conn, err := s.conf.GetConnection(connectionID)
	if err != nil {
		return nil, err
	}
	return s.ch.GetTableSchema(*conn, database, table)
}

// GetClusters 获取 ClickHouse 集群拓扑。
func (s *Service) GetClusters(connectionID string) ([]clickhouse.ClusterInfo, error) {
	conn, err := s.conf.GetConnection(connectionID)
	if err != nil {
		return nil, err
	}
	return s.ch.GetClusters(*conn)
}
