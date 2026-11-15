// Package clickhouse 封装 ClickHouse 的连接管理与查询能力。
//
// 本包刻意通过标准库 database/sql + DSN 的方式建连，而不是使用驱动的
// 不变式 API（clickhouse.Open + Options.Protocol）。原因是该 API 在某些版本下
// 存在协议路由缺陷：即使指定 Protocol 为 HTTP，实际仍会发送 Native 握手包，
// 导致只开放 HTTP 端口的服务端返回 "HTTP/1.1 400 Bad Request"，
// 客户端解析到首字节 'H'(72) 后抛出 [handshake] unexpected packet [72] from server。
// 而 DSN 的 scheme（http/https/clickhouse/clickhouse+secure）能可靠地决定协议。
package clickhouse

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/yuwen002/meteor-ch-workbench/internal/config"
)

// ColumnInfo 描述表的一列结构信息。
type ColumnInfo struct {
	Name    string `json:"name"`    // 列名
	Type    string `json:"type"`    // 列类型（如 String、UInt64、DateTime）
	Comment string `json:"comment"` // 列注释
}

// TableInfo 描述一张表的基本信息。
type TableInfo struct {
	Name    string `json:"name"`    // 表名
	Engine  string `json:"engine"`  // 表引擎（如 MergeTree、Memory）
	Comment string `json:"comment"` // 表注释
}

// ClusterInfo 描述 system.clusters 中的一条集群节点记录。
type ClusterInfo struct {
	Cluster     string `json:"cluster"`      // 集群名称
	Version     string `json:"version"`      // 节点 ClickHouse 版本
	HostName    string `json:"host_name"`    // 节点主机名
	HostAddress string `json:"host_address"` // 节点主机地址
	Port        int    `json:"port"`         // 节点端口
	IsLocal     bool   `json:"is_local"`     // 是否为当前连接所在节点
	ShardNum    int    `json:"shard_num"`    // 分片编号
	ReplicaNum  int    `json:"replica_num"`  // 副本编号
	UUID        string `json:"uuid"`         // 节点唯一标识
}

// QueryResult 是一次 SQL 查询的执行结果。
// 注意：SQL 语法错误等执行期错误放在 Error 字段而非方法返回值，
// 这样前端仍然能拿到结构化的结果对象。
type QueryResult struct {
	Columns  []ColumnInfo    `json:"columns"`         // 结果列信息
	Rows     [][]interface{} `json:"rows"`            // 结果行，行内元素按列顺序排列
	RowCount int64           `json:"row_count"`       // 结果行数
	Error    string          `json:"error,omitempty"` // 执行错误信息，正常时为空
}

// ExecResult 是一次 DDL/DML（CREATE/ALTER/DROP/INSERT 等）的执行结果。
// ClickHouse 的 DDL 通常不影响行数，RowsAffected 多为 0；
// INSERT 时可能反映实际写入的行数。
type ExecResult struct {
	RowsAffected int64  `json:"rows_affected"`  // 受影响的行数
	Error        string `json:"error,omitempty"` // 执行错误信息，正常时为空
}

// Service 是 ClickHouse 服务，按连接 ID 缓存 database/sql 连接池。
// sql.DB 本身即是并发安全的连接池，因此缓存它比缓存单条连接更合适。
type Service struct {
	mu  sync.Mutex         // 保护 dbs 映射的互斥锁
	dbs map[string]*sql.DB // 连接 ID -> 连接池
}

// NewService 创建 ClickHouse 服务实例，初始化空的连接池映射。
func NewService() *Service {
	return &Service{
		dbs: make(map[string]*sql.DB),
	}
}

