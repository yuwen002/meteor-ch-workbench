package app

import (
	"fmt"
	"strings"
	"testing"
	"unsafe"
)

func benchmarkAppResultRows(count int) [][]any {
	rows := make([][]any, count)
	for i := range rows {
		rows[i] = []any{
			fmt.Sprintf("user-%06d", i),
			"event",
			int64(count - i),
			[]string{"success", "warning", "error"}[i%3],
		}
	}
	return rows
}

// BenchmarkBuildResultRowViewLargeResult measures full scans at 10k, 100k and 500k rows.
func BenchmarkBuildResultRowViewLargeResult(b *testing.B) {
	for _, rowCount := range []int{10_000, 100_000, 500_000} {
		b.Run(fmt.Sprintf("rows-%d/filter", rowCount), func(b *testing.B) {
			rows := benchmarkAppResultRows(rowCount)
			view := make([]resultRowView, 0, rowCount)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				view = buildResultRowViewWithPreparedSortKeysInto(view[:0], rows, "user-0001", nil, -1, false, nil)
			}
		})
		b.Run(fmt.Sprintf("rows-%d/filter-sort", rowCount), func(b *testing.B) {
			rows := benchmarkAppResultRows(rowCount)
			keys := extendResultSortKeys(rows, 2, nil)
			view := make([]resultRowView, 0, rowCount)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				view = buildResultRowViewWithPreparedSortKeysInto(view[:0], rows, "success", []string{"", "", ">=1", ""}, 2, true, keys)
			}
		})
	}
}


// BenchmarkExtendResultSortKeysReusesCache verifies appending no new rows does not
// rebuild the cached sort keys for a large result set.
func BenchmarkExtendResultSortKeysReusesCache(b *testing.B) {
	for _, rowCount := range []int{10_000, 100_000, 500_000} {
		b.Run(fmt.Sprintf("rows-%d", rowCount), func(b *testing.B) {
			rows := benchmarkAppResultRows(rowCount)
			keys := extendResultSortKeys(rows, 2, nil)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				keys = extendResultSortKeys(rows, 2, keys)
			}
		})
	}
}

// BenchmarkResultValueText compares the native string fast path with fmt.Sprint.
// It isolates conversion cost; end-to-end filter benchmarks remain the measure of UI-path impact.
func BenchmarkResultValueText(b *testing.B) {
	values := []struct {
		name  string
		value any
	}{
		{name: "string/fast-path", value: "clickhouse-event-name"},
		{name: "string/fmt-sprint", value: "clickhouse-event-name"},
		{name: "int64/fallback", value: int64(123456789)},
		{name: "nil/null", value: nil},
		{name: "bytes/fallback", value: []byte("clickhouse")},
	}
	for _, item := range values {
		b.Run(item.name, func(b *testing.B) {
			b.ReportAllocs()
			var result string
			b.ResetTimer()
			for b.Loop() {
				if item.name == "string/fmt-sprint" {
					result = fmt.Sprint(item.value)
				} else {
					result = resultValueText(item.value)
				}
			}
			benchmarkResultValueTextSink = result
		})
	}
}

var benchmarkResultValueTextSink string


// BenchmarkResultTextContainsLowerNeedle compares allocation-free ASCII matching
// with the previous lowercase-copy implementation, while checking Unicode fallback.
func BenchmarkResultTextContainsLowerNeedle(b *testing.B) {
	cases := []struct {
		name string
		value string
		needle string
	}{
		{name: "lowercase-ascii", value: "user-000123 clickhouse query finished successfully", needle: "clickhouse"},
		{name: "mixed-ascii", value: "User-000123 ClickHouse Query Finished Successfully", needle: "clickhouse"},
		{name: "unicode-fallback", value: "ÄBC 数据库服务已启动", needle: "äbc"},
	}
	for _, item := range cases {
		b.Run(item.name+"/legacy", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				benchmarkResultTextContainsSink = strings.Contains(strings.ToLower(item.value), item.needle)
			}
		})
		b.Run(item.name+"/optimized", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				benchmarkResultTextContainsSink = resultTextContainsLowerNeedle(item.value, item.needle)
			}
		})
	}
}

