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
	owners  map[string]string             // 查询 ID -> 所属连接 ID，用于按连接批量取消
}

// NewService 创建 ClickHouse 服务实例，初始化空的连接池与运行中查询映射。
func NewService() *Service {
	return &Service{
		dbs:     make(map[string]*sql.DB),
		running: make(map[string]context.CancelFunc),
		owners:  make(map[string]string),
	}
}

// register 为一个查询 ID 注册可取消的 context，返回该 context 与注销函数。
// 未提供 queryID 时退化为不可取消的 context，保持向后兼容。
// connID 记录查询归属，使断开连接时能批量取消该连接的全部在途查询。
func (s *Service) register(connID string, queryID string) (context.Context, func()) {
	if queryID == "" {
		return context.Background(), func() {}
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	if old, ok := s.running[queryID]; ok {
		old()
	}
	s.running[queryID] = cancel
	if connID != "" {
		s.owners[queryID] = connID
	} else {
		delete(s.owners, queryID)
	}
	s.mu.Unlock()
	return ctx, func() {
		s.mu.Lock()
		delete(s.running, queryID)
		delete(s.owners, queryID)
		s.mu.Unlock()
		cancel()
	}
}

// cancelByConn 取消指定连接下所有在途查询，返回被取消的数量。
func (s *Service) cancelByConn(connID string) int {
	if connID == "" {
		return 0
	}
	s.mu.Lock()
	cancels := make([]context.CancelFunc, 0)
	for queryID, owner := range s.owners {
		if owner != connID {
			continue
		}
		if cancel, ok := s.running[queryID]; ok {
			cancels = append(cancels, cancel)
			delete(s.running, queryID)
			delete(s.owners, queryID)
		}
	}
	s.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	if len(cancels) > 0 {
		log.Printf("[cancelByConn] connectionID=%q 取消了 %d 个在途查询", connID, len(cancels))
	}
	return len(cancels)
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
		delete(s.owners, queryID)
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
// 先取消该连接下所有在途查询再关闭连接池，避免旧会话继续持有已失效的凭据。
func (s *Service) Invalidate(id string) {
	s.cancelByConn(id)
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

// Query / Exec / 元数据读取分别见 query.go、exec.go、metadata.go。

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