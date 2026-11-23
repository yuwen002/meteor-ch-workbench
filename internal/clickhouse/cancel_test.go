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
				_, _ = s.register("", "q-1")
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
	_, unregister := s.register("", "q-2")
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
	ctx, unregister := s.register("", "q-3")
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
	ctx, unregister := s.register("", "")
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
	oldCtx, oldUnregister := s.register("", "q-4")
	defer oldUnregister()

	_, newUnregister := s.register("", "q-4")
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

func TestCancelByConn(t *testing.T) {
	tests := []struct {
		name     string
		connID   string
		queryIDs []string
		want     int
		wantCtx  bool
	}{
		{
			name:     "空连接 ID 不取消任何查询",
			connID:   "",
			queryIDs: []string{""},
			want:     0,
			wantCtx:  false,
		},
		{
			name:     "同连接多条查询全部取消",
			connID:   "c-1",
			queryIDs: []string{"q-1", "q-2", "q-3"},
			want:     3,
			wantCtx:  true,
		},
		{
			name:     "无在途查询时返回 0",
			connID:   "c-1",
			queryIDs: []string{},
			want:     0,
			wantCtx:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewService()
			ctxs := make([]context.Context, 0, len(tt.queryIDs))
			for _, id := range tt.queryIDs {
				ctx, unregister := s.register(tt.connID, id)
				defer unregister()
				ctxs = append(ctxs, ctx)
			}

			if got := s.cancelByConn(tt.connID); got != tt.want {
				t.Fatalf("cancelByConn(%q) = %d, want %d", tt.connID, got, tt.want)
			}
			for _, ctx := range ctxs {
				if tt.wantCtx && ctx.Err() != context.Canceled {
					t.Errorf("连接内查询应被取消，实际 %v", ctx.Err())
				}
				if !tt.wantCtx && ctx.Err() != nil {
					t.Errorf("不应被取消，实际 %v", ctx.Err())
				}
			}

			s.mu.Lock()
			running := len(s.running)
			owners := len(s.owners)
			s.mu.Unlock()
			if tt.wantCtx && (running != 0 || owners != 0) {
				t.Errorf("取消后 running/owners 应为空，实际 %d/%d", running, owners)
			}
		})
	}
}

func TestCancelByConnOnlySameConnection(t *testing.T) {
	s := NewService()
	ctxA, unregisterA := s.register("c-a", "q-a")
	defer unregisterA()
	ctxB, unregisterB := s.register("c-b", "q-b")
	defer unregisterB()

	if got := s.cancelByConn("c-a"); got != 1 {
		t.Fatalf("cancelByConn(c-a) = %d, want 1", got)
	}
	if ctxA.Err() != context.Canceled {
		t.Errorf("c-a 的查询应被取消，实际 %v", ctxA.Err())
	}
	if ctxB.Err() != nil {
		t.Errorf("c-b 的查询不应受影响，实际 %v", ctxB.Err())
	}

	s.mu.Lock()
	remaining := len(s.running)
	s.mu.Unlock()
	if remaining != 1 {
		t.Errorf("c-b 的在途查询应保留，实际剩 %d 条", remaining)
	}
}

func TestInvalidateCancelsRunningQueries(t *testing.T) {
	s := NewService()
	ctx, unregister := s.register("c-1", "q-1")
	defer unregister()

	s.Invalidate("c-1")
	if ctx.Err() != context.Canceled {
		t.Errorf("Invalidate 应取消该连接的在途查询，实际 %v", ctx.Err())
	}

	s.Invalidate("c-1")

	s.mu.Lock()
	dbs := len(s.dbs)
	running := len(s.running)
	s.mu.Unlock()
	if dbs != 0 || running != 0 {
		t.Errorf("Invalidate 后 dbs/running 应为空，实际 %d/%d", dbs, running)
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