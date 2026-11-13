package clickhouse

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	chdriver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/yuwen002/meteor-ch-workbench/internal/config"
)

const (
	// metaTimeout 限制元数据查询（库 / 表 / 列 / 集群）的等待时间，
	// 避免某个节点不可达时侧边栏长时间无响应。
	metaTimeout = 5 * time.Second
	// versionTimeout 用于向集群所有副本取 version()，跨节点查询更容易慢，单独限时。
	versionTimeout = 5 * time.Second
)

// ColumnInfo 是一列的元信息，用于 GetTableSchema。
type ColumnInfo struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Default string `json:"default,omitempty"`
	Comment string `json:"comment"`
}

// TableInfo 是一张表的元信息。
type TableInfo struct {
	Name    string `json:"name"`
	Engine  string `json:"engine"`
	Comment string `json:"comment"`
}

// ClusterInfo 对应 ClickHouse 系统表 system.clusters 的真实列。
//
// 注意：system.clusters 里没有 version 列，也没有 uuid 列
// （可用列见官方文档 reference/system-tables/clusters：cluster / shard_num /
// shard_name / shard_weight / internal_replication / replica_num / host_name /
// host_address / port / is_local / user / default_database / errors_count /
// slowdowns_count / estimated_recovery_time / database_shard_name /
// database_replica_name / is_active / unsynced_after_recovery /
// replication_lag / recovery_time）。
//
// 因此 Version 是对每个集群额外执行一次
//
//	SELECT hostName(), version() FROM clusterAllReplicas('<cluster>', system.one)
//
// 拿到的节点版本，属于 best-effort：取不到时为空字符串，前端必须容忍空值。
type ClusterInfo struct {
	Cluster             string `json:"cluster"`
	ShardNum            uint32 `json:"shard_num"`
	ShardName           string `json:"shard_name"`
	ReplicaNum          uint32 `json:"replica_num"`
	HostName            string `json:"host_name"`
	HostAddress         string `json:"host_address"`
	Port                uint16 `json:"port"`
	IsLocal             bool   `json:"is_local"`
	ErrorsCount         uint32 `json:"errors_count"`
	DatabaseShardName   string `json:"database_shard_name"`
	DatabaseReplicaName string `json:"database_replica_name"`
	Version             string `json:"version"`
}

// QueryResult 是查询结果。Columns 刻意使用 []string（列名数组），
// 与前端 types.ts 里 `columns: string[]` 的契约保持一致；
// 若以后要展示列类型，应同时改前端类型定义和渲染逻辑。
type QueryResult struct {
	Columns  []string        `json:"columns"`
	Rows     [][]interface{} `json:"rows"`
	RowCount int64           `json:"row_count"`
	Elapsed  float64         `json:"elapsed_ms"`
	Error    string          `json:"error,omitempty"`
}

// Service 维护 connectionID -> 连接 的缓存，避免每次查询重新建连。
type Service struct {
	mu    sync.Mutex
	conns map[string]chdriver.Conn
}

func NewService() *Service {
	return &Service{
		conns: make(map[string]chdriver.Conn),
	}
}

func (s *Service) getConn(conn config.Connection) (chdriver.Conn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if c, ok := s.conns[conn.ID]; ok {
		return c, nil
	}

	opts := &ch.Options{
		Addr:     []string{fmt.Sprintf("%s:%d", conn.Host, conn.Port)},
		Protocol: ch.Native,
		Auth: ch.Auth{
			Database: conn.Database,
			Username: conn.User,
			Password: conn.Password,
		},
	}
	if conn.Secure {
		// 默认严格校验证书。只有在用户显式勾选 insecure_skip_verify
		// （自签证书的内网集群）时才跳过，否则必须保持 false。
		opts.TLS = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: conn.InsecureSkipVerify,
		}
	}
	if conn.UseCompress {
		opts.Compression = &ch.Compression{Method: ch.CompressionLZ4}
	}

	c, err := ch.Open(opts)
	if err != nil {
		return nil, err
	}
	s.conns[conn.ID] = c
	return c, nil
}

