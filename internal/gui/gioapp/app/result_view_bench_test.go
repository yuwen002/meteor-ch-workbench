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
