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
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/yuwen002/meteor-ch-workbench/internal/config"
)

// ColumnInfo 描述表的一列结构信息。
type ColumnInfo struct {
	Name       string `json:"name"`        // 列名
	Type       string `json:"type"`        // 列类型（如 String、UInt64、DateTime）
	Comment    string `json:"comment"`     // 列注释
	IsPrimary  bool   `json:"is_primary"`  // 是否属于主键（DESCRIBE 的 primary_key = 1）
	HasDefault bool   `json:"has_default"` // 是否有默认值表达式（DESCRIBE 的 default_type 非空）
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
	Columns   []ColumnInfo    `json:"columns"`         // 结果列信息
	Rows      [][]interface{} `json:"rows"`            // 结果行，行内元素按列顺序排列
	RowCount  int64           `json:"row_count"`       // 结果行数
	ElapsedMs float64         `json:"elapsed_ms"`      // 查询耗时（毫秒）
	Truncated bool            `json:"truncated"`       // 是否因超过行数上限被截断
	RowLimit  int             `json:"row_limit"`       // 本次生效的行数上限
	Error     string          `json:"error,omitempty"` // 执行错误信息，正常时为空
}

// ExecResult 是一次 DDL/DML（CREATE/ALTER/DROP/INSERT 等）的执行结果。
// ClickHouse 的 DDL 通常不影响行数，RowsAffected 多为 0；
// INSERT 时可能反映实际写入的行数。
type ExecResult struct {
	RowsAffected int64  `json:"rows_affected"`   // 受影响的行数
	Error        string `json:"error,omitempty"` // 执行错误信息，正常时为空
}

// ExecBatchResult 是一批 DDL/DML 语句顺序执行后的汇总回执。
// 每条语句的结果按原始下标保存在 Statements 中，便于调用方精确定位
// 哪些成功、哪些失败、哪些因前序失败而未执行（Skipped / Error 为空）。
type ExecBatchResult struct {
	Statements []StatementResult `json:"statements"` // 逐条执行结果，顺序与入参一致
	Ok         int               `json:"ok"`         // 成功条数
	Failed     int               `json:"failed"`     // 失败条数
	Skipped    int               `json:"skipped"`    // 因中断而未执行的条数
	ElapsedMs  float64           `json:"elapsed_ms"` // 整批耗时（毫秒）
}

// StatementResult 是批量执行中单条语句的结果。
// RowsAffected 来自底层 ExecResult；Error 非空表示该条失败。
type StatementResult struct {
	Index        int    `json:"index"`         // 在入参列表中的下标
	SQL          string `json:"sql"`           // 原始语句
	RowsAffected int64  `json:"rows_affected"` // 受影响的行数（失败时为 0）
	Error        string `json:"error,omitempty"`
}

// Service 是 ClickHouse 服务，按连接 ID 缓存 database/sql 连接池。
// sql.DB 本身即是并发安全的连接池，因此缓存它比缓存单条连接更合适。
// running 保存正在执行中的查询的取消函数，使前端可以主动中止查询。
type Service struct {
	mu      sync.Mutex                    // 保护 dbs 与 running 映射的互斥锁
	dbs     map[string]*sql.DB            // 连接 ID -> 连接池
	running map[string]context.CancelFunc // 查询 ID -> 取消函数
}

// NewService 创建 ClickHouse 服务实例，初始化空的连接池与运行中查询映射。
func NewService() *Service {
	return &Service{
		dbs:     make(map[string]*sql.DB),
		running: make(map[string]context.CancelFunc),
	}
}

