package gioapp

import (
	"reflect"
	"testing"
	"time"
)

func TestBuildResultRowViewSortsWithoutMutatingSource(t *testing.T) {
	rows := [][]interface{}{
		{"c", 10},
		{"a", 2},
		{"b", 1},
	}
	original := [][]interface{}{{"c", 10}, {"a", 2}, {"b", 1}}
	got := buildResultRowView(rows, "", 1, false)
	want := []int{2, 1, 0}
	for i := range want {
		if got[i].Index != want[i] {
			t.Fatalf("ascending row[%d].Index = %d, want %d", i, got[i].Index, want[i])
		}
	}
	desc := buildResultRowView(rows, "", 1, true)
	if desc[0].Index != 0 || desc[1].Index != 1 || desc[2].Index != 2 {
		t.Fatalf("descending order = %#v", desc)
	}
	if !reflect.DeepEqual(rows, original) {
		t.Fatalf("sorting mutated source rows: %#v", rows)
	}
}

func TestBuildResultRowViewKeepsNullAtEnd(t *testing.T) {
	rows := [][]interface{}{{nil}, {"z"}, {"a"}}
	for _, descending := range []bool{false, true} {
		got := buildResultRowView(rows, "", 0, descending)
		if got[len(got)-1].Index != 0 {
			t.Fatalf("descending=%v NULL row should remain last, got %#v", descending, got)
		}
	}
}

func TestBuildResultRowViewFiltersAcrossAllColumnsCaseInsensitively(t *testing.T) {
	rows := [][]interface{}{
		{"alpha", "ClickHouse"},
		{"beta", "postgres"},
		{"gamma", "CLICKHOUSE"},
	}
	got := buildResultRowView(rows, "clickhouse", -1, false)
	if len(got) != 2 || got[0].Index != 0 || got[1].Index != 2 {
		t.Fatalf("filtered rows = %#v", got)
	}
}

func TestBuildResultRowViewCombinesFilterAndSort(t *testing.T) {
	rows := [][]interface{}{
		{"match", 3},
		{"skip", 0},
		{"MATCH", 1},
	}
	got := buildResultRowView(rows, "match", 1, false)
	if len(got) != 2 || got[0].Index != 2 || got[1].Index != 0 {
		t.Fatalf("filtered and sorted rows = %#v", got)
	}
}

func TestCompareResultValuesUsesTimeOrder(t *testing.T) {
	older := "2026-01-01 10:00:00"
	newer := "2026-01-02 09:00:00"
	if compareResultValues(older, newer) >= 0 {
		t.Fatalf("expected older timestamp before newer")
	}
	parsed, err := parseResultTime("2026-01-02T09:00:00Z")
	if err != nil {
		t.Fatalf("parseResultTime() error = %v", err)
	}
	want := time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC)
	if !parsed.Equal(want) {
		t.Fatalf("parsed timestamp = %v, want %v", parsed, want)
	}
}

func TestCompareResultValuesNumericAndText(t *testing.T) {
	if compareResultValues("9", "10") >= 0 {
		t.Fatal("numeric strings should compare numerically")
	}
	if compareResultValues("Alpha", "beta") >= 0 {
		t.Fatal("text comparison should ignore case")
	}
}

func TestBuildResultRowViewCanFilterNullValues(t *testing.T) {
	rows := [][]interface{}{{nil}, {"not null"}}
	got := buildResultRowView(rows, "null", -1, false)
	if len(got) != 1 || got[0].Index != 0 {
		t.Fatalf("NULL filter result = %#v", got)
	}
}

func TestBuildResultRowViewCombinesMultipleColumnFilters(t *testing.T) {
	rows := [][]interface{}{
		{"alpha", "ClickHouse", 10},
		{"alpha", "PostgreSQL", 20},
		{"beta", "ClickHouse", 30},
		{"alpha", "ClickHouse", 40},
	}
	got := buildResultRowViewWithColumnFilters(rows, "", []string{"alpha", "clickhouse", "4"}, -1, false)
	if len(got) != 1 || got[0].Index != 3 {
		t.Fatalf("multiple column filters = %#v, want source row 3", got)
	}
}

