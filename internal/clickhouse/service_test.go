package clickhouse

import (
	"encoding/json"
	"testing"
)

func TestEmptyQueryResultJSON(t *testing.T) {
	cases := []struct {
		name    string
		message string
		elapsed float64
		limit   int
	}{
		{name: "查询取消", message: errQueryCanceled, elapsed: 1.5, limit: 50000},
		{name: "SQL 错误", message: "Syntax error", elapsed: 0, limit: 0},
		{name: "空错误文本", message: "", elapsed: 2.25, limit: 100},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := emptyQueryResult(c.message, c.elapsed, c.limit)
			if r.Columns == nil {
				t.Fatal("Columns 不应为 nil，否则 JSON 序列化为 null")
			}
			if r.Rows == nil {
				t.Fatal("Rows 不应为 nil，否则 JSON 序列化为 null")
			}
			if len(r.Columns) != 0 || len(r.Rows) != 0 {
				t.Fatalf("空结果体不应含数据，got columns=%d rows=%d", len(r.Columns), len(r.Rows))
			}
			if r.Error != c.message {
				t.Errorf("Error 期望 %q，实际 %q", c.message, r.Error)
			}
			if r.ElapsedMs != c.elapsed {
				t.Errorf("ElapsedMs 期望 %v，实际 %v", c.elapsed, r.ElapsedMs)
			}
			if r.RowLimit != c.limit {
				t.Errorf("RowLimit 期望 %d，实际 %d", c.limit, r.RowLimit)
			}

			b, err := json.Marshal(r)
			if err != nil {
				t.Fatalf("序列化失败: %v", err)
			}
			var m map[string]interface{}
			if err := json.Unmarshal(b, &m); err != nil {
				t.Fatalf("反序列化失败: %v", err)
			}
			if m["columns"] == nil {
				t.Fatalf("columns 序列化成了 null: %s", string(b))
			}
			if m["rows"] == nil {
				t.Fatalf("rows 序列化成了 null: %s", string(b))
			}
		})
	}
}

func TestExtractTTLExpression(t *testing.T) {
	tests := []struct {
		name string
		createSQL string
		want string
	}{
		{
			name: "TTL 位于 SETTINGS 之前",
			createSQL: "CREATE TABLE events (ts DateTime) ENGINE = MergeTree ORDER BY ts TTL ts + INTERVAL 30 DAY DELETE SETTINGS index_granularity = 8192",
			want: "ts + INTERVAL 30 DAY DELETE",
		},
		{
			name: "无 TTL",
			createSQL: "CREATE TABLE events (id UInt64) ENGINE = MergeTree ORDER BY id",
			want: "",
		},
		{
			name: "多行 TTL",
			createSQL: "CREATE TABLE events (ts DateTime) ENGINE = MergeTree ORDER BY ts\nTTL ts + INTERVAL 7 DAY TO VOLUME 'cold'",
			want: "ts + INTERVAL 7 DAY TO VOLUME 'cold'",
		},
		{
			name: "保留多个 TTL 规则",
			createSQL: "CREATE TABLE events (ts DateTime) ENGINE = MergeTree ORDER BY ts TTL ts + INTERVAL 1 DAY DELETE, ts + INTERVAL 7 DAY TO DISK 'cold' SETTINGS allow_nullable_key = 1",
			want: "ts + INTERVAL 1 DAY DELETE, ts + INTERVAL 7 DAY TO DISK 'cold'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractTTLExpression(tt.createSQL); got != tt.want {
				t.Fatalf("extractTTLExpression() = %q, want %q", got, tt.want)
			}
		})
	}
}