// register 为一个查询 ID 注册可取消的 context，返回该 context 与注销函数。
// 未提供 queryID 时退化为不可取消的 context，保持向后兼容。
func (s *Service) register(queryID string) (context.Context, func()) {
	if queryID == "" {
		return context.Background(), func() {}
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	if old, ok := s.running[queryID]; ok {
		old()
	}
	s.running[queryID] = cancel
	s.mu.Unlock()
	return ctx, func() {
		s.mu.Lock()
		delete(s.running, queryID)
		s.mu.Unlock()
		cancel()
	}
}

// CancelQuery 取消指定查询 ID 正在执行的查询。
// 返回 true 表示确实找到了在跑的查询；查询已结束时返回 false。
func (s *Service) CancelQuery(queryID string) bool {
	if queryID == "" {
		return false
	}
	s.mu.Lock()
	cancel, ok := s.running[queryID]
	if ok {
		delete(s.running, queryID)
	}
	s.mu.Unlock()
	if ok {
		cancel()
		log.Printf("[CancelQuery] queryID=%q 已取消", queryID)
	}
	return ok
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

// scanRows 把 sql.Rows 读入二维切片，最多读取 limit 行（limit <= 0 表示不限制）。
// 每个值都经过 serializeValue 处理，保证结果可安全地序列化为 JSON 传给前端。
// 每读一行都会检查 ctx 是否已取消，使取消能在一行内及时生效，而不必等到驱动层超时。
// 返回读取到的行、是否因达到上限而仍有剩余行（truncated）。
func scanRows(ctx context.Context, rows *sql.Rows, limit int) ([][]interface{}, bool, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, false, err
	}
	var result [][]interface{} = make([][]interface{}, 0)
	colCount := len(cols)
	truncated := false
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		if limit > 0 && len(result) >= limit {
			truncated = true
			break
		}
		vals := make([]interface{}, colCount)
		ptrs := make([]interface{}, colCount)
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, false, err
		}
		row := make([]interface{}, colCount)
		for i, v := range vals {
			row[i] = serializeValue(v)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return result, truncated, err
	}
	return result, truncated, nil
}

// Query 在指定连接上执行 SQL 并返回结果集，行数上限为 DefaultQueryRowLimit。
// 如需按用户配置调整上限，请改用 QueryWithLimit。
// SQL 执行期错误不会向上返回 error，而是写入 QueryResult.Error，
// 前端据此在结果面板内展示错误；只有连接/列读取等基础设施错误才返回 error。
//
// queryID 用于支持主动取消：非空时注册可取消 context，调用 CancelQuery(queryID)
// 会让本次查询立即返回，错误信息标记为「查询已取消」，已读取的部分结果被丢弃。
func (s *Service) Query(conn config.Connection, sqlText string, queryID string) (*QueryResult, error) {
	return s.QueryWithLimit(conn, sqlText, queryID, DefaultQueryRowLimit)
}

// QueryWithLimit 与 Query 相同，但允许调用方指定行数上限。
// rowLimit <= 0 时退回 DefaultQueryRowLimit；上游（App）负责传入用户配置的值。
func (s *Service) QueryWithLimit(conn config.Connection, sqlText string, queryID string, rowLimit int) (*QueryResult, error) {
	if rowLimit <= 0 {
		rowLimit = DefaultQueryRowLimit
	}
	db, err := s.getDB(conn)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	ctx, unregister := s.register(queryID)
	defer unregister()

	rows, err := db.QueryContext(ctx, sqlText)
	if err != nil {
		if isCanceled(ctx) {
			return emptyQueryResult(errQueryCanceled, elapsedMs(start), rowLimit), nil
		}
		return emptyQueryResult(err.Error(), elapsedMs(start), rowLimit), nil
	}
	defer rows.Close()

	result := &QueryResult{
		RowLimit: rowLimit,
		Columns:  []ColumnInfo{},
		Rows:     make([][]interface{}, 0),
	}
	cols, err := rows.Columns()
	if err != nil {
		return result, err
	}
	for _, col := range cols {
		result.Columns = append(result.Columns, ColumnInfo{Name: col})
	}

	data, truncated, err := scanRows(ctx, rows, rowLimit)
	if err != nil {
		if isCanceled(ctx) {
			return emptyQueryResult(errQueryCanceled, elapsedMs(start), rowLimit), nil
		}
		return result, err
	}
	result.Rows = data
	result.RowCount = int64(len(data))
	result.Truncated = truncated
	result.ElapsedMs = elapsedMs(start)
	return result, nil
}

// errQueryCanceled 是用户主动取消查询时写入 QueryResult.Error 的提示文本。
const errQueryCanceled = "查询已取消"