// buildDSN 根据连接配置拼装 database/sql 使用的 DSN 字符串。
//
// 协议由 URL scheme 决定：
//   - HTTP 明文：http://
//   - HTTP 加密：https://
//   - Native 明文：clickhouse://
//   - Native 加密：clickhouse+secure://
//
// 数据库名为空时不写入 path，以便用户以「大账号」访问所有库。
func buildDSN(conn config.Connection) string {
	u := &url.URL{}
	if conn.Protocol == "http" {
		if conn.Secure {
			u.Scheme = "https"
		} else {
			u.Scheme = "http"
		}
	} else {
		if conn.Secure {
			u.Scheme = "clickhouse+secure"
		} else {
			u.Scheme = "clickhouse"
		}
	}
	username := conn.User
	if username == "" {
		username = "default"
	}
	u.User = url.UserPassword(username, conn.Password)
	u.Host = fmt.Sprintf("%s:%d", conn.Host, conn.Port)
	u.Path = "/" + conn.Database

	q := u.Query()
	if conn.UseCompress {
		q.Set("compress", "true")
	}
	if conn.Database != "" {
		q.Set("database", conn.Database)
	}
	if conn.Secure {
		q.Set("secure", "true")
		q.Set("skip_verify", "true")
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// getDB 获取指定连接对应的连接池，不存在时惰性创建。
// sql.Open 不会立即建连，真正的握手发生在首次 Ping 或执行查询时。
func (s *Service) getDB(conn config.Connection) (*sql.DB, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if db, ok := s.dbs[conn.ID]; ok {
		return db, nil
	}

	db, err := sql.Open("clickhouse", buildDSN(conn))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	s.dbs[conn.ID] = db
	return db, nil
}

// TestConnection 使用给定参数做一次真实连接测试。
// 先丢弃该 ID 下的旧连接池，确保测试的是最新参数而不是缓存结果。
// 测试失败时同样清理连接池，避免把坏连接留在缓存里。
func (s *Service) TestConnection(conn config.Connection) error {
	s.Invalidate(conn.ID)

	log.Printf("[TestConnection] id=%q host=%q port=%d protocol=%q database=%q secure=%v compress=%v",
		conn.ID, conn.Host, conn.Port, conn.Protocol, conn.Database, conn.Secure, conn.UseCompress)

	db, err := s.getDB(conn)
	if err != nil {
		s.Invalidate(conn.ID)
		return err
	}
	if err := db.Ping(); err != nil {
		s.Invalidate(conn.ID)
		return err
	}
	return nil
}

// Invalidate 关闭并移除指定连接的连接池。
// 连接参数变更、删除连接或测试失败时调用，确保后续操作重新建连。
func (s *Service) Invalidate(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if db, ok := s.dbs[id]; ok {
		_ = db.Close()
		delete(s.dbs, id)
	}
}

// scanRows 把 sql.Rows 全部读入二维切片。
// 每个值都经过 serializeValue 处理，保证结果可安全地序列化为 JSON 传给前端。
func scanRows(rows *sql.Rows) ([][]interface{}, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var result [][]interface{}
	colCount := len(cols)
	for rows.Next() {
		vals := make([]interface{}, colCount)
		ptrs := make([]interface{}, colCount)
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make([]interface{}, colCount)
		for i, v := range vals {
			row[i] = serializeValue(v)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	return result, nil
}

// Query 在指定连接上执行 SQL 并返回完整结果集。
// SQL 执行期错误不会向上返回 error，而是写入 QueryResult.Error，
// 前端据此在结果面板内展示错误；只有连接/列读取等基础设施错误才返回 error。
func (s *Service) Query(conn config.Connection, sqlText string) (*QueryResult, error) {
	db, err := s.getDB(conn)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	rows, err := db.QueryContext(ctx, sqlText)
	if err != nil {
		return &QueryResult{Error: err.Error()}, nil
	}
	defer rows.Close()

	result := &QueryResult{}
	cols, err := rows.Columns()
	if err != nil {
		return result, err
	}
	for _, col := range cols {
		result.Columns = append(result.Columns, ColumnInfo{Name: col})
	}

	data, err := scanRows(rows)
	if err != nil {
		return result, err
	}
	result.Rows = data
	result.RowCount = int64(len(data))
	return result, nil
}

// Exec 执行一条不返回结果集的 SQL（DDL / DML）。
// 包括 CREATE / ALTER / DROP / TRUNCATE / INSERT / RENAME 等。
// 与 Query 保持一致：SQL 执行期错误写入 ExecResult.Error，
// 只有连接等基础设施错误才作为 error 返回。
func (s *Service) Exec(conn config.Connection, sqlText string) (*ExecResult, error) {
	db, err := s.getDB(conn)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	res, err := db.ExecContext(ctx, sqlText)
	if err != nil {
		return &ExecResult{Error: err.Error()}, nil
	}

	affected, _ := res.RowsAffected()
	return &ExecResult{RowsAffected: affected}, nil
}

// GetDatabases 返回该连接可见的全部数据库名（SHOW DATABASES）。
// 使用大权限账号且未指定默认库时，前端可据此在侧边栏列出所有库。
func (s *Service) GetDatabases(conn config.Connection) ([]string, error) {
	db, err := s.getDB(conn)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	rows, err := db.QueryContext(ctx, "SHOW DATABASES")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dbs []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		dbs = append(dbs, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return dbs, nil
}

// GetTables 返回指定数据库下的全部表信息（名称、引擎、注释）。
// 数据库名通过 quoteLiteral 转义后拼入 SQL，避免特殊字符导致语句异常。
func (s *Service) GetTables(conn config.Connection, database string) ([]TableInfo, error) {
	db, err := s.getDB(conn)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	query := "SELECT name, engine, comment FROM system.tables WHERE database = " + quoteLiteral(database)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var name, engine, comment string
		if err := rows.Scan(&name, &engine, &comment); err != nil {
			return nil, err
		}
		tables = append(tables, TableInfo{
			Name:    name,
			Engine:  engine,
			Comment: comment,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tables, nil
}

// GetTableSchema 返回指定表的列结构。
// 按结果列名（name/type/comment）匹配取值，而不是按固定列序，
// 这样在不同 ClickHouse 版本上（DESCRIBE 返回的列数可能不同）都能正确解析。
func (s *Service) GetTableSchema(conn config.Connection, database string, table string) ([]ColumnInfo, error) {
	db, err := s.getDB(conn)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	query := fmt.Sprintf("DESCRIBE TABLE %s.%s", quoteIdent(database), quoteIdent(table))
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var schema []ColumnInfo
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		info := ColumnInfo{}
		for i, name := range cols {
			sval, _ := vals[i].(string)
			switch name {
			case "name":
				info.Name = sval
			case "type":
				info.Type = sval
			case "comment":
				info.Comment = sval
			}
		}
		schema = append(schema, info)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return schema, nil
}

// GetClusters 返回 system.clusters 中的集群拓扑信息。
// 不同版本的 ClickHouse 在 system.clusters 表中提供的列略有差异
// （例如 version、uuid 是较新版本才加入的），这里先通过 introspect
// 确认有哪些列可用，再动态构建 SELECT，保证对老版本（如 22.5）兼容。
// 库内 port、shard_num、replica_num 为无符号类型，这里先按 uint64 读取再转 int。
func (s *Service) GetClusters(conn config.Connection) ([]ClusterInfo, error) {
	db, err := s.getDB(conn)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	colRows, err := db.QueryContext(ctx,
		"SELECT name FROM system.columns WHERE database = 'system' AND table = 'clusters'")
	if err != nil {
		return nil, err
	}
	defer colRows.Close()

	available := map[string]bool{}
	for colRows.Next() {
		var name string
		if err := colRows.Scan(&name); err != nil {
			return nil, err
		}
		available[name] = true
	}
	if err := colRows.Err(); err != nil {
		return nil, err
	}

	type colSpec struct {
		name     string
		required bool
	}
	desired := []colSpec{
		{"cluster", true},
		{"host_name", true},
		{"host_address", true},
		{"port", true},
		{"is_local", true},
		{"shard_num", true},
		{"replica_num", true},
		{"version", false},
		{"uuid", false},
	}

	var selectCols []string
	for _, c := range desired {
		if c.required && !available[c.name] {
			return nil, fmt.Errorf("system.clusters 缺少必要列 %q，无法获取集群信息", c.name)
		}
		if available[c.name] {
			selectCols = append(selectCols, c.name)
		}
	}

	query := "SELECT " + strings.Join(selectCols, ", ") + " FROM system.clusters"
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var clusters []ClusterInfo
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		info := ClusterInfo{}
		for i, name := range cols {
			switch name {
			case "cluster":
				info.Cluster = toString(vals[i])
			case "version":
				info.Version = toString(vals[i])
			case "host_name":
				info.HostName = toString(vals[i])
			case "host_address":
				info.HostAddress = toString(vals[i])
			case "port":
				info.Port = toInt(vals[i])
			case "is_local":
				if b, ok := vals[i].(bool); ok {
					info.IsLocal = b
				}
			case "shard_num":
				info.ShardNum = toInt(vals[i])
			case "replica_num":
				info.ReplicaNum = toInt(vals[i])
			case "uuid":
				info.UUID = toString(vals[i])
			}
		}
		clusters = append(clusters, info)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return clusters, nil
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func toInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case uint64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

// CloseAll 关闭并清空全部缓存的连接池，应用退出时调用。
func (s *Service) CloseAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, db := range s.dbs {
		_ = db.Close()
		delete(s.dbs, id)
	}
}

// quoteLiteral 把字符串转义为合法的 ClickHouse 字符串字面量：
// 外层加单引号，内部单引号按 SQL 标准双写。
func quoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// quoteIdent 把标识符（库名/表名）转义为合法的反引号标识符，
// 内部反引号按 ClickHouse 规则加反斜杠转义。
func quoteIdent(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "\\`") + "`"
}

// serializeValue 把数据库驱动返回的原始值转换成可 JSON 序列化的值。
// 先尝试 JSON 编解码归一化（可处理数组、日期等特殊类型），失败则退化为字符串输出。
// nil 保持为 nil，前端可据此渲染为 NULL。
func serializeValue(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	if b, err := json.Marshal(v); err == nil {
		var out interface{}
		if err := json.Unmarshal(b, &out); err == nil {
			return out
		}
	}
	return fmt.Sprintf("%v", v)
}