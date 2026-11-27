package app

import (
	"testing"

	queryapp "github.com/yuwen002/meteor-ch-workbench/internal/application/query"
)

func TestSQLLiteral(t *testing.T) {
	tests := []struct {
		name string
		value any
		want string
	}{
		{name: "null", value: nil, want: "NULL"},
		{name: "string quote", value: "O'Reilly", want: "'O''Reilly'"},
		{name: "integer", value: int64(42), want: "42"},
		{name: "boolean true", value: true, want: "1"},
		{name: "boolean false", value: false, want: "0"},
		{name: "bytes", value: []byte("a'b"), want: "'a''b'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sqlLiteral(tt.value); got != tt.want {
				t.Fatalf("sqlLiteral(%#v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestContextMenuActionsAreAvailableForEachTarget(t *testing.T) {
	state := &workspace{}
	for _, kind := range []string{"database", "object", "cell", "header"} {
		state.openContextMenu(kind, "default", "events", 0, 0)
		actions := state.contextActions()
		if len(actions) == 0 {
			t.Fatalf("context menu %q has no actions", kind)
		}
		for _, action := range actions {
			if action.label == "" || action.run == nil {
				t.Fatalf("context menu %q contains an incomplete action: %#v", kind, action)
			}
		}
	}
}

func TestVisibleResultColumnIndexes(t *testing.T) {
	got := visibleResultColumnIndexes(map[int]bool{1: true, 3: true, -1: true, 9: true}, 4)
	want := []int{0, 2}
	if len(got) != len(want) {
		t.Fatalf("visibleResultColumnIndexes() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] { t.Fatalf("visibleResultColumnIndexes() = %v, want %v", got, want) }
	}
}

func TestResultInsertTarget(t *testing.T) {
	tests := []struct { name, database, table, want string }{
		{name: "selected table", database: "analytics", table: "events", want: "`analytics`.`events`"},
		{name: "table without database", table: "events", want: "`events`"},
		{name: "placeholder", want: "`table_name`"},
		{name: "escaped identifiers", database: "a`b", table: "e`v", want: "`a``b`.`e``v`"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resultInsertTarget(tt.database, tt.table); got != tt.want {
				t.Fatalf("resultInsertTarget() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestContextInsertSQLUsesVisibleColumnsAndSelectedTable(t *testing.T) {
	state := &workspace{
		contextMenu: contextMenuState{row: 0},
		selectedDatabase: "analytics",
		selectedTable: "events",
		queryColumns: []queryapp.Column{{Name: "id"}, {Name: "secret"}, {Name: "name"}},
		queryData: [][]any{{int64(7), "hidden", "O'Reilly"}},
		resultView: []resultRowView{{Index: 0}},
		hiddenResultColumns: map[int]bool{1: true},
	}
	got := state.contextInsertSQL(false)
	want := "INSERT INTO `analytics`.`events` (`id`, `name`) VALUES\n(7, 'O''Reilly');"
	if got != want {
		t.Fatalf("contextInsertSQL() = %q, want %q", got, want)
	}
}

func TestContextRowTSVSkipsHiddenColumns(t *testing.T) {
	state := &workspace{
		contextMenu: contextMenuState{row: 0},
		queryColumns: []queryapp.Column{{Name: "id"}, {Name: "secret"}, {Name: "name"}},
		queryData: [][]any{{int64(7), "hidden", "Alice"}},
		resultView: []resultRowView{{Index: 0}},
		hiddenResultColumns: map[int]bool{1: true},
	}
	if got, want := state.contextRowTSV(), "7\tAlice"; got != want {
		t.Fatalf("contextRowTSV() = %q, want %q", got, want)
	}
}