func TestBuildResultRowViewCombinesGlobalAndColumnFilters(t *testing.T) {
	rows := [][]interface{}{
		{"alpha", "ClickHouse"},
		{"beta", "ClickHouse"},
		{"alpha", "PostgreSQL"},
	}
	got := buildResultRowViewWithColumnFilters(rows, "click", []string{"alpha"}, -1, false)
	if len(got) != 1 || got[0].Index != 0 {
		t.Fatalf("global and column filters = %#v", got)
	}
}

func TestResultColumnFilterSupportsNullAndIgnoresEmptyColumns(t *testing.T) {
	rows := [][]interface{}{{nil, "ready"}, {"value", "ready"}}
	got := buildResultRowViewWithColumnFilters(rows, "", []string{"NULL", ""}, -1, false)
	if len(got) != 1 || got[0].Index != 0 {
		t.Fatalf("NULL column filter = %#v", got)
	}
	got = buildResultRowViewWithColumnFilters(rows, "", []string{"", "ready"}, -1, false)
	if len(got) != 2 {
		t.Fatalf("empty first-column filter should be ignored: %#v", got)
	}
}

func TestResultValueMatchesColumnOperators(t *testing.T) {
	tests := []struct {
		name string
		value interface{}
		filter string
		want bool
	}{
		{"contains", "ClickHouse", "house", true},
		{"not contains", "ClickHouse", "!house", false},
		{"equals case insensitive", "ClickHouse", "=clickhouse", true},
		{"not equals", "ClickHouse", "!=PostgreSQL", true},
		{"numeric greater", 12, ">10", true},
		{"numeric less", "9", "<10", true},
		{"numeric greater equal", "10", ">=10", true},
		{"numeric less equal", "10", "<=9", false},
		{"numeric equality", "10", "=10.0", true},
		{"null match", nil, "is:null", true},
		{"not null match", "x", "is:not-null", true},
		{"null numeric comparison", nil, ">0", false},
		{"text numeric comparison", "abc", ">0", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resultValueMatchesColumnCondition(tt.value, tt.filter); got != tt.want {
				t.Fatalf("resultValueMatchesColumnCondition(%v, %q) = %v, want %v", tt.value, tt.filter, got, tt.want)
			}
		})
	}
}

func TestBuildResultRowViewCombinesDifferentColumnOperators(t *testing.T) {
	rows := [][]interface{}{
		{"ClickHouse", 9},
		{"ClickHouse", 12},
		{"PostgreSQL", 15},
		{"clickhouse", 20},
	}
	got := buildResultRowViewWithColumnFilters(rows, "", []string{"=clickhouse", ">=12"}, -1, false)
	if len(got) != 2 || got[0].Index != 1 || got[1].Index != 3 {
		t.Fatalf("operator filters = %#v, want rows 1 and 3", got)
	}
}

func TestGlobalFilterKeepsLiteralOperatorCharacters(t *testing.T) {
	rows := [][]interface{}{{"!important"}, {"important"}}
	got := buildResultRowView(rows, "!", -1, false)
	if len(got) != 1 || got[0].Index != 0 {
		t.Fatalf("global keyword filter should treat operator characters literally: %#v", got)
	}
}


func TestResultValueMatchesColumnConditionSupportsTimeComparisons(t *testing.T) {
	value := "2026-11-23 12:30:00.123456"
	tests := []struct {
		name string
		expression string
		want bool
	}{
		{"after earlier date", ">2026-11-22", true},
		{"before next day", "<2026-11-24", true},
		{"inclusive exact instant", ">=2026-11-23 12:30:00.123456", true},
		{"exclusive exact instant", ">2026-11-23 12:30:00.123456", false},
		{"RFC3339 offset equivalent", "=2026-11-23T13:30:00.123456+01:00", true},
		{"invalid date", ">not-a-date", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resultValueMatchesColumnCondition(value, tt.expression); got != tt.want {
				t.Fatalf("resultValueMatchesColumnCondition(%q, %q) = %v, want %v", value, tt.expression, got, tt.want)
			}
		})
	}
}

func TestResultValueMatchesColumnConditionKeepsNumericComparison(t *testing.T) {
	if !resultValueMatchesColumnCondition("1700000000", ">1699999999") {
		t.Fatal("numeric comparison should continue to use numeric semantics")
	}
	if resultValueMatchesColumnCondition("1700000000", ">2026-01-01") {
		t.Fatal("numeric value must not be treated as a timestamp")
	}
}
