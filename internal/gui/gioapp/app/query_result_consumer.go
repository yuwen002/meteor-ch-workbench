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
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, err := s.services.History.Add(ctx, entry)
			select {
			case s.historyResults <- historySaveResult{err: err}:
			case <-ctx.Done():
			}
			s.window.Invalidate()
		}()
	}
	if result.tab != s.sqlWorkspace.ActiveID() {
		state := s.sqlTabStates[result.tab]
		state.Apply(result)
		s.sqlTabStates[result.tab] = state
		return
	}
	if len(result.columns) > 0 {
		s.queryColumns = result.columns
		s.queryData = nil
		s.clearQueryCellSelection()
		s.resetResultWidgets(result.columns)
	}
	if len(result.batch) > 0 {
		s.queryData = append(s.queryData, result.batch...)
		s.resultViewDirty = true
	}
	if result.done {
		s.queryExecution.Finish(result.seq, result.tab)
		s.queryRunning = false
		s.queryCancelFunc = nil
		s.queryStatus = queryResultStatus(result, s.queryColumns)
	}
}
