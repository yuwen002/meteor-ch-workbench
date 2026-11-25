package app

import (
	"bytes"
	"strings"
	"testing"

	queryapp "github.com/yuwen002/meteor-ch-workbench/internal/application/query"
)

func TestMatchesResultColumnFilter(t *testing.T) {
	tests := []struct {
		name string
		value any
		filter string
		want bool
	}{
		{name: "contains", value: "Hello ClickHouse", filter: "click", want: true},
		{name: "not contains", value: "Hello ClickHouse", filter: "!click", want: false},
		{name: "equals numeric", value: int64(42), filter: "=42", want: true},
		{name: "not equals", value: int64(42), filter: "!=41", want: true},
		{name: "greater", value: int64(42), filter: ">41", want: true},
		{name: "less or equal", value: int64(42), filter: "<=42", want: true},
		{name: "between", value: float64(4.5), filter: "between:4..5", want: true},
		{name: "null", value: nil, filter: "is:null", want: true},
		{name: "invalid between", value: int64(4), filter: "between:4", want: false},
		{name: "empty", value: nil, filter: "", want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := matchesResultColumnFilter(test.value, test.filter); got != test.want {
				t.Fatalf("matchesResultColumnFilter(%v, %q) = %v, want %v", test.value, test.filter, got, test.want)
			}
		})
	}
}

func TestCompareResultValues(t *testing.T) {
	if got := compareResultValues(int64(9), "10"); got >= 0 {
		t.Fatalf("numeric comparison = %d, want 9 < 10", got)
	}
	if got := compareResultValues("Alpha", "beta"); got >= 0 {
		t.Fatalf("text comparison = %d, want Alpha < beta", got)
	}
	if got := compareResultValues(nil, "NULL"); got >= 0 {
		t.Fatalf("NULL ordering = %d, want nil first", got)
	}
}

func TestResultCellKeySeparatesRowsAndColumns(t *testing.T) {
	if resultCellKey(1, 2) == resultCellKey(2, 1) {
		t.Fatal("different row/column pairs must have different keys")
	}
}

func TestResultColumnFilterSummary(t *testing.T) {
	columns := []queryapp.Column{{Name: "id"}, {Name: "status"}}
	got := resultColumnFilterSummary(columns, []string{" >= 10 ", "", "ignored"})
	if len(got) != 1 || got[0] != "id: >= 10" {
		t.Fatalf("summaries = %#v, want [id: >= 10]", got)
	}
}

func TestWriteQueryExportCSV(t *testing.T) {
	var output bytes.Buffer
	columns := []queryapp.Column{{Name: "id"}, {Name: "message"}}
	rows := [][]any{{int64(7), "hello, world"}, {nil, "line1\nline2"}}
	if err := writeQueryExport(&output, columns, rows, "csv"); err != nil {
		t.Fatalf("writeQueryExport() error = %v", err)
	}
	got := output.String()
	for _, want := range []string{"id,message", "7,\"hello, world\"", "NULL,\"line1\nline2\""} {
		if !strings.Contains(got, want) {
			t.Fatalf("CSV output %q does not contain %q", got, want)
		}
	}
}

func TestWriteQueryExportTSV(t *testing.T) {
	var output bytes.Buffer
	columns := []queryapp.Column{{Name: "id"}, {Name: "value"}}
	rows := [][]any{{1, "two"}}
	if err := writeQueryExport(&output, columns, rows, "tsv"); err != nil {
		t.Fatalf("writeQueryExport() error = %v", err)
	}
	if got, want := output.String(), "id\tvalue\n1\ttwo\n"; got != want {
		t.Fatalf("TSV output = %q, want %q", got, want)
	}
}
