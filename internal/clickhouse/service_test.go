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
