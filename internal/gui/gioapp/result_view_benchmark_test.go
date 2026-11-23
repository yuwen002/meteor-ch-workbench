package gioapp

import (
	"fmt"
	"testing"
)

func benchmarkResultRows(count int) [][]interface{} {
	rows := make([][]interface{}, count)
	for i := 0; i < count; i++ {
		rows[i] = []interface{}{
			fmt.Sprintf("user-%06d", i),
			int64(i % 100000),
			fmt.Sprintf("2026-11-%02d 12:%02d:%02d", i%28+1, i%60, i%60),
			[]string{"success", "warning", "error"}[i%3],
		}
	}
	return rows
}

// BenchmarkBuildResultRowView 用固定数据集观察筛选 / 排序的时间与分配情况。
// 使用 -benchmem 可同时查看每次操作的内存分配。
func BenchmarkBuildResultRowView(b *testing.B) {
	for _, size := range []int{10_000, 100_000} {
		rows := benchmarkResultRows(size)
		b.Run(fmt.Sprintf("rows-%d/global-keyword", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = buildResultRowViewWithColumnFilters(rows, "user-0001", nil, -1, false)
			}
		})
		b.Run(fmt.Sprintf("rows-%d/multi-column", size), func(b *testing.B) {
			filters := []string{"", ">=50000", ">=2026-11-10", "error"}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = buildResultRowViewWithColumnFilters(rows, "", filters, -1, false)
			}
		})
		b.Run(fmt.Sprintf("rows-%d/combined-sort", size), func(b *testing.B) {
			filters := []string{"user-000", ">=1000", "", ""}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = buildResultRowViewWithColumnFilters(rows, "success", filters, 1, true)
			}
		})
	}
}
