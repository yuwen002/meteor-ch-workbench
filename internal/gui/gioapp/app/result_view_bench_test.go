package app

import "testing"

func BenchmarkBuildResultRowViewLargeResult(b *testing.B) {
	const rowCount = 10000
	rows := make([][]any, rowCount)
	for i := range rows {
		rows[i] = []any{i, "event", int64(rowCount - i), "payload"}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = buildResultRowView(rows, "", 2, false)
	}
}

func BenchmarkBuildResultRowViewFilteredLargeResult(b *testing.B) {
	const rowCount = 10000
	rows := make([][]any, rowCount)
	for i := range rows {
		rows[i] = []any{i, "event", int64(rowCount - i), "payload"}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = buildResultRowView(rows, "event", 2, false)
	}
}

func BenchmarkBuildResultRowViewWithCachedSortKeysFilteredLargeResult(b *testing.B) {
	const rowCount = 10000
	rows := make([][]any, rowCount)
	for i := range rows {
		rows[i] = []any{i, "event", int64(rowCount - i), "payload"}
	}
	keys := extendResultSortKeys(rows, 2, nil)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = buildResultRowViewWithPreparedSortKeys(rows, "event", nil, 2, false, keys)
	}
}

func BenchmarkExtendResultSortKeysReusesCache(b *testing.B) {
	const rowCount = 10000
	rows := make([][]any, rowCount)
	for i := range rows {
		rows[i] = []any{i, "event", int64(rowCount - i), "payload"}
	}
	keys := extendResultSortKeys(rows, 2, nil)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		keys = extendResultSortKeys(rows, 2, keys)
	}
}
