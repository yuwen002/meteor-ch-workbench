package clickhouse

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeSQLRows struct {
	columns []string
	rows    [][]interface{}
	index   int
	err     error
}

func (r *fakeSQLRows) Columns() ([]string, error) { return r.columns, nil }
func (r *fakeSQLRows) Next() bool { return r.index < len(r.rows) }
func (r *fakeSQLRows) Scan(dest ...any) error {
	if r.index >= len(r.rows) { return errors.New("scan past end") }
	for i := range dest {
		if i >= len(r.rows[r.index]) { break }
		if p, ok := dest[i].(*interface{}); ok { *p = r.rows[r.index][i] }
	}
	r.index++
	return nil
}
func (r *fakeSQLRows) Err() error { return r.err }
func (r *fakeSQLRows) Close() error { return nil }

func TestStreamSQLRowsToCSV(t *testing.T) {
	rows := &fakeSQLRows{
		columns: []string{"姓名", "备注", "空值"},
		rows: [][]interface{}{
			{"张三", "含,逗号和\"引号\"", nil},
			{"李四", "第一行\n第二行", 42},
		},
	}
	var output bytes.Buffer
	count, err := streamSQLRowsTo(context.Background(), rows, &output, "csv")
	if err != nil { t.Fatal(err) }
	if count != 2 { t.Fatalf("count = %d, want 2", count) }
	want := "姓名,备注,空值\n张三,\"含,逗号和\"\"引号\"\"\",\n李四,\"第一行\n第二行\",42\n"
	if output.String() != want { t.Fatalf("CSV mismatch:\n got: %q\nwant: %q", output.String(), want) }
}

func TestStreamSQLRowsToTSV(t *testing.T) {
	rows := &fakeSQLRows{
		columns: []string{"id", "内容"},
		rows: [][]interface{}{{1, "第一列\t第二列\r\n第三行\\尾部"}},
	}
	var output bytes.Buffer
	count, err := streamSQLRowsTo(context.Background(), rows, &output, "tsv")
	if err != nil { t.Fatal(err) }
	if count != 1 { t.Fatalf("count = %d, want 1", count) }
	want := "id\t内容\n1\t第一列\\t第二列\\n第三行\\\\尾部\n"
	if output.String() != want { t.Fatalf("TSV mismatch:\n got: %q\nwant: %q", output.String(), want) }
}

type failingWriter struct{}
func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestStreamSQLRowsToPropagatesWriterError(t *testing.T) {
	rows := &fakeSQLRows{columns: []string{"id"}, rows: [][]interface{}{{1}}}
	_, err := streamSQLRowsTo(context.Background(), rows, failingWriter{}, "tsv")
	if err == nil || !strings.Contains(err.Error(), "disk full") { t.Fatalf("expected disk error, got %v", err) }
}

func TestStreamSQLRowsToRejectsUnknownFormat(t *testing.T) {
	rows := &fakeSQLRows{columns: []string{"id"}}
	if _, err := streamSQLRowsTo(context.Background(), rows, &bytes.Buffer{}, "json"); err == nil {
		t.Fatal("expected unsupported format error")
	}
}

func TestStreamRowsHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rows := &fakeSQLRows{columns: []string{"id"}, rows: [][]interface{}{{1}}}
	_, err := streamSQLRowsTo(ctx, rows, &bytes.Buffer{}, "csv")
	if !errors.Is(err, context.Canceled) { t.Fatalf("got %v, want context.Canceled", err) }
}


func TestStreamSQLRowsToReportsProgress(t *testing.T) {
	rows := &fakeSQLRows{columns: []string{"id"}, rows: make([][]interface{}, 2001)}
	for i := range rows.rows {
		rows.rows[i] = []interface{}{i}
	}
	var output bytes.Buffer
	var updates []int64
	count, err := streamSQLRowsToWithProgress(context.Background(), rows, &output, "csv", func(n int64) {
		updates = append(updates, n)
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 2001 {
		t.Fatalf("count = %d, want 2001", count)
	}
	want := []int64{1000, 2000, 2001}
	if len(updates) != len(want) {
		t.Fatalf("progress updates = %v, want %v", updates, want)
	}
	for i := range want {
		if updates[i] != want[i] {
			t.Fatalf("progress updates = %v, want %v", updates, want)
		}
	}
}


func TestStreamSQLRowsToSerializesCommonClickHouseTypes(t *testing.T) {
	timestamp := time.Date(2026, 11, 23, 10, 30, 0, 0, time.UTC)
	rows := &fakeSQLRows{
		columns: []string{"string_bytes", "array", "map", "datetime64"},
		rows: [][]interface{}{{
			[]byte("中文文本"),
			[]string{"alpha", "beta"},
			map[string]string{"region": "sg"},
			timestamp,
		}},
	}
	var output bytes.Buffer
	count, err := streamSQLRowsTo(context.Background(), rows, &output, "csv")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
	want := "string_bytes,array,map,datetime64\n中文文本,\"[\"\"alpha\"\",\"\"beta\"\"]\",\"{\"\"region\"\":\"\"sg\"\"}\",2026-11-23T10:30:00Z\n"
	if output.String() != want {
		t.Fatalf("CSV mismatch:\n got: %q\nwant: %q", output.String(), want)
	}
}

func TestSerializeValuePreservesByteSliceAsText(t *testing.T) {
	got, ok := serializeValue([]byte("ClickHouse 字符串")).(string)
	if !ok {
		t.Fatalf("serialized byte slice type = %T, want string", serializeValue([]byte("ClickHouse 字符串")))
	}
	if got != "ClickHouse 字符串" {
		t.Fatalf("serialized byte slice = %q", got)
	}
}