// Close 丢弃某个连接的缓存。编辑 / 删除连接后必须调用，
// 否则会继续沿用按旧参数建立的连接（表现为"改了配置没生效"）。
func (s *Service) Close(connectionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.conns[connectionID]; ok {
		_ = c.Close()
		delete(s.conns, connectionID)
	}
}

func (s *Service) TestConnection(conn config.Connection) error {
	// 测试用的是表单里的临时参数，可能和已缓存的同 ID 连接不同，
	// 所以先丢弃缓存，保证真的按当前参数建连。
	if conn.ID != "" {
		s.Close(conn.ID)
	}
	c, err := s.getConn(conn)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), metaTimeout)
	defer cancel()
	if err := c.Ping(ctx); err != nil {
		// 失败的连接不能留在缓存里，否则下次查询会复用一个坏连接。
		s.Close(conn.ID)
		return err
	}
	return nil
}

func scanRows(rows chdriver.Rows) ([][]interface{}, error) {
	colCount := len(rows.Columns())
	var result [][]interface{}
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
	return result, nil
}

func (s *Service) Query(conn config.Connection, sql string) (*QueryResult, error) {
	c, err := s.getConn(conn)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	ctx := context.Background()
	rows, err := c.Query(ctx, sql)
	if err != nil {
		return &QueryResult{Error: err.Error(), Elapsed: elapsedMS(start)}, nil
	}
	defer rows.Close()

	result := &QueryResult{Columns: rows.Columns()}
	data, err := scanRows(rows)
	result.Elapsed = elapsedMS(start)
	if err != nil {
		return result, err
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	result.Rows = data
	result.RowCount = int64(len(data))
	return result, nil
}

func elapsedMS(start time.Time) float64 {
	return float64(time.Since(start).Microseconds()) / 1000.0
}

func (s *Service) GetDatabases(conn config.Connection) ([]string, error) {
	c, err := s.getConn(conn)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), metaTimeout)
	defer cancel()
	rows, err := c.Query(ctx, "SHOW DATABASES")
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
	return dbs, rows.Err()
}

func (s *Service) GetTables(conn config.Connection, database string) ([]TableInfo, error) {
	c, err := s.getConn(conn)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), metaTimeout)
	defer cancel()
	// database 来自 UI，但库名可能含任意字符，用字面量转义而不是裸拼字符串。
	query := "SELECT name, engine, comment FROM system.tables WHERE database = " +
		quoteLiteral(database) + " ORDER BY name"
	rows, err := c.Query(ctx, query)
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
		tables = append(tables, TableInfo{Name: name, Engine: engine, Comment: comment})
	}
	return tables, rows.Err()
}

// GetTableSchema 返回表的列信息。
// 这里改用 system.columns 而不是 DESCRIBE TABLE：
// DESCRIBE 的结果列数随版本和表引擎变化（name / type / default_expression /
// comment / codec_expression / ttl_expression），按固定列数 Scan 容易直接报错。
func (s *Service) GetTableSchema(conn config.Connection, database, table string) ([]ColumnInfo, error) {
	c, err := s.getConn(conn)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), metaTimeout)
	defer cancel()
	query := `SELECT name, type, comment, default_kind, default_expression
		FROM system.columns
		WHERE database = ` + quoteLiteral(database) + ` AND table = ` + quoteLiteral(table) + `
		ORDER BY position`
	rows, err := c.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []ColumnInfo
	for rows.Next() {
		var name, typ, comment, defaultKind, defaultExpr string
		if err := rows.Scan(&name, &typ, &comment, &defaultKind, &defaultExpr); err != nil {
			return nil, err
		}
		cols = append(cols, ColumnInfo{
			Name:    name,
			Type:    typ,
			Default: joinDefault(defaultKind, defaultExpr),
			Comment: comment,
		})
	}
	return cols, rows.Err()
}

