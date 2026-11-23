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
