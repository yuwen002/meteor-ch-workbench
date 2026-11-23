package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yuwen002/meteor-ch-workbench/internal/application/query"
	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

const (
	defaultQueryTimeout = 30 * time.Second
	defaultQueryMaxRows = 10000
	defaultQueryBatchSize = 256
)

type QueryExecutor struct {
	pools *PoolManager
}

func NewQueryExecutor(pools *PoolManager) (*QueryExecutor, error) {
	if pools == nil { return nil, errors.New("ClickHouse 连接池管理器不能为空") }
	return &QueryExecutor{pools: pools}, nil
}

func (e *QueryExecutor) Execute(ctx context.Context, item connection.Connection, request query.Request, onColumns func([]query.Column) error, onBatch query.BatchHandler) (query.Summary, error) {
	started := time.Now()
	if err := ctx.Err(); err != nil { return query.Summary{}, err }
	if strings.TrimSpace(request.SQL) == "" { return query.Summary{}, query.ErrEmptySQL }
	if request.Mode == "" { request.Mode = query.ModeRows }
	if request.Mode != query.ModeRows && request.Mode != query.ModeExec { return query.Summary{}, query.ErrInvalidMode }

	timeout := request.Timeout
	if timeout <= 0 { timeout = defaultQueryTimeout }
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	db, err := e.pools.Acquire(item)
	if err != nil { return query.Summary{}, fmt.Errorf("获取 ClickHouse 连接池: %w", err) }

	summary := query.Summary{}
	if request.Mode == query.ModeExec {
		result, err := db.ExecContext(execCtx, request.SQL)
		if err != nil { return query.Summary{}, fmt.Errorf("执行 SQL: %w", err) }
		if affected, affectedErr := result.RowsAffected(); affectedErr == nil { summary.RowsAffected = affected } else { summary.RowsAffected = -1 }
		summary.Elapsed = time.Since(started)
		return summary, nil
	}

	rows, err := db.QueryContext(execCtx, request.SQL)
	if err != nil { return query.Summary{}, fmt.Errorf("查询 SQL: %w", err) }
	defer rows.Close()

	columnTypes, err := rows.ColumnTypes()
	if err != nil { return query.Summary{}, fmt.Errorf("读取结果列信息: %w", err) }
	columns := make([]query.Column, len(columnTypes))
	for i, column := range columnTypes {
		columns[i] = query.Column{Name: column.Name(), DatabaseType: column.DatabaseTypeName()}
		if scanType := column.ScanType(); scanType != nil { columns[i].ScanType = scanType.String() }
	}
	if onColumns != nil {
		if err := onColumns(columns); err != nil { return query.Summary{}, fmt.Errorf("交付结果列信息: %w", err) }
	}

	maxRows := request.MaxRows
	if maxRows <= 0 { maxRows = defaultQueryMaxRows }
	batchSize := request.BatchSize
	if batchSize <= 0 { batchSize = defaultQueryBatchSize }
	batch := make([][]any, 0, batchSize)
	values := make([]any, len(columns))
	destinations := make([]any, len(columns))
	for i := range values { destinations[i] = &values[i] }

	flush := func() error {
		if len(batch) == 0 { return nil }
		if onBatch != nil {
			if err := onBatch(batch); err != nil { return fmt.Errorf("交付查询结果批次: %w", err) }
		}
		batch = make([][]any, 0, batchSize)
		return nil
	}

	for rows.Next() {
		if err := execCtx.Err(); err != nil { return query.Summary{}, err }
		if int(summary.RowCount) >= maxRows {
			summary.Truncated = true
			break
		}
		if err := rows.Scan(destinations...); err != nil { return query.Summary{}, fmt.Errorf("读取查询结果行 %d: %w", summary.RowCount+1, err) }
		row := make([]any, len(values))
		for i, value := range values {
			if bytes, ok := value.([]byte); ok {
				row[i] = append([]byte(nil), bytes...)
			} else {
				row[i] = value
			}
		}
		batch = append(batch, row)
		summary.RowCount++
		if len(batch) >= batchSize {
			if err := flush(); err != nil { return query.Summary{}, err }
		}
	}
	if err := rows.Err(); err != nil {
		if execCtx.Err() != nil { return query.Summary{}, execCtx.Err() }
		return query.Summary{}, fmt.Errorf("遍历查询结果: %w", err)
	}
	if err := flush(); err != nil { return query.Summary{}, err }
	if err := execCtx.Err(); err != nil { return query.Summary{}, err }
	summary.Elapsed = time.Since(started)
	return summary, nil
}

var _ query.Executor = (*QueryExecutor)(nil)
