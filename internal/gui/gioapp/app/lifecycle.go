package app

import "context"

// lifecycleParent 为后台工作提供统一的窗口生命周期父 Context。
// 测试或独立构造 workspace 时允许为空，此时退回 Background。
func (s *workspace) lifecycleParent() context.Context {
	if s != nil && s.lifecycleCtx != nil {
		return s.lifecycleCtx
	}
	return context.Background()
}

// goWorker 注册并启动一个受窗口生命周期管理的后台任务。
// DestroyEvent 会先取消生命周期 Context，再等待所有已注册任务退出。
func (s *workspace) goWorker(task func()) {
	if s == nil || task == nil {
		return
	}
	if s.lifecycleCtx != nil && s.lifecycleCtx.Err() != nil {
		return
	}
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		task()
	}()
}
