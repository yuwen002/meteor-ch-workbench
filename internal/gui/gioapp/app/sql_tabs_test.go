package app

import (
	"errors"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

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

func TestDatabaseNameExists(t *testing.T) {
	if !databaseNameExists([]string{"default", "analytics"}, "analytics") {
		t.Fatal("expected existing database to be found")
	}
	if databaseNameExists([]string{"default", "analytics"}, "missing") {
		t.Fatal("expected missing database not to be found")
	}
}

func TestSQLTabStateApplyPreservesConnectionContext(t *testing.T) {
	state := sqlTabState{
		ConnectionID:   "connection-a",
		ConnectionName: "生产集群",
		Database:       "analytics",
	}
	state.Apply(queryEvent{
		columns: []query.Column{{Name: "value"}},
		batch:   [][]any{{int64(1)}},
		done:    true,
		summary: query.Summary{RowCount: 1},
	})
	if state.ConnectionID != "connection-a" || state.ConnectionName != "生产集群" || state.Database != "analytics" {
		t.Fatalf("query result should not change tab connection context: id=%q name=%q database=%q", state.ConnectionID, state.ConnectionName, state.Database)
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

func TestSQLTabResultViewStateRoundTrip(t *testing.T) {
	columns := []query.Column{{Name: "name"}, {Name: "count"}}
	state := &workspace{
		queryColumns:        columns,
		queryData:           [][]any{{"alpha", int64(2)}},
		resultFilter:        widget.Editor{},
		resultSortColumn:    1,
		resultSortDescending: true,
		columnWidths:        []unit.Dp{140, 220},
		columnFilterEditors: make([]widget.Editor, 2),
	}
	state.resultFilter.SetText("alpha")
	state.columnFilterEditors[0].SetText("alpha")
	state.columnFilterEditors[1].SetText(">1")

	var tab sqlTabState
	state.saveResultViewState(&tab)

	// 修改原始切片，确保归档状态不与工作区共享底层数组。
	state.columnWidths[0] = 300
	state.columnFilterEditors[0].SetText("changed")

	state.queryColumns = columns
	state.resetResultWidgets(columns)
	state.restoreResultViewState(tab)

	if got := state.resultFilter.Text(); got != "alpha" {
		t.Fatalf("global filter = %q, want alpha", got)
	}
	if got := state.columnFilterEditors[0].Text(); got != "alpha" {
		t.Fatalf("first column filter = %q, want alpha", got)
	}
	if got := state.columnFilterEditors[1].Text(); got != ">1" {
		t.Fatalf("second column filter = %q, want >1", got)
	}
	if state.resultSortColumn != 1 || !state.resultSortDescending {
		t.Fatalf("sort = (%d, %t), want (1, true)", state.resultSortColumn, state.resultSortDescending)
	}
	if len(state.columnWidths) != 2 || state.columnWidths[0] != 140 || state.columnWidths[1] != 220 {
		t.Fatalf("column widths = %v, want [140 220]", state.columnWidths)
	}
}

func TestSQLTabResultViewStateIgnoresChangedColumnSchema(t *testing.T) {
	columns := []query.Column{{Name: "new_name"}, {Name: "new_count"}}
	state := &workspace{
		queryColumns: columns,
		queryData:    [][]any{{"beta", int64(3)}},
	}
	state.resetResultWidgets(columns)
	state.restoreResultViewState(sqlTabState{
		FilterText:     "beta",
		ColumnNames:    []string{"old_name", "old_count"},
		ColumnFilters:  []string{"old", ">10"},
		SortColumn:     0,
		SortDescending: true,
		ColumnWidths:   []unit.Dp{280, 300},
	})

	if got := state.resultFilter.Text(); got != "beta" {
		t.Fatalf("global filter = %q, want beta", got)
	}
	if state.columnFilterEditors[0].Text() != "" || state.columnFilterEditors[1].Text() != "" {
		t.Fatal("column filters from a different result schema must not be restored")
	}
	if state.resultSortColumn != -1 {
		t.Fatalf("sort column = %d, want no sort for changed schema", state.resultSortColumn)
	}
	if state.columnWidths[0] == 280 || state.columnWidths[1] == 300 {
		t.Fatalf("column widths from a different result schema must not be restored: %v", state.columnWidths)
	}
}

func TestSQLTabResultNavigationStateRoundTrip(t *testing.T) {
	selection := &queryCellSelection{
		row: 7,
		column: query.Column{Name: "payload", DatabaseType: "String"},
		value: "{\"ok\":true}",
	}
	state := &workspace{
		queryRows: widget.List{List: layout.List{Position: layout.Position{First: 12, Offset: 8}}},
		queryHorizontal: widget.List{List: layout.List{Position: layout.Position{First: 0, Offset: -24}}},
		queryDetailRows: widget.List{List: layout.List{Position: layout.Position{First: 3, Offset: 5}}},
		selectedQueryCell: selection,
	}
	var tab sqlTabState
	state.saveResultViewState(&tab)

	// 后续修改当前工作区，不应污染已保存的标签状态。
	state.queryRows.Position = layout.Position{}
	state.queryHorizontal.Position = layout.Position{}
	state.queryDetailRows.Position = layout.Position{}
	state.selectedQueryCell.value = "changed"

	state.restoreResultViewState(tab)
	if state.queryRows.Position != (layout.Position{First: 12, Offset: 8}) {
		t.Fatalf("result position = %+v", state.queryRows.Position)
	}
	if state.queryHorizontal.Position != (layout.Position{First: 0, Offset: -24}) {
		t.Fatalf("horizontal position = %+v", state.queryHorizontal.Position)
	}
	if state.queryDetailRows.Position != (layout.Position{First: 3, Offset: 5}) {
		t.Fatalf("detail position = %+v", state.queryDetailRows.Position)
	}
	if state.selectedQueryCell == nil || state.selectedQueryCell.value != "{\"ok\":true}" {
		t.Fatalf("selected cell was not restored: %+v", state.selectedQueryCell)
	}
	if state.selectedQueryCell == tab.SelectedCell {
		t.Fatal("selected cell pointer must be copied when restoring")
	}
}

func TestSQLTabResultViewDoesNotRestoreStaleCellDetail(t *testing.T) {
	columns := []query.Column{{Name: "new_payload", DatabaseType: "String"}}
	state := &workspace{queryColumns: columns}
	state.resetResultWidgets(columns)
	state.restoreResultViewState(sqlTabState{
		ColumnNames: []string{"old_payload"},
		SelectedCell: &queryCellSelection{
			row: 2,
			column: query.Column{Name: "old_payload", DatabaseType: "String"},
			value: "stale value",
		},
		DetailPosition: layout.Position{First: 4, Offset: 2},
	})
	if state.selectedQueryCell != nil {
		t.Fatalf("stale selected cell should not be restored: %+v", state.selectedQueryCell)
	}
	if state.queryDetailRows.Position != (layout.Position{}) {
		t.Fatalf("stale detail scroll position should be reset: %+v", state.queryDetailRows.Position)
	}
}

func TestShouldPreserveTabDatabaseWhenRetryingSameConnection(t *testing.T) {
	if !shouldPreserveTabDatabase("connection-a", "connection-a", false) {
		t.Fatal("retrying the same connection should preserve its database context")
	}
	if shouldPreserveTabDatabase("connection-a", "connection-b", false) {
		t.Fatal("switching to another connection should clear the previous database context")
	}
	if !shouldPreserveTabDatabase("connection-a", "connection-b", true) {
		t.Fatal("tab restoration should preserve its saved database context")
	}
}

func TestSQLTabResultViewIgnoresChangedColumnType(t *testing.T) {
	columns := []query.Column{{Name: "payload", DatabaseType: "JSON"}}
	state := &workspace{queryColumns: columns}
	state.resetResultWidgets(columns)
	state.restoreResultViewState(sqlTabState{
		ColumnNames:   []string{"payload"},
		ColumnTypes:   []string{"String"},
		ColumnFilters: []string{"contains"},
		SortColumn:    0,
		ColumnWidths:  []unit.Dp{300},
	})
	if state.columnFilterEditors[0].Text() != "" || state.resultSortColumn != -1 {
		t.Fatal("view settings from a different column type must not be restored")
	}
	if state.columnWidths[0] == 300 {
		t.Fatal("column width from a different column type must not be restored")
	}
}
