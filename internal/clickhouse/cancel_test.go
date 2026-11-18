package clickhouse

import (
	"context"
	"testing"
)

func TestCancelQuery(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Service) string
		want  bool
	}{
		{
			name: "空 queryID 直接返回 false",
			setup: func(s *Service) string {
				return ""
			},
			want: false,
		},
		{
			name: "未注册的 queryID 返回 false",
			setup: func(s *Service) string {
				return "not-exist"
			},
			want: false,
		},
		{
			name: "已注册的 queryID 取消成功",
			setup: func(s *Service) string {
				_, _ = s.register("q-1")
				return "q-1"
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewService()
			id := tt.setup(s)
			if got := s.CancelQuery(id); got != tt.want {
				t.Errorf("CancelQuery(%q) = %v, want %v", id, got, tt.want)
			}
		})
	}
}

func TestCancelQueryRemovesEntry(t *testing.T) {
	s := NewService()
	_, unregister := s.register("q-2")
	defer unregister()

	if !s.CancelQuery("q-2") {
		t.Fatal("首次取消应返回 true")
	}
	if s.CancelQuery("q-2") {
		t.Fatal("重复取消应返回 false")
	}

	s.mu.Lock()
	count := len(s.running)
	s.mu.Unlock()
	if count != 0 {
		t.Errorf("取消后 running 应为空，实际剩 %d 条", count)
	}
}

func TestRegisterUnregisterCleansUp(t *testing.T) {
	s := NewService()
	ctx, unregister := s.register("q-3")
	if ctx.Err() != nil {
		t.Fatal("刚注册的 context 不应已取消")
	}
	unregister()

	if ctx.Err() != context.Canceled {
		t.Errorf("注销后 context 应为 Canceled，实际 %v", ctx.Err())
	}

	s.mu.Lock()
	count := len(s.running)
	s.mu.Unlock()
	if count != 0 {
		t.Errorf("注销后 running 应为空，实际剩 %d 条", count)
	}
}

func TestRegisterEmptyIDReturnsBackground(t *testing.T) {
	s := NewService()
	ctx, unregister := s.register("")
	defer unregister()

	if ctx != context.Background() {
		t.Error("空 queryID 应返回 context.Background()")
	}

	s.mu.Lock()
	count := len(s.running)
	s.mu.Unlock()
	if count != 0 {
		t.Errorf("空 queryID 不应登记到 running，实际 %d 条", count)
	}
}

func TestRegisterDuplicateIDCancelsOld(t *testing.T) {
	s := NewService()
	oldCtx, oldUnregister := s.register("q-4")
	defer oldUnregister()

	_, newUnregister := s.register("q-4")
	defer newUnregister()

	if oldCtx.Err() != context.Canceled {
		t.Errorf("重复注册同一 ID 应取消旧 context，实际 %v", oldCtx.Err())
	}

	s.mu.Lock()
	count := len(s.running)
	s.mu.Unlock()
	if count != 1 {
		t.Errorf("重复注册后 running 应只保留 1 条，实际 %d 条", count)
	}
}

func TestIsCanceled(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want bool
	}{
		{
			name: "普通 context 不算取消",
			ctx:  context.Background(),
			want: false,
		},
		{
			name: "主动取消返回 true",
			ctx:  canceledContext(),
			want: true,
		},
		{
			name: "超时不算用户取消",
			ctx:  deadlineContext(),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCanceled(tt.ctx); got != tt.want {
				t.Errorf("isCanceled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func canceledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func deadlineContext() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	cancel()
	return ctx
}

func TestErrQueryCanceledText(t *testing.T) {
	if errQueryCanceled != "查询已取消" {
		t.Errorf("取消提示文案变化，前端依赖此文案判断，实际 %q", errQueryCanceled)
	}
}
