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

func TestResultValueTextPreservesFormatting(t *testing.T) {
	cases := []struct {
		name string
		value any
		want string
	}{
		{name: "string", value: "ClickHouse", want: "ClickHouse"},
		{name: "number", value: int64(42), want: "42"},
		{name: "null", value: nil, want: "NULL"},
		{name: "byte slice", value: []byte("ab"), want: "[97 98]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resultValueText(tc.value); got != tc.want {
				t.Fatalf("resultValueText(%#v) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}


func TestResultValueTextUsedBySortKeysAndColumnFilters(t *testing.T) {
	key := newResultSortKey("MiXeD")
	if key.text != "mixed" {
		t.Fatalf("sort key text = %q, want mixed", key.text)
	}
	if !resultValueMatchesColumnCondition("MiXeD", "mixed") {
		t.Fatal("column contains filter should match string values case-insensitively")
	}
	if !resultValueMatchesFilter("MiXeD", "mixed") {
		t.Fatal("single-value filter should match string values case-insensitively")
	}
}

func TestResultTextContainsLowerNeedlePreservesASCIIFoldingAndUnicode(t *testing.T) {
	cases := []struct {
		name  string
		value string
		needle string
		want  bool
	}{
		{name: "lowercase ASCII", value: "clickhouse server", needle: "click", want: true},
		{name: "uppercase ASCII", value: "ClickHouse Server", needle: "clickhouse", want: true},
		{name: "mixed ASCII substring", value: "user-ABC-123", needle: "abc", want: true},
		{name: "ASCII miss", value: "clickhouse", needle: "mysql", want: false},
		{name: "Unicode uppercase", value: "ÄBC Server", needle: "äbc", want: true},
		{name: "Unicode lowercase", value: "Straße", needle: "straße", want: true},
		{name: "Unicode miss", value: "ÄBC Server", needle: "ö", want: false},
		{name: "empty needle", value: "anything", needle: "", want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resultTextContainsLowerNeedle(tc.value, tc.needle); got != tc.want {
				t.Fatalf("resultTextContainsLowerNeedle(%q, %q) = %v, want %v", tc.value, tc.needle, got, tc.want)
			}
		})
	}
}

func TestResultRowMatchesLowerFilter(t *testing.T) {
	row := []any{"ClickHouse", int64(42), nil, []byte("ab")}
	if !resultRowMatchesLowerFilter(row, "click") {
		t.Fatal("case-insensitive keyword should match string values")
	}
	if !resultRowMatchesLowerFilter(row, "42") {
		t.Fatal("keyword should match formatted numeric values")
	}
	if !resultRowMatchesLowerFilter(row, "null") {
		t.Fatal("keyword should match NULL values")
	}
	if !resultRowMatchesLowerFilter(row, "[97 98]") {
		t.Fatal("byte slices should retain fmt.Sprint matching semantics")
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


func TestPruneResultCellCachesKeepsVisibleRowsAndOverscan(t *testing.T) {
	clicks := make(map[int64]*widget.Clickable)
	texts := make(map[int64]string)
	for row := 0; row < 20; row++ {
		key := resultCellKey(row, 0)
		clicks[key] = new(widget.Clickable)
		texts[key] = "cached"
	}

	pruneResultCellCaches(clicks, texts, layout.Position{First: 8, Count: 4}, 20, 2)
	if len(clicks) != 8 || len(texts) != 8 {
		t.Fatalf("cache sizes = %d/%d, want 8/8", len(clicks), len(texts))
	}
	for row := 6; row < 14; row++ {
		key := resultCellKey(row, 0)
		if clicks[key] == nil || texts[key] != "cached" {
			t.Errorf("row %d should remain cached", row)
		}
	}
	for _, row := range []int{5, 14, 19} {
		key := resultCellKey(row, 0)
		if _, ok := clicks[key]; ok {
			t.Errorf("row %d click cache should be pruned", row)
		}
		if _, ok := texts[key]; ok {
			t.Errorf("row %d text cache should be pruned", row)
		}
	}
}

func TestPruneResultCellCachesHandlesEmptyAndOutOfRangePositions(t *testing.T) {
	clicks := map[int64]*widget.Clickable{
		resultCellKey(0, 0): new(widget.Clickable),
		resultCellKey(4, 0): new(widget.Clickable),
	}
	texts := map[int64]string{
		resultCellKey(0, 0): "first",
		resultCellKey(4, 0): "last",
	}
	pruneResultCellCaches(clicks, texts, layout.Position{First: 99, Count: 0}, 5, 1)
	if len(clicks) != 0 || len(texts) != 0 {
		t.Fatalf("empty out-of-range viewport should clear caches, got %d/%d", len(clicks), len(texts))
	}
}


// TestBuildResultRowViewIntoReusesBackingArray verifies refreshes reuse the index storage.
func TestBuildResultRowViewIntoReusesBackingArray(t *testing.T) {
	rows := [][]any{{"alpha", 3}, {"beta", 1}, {"gamma", 2}}
	storage := make([]resultRowView, 0, len(rows))
	storage = append(storage, resultRowView{Index: 99})
	base := &storage[:cap(storage)][0]

	view := buildResultRowViewWithPreparedSortKeysInto(storage[:0], rows, "a", nil, -1, false, nil)
	if len(view) != 3 {
		t.Fatalf("filtered view length = %d, want 3", len(view))
	}
	if &view[:cap(view)][0] != base {
		t.Fatal("view refresh allocated a new backing array despite sufficient capacity")
	}

	view = buildResultRowViewWithPreparedSortKeysInto(view[:0], rows, "beta", nil, -1, false, nil)
	if len(view) != 1 || view[0].Index != 1 {
		t.Fatalf("narrowed view = %#v, want source row 1", view)
	}
	view = buildResultRowViewWithPreparedSortKeysInto(view[:0], rows, "", nil, 1, false, extendResultSortKeys(rows, 1, nil))
	if len(view) != 3 || view[0].Index != 1 || view[1].Index != 2 || view[2].Index != 0 {
		t.Fatalf("expanded sorted view = %#v, want [1 2 0]", view)
	}
	if &view[:cap(view)][0] != base {
		t.Fatal("expanding the view should reuse its retained capacity")
	}
	if rows[0][0] != "alpha" || rows[1][0] != "beta" || rows[2][0] != "gamma" {
		t.Fatalf("building the view mutated source rows: %#v", rows)
	}
}


func TestParseResultTimeFastRejectsNonDateText(t *testing.T) {
	for _, value := range []string{"query-event-000123", "123456789", "", "2026/11/27 10:00:00", " 2026-11-27"} {
		if _, err := parseResultTime(value); err == nil {
			t.Errorf("parseResultTime(%q) should reject unsupported prefix", value)
		}
	}
}

func TestParseResultTimeAcceptsSupportedDateLayouts(t *testing.T) {
	for _, value := range []string{
		"2026-11-27",
		"2026-11-27 10:15:30",
		"2026-11-27 10:15:30.123456789",
		"2026-11-27T10:15:30Z",
	} {
		if _, err := parseResultTime(value); err != nil {
			t.Errorf("parseResultTime(%q) returned error: %v", value, err)
		}
	}
}


func TestCompareResultSortKeysPreservesTimestampInstantAndPrecision(t *testing.T) {
	values := []any{
		"0000-01-01T00:00:00Z",
		"2026-11-27T10:15:30.123456789Z",
		"2026-11-27T12:15:30.123456789+02:00",
		"2026-11-27T10:15:30.123456788Z",
		"9999-12-31T23:59:59.999999999Z",
	}
	keys := make([]resultSortKey, len(values))
	for i, value := range values {
		keys[i] = newResultSortKey(value)
		if !keys[i].timeValid {
			t.Fatalf("value %q should be parsed as a timestamp", value)
		}
	}
	for i := range values {
		for j := range values {
			want := compareResultValues(values[i], values[j])
			got := compareResultSortKeys(keys[i], keys[j])
			if resultCompareSign(got) != resultCompareSign(want) {
				t.Errorf("compare(%q, %q) = %d, want sign %d", values[i], values[j], got, resultCompareSign(want))
			}
		}
	}
	if got := compareResultSortKeys(keys[1], keys[2]); got != 0 {
		t.Fatalf("equivalent instants with different offsets compare as %d, want 0", got)
	}
	if got := compareResultSortKeys(keys[3], keys[1]); got >= 0 {
		t.Fatalf("nanosecond precision lost: preceding instant compare = %d", got)
	}
}


func TestResultSortKeyAllocatesNumericPayloadOnlyForNumbers(t *testing.T) {
	numeric := newResultSortKey("18446744073709551615.0000000001")
	if !numeric.numeric || numeric.number == nil {
		t.Fatalf("numeric key = %#v, want allocated arbitrary-precision payload", numeric)
	}
	text := newResultSortKey("query-event-000123")
	if text.numeric || text.number != nil {
		t.Fatalf("text key allocated numeric payload: %#v", text)
	}
	timestamp := newResultSortKey("2026-11-27T10:15:30Z")
	if timestamp.numeric || timestamp.number != nil || !timestamp.timeValid {
		t.Fatalf("timestamp key should not allocate numeric payload: %#v", timestamp)
	}
	if got := compareResultSortKeys(numeric, newResultSortKey("18446744073709551615.0000000002")); got >= 0 {
		t.Fatalf("high precision numeric comparison = %d, want less than zero", got)
	}
}


func TestIsResultSortNumberCandidate(t *testing.T) {
	cases := []struct { value string; want bool }{
		{value: "42", want: true},
		{value: " 42 ", want: false},
		{value: "-12.50", want: true},
		{value: "1/3", want: true},
		{value: "1.2e+10", want: true},
		{value: "query-event-42", want: false},
		{value: "2026-11-27T10:00:00Z", want: false},
		{value: "", want: false},
		{value: "---", want: false},
	}
	for _, tc := range cases {
		if got := isResultSortNumberCandidate(tc.value); got != tc.want {
			t.Errorf("isResultSortNumberCandidate(%q) = %v, want %v", tc.value, got, tc.want)
		}
	}
}
