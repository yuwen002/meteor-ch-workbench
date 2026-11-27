package app

import (
	"strings"
	"testing"
)

func TestBuildCreateTableSQL(t *testing.T) {
	sql, err := buildCreateTableSQL(createTableDefinition{
		Database: "analytics",
		Table: "event`log",
		Comment: "user's events",
		Engine: "MergeTree",
		OrderBy: "event_time, id",
		PartitionBy: "toYYYYMM(event_time)",
		PrimaryKey: "event_time",
		Settings: "index_granularity = 8192",
		Columns: []createTableColumn{
			{Name: "id", Type: "UInt64"},
			{Name: "event_time", Type: "DateTime64(3)", Default: "now64(3)", Comment: "event's time"},
		},
	})
	if err != nil { t.Fatalf("buildCreateTableSQL() error = %v", err) }
	for _, part := range []string{
		"CREATE TABLE `analytics`.`event``log`",
		"`id` UInt64",
		"`event_time` DateTime64(3) DEFAULT now64(3) COMMENT 'event''s time'",
		"ORDER BY (event_time, id)",
		"PARTITION BY toYYYYMM(event_time)",
		"PRIMARY KEY (event_time)",
		"COMMENT 'user''s events'",
		"SETTINGS index_granularity = 8192",
	} {
		if !strings.Contains(sql, part) { t.Errorf("generated SQL missing %q:\n%s", part, sql) }
	}
}

func TestBuildCreateTableSQLDefaultsMergeTreeOrderBy(t *testing.T) {
	sql, err := buildCreateTableSQL(createTableDefinition{
		Database: "default", Table: "items",
		Columns: []createTableColumn{{Name: "id", Type: "UInt64"}},
	})
	if err != nil { t.Fatal(err) }
	if !strings.Contains(sql, "ENGINE = MergeTree\nORDER BY tuple()") {
		t.Fatalf("MergeTree default ORDER BY missing: %s", sql)
	}
}

func TestBuildCreateTableSQLDoesNotAddOrderByForMemory(t *testing.T) {
	sql, err := buildCreateTableSQL(createTableDefinition{
		Database: "default", Table: "items", Engine: "Memory",
		Columns: []createTableColumn{{Name: "id", Type: "UInt64"}},
	})
	if err != nil { t.Fatal(err) }
	if strings.Contains(sql, "ORDER BY") { t.Fatalf("Memory engine must not receive ORDER BY: %s", sql) }
}

func TestBuildCreateTableSQLRejectsInvalidDefinitions(t *testing.T) {
	tests := []struct {
		name string
		def createTableDefinition
		want string
	}{
		{name: "missing database", def: createTableDefinition{Table: "t", Columns: []createTableColumn{{Name:"id",Type:"UInt64"}}}, want: "数据库"},
		{name: "missing table", def: createTableDefinition{Database: "default", Columns: []createTableColumn{{Name:"id",Type:"UInt64"}}}, want: "表名"},
		{name: "no columns", def: createTableDefinition{Database: "default", Table: "t"}, want: "至少"},
		{name: "incomplete column", def: createTableDefinition{Database: "default", Table: "t", Columns: []createTableColumn{{Name:"id"}}}, want: "名称和类型"},
		{name: "duplicate column", def: createTableDefinition{Database: "default", Table: "t", Columns: []createTableColumn{{Name:"id",Type:"UInt64"},{Name:"id",Type:"String"}}}, want: "重复"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildCreateTableSQL(tt.def)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("buildCreateTableSQL() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}
