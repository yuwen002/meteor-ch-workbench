package gioapp

import (
	"strings"
	"testing"

	"gioui.org/unit"
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

func TestFormatClipboardCellKeepsFullValue(t *testing.T) {
	input := strings.Repeat("完整内容", 40)
	if got := formatClipboardCell(input); got != input {
		t.Fatalf("formatClipboardCell() 截断了原始值，长度 = %d，期望 = %d", len([]rune(got)), len([]rune(input)))
	}
	if got := formatClipboardCell(nil); got != "NULL" {
		t.Fatalf("formatClipboardCell(nil) = %q，期望 NULL", got)
	}
}

func TestFormatResultRowUsesQuotedTSV(t *testing.T) {
	row := []interface{}{"普通文本", "包含\t制表符", "包含\n换行", nil, "包含\"引号\""}
	want := "普通文本\t\"包含\t制表符\"\t\"包含\n换行\"\tNULL\t\"包含\"\"引号\"\"\""
	if got := formatResultRow(row); got != want {
		t.Fatalf("formatResultRow() = %q，期望 %q", got, want)
	}
}

func TestAdjustResultColumnWidth(t *testing.T) {
	tests := []struct {
		name string
		current unit.Dp
		delta float32
		want unit.Dp
	}{
		{name: "正常增加", current: 160, delta: 24, want: 184},
		{name: "正常缩小", current: 160, delta: -24, want: 136},
		{name: "限制最小宽度", current: 90, delta: -30, want: minResultColumnWidth},
		{name: "限制最大宽度", current: 470, delta: 30, want: maxResultColumnWidth},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := adjustResultColumnWidth(tt.current, tt.delta); got != tt.want {
				t.Fatalf("adjustResultColumnWidth(%v, %v) = %v，期望 %v", tt.current, tt.delta, got, tt.want)
			}
		})
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

func TestSplitCellDetailLines(t *testing.T) {
	tests := []struct {
		name string
		value string
		want []string
	}{
		{name: "保留多行文本", value: "第一行\n第二行", want: []string{"第一行", "第二行"}},
		{name: "统一 Windows 换行", value: "第一行\r\n第二行", want: []string{"第一行", "第二行"}},
		{name: "统一旧式换行", value: "第一行\r第二行", want: []string{"第一行", "第二行"}},
		{name: "保留空行", value: "第一行\n\n第三行", want: []string{"第一行", "", "第三行"}},
		{name: "空内容仍可展示", value: "", want: []string{""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitCellDetailLines(tt.value)
			if len(got) != len(tt.want) {
				t.Fatalf("splitCellDetailLines() = %#v，期望 %#v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("splitCellDetailLines()[%d] = %q，期望 %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