// emptyQueryResult 构造一个不含结果集的 QueryResult。
// Columns / Rows 显式初始化为空切片，保证 JSON 序列化成 [] 而不是 null，
// 避免前端访问 result.columns[i] 时触发 "Cannot read properties of null"。
func emptyQueryResult(message string, elapsed float64, rowLimit int) *QueryResult {
	return &QueryResult{
		Columns:   []ColumnInfo{},
		Rows:      make([][]interface{}, 0),
		ElapsedMs: elapsed,
		RowLimit:  rowLimit,
		Error:     message,
	}
}

// isCanceled 判断 context 是否已被主动取消。
// 只认 context.Canceled；由超时导致的 DeadlineExceeded 不算用户取消。
func isCanceled(ctx context.Context) bool {
	return ctx.Err() == context.Canceled
}

// elapsedMs 返回从 start 到当前时刻的毫秒数，保留两位小数。
func elapsedMs(start time.Time) float64 {
	return float64(time.Since(start).Microseconds()) / 1000.0
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
	res, err := db.ExecContext(ctx, ensureMutationSync(sqlText))
	if err != nil {
		return &ExecResult{Error: err.Error()}, nil
	}

	affected, _ := res.RowsAffected()
	return &ExecResult{RowsAffected: affected}, nil
}

// execFunc 执行单条语句并返回受影响行数，供 ExecBatch 注入以便单测。
type execFunc func(ctx context.Context, sqlText string) (int64, error)

// ExecBatch 在同一个连接池上顺序执行多条 DDL/DML 语句。
//
// 相对逐条调用 Exec，批量执行只建立一次连接、只做一次跨进程往返，
// 避免「改 N 个单元格 = N 次 API 调用」的串行等待。
// 语句始终串行执行：ClickHouse 的 ALTER ... UPDATE 是 mutation，
// 并发提交只会让服务端排队，收益为零。
//
// continueOnError 为 false 时遇到第一条失败即停止，后续语句标记为 skipped；
// 为 true 时继续执行剩余语句，一次收集全部失败点。
//
// 与 Exec 保持一致：SQL 执行期错误写入回执中的 Error，不向上返回 error；
// 只有获取连接等基础设施错误才作为 error 返回。
func (s *Service) ExecBatch(conn config.Connection, statements []string, continueOnError bool) (*ExecBatchResult, error) {
	return s.execBatchWith(statements, continueOnError, func(ctx context.Context, sqlText string) (int64, error) {
		db, err := s.getDB(conn)
		if err != nil {
			return 0, err
		}
		res, err := db.ExecContext(ctx, ensureMutationSync(sqlText))
		if err != nil {
			return 0, err
		}
		affected, _ := res.RowsAffected()
		return affected, nil
	})
}

// execBatchWith 是 ExecBatch 的内部实现，执行动作由 exec 注入，
// 便于在没有真实 ClickHouse 的单元测试中覆盖中断 / 全量收集 / 下标回执等分支。
func (s *Service) execBatchWith(statements []string, continueOnError bool, exec execFunc) (*ExecBatchResult, error) {
	start := time.Now()
	result := &ExecBatchResult{Statements: make([]StatementResult, 0, len(statements))}

	ctx := context.Background()
	stopped := false
	for i, stmt := range statements {
		if stopped {
			result.Statements = append(result.Statements, StatementResult{Index: i, SQL: stmt})
			result.Skipped++
			continue
		}
		affected, err := exec(ctx, stmt)
		entry := StatementResult{Index: i, SQL: stmt}
		if err != nil {
			entry.Error = err.Error()
			result.Statements = append(result.Statements, entry)
			result.Failed++
			if !continueOnError {
				stopped = true
			}
			continue
		}
		entry.RowsAffected = affected
		result.Statements = append(result.Statements, entry)
		result.Ok++
	}
	result.ElapsedMs = elapsedMs(start)
	return result, nil
}

