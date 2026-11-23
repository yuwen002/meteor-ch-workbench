package clickhouse

import (
	"context"
	"fmt"
	"strings"

	"github.com/yuwen002/meteor-ch-workbench/internal/config"
)

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