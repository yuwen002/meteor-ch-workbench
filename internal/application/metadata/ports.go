// Package metadata 定义数据库对象元数据查询用例。
package metadata

import (
	"context"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

type Object struct {
	Name string
	Engine string
	Comment string
}

type Column struct {
	Name string
	Type string
	Comment string
	IsPrimary bool
	HasDefault bool
}

type TableDetails struct {
	Engine string
	Comment string
	CreateTableQuery string
	PartitionKey string
	SortingKey string
	PrimaryKey string
	TTLExpression string
}

type Reader interface {
	ListDatabases(ctx context.Context, item connection.Connection) ([]string, error)
	ListTables(ctx context.Context, item connection.Connection, database string) ([]Object, error)
	ListViews(ctx context.Context, item connection.Connection, database string) ([]Object, error)
	GetTableSchema(ctx context.Context, item connection.Connection, database, table string) ([]Column, error)
	GetTableDetails(ctx context.Context, item connection.Connection, database, table string) (TableDetails, error)
}

type ConnectionLookup interface {
	Get(ctx context.Context, id string) (connection.Connection, error)
}
