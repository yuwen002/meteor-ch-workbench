package app

import (
	"errors"
	"testing"
	"time"

	"github.com/yuwen002/meteor-ch-workbench/internal/application/query"
	workspaceui "github.com/yuwen002/meteor-ch-workbench/internal/gui/gioapp/workspace"
)

func TestSwitchingToActiveSQLTabDoesNotCancelQuery(t *testing.T) {
	tabs := workspaceui.NewTabs()
	activeID := tabs.ActiveID()
	state := &workspace{
		sqlWorkspace: tabs,
		queryRunning: true,
		queryStatus: "正在执行 SQL…",
	}
	state.switchSQLTab(activeID)
	if !state.queryRunning {
		t.Fatal("clicking the active tab should not cancel its running query")
	}
	if state.queryStatus != "正在执行 SQL…" {
		t.Fatalf("query status = %q, want running status preserved", state.queryStatus)
	}
}

func TestSQLTabStateAppliesQueryBatchesAndCompletion(t *testing.T) {
	state := sqlTabState{Data: [][]any{{"old"}}}
	state.Apply(queryEvent{columns: []query.Column{{}}, batch: [][]any{{int64(1)}, {int64(2)}}})
	if len(state.Columns) != 1 || len(state.Data) != 2 {
		t.Fatalf("after first batch: columns=%d rows=%d", len(state.Columns), len(state.Data))
	}
	state.Apply(queryEvent{batch: [][]any{{int64(3)}}})
	if len(state.Data) != 3 {
		t.Fatalf("rows after second batch = %d, want 3", len(state.Data))
	}
	state.Apply(queryEvent{done: true, summary: query.Summary{RowCount: 3, Elapsed: 10 * time.Millisecond}})
	if state.Status == "" {
		t.Fatal("completed query should expose a status")
	}
}

func TestSQLTabStateNewColumnsResetPreviousRows(t *testing.T) {
	state := sqlTabState{Columns: []query.Column{{}}, Data: [][]any{{"previous"}}}
	state.Apply(queryEvent{columns: []query.Column{{}, {}}, batch: [][]any{{"new"}}})
	if len(state.Columns) != 2 || len(state.Data) != 1 || state.Data[0][0] != "new" {
		t.Fatalf("new result did not replace previous data: columns=%d data=%v", len(state.Columns), state.Data)
	}
}

func TestSQLTabStateApplyPreservesConnectionContext(t *testing.T) {
	state := sqlTabState{
		ConnectionID:   "connection-a",
		ConnectionName: "生产集群",
	}
	state.Apply(queryEvent{
		columns: []query.Column{{Name: "value"}},
		batch:   [][]any{{int64(1)}},
		done:    true,
		summary: query.Summary{RowCount: 1},
	})
	if state.ConnectionID != "connection-a" || state.ConnectionName != "生产集群" {
		t.Fatalf("query result should not change tab connection context: id=%q name=%q", state.ConnectionID, state.ConnectionName)
	}
}

func TestSQLTabStateRecordsFailure(t *testing.T) {
	state := sqlTabState{}
	state.Apply(queryEvent{done: true, err: errors.New("connection lost")})
	if state.Status != "执行失败：connection lost" {
		t.Fatalf("failure status = %q", state.Status)
	}
}

func TestCancelActiveQueryInvalidatesPendingEvents(t *testing.T) {
	tabs := workspaceui.NewTabs()
	tabID := tabs.ActiveID()
	var execution workspaceui.QueryExecution
	seq := execution.Start(tabID)
	cancelCalled := false
	state := &workspace{
		queryExecution: execution,
		queryRunning: true,
		queryStatus: "正在执行 SQL…",
		queryCancelFunc: func() { cancelCalled = true },
	}

	state.cancelActiveQuery("查询已取消（切换连接）")

	if !cancelCalled {
		t.Fatal("active query context should be canceled")
	}
	if state.queryRunning {
		t.Fatal("queryRunning should be false after cancellation")
	}
	if state.queryStatus != "查询已取消（切换连接）" {
		t.Fatalf("query status = %q", state.queryStatus)
	}
	if state.queryExecution.IsCurrent(seq, tabID) {
		t.Fatal("pending events from canceled query should be invalidated")
	}
	if state.queryCancelFunc != nil {
		t.Fatal("query cancel function should be cleared")
	}
}
