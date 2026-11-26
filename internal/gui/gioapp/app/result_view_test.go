package app

import (
	"testing"

	"gioui.org/layout"
	"gioui.org/widget"

	queryapp "github.com/yuwen002/meteor-ch-workbench/internal/application/query"
)

func TestReleaseResultCellCachesKeepsArchivedQueryData(t *testing.T) {
	rows := [][]any{{"retained", int64(7)}}
	state := &workspace{
		queryResultState: queryResultState{
			queryColumns: []queryapp.Column{{Name: "value"}},
			queryData: rows,
		},
		queryCellClicks: map[int64]*widget.Clickable{1: new(widget.Clickable)},
		resultCellText: map[int64]string{1: "cached"},
		resultHeaderChildren: make([]layout.FlexChild, 0, 8),
		resultRowChildren: make([]layout.FlexChild, 0, 8),
		resultPixelWidths: make([]int, 0, 8),
	}
	state.releaseResultCellCaches()
	if state.queryCellClicks != nil || state.resultCellText != nil {
		t.Fatal("cell cache maps should release their backing storage")
	}
	if state.resultHeaderChildren != nil || state.resultRowChildren != nil || state.resultPixelWidths != nil {
		t.Fatal("layout cache slices should be released")
	}
	if len(state.queryColumns) != 1 || len(state.queryData) != 1 || state.queryData[0][0] != "retained" {
		t.Fatal("releasing view caches must not discard query data")
	}
}

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
			if resultCompareSign(got) != resultCompareSign(want) {
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

func resultCompareSign(value int) int {
	switch {
	case value < 0:
		return -1
	case value > 0:
		return 1
	default:
		return 0
	}
}

func TestExtendResultSortKeysReusesExistingAndAppendsNewRows(t *testing.T) {
	rows := [][]any{{"first", 3}, {"second", 1}}
	keys := extendResultSortKeys(rows, 1, nil)
	if len(keys) != 2 {
		t.Fatalf("initial key count = %d, want 2", len(keys))
	}
	first := keys[0]

	rows = append(rows, []any{"third", 2})
	keys = extendResultSortKeys(rows, 1, keys)
	if len(keys) != 3 {
		t.Fatalf("extended key count = %d, want 3", len(keys))
	}
	if compareResultSortKeys(keys[0], first) != 0 {
		t.Fatal("existing sort key changed while appending a result row")
	}
	view := buildResultRowViewWithPreparedSortKeys(rows, "", nil, 1, false, keys)
	want := []int{1, 2, 0}
	for i, index := range want {
		if i >= len(view) || view[i].Index != index {
			t.Fatalf("sorted view = %#v, want row indexes %v", view, want)
		}
	}
}

func TestExtendResultSortKeysHandlesMissingSortColumn(t *testing.T) {
	rows := [][]any{{"value"}}
	if got := extendResultSortKeys(rows, -1, []resultSortKey{{text: "stale"}}); got != nil {
		t.Fatalf("keys for disabled sorting = %#v, want nil", got)
	}
}

func TestBuildResultRowViewWithPreparedSortKeysExtendsShortCache(t *testing.T) {
	rows := [][]any{{"a", 2}, {"b", 1}}
	keys := extendResultSortKeys(rows[:1], 1, nil)
	view := buildResultRowViewWithPreparedSortKeys(rows, "", nil, 1, false, keys)
	if len(view) != 2 || view[0].Index != 1 || view[1].Index != 0 {
		t.Fatalf("view with short cache = %#v", view)
	}
}

func TestExtendResultSortKeysGrowsCapacityGeometrically(t *testing.T) {
	rows := make([][]any, 128)
	for i := range rows {
		rows[i] = []any{i}
	}
	keys := extendResultSortKeys(rows, 0, nil)
	if cap(keys) < len(keys) {
		t.Fatalf("initial capacity %d is smaller than length %d", cap(keys), len(keys))
	}
	if cap(keys) == len(keys) {
		t.Fatalf("expected spare capacity for the next streamed batch, got len=%d cap=%d", len(keys), cap(keys))
	}
	firstKey := &keys[0]

	for i := 0; i < 32; i++ {
		rows = append(rows, []any{128 + i})
	}
	keys = extendResultSortKeys(rows, 0, keys)
	if &keys[0] != firstKey {
		t.Fatal("cache reallocated before exhausting its reserved capacity")
	}
	if len(keys) != len(rows) {
		t.Fatalf("key count = %d, want %d", len(keys), len(rows))
	}
}

func TestPreparedResultColumnFiltersPreserveColumnPositions(t *testing.T) {
	rows := [][]any{{"alpha", 3}, {"beta", 8}, {"gamma", 5}}
	filters := prepareResultColumnFilters([]string{"", ">=5"})
	if len(filters) != 2 {
		t.Fatalf("prepared filter count = %d, want 2", len(filters))
	}
	if resultRowMatchesPreparedColumnFilters(rows[0], filters) {
		t.Fatal("row below numeric threshold should not match")
	}
	if !resultRowMatchesPreparedColumnFilters(rows[1], filters) || !resultRowMatchesPreparedColumnFilters(rows[2], filters) {
		t.Fatal("rows meeting numeric threshold should match")
	}
}

func TestPreparedResultColumnFiltersKeepTextNumericTimeAndNullSemantics(t *testing.T) {
	cases := []struct {
		name string
		value any
		expression string
		want bool
	}{
		{name: "text equal", value: "ClickHouse", expression: "=clickhouse", want: true},
		{name: "text contains", value: "ClickHouse Server", expression: "click", want: true},
		{name: "text not contains", value: "ClickHouse", expression: "!mysql", want: true},
		{name: "numeric greater", value: int64(12), expression: ">=10", want: true},
		{name: "numeric between", value: int64(12), expression: "between:10..15", want: true},
		{name: "numeric reversed range", value: int64(12), expression: "between:15..10", want: false},
		{name: "timestamp greater", value: "2026-11-26 12:00:00", expression: ">2026-11-26 11:00:00", want: true},
		{name: "timestamp between", value: "2026-11-26", expression: "between:2026-11-25..2026-11-27", want: true},
		{name: "null", value: nil, expression: "is:null", want: true},
		{name: "not null", value: "x", expression: "is:not-null", want: true},
		{name: "invalid range", value: 12, expression: "between:bad", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := preparedResultColumnFilterFor(tc.expression).matches(tc.value)
			if got != tc.want {
				t.Fatalf("matches(%#v, %q) = %v, want %v", tc.value, tc.expression, got, tc.want)
			}
		})
	}
}

func TestResultRowMatchesLowerFilter(t *testing.T) {
	row := []any{"ClickHouse", int64(42), nil}
	if !resultRowMatchesLowerFilter(row, "click") {
		t.Fatal("case-insensitive keyword should match string values")
	}
	if !resultRowMatchesLowerFilter(row, "42") {
		t.Fatal("keyword should match formatted numeric values")
	}
	if !resultRowMatchesLowerFilter(row, "null") {
		t.Fatal("keyword should match NULL values")
	}
	if resultRowMatchesLowerFilter(row, "missing") {
		t.Fatal("unmatched keyword should not match the row")
	}
}

func TestBuildResultRowViewNormalizesGlobalFilterOnceSemantically(t *testing.T) {
	rows := [][]any{{"Alpha"}, {"BETA"}, {"gamma"}}
	got := buildResultRowView(rows, "  bEtA  ", -1, false)
	if len(got) != 1 || got[0].Index != 1 {
		t.Fatalf("case-insensitive trimmed filter = %#v, want row 1", got)
	}
}
