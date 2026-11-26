package app

import (
	"testing"

	queryapp "github.com/yuwen002/meteor-ch-workbench/internal/application/query"
)

func TestBuildResultRowViewSortsWithoutMutatingSource(t *testing.T) {
	rows := [][]any{{"c", 10}, {"a", 2}, {"b", 1}}
	got := buildResultRowView(rows, "", 1, false)
	if len(got) != 3 || got[0].Index != 2 || got[1].Index != 1 || got[2].Index != 0 {
		t.Fatalf("ascending order = %#v", got)
	}
	if rows[0][0] != "c" || rows[1][0] != "a" || rows[2][0] != "b" {
		t.Fatalf("sorting mutated source rows: %#v", rows)
	}
}

func TestBuildResultRowViewCombinesFilterAndColumnCondition(t *testing.T) {
	rows := [][]any{
		{"alpha", 3},
		{"skip", 1},
		{"ALPHA", 9},
	}
	got := buildResultRowViewWithColumnFilters(rows, "alpha", []string{"", ">=5"}, 1, false)
	if len(got) != 1 || got[0].Index != 2 {
		t.Fatalf("filtered rows = %#v", got)
	}
}

func TestResultColumnLabelShowsSortDirection(t *testing.T) {
	columns := []queryapp.Column{{Name: "id", DatabaseType: "UInt64"}}
	if got := resultColumnLabel(columns, 0, -1, false); got != "id" {
		t.Fatalf("unsorted label = %q", got)
	}
	if got := resultColumnLabel(columns, 0, 0, true); got != "id ↓" {
		t.Fatalf("descending label = %q", got)
	}
}
