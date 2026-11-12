package clickhouse

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"sync"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	chdriver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/yuwen002/meteor-ch-workbench/internal/config"
)

type ColumnInfo struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Comment string `json:"comment"`
}

type TableInfo struct {
	Name    string `json:"name"`
	Engine  string `json:"engine"`
	Comment string `json:"comment"`
}

type ClusterInfo struct {
	Cluster    string `json:"cluster"`
	Version    string `json:"version"`
	HostName   string `json:"host_name"`
	HostAddress string `json:"host_address"`
	Port       int    `json:"port"`
	IsLocal    bool   `json:"is_local"`
	ShardNum   int    `json:"shard_num"`
	ReplicaNum int    `json:"replica_num"`
	UUID       string `json:"uuid"`
}

type QueryResult struct {
	Columns  []ColumnInfo    `json:"columns"`
	Rows     [][]interface{} `json:"rows"`
	RowCount int64           `json:"row_count"`
	Error    string          `json:"error,omitempty"`
}

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
		opts.TLS = &tls.Config{InsecureSkipVerify: true}
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

func (s *Service) TestConnection(conn config.Connection) error {
	c, err := s.getConn(conn)
	if err != nil {
		return err
	}
	return c.Ping(context.Background())
}

func scanRows(rows chdriver.Rows) ([][]interface{}, error) {
	var result [][]interface{}
	colCount := len(rows.Columns())
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

	ctx := context.Background()
	rows, err := c.Query(ctx, sql)
	if err != nil {
		return &QueryResult{Error: err.Error()}, nil
	}
	defer rows.Close()

	result := &QueryResult{}
	for _, col := range rows.Columns() {
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

func (s *Service) GetDatabases(conn config.Connection) ([]string, error) {
	c, err := s.getConn(conn)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
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
	return dbs, nil
}

func (s *Service) GetTables(conn config.Connection, database string) ([]TableInfo, error) {
	c, err := s.getConn(conn)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	query := fmt.Sprintf("SELECT name, engine, comment FROM system.tables WHERE database = '%s'", database)
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
		tables = append(tables, TableInfo{
			Name:    name,
			Engine:  engine,
			Comment: comment,
		})
	}
	return tables, nil
}

func (s *Service) GetTableSchema(conn config.Connection, database string, table string) ([]ColumnInfo, error) {
	c, err := s.getConn(conn)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	query := fmt.Sprintf("DESCRIBE TABLE %s.%s", database, table)
	rows, err := c.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []ColumnInfo
	for rows.Next() {
		var name, typ, defaultVal, comment string
		if err := rows.Scan(&name, &typ, &defaultVal, &comment); err != nil {
			return nil, err
		}
		cols = append(cols, ColumnInfo{
			Name:    name,
			Type:    typ,
			Comment: comment,
		})
	}
	return cols, nil
}

func (s *Service) GetClusters(conn config.Connection) ([]ClusterInfo, error) {
	c, err := s.getConn(conn)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	query := `SELECT
		cluster,
		version,
		host_name,
		host_address,
		port,
		is_local,
		shard_num,
		replica_num,
		uuid
	FROM system.clusters`
	rows, err := c.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clusters []ClusterInfo
	for rows.Next() {
		var (
			cluster, version, hostName, hostAddress string
			port, shardNum, replicaNum             uint64
			isLocal                                bool
			uuid                                   string
		)
		if err := rows.Scan(&cluster, &version, &hostName, &hostAddress, &port, &isLocal, &shardNum, &replicaNum, &uuid); err != nil {
			return nil, err
		}
		clusters = append(clusters, ClusterInfo{
			Cluster:     cluster,
			Version:     version,
			HostName:    hostName,
			HostAddress: hostAddress,
			Port:        int(port),
			IsLocal:     isLocal,
			ShardNum:    int(shardNum),
			ReplicaNum:  int(replicaNum),
			UUID:        uuid,
		})
	}
	return clusters, nil
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