var benchmarkResultTextContainsSink bool


// BenchmarkBuildResultSortKeys measures the one-time cost of preparing sort keys.
// The cache is rebuilt on each iteration so parsing and key allocations are included.
func BenchmarkBuildResultSortKeys(b *testing.B) {
	cases := []struct {
		name string
		value func(int) any
	}{
		{name: "int64", value: func(i int) any { return int64(i) }},
		{name: "decimal-string", value: func(i int) any { return fmt.Sprintf("%d.%09d", i, i%1_000_000_000) }},
		{name: "timestamp", value: func(i int) any { return fmt.Sprintf("2026-11-%02d 10:15:%02d", i%28+1, i%60) }},
		{name: "plain-text", value: func(i int) any { return fmt.Sprintf("query-event-%06d", i) }},
		{name: "null", value: func(int) any { return nil }},
	}
	for _, item := range cases {
		for _, rowCount := range []int{10_000, 100_000, 500_000} {
			b.Run(fmt.Sprintf("%s/rows-%d", item.name, rowCount), func(b *testing.B) {
				rows := make([][]any, rowCount)
				for i := range rows {
					rows[i] = []any{item.value(i)}
				}
				b.ReportAllocs()
				b.ResetTimer()
				var keys []resultSortKey
				for b.Loop() {
					keys = extendResultSortKeys(rows, 0, nil)
				}
				benchmarkResultSortKeyCountSink = len(keys)
				b.ReportMetric(float64(unsafe.Sizeof(resultSortKey{})), "B/key")
			})
		}
	}
}

var benchmarkResultSortKeyCountSink int


// BenchmarkBuildResultRowViewTextNumericFilter measures numeric comparisons over
// ordinary text columns, where the numeric prefilter should skip big.Rat parsing.
func BenchmarkBuildResultRowViewTextNumericFilter(b *testing.B) {
	for _, rowCount := range []int{10_000, 100_000, 500_000} {
		b.Run(fmt.Sprintf("rows-%d", rowCount), func(b *testing.B) {
			rows := make([][]any, rowCount)
			for i := range rows {
				rows[i] = []any{fmt.Sprintf("query-event-%06d", i), "event"}
			}
			view := make([]resultRowView, 0, rowCount)
			filters := []string{">=100", ""}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				view = buildResultRowViewWithPreparedSortKeysInto(view[:0], rows, "", filters, -1, false, nil)
			}
			benchmarkResultSortKeyCountSink = len(view)
		})
	}
}


// BenchmarkBuildResultRowViewManyInactiveColumnFilters measures a wide result
// where the user has not enabled any of the available column filter controls.
func BenchmarkBuildResultRowViewManyInactiveColumnFilters(b *testing.B) {
	for _, rowCount := range []int{10_000, 100_000, 500_000} {
		b.Run(fmt.Sprintf("rows-%d/64-columns", rowCount), func(b *testing.B) {
			rows := make([][]any, rowCount)
			for i := range rows {
				rows[i] = []any{fmt.Sprintf("row-%06d", i)}
			}
			filters := make([]string, 64)
			view := make([]resultRowView, 0, rowCount)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				view = buildResultRowViewWithPreparedSortKeysInto(view[:0], rows, "", filters, -1, false, nil)
			}
			benchmarkResultSortKeyCountSink = len(view)
		})
	}
}


