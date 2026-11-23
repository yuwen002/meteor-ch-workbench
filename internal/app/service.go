// Package app 提供与具体 GUI 框架无关的应用服务。
//
// 本包只依赖领域服务与配置管理，不依赖 Wails、Gio 或任何前端运行时。
// GUI 适配层应把用户操作转换为本包的方法调用，再把结果映射到对应界面。
package app

import (
	"io"
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
	tabs  *config.TabsManager
	saved *config.SavedSQLManager
	ch    *clickhouse.Service
	prefs *config.SettingsManager
}

// NewService 创建与 GUI 无关的应用服务。传入的依赖由应用入口统一创建和管理。
func NewService(
	conf *config.Manager,
	hist *config.HistoryManager,
	tabs *config.TabsManager,
	saved *config.SavedSQLManager,
	ch *clickhouse.Service,
	prefs *config.SettingsManager,
) *Service {
	return &Service{conf: conf, hist: hist, tabs: tabs, saved: saved, ch: ch, prefs: prefs}
}

// SaveConnection 保存连接配置，并使旧连接池失效。
func (s *Service) SaveConnection(conn config.Connection) error {
	// Manager.SaveConnection 接收值参数并在内部生成 ID；提前生成可确保
	// 保存后失效的是正确连接池，而不是空 ID 对应的条目。
	if conn.ID == "" {
		conn.GenID()
	}
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

// GetTableDetails 获取指定表的 ClickHouse 引擎、DDL、分区键、排序键、主键和 TTL。
func (s *Service) GetTableDetails(connectionID, database, table string) (*clickhouse.TableDetails, error) {
	conn, err := s.conf.GetConnection(connectionID)
	if err != nil {
		return nil, err
	}
	return s.ch.GetTableDetails(*conn, database, table)
}

// GetClusters 获取 ClickHouse 集群拓扑。
func (s *Service) GetClusters(connectionID string) ([]clickhouse.ClusterInfo, error) {
	conn, err := s.conf.GetConnection(connectionID)
	if err != nil {
		return nil, err
	}
	return s.ch.GetClusters(*conn)
}


// --- 配置与本地工作区用例 ---

func (s *Service) GetAll() (*config.DataFile, error) { return s.conf.LoadAll() }
func (s *Service) GetConnections() ([]config.Connection, error) { return s.conf.ListConnections() }
func (s *Service) GetConnection(id string) (*config.Connection, error) { return s.conf.GetConnection(id) }
func (s *Service) GetFolders() ([]config.Folder, error) { return s.conf.ListFolders() }
func (s *Service) SaveFolder(folder config.Folder) error { return s.conf.SaveFolder(folder) }
func (s *Service) DeleteFolder(id string) error { return s.conf.DeleteFolder(id) }
func (s *Service) ReorderFolders(ids []string) error { return s.conf.ReorderFolders(ids) }
func (s *Service) ReorderConnections(ids []string) error { return s.conf.ReorderConnections(ids) }
func (s *Service) ExportConfig() (string, error) { return s.conf.ExportData() }
func (s *Service) GetExportFormats() []config.ExportFormat { return config.ExportFormats() }
func (s *Service) ExportConfigByFormat(format string, includePassword bool) (string, string, error) {
	return s.conf.ExportByFormat(format, includePassword)
}
func (s *Service) ImportConfig(raw string) error { return s.conf.ImportData(raw) }
func (s *Service) GetImportFormats() []config.ImportFormat { return config.ImportFormats() }
func (s *Service) ImportConfigWithOptions(raw string, opts config.ImportOptions) (config.ImportResult, error) {
	return s.conf.ImportDataWithOptions(raw, opts)
}
func (s *Service) GetSettings() (config.SettingsFile, error) { return s.prefs.Load() }
func (s *Service) SaveSettings(settings config.SettingsFile) (config.SettingsFile, error) {
	if err := s.prefs.Save(settings); err != nil { return config.SettingsFile{}, err }
	return s.prefs.Load()
}
func (s *Service) GetQueryHistory() ([]config.HistoryEntry, error) { return s.hist.List() }
func (s *Service) ClearQueryHistory() error { return s.hist.Clear() }
func (s *Service) DeleteQueryHistory(id string) error { return s.hist.Delete(id) }
func (s *Service) GetSavedSQL() ([]config.SavedSQL, error) { return s.saved.List() }
func (s *Service) SaveSQL(name, sql, connectionID string) (*config.SavedSQL, error) {
	item, err := s.saved.Add(config.SavedSQL{Name:name, SQL:sql, ConnectionID:connectionID})
	if err != nil { return nil, err }
	return &item, nil
}
func (s *Service) UpdateSavedSQL(id, name, sql string) (*config.SavedSQL, error) {
	item, err := s.saved.Update(id, name, sql)
	if err != nil { return nil, err }
	return &item, nil
}
func (s *Service) DeleteSavedSQL(id string) error { return s.saved.Delete(id) }
func (s *Service) GetTabs() (config.TabsFile, error) { return s.tabs.Load() }
func (s *Service) SaveTabs(snapshot config.TabsFile) error { return s.tabs.Save(snapshot) }
func (s *Service) ConfigDir() string { return s.conf.ConfigDir() }


// GetResultExportFormats 返回可用的查询结果导出格式。
func (s *Service) GetResultExportFormats() []clickhouse.ResultExportFormat {
	return clickhouse.ResultExportFormats()
}

// ExportQueryResult 将查询结果写入调用方提供的目标流；文件路径与对话框由 GUI 适配层负责。
func (s *Service) ExportQueryResult(w io.Writer, format string, columns []string, rows [][]interface{}, target string) (string, error) {
	return clickhouse.ExportResultTo(w, format, columns, rows, target)
}


// Close 释放应用持有的数据库连接池，应在 GUI 生命周期结束时调用。
func (s *Service) Close() {
	s.ch.CloseAll()
}
