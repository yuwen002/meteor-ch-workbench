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


func TestCompareResultSortKeysMatchesExistingSemantics(t *testing.T) {
	values := []any{
		nil,
		int64(0),
		uint64(9007199254740993),
		"9007199254740992",
		"1.0000000000000000001",
		"1.0000000000000000002",
		"2026-11-25 10:00:00",
		"2026-11-25T10:00:00Z",
		"alpha",
		"ALPHA",
		"beta",
	}
	keys := make([]resultSortKey, len(values))
	for i, value := range values {
		keys[i] = newResultSortKey(value)
	}
	for i := range values {
		for j := range values {
			want := compareResultValues(values[i], values[j])
			got := compareResultSortKeys(keys[i], keys[j])
			if sign(got) != sign(want) {
				t.Errorf("compare(%#v, %#v) = %d, want sign %d", values[i], values[j], got, want)
			}
		}
	}
}

func TestBuildResultRowViewKeepsEqualSortValuesStable(t *testing.T) {
	rows := [][]any{{"first", 2}, {"second", 2}, {"third", 1}}
	got := buildResultRowView(rows, "", 1, false)
	want := []int{2, 0, 1}
	if len(got) != len(want) {
		t.Fatalf("view length = %d, want %d", len(got), len(want))
	}
	for i, index := range want {
		if got[i].Index != index {
			t.Errorf("view[%d].Index = %d, want %d", i, got[i].Index, index)
		}
	}
}

func sign(value int) int {
	switch {
	case value < 0:
		return -1
	case value > 0:
		return 1
	default:
		return 0
	}
}
