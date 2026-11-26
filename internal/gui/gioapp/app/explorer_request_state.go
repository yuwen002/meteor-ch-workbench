package app

import "context"

// explorerRequestState 统一持有 Explorer 当前请求序号和取消函数。
// 该状态只在 Gio 窗口事件循环中读写；后台 goroutine 只持有各自的 Context。
type explorerRequestState struct {
	sequence uint64
	cancel   context.CancelFunc
}

// Begin 取消上一请求并登记新请求，返回可用于过滤迟到响应的序号。
func (s *explorerRequestState) Begin(cancel context.CancelFunc) uint64 {
	if s.cancel != nil {
		s.cancel()
	}
	s.sequence++
	s.cancel = cancel
	return s.sequence
}

// Cancel 取消当前请求并递增序号，使已经排队的响应立即失效。
func (s *explorerRequestState) Cancel() {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.sequence++
}

// Sequence 返回当前请求序号，供结果消费端验证响应上下文。
func (s *explorerRequestState) Sequence() uint64 {
	return s.sequence
}
