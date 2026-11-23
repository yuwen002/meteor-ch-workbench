package gioapp

import "testing"

func TestBuildTablePreviewSQL(t *testing.T) {
	tests := []struct {
		name, database, table, want string
		limit int
	}{
		{name: "普通库表", database: "default", table: "events", limit: 100, want: "SELECT * FROM `default`.`events` LIMIT 100;"},
		{name: "标识符含反引号", database: "db`name", table: "table`name", limit: 50, want: "SELECT * FROM `db``name`.`table``name` LIMIT 50;"},
		{name: "非正数限制回退默认值", database: "default", table: "events", limit: 0, want: "SELECT * FROM `default`.`events` LIMIT 100;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildTablePreviewSQL(tt.database, tt.table, tt.limit); got != tt.want {
				t.Fatalf("buildTablePreviewSQL() = %q, want %q", got, tt.want)
			}
		})
	}
}