func joinDefault(kind, expr string) string {
	if strings.TrimSpace(kind) == "" && strings.TrimSpace(expr) == "" {
		return ""
	}
	return strings.TrimSpace(kind + " " + expr)
}

// quoteLiteral 把字符串包成 ClickHouse 字符串字面量，反斜杠和单引号都要转义。
func quoteLiteral(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `'`, `\'`)
	return "'" + r.Replace(s) + "'"
}

func (s *Service) GetClusters(conn config.Connection) ([]ClusterInfo, error) {
	c, err := s.getConn(conn)
	if err != nil {
		return nil, err
	}

	// 优先查完整列；老版本 ClickHouse 没有 shard_name / database_* 时退回最小列集合。
	nodes, err := s.queryClusters(c, true)
	if err != nil {
		nodes, err = s.queryClusters(c, false)
		if err != nil {
			return nil, err
		}
	}

	clusterNames := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		clusterNames[n.Cluster] = true
	}
	versions := s.fetchNodeVersions(c, clusterNames)
	for i := range nodes {
		if v, ok := versions[nodes[i].HostName]; ok {
			nodes[i].Version = v
		} else if v, ok := versions[nodes[i].HostAddress]; ok {
			nodes[i].Version = v
		}
	}
	return nodes, nil
}

func (s *Service) queryClusters(c chdriver.Conn, full bool) ([]ClusterInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), metaTimeout)
	defer cancel()

	selectCols := "cluster, shard_num, replica_num, host_name, host_address, port, is_local, errors_count"
	if full {
		selectCols = "cluster, shard_num, shard_name, replica_num, host_name, host_address, port, is_local, errors_count, database_shard_name, database_replica_name"
	}
	query := "SELECT " + selectCols + " FROM system.clusters ORDER BY cluster, shard_num, replica_num"

	rows, err := c.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ClusterInfo
	for rows.Next() {
		var (
			cluster, shardName, hostName, hostAddress, dbShardName, dbReplicaName string
			shardNum, replicaNum, errorsCount                                     uint32
			port                                                                  uint16
			isLocal                                                               uint8
		)
		if full {
			err = rows.Scan(&cluster, &shardNum, &shardName, &replicaNum, &hostName,
				&hostAddress, &port, &isLocal, &errorsCount, &dbShardName, &dbReplicaName)
		} else {
			err = rows.Scan(&cluster, &shardNum, &replicaNum, &hostName,
				&hostAddress, &port, &isLocal, &errorsCount)
		}
		if err != nil {
			return nil, err
		}
		out = append(out, ClusterInfo{
			Cluster:             cluster,
			ShardNum:            shardNum,
			ShardName:           shardName,
			ReplicaNum:          replicaNum,
			HostName:            hostName,
			HostAddress:         hostAddress,
			Port:                port,
			IsLocal:             isLocal != 0,
			ErrorsCount:         errorsCount,
			DatabaseShardName:   dbShardName,
			DatabaseReplicaName: dbReplicaName,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// fetchNodeVersions 尽力获取集群内每个节点的 ClickHouse 版本。
// 任何集群查询失败都只是让该集群的版本留空，不影响整体结果。
// 返回 hostName()/地址 -> 版本。
func (s *Service) fetchNodeVersions(c chdriver.Conn, clusters map[string]bool) map[string]string {
	out := make(map[string]string)
	names := make([]string, 0, len(clusters))
	for name := range clusters {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		if strings.TrimSpace(name) == "" {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), versionTimeout)
		query := "SELECT hostName(), version() FROM clusterAllReplicas(" + quoteLiteral(name) + ", system.one)"
		rows, err := c.Query(ctx, query)
		if err != nil {
			cancel()
			continue
		}
		for rows.Next() {
			var host, version string
			if err := rows.Scan(&host, &version); err == nil {
				out[host] = version
			}
		}
		_ = rows.Close()
		cancel()
	}
	return out
}

func (s *Service) CloseAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, c := range s.conns {
		_ = c.Close()
		delete(s.conns, id)
	}
}

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
