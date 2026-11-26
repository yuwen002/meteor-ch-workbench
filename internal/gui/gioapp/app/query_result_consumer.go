package app

import (
	"context"
	"strings"
	"time"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/queryhistory"
)

// queryEventIsApplicable 丢弃过期请求及已关闭标签页的迟到事件。
func queryEventIsApplicable(current, tabExists bool) bool {
	return current && tabExists
}

// handleQueryResult 负责查询事件消费、历史记录和结果归档；不直接依赖 drainResults 的事件循环。
func (s *workspace) handleQueryResult(result queryEvent) {
	if !queryEventIsApplicable(s.queryExecution.IsCurrent(result.seq, result.tab), func() bool {
		_, exists := s.sqlWorkspace.Get(result.tab)
		return exists
	}()) {
		return
	}
	if result.done && strings.TrimSpace(result.sql) != "" {
		entry := queryhistory.Entry{
			SQL: result.sql,
			ConnectionID: result.connectionID,
			DurationMs: float64(result.summary.Elapsed) / float64(time.Millisecond),
			Success: result.err == nil,
		}
		s.goWorker(func() {
			ctx, cancel := context.WithTimeout(s.lifecycleParent(), 5*time.Second)
			defer cancel()
			_, err := s.services.History.Add(ctx, entry)
			select {
			case s.historyResults <- historySaveResult{err: err}:
			case <-ctx.Done():
			}
			s.window.Invalidate()
		})
	}
	if result.tab != s.sqlWorkspace.ActiveID() {
		state := s.sqlTabStates[result.tab]
		state.Apply(result)
		s.sqlTabStates[result.tab] = state
		return
	}
	columnsChanged, rowsChanged, finished := s.queryResultState.apply(result)
	if columnsChanged {
		s.clearQueryCellSelection()
		s.resetResultWidgets(s.queryColumns)
	}
	if rowsChanged {
		s.resultViewDirty = true
	}
	if finished {
		s.queryExecution.Finish(result.seq, result.tab)
		s.queryRunning = false
		s.queryCancelFunc = nil
	}
}


// drainQueryResults 在单帧预算内消费查询事件；剩余事件留到下一帧继续处理。
// maxQueryEventsPerFrame 限制单帧处理的查询消息数量，避免持续结果流独占 Gio 事件循环。
const maxQueryEventsPerFrame = 4

func (s *workspace) drainQueryResults() {
	for processed := 0; processed < maxQueryEventsPerFrame; processed++ {
		select {
		case result := <-s.queryResults:
			s.handleQueryResult(result)
		default:
			return
		}
	}
}
