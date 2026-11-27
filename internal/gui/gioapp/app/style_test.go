package app

import "testing"

func TestStatusForegroundMarksFailures(t *testing.T) {
	if got := statusForeground("连接失败：timeout"); got != colorWarning {
		t.Fatalf("failure color = %v", got)
	}
	if got := statusForeground("查询完成 · 12 行"); got != colorTextMuted {
		t.Fatalf("normal color = %v", got)
	}
}


func TestWorkspaceSidebarWidth(t *testing.T) {
	tests := []struct {
		name      string
		available int
		compact   bool
		want      int
	}{
		{name: "宽屏标准宽度", available: 1400, want: 280},
		{name: "窄屏收缩宽度", available: 900, compact: true, want: 220},
		{name: "窗口极窄时不溢出", available: 180, compact: true, want: 180},
		{name: "负可用宽度保护", available: -1, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := workspaceSidebarWidth(tt.available, tt.compact); got != tt.want {
				t.Fatalf("workspaceSidebarWidth(%d, %v) = %d, want %d", tt.available, tt.compact, got, tt.want)
			}
		})
	}
}

func TestConnectionPaneHeight(t *testing.T) {
	tests := []struct {
		name      string
		available int
		compact   bool
		want      int
	}{
		{name: "宽屏连接区", available: 900, want: 220},
		{name: "窄屏连接区", available: 900, compact: true, want: 176},
		{name: "矮窗口为对象树保留空间", available: 300, want: 150},
		{name: "零高度窗口", available: 0, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := connectionPaneHeight(tt.available, tt.compact); got != tt.want {
				t.Fatalf("connectionPaneHeight(%d, %v) = %d, want %d", tt.available, tt.compact, got, tt.want)
			}
		})
	}
}
