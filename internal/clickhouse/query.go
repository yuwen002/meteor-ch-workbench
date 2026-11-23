package clickhouse

import (
	"context"
	"time"

	"github.com/yuwen002/meteor-ch-workbench/internal/config"
)

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
	ctx, unregister := s.register(conn.ID, queryID)
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