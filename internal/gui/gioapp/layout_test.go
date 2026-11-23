package gioapp

import "testing"

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
