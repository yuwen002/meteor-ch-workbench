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
