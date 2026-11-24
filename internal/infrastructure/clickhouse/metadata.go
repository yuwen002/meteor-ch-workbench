package clickhouse

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	appmetadata "github.com/yuwen002/meteor-ch-workbench/internal/application/metadata"
	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

type MetadataReader struct {
	pools *PoolManager
}

func NewMetadataReader(pools *PoolManager) (*MetadataReader, error) {
	if pools == nil { return nil, errors.New("ClickHouse 连接池管理器不能为空") }
	return &MetadataReader{pools: pools}, nil
}

func (r *MetadataReader) db(item connection.Connection) (*sql.DB, error) {
	return r.pools.Acquire(item)
}

func (r *MetadataReader) ListDatabases(ctx context.Context, item connection.Connection) ([]string, error) {
	db, err := r.db(item)
	if err != nil { return nil, err }
	rows, err := db.QueryContext(ctx, "SHOW DATABASES")
	if err != nil { return nil, fmt.Errorf("查询数据库列表: %w", err) }
	defer rows.Close()
	result := make([]string, 0)
	for rows.Next() {
		if err := ctx.Err(); err != nil { return nil, err }
		var name string
		if err := rows.Scan(&name); err != nil { return nil, fmt.Errorf("读取数据库名称: %w", err) }
		result = append(result, name)
	}
	if err := rows.Err(); err != nil { return nil, fmt.Errorf("遍历数据库列表: %w", err) }
	return result, nil
}

func (r *MetadataReader) ListTables(ctx context.Context, item connection.Connection, database string) ([]appmetadata.Object, error) {
	return r.listObjects(ctx, item, database, false)
}

func (r *MetadataReader) ListViews(ctx context.Context, item connection.Connection, database string) ([]appmetadata.Object, error) {
	return r.listObjects(ctx, item, database, true)
}

func (r *MetadataReader) listObjects(ctx context.Context, item connection.Connection, database string, views bool) ([]appmetadata.Object, error) {
	db, err := r.db(item)
	if err != nil { return nil, err }
	filter := "engine NOT LIKE '%View'"
	if views { filter = "engine LIKE '%View'" }
	statement := "SELECT name, engine, comment FROM system.tables WHERE database = ? AND " + filter + " ORDER BY name"
	rows, err := db.QueryContext(ctx, statement, database)
	if err != nil { return nil, fmt.Errorf("查询数据库 %q 的对象列表: %w", database, err) }
	defer rows.Close()
	result := make([]appmetadata.Object, 0)
	for rows.Next() {
		if err := ctx.Err(); err != nil { return nil, err }
		var item appmetadata.Object
		if err := rows.Scan(&item.Name, &item.Engine, &item.Comment); err != nil { return nil, fmt.Errorf("读取对象信息: %w", err) }
		result = append(result, item)
	}
	if err := rows.Err(); err != nil { return nil, fmt.Errorf("遍历对象列表: %w", err) }
	return result, nil
}

func (r *MetadataReader) GetTableSchema(ctx context.Context, item connection.Connection, database, table string) ([]appmetadata.Column, error) {
	db, err := r.db(item)
	if err != nil { return nil, err }
	statement := "DESCRIBE TABLE " + quoteIdentifier(database) + "." + quoteIdentifier(table)
	rows, err := db.QueryContext(ctx, statement)
	if err != nil { return nil, fmt.Errorf("读取表 %s.%s 结构: %w", database, table, err) }
	defer rows.Close()
	names, err := rows.Columns()
	if err != nil { return nil, fmt.Errorf("读取表结构列信息: %w", err) }
	result := make([]appmetadata.Column, 0)
	for rows.Next() {
		if err := ctx.Err(); err != nil { return nil, err }
		values := make([]any, len(names))
		destinations := make([]any, len(names))
		for i := range values { destinations[i] = &values[i] }
		if err := rows.Scan(destinations...); err != nil { return nil, fmt.Errorf("读取字段定义: %w", err) }
		column := appmetadata.Column{}
		for i, name := range names {
			switch name {
			case "name": column.Name = metadataString(values[i])
			case "type": column.Type = metadataString(values[i])
			case "comment": column.Comment = metadataString(values[i])
			case "primary_key": column.IsPrimary = metadataTruthy(values[i])
			case "default_type": column.HasDefault = metadataString(values[i]) != ""
			}
		}
		result = append(result, column)
	}
	if err := rows.Err(); err != nil { return nil, fmt.Errorf("遍历字段定义: %w", err) }
	return result, nil
}

func (r *MetadataReader) GetTableDetails(ctx context.Context, item connection.Connection, database, table string) (appmetadata.TableDetails, error) {
	db, err := r.db(item)
	if err != nil { return appmetadata.TableDetails{}, err }
	const statement = "SELECT engine, comment, create_table_query, partition_key, sorting_key, primary_key FROM system.tables WHERE database = ? AND name = ? LIMIT 1"
	var details appmetadata.TableDetails
	err = db.QueryRowContext(ctx, statement, database, table).Scan(
		&details.Engine, &details.Comment, &details.CreateTableQuery,
		&details.PartitionKey, &details.SortingKey, &details.PrimaryKey,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) { return appmetadata.TableDetails{}, fmt.Errorf("表 %s.%s 不存在: %w", database, table, err) }
		return appmetadata.TableDetails{}, fmt.Errorf("读取表 %s.%s 详情: %w", database, table, err)
	}
	details.TTLExpression = extractTTL(details.CreateTableQuery)
	return details, nil
}

func quoteIdentifier(value string) string {
	return "`" + strings.ReplaceAll(value, "`", "``") + "`"
}

func metadataString(value any) string {
	switch v := value.(type) {
	case nil: return ""
	case string: return v
	case []byte: return string(v)
	default: return fmt.Sprint(v)
	}
}

func metadataTruthy(value any) bool {
	switch v := value.(type) {
	case bool: return v
	case nil: return false
	default:
		s := metadataString(v)
		return s == "1" || strings.EqualFold(s, "true")
	}
}

var ttlExpressionPattern = regexp.MustCompile("(?is)\\bTTL\\s+(.*?)(?:\\s+\\bSETTINGS\\b|$)")

func extractTTL(createSQL string) string {
	match := ttlExpressionPattern.FindStringSubmatch(createSQL)
	if len(match) < 2 { return "" }
	return strings.TrimSpace(strings.TrimRight(match[1], ";"))
}

var _ appmetadata.Reader = (*MetadataReader)(nil)
