package app

import (
	"fmt"
	"testing"
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
