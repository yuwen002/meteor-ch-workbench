package app

import "testing"

func BenchmarkFormatResultCell(b *testing.B) {
	values := []any{42, "hello\\nworld", "2026-11-25 10:00:00", nil, []byte("clickhouse")}
	b.ReportAllocs()
	for b.Loop() {
		for _, value := range values {
			_ = formatResultCell(value)
		}
	}
}

func BenchmarkResultCellKey(b *testing.B) {
	var total int64
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		total += resultCellKey(i, i%17)
	}
	_ = total
}
