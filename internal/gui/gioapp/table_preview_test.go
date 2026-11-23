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

func TestBuildTablePreviewPageSQL(t *testing.T) {
	tests := []struct {
		name string
		limit, page int
		want string
	}{
		{name: "第一页", limit: 100, page: 0, want: "SELECT * FROM `default`.`events` LIMIT 100 OFFSET 0;"},
		{name: "第二页", limit: 100, page: 1, want: "SELECT * FROM `default`.`events` LIMIT 100 OFFSET 100;"},
		{name: "更换每页行数", limit: 50, page: 3, want: "SELECT * FROM `default`.`events` LIMIT 50 OFFSET 150;"},
		{name: "负页码回退第一页", limit: 25, page: -1, want: "SELECT * FROM `default`.`events` LIMIT 25 OFFSET 0;"},
		{name: "无效行数回退默认", limit: 0, page: 2, want: "SELECT * FROM `default`.`events` LIMIT 100 OFFSET 200;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildTablePreviewPageSQL("default", "events", tt.limit, tt.page)
			if got != tt.want { t.Fatalf("got %q, want %q", got, tt.want) }
		})
	}
}
