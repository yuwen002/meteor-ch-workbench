package app

import (
	"context"
	"time"
)

// queryResultQueueCapacity 限制 GUI 尚未消费的结果事件数量。
// 每个批次最多 128 行，因此容量为 8 时最多缓存约 1024 行结果。
const queryResultQueueCapacity = 8

// enqueueQueryEvent 在队列有空间时写入事件；队列满时等待消费者，取消后立即返回。
func enqueueQueryEvent(ctx context.Context, queue chan<- queryEvent, event queryEvent) error {
	select {
	case queue <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// publishQueryEvent 入队后立即请求下一帧，确保背压状态下 GUI 能持续消费队列。
func (s *workspace) publishQueryEvent(ctx context.Context, event queryEvent) error {
	if err := enqueueQueryEvent(ctx, s.queryResults, event); err != nil {
		return err
	}
	s.window.Invalidate()
	return nil
}

// enqueueQueryCompletion 使用独立的短超时投递终态事件，避免查询上下文超时后无法通知 GUI。
func enqueueQueryCompletion(queue chan<- queryEvent, event queryEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return enqueueQueryEvent(ctx, queue, event)
}
