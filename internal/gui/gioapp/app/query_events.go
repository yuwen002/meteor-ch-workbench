package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	queryapp "github.com/yuwen002/meteor-ch-workbench/internal/application/query"
	"github.com/yuwen002/meteor-ch-workbench/internal/domain/queryhistory"
	workspaceui "github.com/yuwen002/meteor-ch-workbench/internal/gui/gioapp/workspace"
)

// queryResultQueueCapacity 限制 GUI 尚未消费的结果事件数量。
// 每个批次最多 128 行，因此容量为 8 时最多缓存约 1024 行结果。
const queryResultQueueCapacity = 8

// queryEvent 是 SQL 执行器向 Gio 工作区投递的不可变结果消息。
type queryEvent struct {
	seq     uint64
	tab     workspaceui.TabID
	columns []queryapp.Column
	batch   [][]any
	summary queryapp.Summary
	sql string
	connectionID string
	err     error
	done    bool
}

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

func queryResultStatus(result queryEvent, columns []queryapp.Column) string {
	if result.err != nil {
		return "执行失败：" + result.err.Error()
	}
	if result.summary.Truncated {
		return fmt.Sprintf("已返回 %d 行（达到结果上限）· %s", result.summary.RowCount, result.summary.Elapsed.Round(time.Millisecond))
	}
	if len(columns) > 0 {
		return fmt.Sprintf("查询完成 · %d 行 · %s", result.summary.RowCount, result.summary.Elapsed.Round(time.Millisecond))
	}
	return fmt.Sprintf("执行完成 · 影响 %d 行 · %s", result.summary.RowsAffected, result.summary.Elapsed.Round(time.Millisecond))
}
func (s *workspace) executeQuery() {
	tab := s.sqlWorkspace.ActiveID()
	activeState := s.sqlTabStates[tab]
	if activeState.ConnectionID != "" && activeState.ConnectionID != s.selectedID {
		if s.selectedID == "" {
			s.queryStatus = "当前标签页关联的连接不可用，请重新选择连接"
		} else {
			s.queryStatus = "当前标签页关联连接与已选连接不一致，请切回关联连接或新建查询标签页"
		}
		return
	}
	if !s.connected || s.selectedID == "" {
		s.queryStatus = "请先连接 ClickHouse"
		return
	}
	if activeState.ConnectionID == "" {
		activeState.ConnectionID = s.selectedID
		activeState.ConnectionName = s.selectedName
		s.sqlTabStates[tab] = activeState
	}
	sqlText := strings.TrimSpace(s.queryEditor.Text())
	if sqlText == "" {
		s.queryStatus = "请输入 SQL 语句"
		return
	}
	s.sqlWorkspace.UpdateSQL(s.sqlWorkspace.ActiveID(), s.queryEditor.Text())
	if s.queryCancelFunc != nil {
		s.queryCancelFunc()
	}
	connectionID := activeState.ConnectionID
	seq := s.queryExecution.Start(tab)
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	s.queryCancelFunc = cancel
	s.queryRunning = true
	s.queryColumns, s.queryData = nil, nil
	// 新查询开始后，释放当前标签缓存的上一轮结果，避免旧切片继续占用内存。
	activeState = s.sqlTabStates[tab]
	activeState.Columns, activeState.Data = nil, nil
	activeState.Status = "正在执行 SQL…"
	s.sqlTabStates[tab] = activeState
	s.clearQueryCellSelection()
	s.resetResultWidgets(nil)
	s.queryStatus = "正在执行 SQL…"
	request := queryapp.Request{ConnectionID: connectionID, SQL: sqlText, Mode: classifySQLMode(sqlText), MaxRows: 10000, BatchSize: 128, Timeout: 30 * time.Second}
	go func() {
		defer cancel()
		summary, err := s.services.Queries.Execute(ctx, request,
			func(columns []queryapp.Column) error {
				return s.publishQueryEvent(ctx, queryEvent{seq: seq, tab: tab, columns: columns})
			},
			func(rows [][]any) error {
				batch := make([][]any, len(rows))
				for i := range rows {
					batch[i] = append([]any(nil), rows[i]...)
				}
				return s.publishQueryEvent(ctx, queryEvent{seq: seq, tab: tab, batch: batch})
			},
		)
		_ = s.publishQueryCompletion(queryEvent{seq: seq, tab: tab, summary: summary, sql: sqlText, connectionID: connectionID, err: err, done: true})
	}()
}

// cancelActiveQuery 在离开当前 SQL 标签页时取消仍在执行的请求，并使其后续事件失效。
func (s *workspace) publishQueryCompletion(event queryEvent) error {
	if err := enqueueQueryCompletion(s.queryResults, event); err != nil {
		return err
	}
	s.window.Invalidate()
	return nil
}

func (s *workspace) cancelActiveQuery(reason string) {
	if !s.queryRunning {
		return
	}
	if s.queryCancelFunc != nil {
		s.queryCancelFunc()
		s.queryCancelFunc = nil
	}
	s.queryExecution.Cancel()
	s.queryRunning = false
	s.queryStatus = reason
}

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
