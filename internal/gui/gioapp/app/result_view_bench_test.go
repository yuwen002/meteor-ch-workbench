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
