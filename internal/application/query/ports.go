// Package query 定义 SQL 查询执行用例及其应用层端口。
package query

import (
	"context"
	"time"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

type Mode string

const (
	ModeRows Mode = "rows"
	ModeExec Mode = "exec"
)

type Request struct {
	ConnectionID string
	SQL string
	Mode Mode
	// MaxRows 为 0 表示不限制，负数表示使用执行器安全默认值，正数表示最大返回行数。
	MaxRows int
	BatchSize int
	Timeout time.Duration
}

type Column struct {
	Name string
	DatabaseType string
	ScanType string
}

type Summary struct {
	RowCount int64
	RowsAffected int64
	Elapsed time.Duration
	Truncated bool
}

type BatchHandler func(rows [][]any) error

type Executor interface {
	Execute(ctx context.Context, item connection.Connection, request Request, onColumns func([]Column) error, onBatch BatchHandler) (Summary, error)
}

type ConnectionLookup interface {
	Get(ctx context.Context, id string) (connection.Connection, error)
}
