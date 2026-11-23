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
