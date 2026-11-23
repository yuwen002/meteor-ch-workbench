package gioapp

import "testing"

func TestIsCurrentSchemaResult(t *testing.T) {
	current := schemaResult{
		connectionID: "conn-a",
		connectionSequence: 3,
		database: "analytics",
		table: "events",
		requestID: 8,
	}
	tests := []struct {
		name string
		result schemaResult
		connectionID string
		connectionSequence uint64
		schemaSequence uint64
		database string
		table string
		want bool
	}{
		{
			name: "当前请求匹配",
			result: current,
			connectionID: "conn-a",
			connectionSequence: 3,
			schemaSequence: 8,
			database: "analytics",
			table: "events",
			want: true,
		},
		{
			name: "连接已切换",
			result: current,
			connectionID: "conn-b",
			connectionSequence: 4,
			schemaSequence: 8,
			database: "analytics",
			table: "events",
		},
		{
			name: "同一连接内已切换表",
			result: current,
			connectionID: "conn-a",
			connectionSequence: 3,
			schemaSequence: 9,
			database: "analytics",
			table: "sessions",
		},
		{
			name: "数据库不匹配",
			result: current,
			connectionID: "conn-a",
			connectionSequence: 3,
			schemaSequence: 8,
			database: "system",
			table: "events",
		},
		{
			name: "表不匹配",
			result: current,
			connectionID: "conn-a",
			connectionSequence: 3,
			schemaSequence: 8,
			database: "analytics",
			table: "sessions",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isCurrentSchemaResult(tt.result, tt.connectionID, tt.connectionSequence, tt.schemaSequence, tt.database, tt.table)
			if got != tt.want {
				t.Fatalf("isCurrentSchemaResult() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSplitDDLLines(t *testing.T) {
	tests := []struct {
		name string
		ddl string
		want []string
	}{
		{name: "空语句", ddl: "", want: nil},
		{name: "LF 换行", ddl: "CREATE TABLE t (\n    id UInt64\n)", want: []string{"CREATE TABLE t (", "    id UInt64", ")"}},
		{name: "CRLF 换行", ddl: "CREATE TABLE t (\r\n    id UInt64\r\n)", want: []string{"CREATE TABLE t (", "    id UInt64", ")"}},
		{name: "保留空行", ddl: "CREATE TABLE t (\n\n    id UInt64\n)", want: []string{"CREATE TABLE t (", "", "    id UInt64", ")"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitDDLLines(tt.ddl)
			if len(got) != len(tt.want) {
				t.Fatalf("splitDDLLines() 行数 = %d，期望 %d：%q", len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("splitDDLLines()[%d] = %q，期望 %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
