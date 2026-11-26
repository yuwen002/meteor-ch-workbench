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
