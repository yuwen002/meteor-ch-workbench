package clickhouse

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/yuwen002/meteor-ch-workbench/internal/application/query"
	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

const queryTestDriverName = "meteor-workbench-query-test"

var registerQueryTestDriver sync.Once
var queryTestRowsClosed bool

type queryTestDriver struct{}
func (queryTestDriver) Open(string) (driver.Conn, error) { return queryTestConn{}, nil }

type queryTestConn struct{}
func (queryTestConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not implemented") }
func (queryTestConn) Close() error { return nil }
func (queryTestConn) Begin() (driver.Tx, error) { return nil, errors.New("not implemented") }
func (queryTestConn) QueryContext(ctx context.Context, statement string, _ []driver.NamedValue) (driver.Rows, error) {
	if statement == "wait" {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return &queryTestRows{index: 0}, nil
}
func (queryTestConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return queryTestResult{}, nil
}

type queryTestRows struct { index int }
func (r *queryTestRows) Columns() []string { return []string{"number", "label"} }
func (r *queryTestRows) Close() error { queryTestRowsClosed = true; return nil }
func (r *queryTestRows) Next(dest []driver.Value) error {
	values := [][]driver.Value{{int64(1), []byte("one")}, {int64(2), []byte("two")}, {int64(3), []byte("three")}}
	if r.index >= len(values) { return io.EOF }
	copy(dest, values[r.index])
	r.index++
	return nil
}
func (r *queryTestRows) ColumnTypeDatabaseTypeName(index int) string {
	if index == 0 { return "Int64" }
	return "String"
}
func (r *queryTestRows) ColumnTypeScanType(index int) reflect.Type {
	if index == 0 { return reflect.TypeOf(int64(0)) }
	return reflect.TypeOf("")
}

type queryTestResult struct{}
func (queryTestResult) LastInsertId() (int64, error) { return 0, nil }
func (queryTestResult) RowsAffected() (int64, error) { return 4, nil }

func newQueryTestExecutor(t *testing.T) (*QueryExecutor, *PoolManager) {
	t.Helper()
	registerQueryTestDriver.Do(func() { sql.Register(queryTestDriverName, queryTestDriver{}) })
	pools := newPoolManager(func(_, dsn string) (*sql.DB, error) { return sql.Open(queryTestDriverName, dsn) })
	executor, err := NewQueryExecutor(pools)
	if err != nil { t.Fatal(err) }
	return executor, pools
}

func queryTestConnection() connection.Connection {
	return connection.Connection{ID: "query-test", Name: "Test", Host: "localhost", Port: 9000, Protocol: connection.ProtocolNative}
}

func TestQueryExecutorStreamsBatchesAndTruncates(t *testing.T) {
	executor, pools := newQueryTestExecutor(t)
	defer pools.Close()
	queryTestRowsClosed = false
	var columns []query.Column
	var batches [][][]any
	summary, err := executor.Execute(context.Background(), queryTestConnection(), query.Request{SQL: "select number, label", MaxRows: 2, BatchSize: 1}, func(got []query.Column) error {
		columns = got
		return nil
	}, func(batch [][]any) error {
		batches = append(batches, batch)
		return nil
	})
	if err != nil { t.Fatal(err) }
	if len(columns) != 2 || columns[0].Name != "number" || columns[0].DatabaseType != "Int64" { t.Fatalf("列元数据不正确: %+v", columns) }
	if summary.RowCount != 2 || !summary.Truncated || len(batches) != 2 { t.Fatalf("查询摘要或批次不正确: %+v batches=%d", summary, len(batches)) }
	if got := string(batches[0][0][1].([]byte)); got != "one" { t.Fatalf("字节值没有正确保留: %q", got) }
	if !queryTestRowsClosed { t.Fatal("查询结束后必须关闭 rows") }
}

func TestQueryExecutorExecMode(t *testing.T) {
	executor, pools := newQueryTestExecutor(t)
	defer pools.Close()
	summary, err := executor.Execute(context.Background(), queryTestConnection(), query.Request{SQL: "truncate table t", Mode: query.ModeExec}, nil, nil)
	if err != nil { t.Fatal(err) }
	if summary.RowsAffected != 4 { t.Fatalf("受影响行数=%d", summary.RowsAffected) }
}

func TestQueryExecutorPropagatesBatchErrorAndClosesRows(t *testing.T) {
	executor, pools := newQueryTestExecutor(t)
	defer pools.Close()
	queryTestRowsClosed = false
	want := errors.New("consumer stopped")
	_, err := executor.Execute(context.Background(), queryTestConnection(), query.Request{SQL: "select 1", MaxRows: -1, BatchSize: 1}, nil, func([][]any) error { return want })
	if !errors.Is(err, want) { t.Fatalf("未保留批次错误: %v", err) }
	if !queryTestRowsClosed { t.Fatal("批次回调失败后必须关闭 rows") }
}

func TestQueryExecutorHonorsCanceledContext(t *testing.T) {
	executor, pools := newQueryTestExecutor(t)
	defer pools.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := executor.Execute(ctx, queryTestConnection(), query.Request{SQL: "select 1"}, nil, nil)
	if !errors.Is(err, context.Canceled) { t.Fatalf("期望 context.Canceled，得到 %v", err) }
}

func TestQueryExecutorHonorsTimeout(t *testing.T) {
	executor, pools := newQueryTestExecutor(t)
	defer pools.Close()
	_, err := executor.Execute(context.Background(), queryTestConnection(), query.Request{SQL: "wait", Timeout: 10 * time.Millisecond}, nil, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("期望 context.DeadlineExceeded，得到 %v", err)
	}
}

func TestQueryExecutorZeroMaxRowsMeansUnlimited(t *testing.T) {
	executor, pools := newQueryTestExecutor(t)
	defer pools.Close()
	var count int
	summary, err := executor.Execute(context.Background(), queryTestConnection(), query.Request{SQL: "select number, label", MaxRows: 0, BatchSize: 2}, nil, func(batch [][]any) error {
		count += len(batch)
		return nil
	})
	if err != nil { t.Fatal(err) }
	if count != 3 || summary.RowCount != 3 { t.Fatalf("unlimited query returned %d rows, summary=%+v", count, summary) }
	if summary.Truncated { t.Fatal("unlimited query must not be marked truncated") }
}
