package gioapp

import (
	"testing"

	"github.com/yuwen002/meteor-ch-workbench/internal/clickhouse"
)

func TestClampEditorRatio(t *testing.T) {
	tests := []struct {
		name string
		input float32
		want float32
	}{
		{name: "低于最小比例", input: 0.1, want: 0.35},
		{name: "高于最大比例", input: 0.95, want: 0.80},
		{name: "保留有效比例", input: 0.62, want: 0.62},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampEditorRatio(tt.input); got != tt.want {
				t.Fatalf("clampEditorRatio(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestAdjustEditorRatio(t *testing.T) {
	tests := []struct {
		name           string
		current, delta float32
		height         int
		want           float32
	}{
		{name: "向下拖动扩大编辑器", current: 0.62, delta: 90, height: 900, want: 0.72},
		{name: "向上拖动缩小编辑器", current: 0.62, delta: -90, height: 900, want: 0.52},
		{name: "拖动不超过最大值", current: 0.78, delta: 90, height: 900, want: 0.80},
		{name: "无有效工作区高度", current: 0.62, delta: 90, height: 0, want: 0.62},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := adjustEditorRatio(tt.current, tt.delta, tt.height); got != tt.want {
				t.Fatalf("adjustEditorRatio(%v, %v, %d) = %v, want %v", tt.current, tt.delta, tt.height, got, tt.want)
			}
		})
	}
}

func TestResultColumnWeights(t *testing.T) {
	columns := []clickhouse.ColumnInfo{
		{Name: "id"},
		{Name: "description"},
		{Name: "created_at"},
	}
	rows := [][]interface{}{
		{int64(1), "a short value", "2026-11-23 10:00:00"},
		{int64(2), "a much longer description used to allocate more room", "2026-11-23 10:01:00"},
	}
	weights := resultColumnWeights(columns, rows)
	if len(weights) != len(columns) {
		t.Fatalf("got %d weights, want %d", len(weights), len(columns))
	}
	if weights[1] <= weights[0] {
		t.Fatalf("description column should receive more width: %#v", weights)
	}
	for i, weight := range weights {
		if weight < 10 || weight > 32 {
			t.Fatalf("weight[%d]=%v is outside [10, 32]", i, weight)
		}
	}
}

func TestResultColumnWeightsHandlesMissingCellsAndEmptyRows(t *testing.T) {
	columns := []clickhouse.ColumnInfo{{Name: "a"}, {Name: "a_very_long_column_name_that_needs_clamping"}}
	weights := resultColumnWeights(columns, [][]interface{}{{"x"}})
	if weights[0] != 10 {
		t.Fatalf("short column weight = %v, want 10", weights[0])
	}
	if weights[1] != 32 {
		t.Fatalf("long column weight = %v, want 32", weights[1])
	}
}
