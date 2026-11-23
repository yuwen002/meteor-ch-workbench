package gioapp

import (
	"strings"
	"testing"
)

func TestFormatResultCell(t *testing.T) {
	tests := []struct {
		name string
		value interface{}
		want string
	}{
		{name: "空值显示 NULL", value: nil, want: "NULL"},
		{name: "普通字符串原样显示", value: "hello", want: "hello"},
		{name: "数字转换为文本", value: int64(42), want: "42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatResultCell(tt.value); got != tt.want {
				t.Fatalf("formatResultCell(%v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestFormatResultCellTruncatesByRune(t *testing.T) {
	input := strings.Repeat("界", 130)
	got := formatResultCell(input)
	if wantRunes := 120; len([]rune(got)) != wantRunes {
		t.Fatalf("formatResultCell rune count = %d, want %d", len([]rune(got)), wantRunes)
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("truncated value should end with ellipsis, got %q", got)
	}
}

func TestFormatResultRowUsesQuotedTSV(t *testing.T) {
	row := []interface{}{"普通文本", "包含\t制表符", "包含\n换行", nil, "包含\"引号\""}
	want := "普通文本\t\"包含\t制表符\"\t\"包含\n换行\"\tNULL\t\"包含\"\"引号\"\"\""
	if got := formatResultRow(row); got != want {
		t.Fatalf("formatResultRow() = %q，期望 %q", got, want)
	}
}

func TestResultColumnWidth(t *testing.T) {
	tests := []struct {
		name string
		weight float32
		want float32
	}{
		{name: "低于最小权重", weight: 5, want: 80},
		{name: "正常权重", weight: 20, want: 160},
		{name: "高于最大权重", weight: 40, want: 256},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := float32(resultColumnWidth(tt.weight)); got != tt.want {
				t.Fatalf("resultColumnWidth(%v) = %v dp，期望 %v dp", tt.weight, got, tt.want)
			}
		})
	}
}