// ensureMutationSync 对 ALTER ... UPDATE / DELETE 类 mutation 语句追加 SETTINGS mutations_sync=2，
// 让服务端等 mutation 在全部副本上完成后才返回，避免调用方紧接着查询时仍读到旧数据。
// 非 mutation 语句，或调用方已显式指定 mutations_sync 时原样返回。
func ensureMutationSync(sqlText string) string {
	if !isMutationStatement(sqlText) {
		return sqlText
	}
	if hasSetting(sqlText, "mutations_sync") {
		return sqlText
	}
	trimmed := strings.TrimRight(strings.TrimSpace(sqlText), ";")
	return trimmed + " SETTINGS mutations_sync = 2"
}

// isMutationStatement 判断语句是否为 ClickHouse 需要等待完成的 mutation。
// 只看首两个关键字：ALTER TABLE ... UPDATE / DELETE，以及独立的 DELETE FROM。
func isMutationStatement(sqlText string) bool {
	fields := strings.Fields(strings.ToUpper(stripLeadingComments(sqlText)))
	if len(fields) == 0 {
		return false
	}
	if fields[0] == "DELETE" {
		return len(fields) > 1 && fields[1] == "FROM"
	}
	if fields[0] != "ALTER" || len(fields) < 3 {
		return false
	}
	for _, f := range fields {
		if f == "UPDATE" || f == "DELETE" {
			return true
		}
	}
	return false
}

// stripLeadingComments 去掉语句开头的空白与 -- / /* */ 注释，便于按关键字判断语句类型。
func stripLeadingComments(sqlText string) string {
	s := strings.TrimSpace(sqlText)
	for {
		if strings.HasPrefix(s, "--") {
			if idx := strings.IndexByte(s, '\n'); idx >= 0 {
				s = strings.TrimSpace(s[idx+1:])
				continue
			}
			return ""
		}
		if strings.HasPrefix(s, "/*") {
			if idx := strings.Index(s, "*/"); idx >= 0 {
				s = strings.TrimSpace(s[idx+2:])
				continue
			}
			return ""
		}
		return s
	}
}

// hasSetting 判断语句中是否已显式包含指定的 SETTINGS 键。
func hasSetting(sqlText, key string) bool {
	lower := strings.ToLower(sqlText)
	return strings.Contains(lower, strings.ToLower(key)+" ")
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
	query := "SELECT name, engine, comment FROM system.tables WHERE database = " + quoteLiteral(database) +
		" AND engine NOT LIKE '%View'"
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

// GetViews 返回指定数据库下的全部视图信息（名称、引擎、注释）。
// 视图同样存储在 system.tables 中，通过引擎以 View 结尾来区分。
func (s *Service) GetViews(conn config.Connection, database string) ([]TableInfo, error) {
	db, err := s.getDB(conn)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	query := "SELECT name, engine, comment FROM system.tables WHERE database = " + quoteLiteral(database) +
		" AND engine LIKE '%View'"
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var views []TableInfo
	for rows.Next() {
		var name, engine, comment string
		if err := rows.Scan(&name, &engine, &comment); err != nil {
			return nil, err
		}
		views = append(views, TableInfo{
			Name:    name,
			Engine:  engine,
			Comment: comment,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return views, nil
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
			switch name {
			case "name", "type", "comment":
				sval, _ := vals[i].(string)
				switch name {
				case "name":
					info.Name = sval
				case "type":
					info.Type = sval
				case "comment":
					info.Comment = sval
				}
			case "primary_key":
				info.IsPrimary = isTruthy(vals[i])
			case "default_type":
				sval, _ := vals[i].(string)
				info.HasDefault = sval != ""
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

// isTruthy 判断驱动返回的标志位是否等价于「真」。
// ClickHouse 的 DESCRIBE 中 primary_key 为 UInt8，驱动可能返回 uint8/int64/bool，
// 也兼容字符串形式的 "1"，避免因类型断言失败而丢失主键信息。
func isTruthy(v interface{}) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case string:
		return t == "1" || strings.EqualFold(t, "true")
	case []byte:
		return string(t) == "1" || strings.EqualFold(string(t), "true")
	}
	n, err := strconv.ParseFloat(fmt.Sprint(v), 64)
	return err == nil && n != 0
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
