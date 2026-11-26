package app

import (
	"context"
	"sync"
)

// lifecycleParent 为后台工作提供统一的窗口生命周期父 Context。
// 测试或独立构造 workspace 时允许为空，此时退回 Background。
func (s *workspace) lifecycleParent() context.Context {
	if s != nil && s.lifecycleCtx != nil {
		return s.lifecycleCtx
	}
	return context.Background()
}

// goWorker 在锁内完成“检查关闭状态 + WaitGroup.Add”，避免 Add 与 Wait 并发竞态。
func (s *workspace) goWorker(task func()) {
	if s == nil || task == nil {
		return
	}
	s.lifecycleMu.Lock()
	if s.lifecycleClosing || (s.lifecycleCtx != nil && s.lifecycleCtx.Err() != nil) {
		s.lifecycleMu.Unlock()
		return
	}
	s.workers.Add(1)
	s.lifecycleMu.Unlock()

	go func() {
		defer s.workers.Done()
		task()
	}()
}

// shutdownWorkers 关闭任务入口、取消生命周期并等待已登记任务全部退出。
// 先关闭入口再 Wait，确保 WaitGroup 不会在计数归零后又收到新的 Add。
func (s *workspace) shutdownWorkers() {
	if s == nil {
		return
	}
	s.lifecycleMu.Lock()
	s.lifecycleClosing = true
	if s.cancelLifecycle != nil {
		s.cancelLifecycle()
	}
	s.lifecycleMu.Unlock()
	s.workers.Wait()
}