// BenchmarkBuildResultRowViewSparseColumnFilters measures a wide result with
// only one active condition in the final column.
func BenchmarkBuildResultRowViewSparseColumnFilters(b *testing.B) {
	for _, rowCount := range []int{10_000, 100_000, 500_000} {
		b.Run(fmt.Sprintf("rows-%d/64-columns/1-active", rowCount), func(b *testing.B) {
			rows := make([][]any, rowCount)
			for i := range rows {
				row := make([]any, 64)
				row[63] = int64(i % 1000)
				rows[i] = row
			}
			filters := make([]string, 64)
			filters[63] = ">=100"
			view := make([]resultRowView, 0, rowCount)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				view = buildResultRowViewWithPreparedSortKeysInto(view[:0], rows, "", filters, -1, false, nil)
			}
			benchmarkResultSortKeyCountSink = len(view)
		})
	}
}


// benchmarkBuildResultRowViewInFilterOrder compares equivalent AND filters while
// changing only their evaluation order. Prepared filters are shared by both paths.
func benchmarkBuildResultRowViewInFilterOrder(dst []resultRowView, rows [][]any, needle string, filters []indexedResultColumnFilter, columnFirst bool) []resultRowView {
	view := dst[:0]
	for i, row := range rows {
		if columnFirst {
			if !resultRowMatchesPreparedColumnFilters(row, filters) {
				continue
			}
			if needle != "" && !resultRowMatchesLowerFilter(row, needle) {
				continue
			}
		} else {
			if needle != "" && !resultRowMatchesLowerFilter(row, needle) {
				continue
			}
			if !resultRowMatchesPreparedColumnFilters(row, filters) {
				continue
			}
		}
		view = append(view, resultRowView{Index: i})
	}
	return view
}

// BenchmarkBuildResultRowViewCombinedFilters measures the scan-order trade-off
// for a 64-column result with one sparse numeric filter and a selective keyword.
// Rows share immutable synthetic row data to keep benchmark setup memory bounded.
func BenchmarkBuildResultRowViewCombinedFilters(b *testing.B) {
	for _, rowCount := range []int{10_000, 100_000, 500_000} {
		b.Run(fmt.Sprintf("rows-%d/64-columns/keyword-1-percent/column-10-percent", rowCount), func(b *testing.B) {
			const patternCount = 1000
			patterns := make([][]any, patternCount)
			for i := range patterns {
				row := make([]any, 64)
				for column := range row {
					row[column] = "value"
				}
				if i%100 == 0 {
					row[0] = "needle-event"
				} else {
					row[0] = "ordinary-event"
				}
				row[63] = int64(i % 1000)
				patterns[i] = row
			}
			rows := make([][]any, rowCount)
			for i := range rows {
				rows[i] = patterns[i%patternCount]
			}
			columnFilters := make([]string, 64)
			columnFilters[63] = ">=900"
			prepared := prepareResultColumnFilters(columnFilters)
			needle := "needle"
			globalFirst := benchmarkBuildResultRowViewInFilterOrder(nil, rows, needle, prepared, false)
			columnFirst := benchmarkBuildResultRowViewInFilterOrder(nil, rows, needle, prepared, true)
			if len(globalFirst) != len(columnFirst) {
				b.Fatalf("filter order result counts differ: global-first=%d column-first=%d", len(globalFirst), len(columnFirst))
			}
			for i := range globalFirst {
				if globalFirst[i].Index != columnFirst[i].Index {
					b.Fatalf("filter order differs at result %d: %d != %d", i, globalFirst[i].Index, columnFirst[i].Index)
				}
			}
			b.Run("global-first", func(b *testing.B) {
				view := make([]resultRowView, 0, rowCount)
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					view = benchmarkBuildResultRowViewInFilterOrder(view[:0], rows, needle, prepared, false)
				}
				benchmarkResultSortKeyCountSink = len(view)
			})
			b.Run("column-first", func(b *testing.B) {
				view := make([]resultRowView, 0, rowCount)
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					view = benchmarkBuildResultRowViewInFilterOrder(view[:0], rows, needle, prepared, true)
				}
				benchmarkResultSortKeyCountSink = len(view)
			})
		})
	}
}
