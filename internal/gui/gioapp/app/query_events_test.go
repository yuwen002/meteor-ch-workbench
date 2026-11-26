package app

import (
	"context"
	"errors"
	"testing"
	"time"

	queryapp "github.com/yuwen002/meteor-ch-workbench/internal/application/query"
	workspaceui "github.com/yuwen002/meteor-ch-workbench/internal/gui/gioapp/workspace"
)

func TestEnqueueQueryEventAppliesBackpressure(t *testing.T) {
	queue := make(chan queryEvent, 1)
	first := queryEvent{seq: 1}
	second := queryEvent{seq: 2}
	queue <- first

	done := make(chan error, 1)
	go func() {
		done <- enqueueQueryEvent(context.Background(), queue, second)
	}()

	select {
	case err := <-done:
		t.Fatalf("enqueueQueryEvent() returned before queue space was available: %v", err)
	case <-time.After(30 * time.Millisecond):
	}

	gotFirst := <-queue
	if gotFirst.seq != first.seq {
		t.Fatalf("first event seq = %d, want %d", gotFirst.seq, first.seq)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("enqueueQueryEvent() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("enqueueQueryEvent() did not resume after queue space became available")
	}

	gotSecond := <-queue
	if gotSecond.seq != second.seq {
		t.Fatalf("second event seq = %d, want %d", gotSecond.seq, second.seq)
	}
}

func TestEnqueueQueryEventStopsWhenContextIsCanceled(t *testing.T) {
	queue := make(chan queryEvent, 1)
	queue <- queryEvent{seq: 1}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- enqueueQueryEvent(ctx, queue, queryEvent{seq: 2})
	}()

	select {
	case err := <-done:
		t.Fatalf("enqueueQueryEvent() returned before cancellation: %v", err)
	case <-time.After(30 * time.Millisecond):
	}

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("enqueueQueryEvent() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("enqueueQueryEvent() did not stop after context cancellation")
	}
}

func TestQueryResultQueueCapacityBoundsPendingEvents(t *testing.T) {
	if queryResultQueueCapacity != 8 {
		t.Fatalf("queryResultQueueCapacity = %d, want 8", queryResultQueueCapacity)
	}
	queue := make(chan queryEvent, queryResultQueueCapacity)
	for i := 0; i < queryResultQueueCapacity; i++ {
		if err := enqueueQueryEvent(context.Background(), queue, queryEvent{seq: uint64(i + 1)}); err != nil {
			t.Fatalf("enqueue event %d: %v", i, err)
		}
	}
	if len(queue) != queryResultQueueCapacity {
		t.Fatalf("queued events = %d, want %d", len(queue), queryResultQueueCapacity)
	}
}

func TestEnqueueQueryCompletionDeliversTerminalEventAfterQueueUnblocks(t *testing.T) {
	queue := make(chan queryEvent, 1)
	queue <- queryEvent{seq: 1}
	go func() {
		time.Sleep(10 * time.Millisecond)
		<-queue
	}()

	want := queryEvent{seq: 2, done: true}
	if err := enqueueQueryCompletion(queue, want); err != nil {
		t.Fatalf("enqueue terminal event: %v", err)
	}
	got := <-queue
	if got.seq != want.seq || !got.done {
		t.Fatalf("terminal event = %+v, want %+v", got, want)
	}
}

func TestQueryEventIsApplicableRejectsStaleOrClosedTab(t *testing.T) {
	tests := []struct {
		name      string
		current   bool
		tabExists bool
		want      bool
	}{
		{name: "current query and existing tab", current: true, tabExists: true, want: true},
		{name: "stale query", current: false, tabExists: true, want: false},
		{name: "closed tab", current: true, tabExists: false, want: false},
		{name: "stale query and closed tab", current: false, tabExists: false, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := queryEventIsApplicable(tc.current, tc.tabExists); got != tc.want {
				t.Fatalf("queryEventIsApplicable(%t, %t) = %t, want %t", tc.current, tc.tabExists, got, tc.want)
			}
		})
	}
}

func TestHandleQueryResultIgnoresEventForClosedTab(t *testing.T) {
	tabs := workspaceui.NewTabs()
	closed := tabs.ActiveID()
	active := tabs.Create().ID
	if _, ok := tabs.Close(closed); !ok {
		t.Fatal("expected the original tab to close")
	}
	var execution workspaceui.QueryExecution
	seq := execution.Start(closed)
	state := &workspace{
		sqlWorkspace: tabs,
		sqlTabStates: map[workspaceui.TabID]sqlTabState{
			closed: {Status: "关闭前状态", Data: [][]any{{"keep"}}},
		},
		queryExecution: execution,
		queryStatus: "当前标签状态",
		queryColumns: []queryapp.Column{{Name: "keep"}},
		queryData: [][]any{{"active"}},
	}
	state.handleQueryResult(queryEvent{
		seq: seq, tab: closed, done: true, sql: "SELECT 1",
		columns: []queryapp.Column{{Name: "stale"}},
		batch: [][]any{{"stale"}},
	})

	if tabs.ActiveID() != active {
		t.Fatalf("active tab = %d, want %d", tabs.ActiveID(), active)
	}
	if state.queryStatus != "当前标签状态" || len(state.queryColumns) != 1 || state.queryColumns[0].Name != "keep" {
		t.Fatalf("closed-tab event changed active result state: status=%q columns=%v", state.queryStatus, state.queryColumns)
	}
	if got := state.sqlTabStates[closed].Status; got != "关闭前状态" {
		t.Fatalf("closed tab status = %q, want unchanged", got)
	}
	if state.queryExecution.IsCurrent(seq, closed) != true {
		t.Fatal("test setup should retain the generation; tab existence alone must reject the event")
	}
}

func TestHandleQueryResultIgnoresSupersededGeneration(t *testing.T) {
	tabs := workspaceui.NewTabs()
	tabID := tabs.ActiveID()
	var execution workspaceui.QueryExecution
	oldSeq := execution.Start(tabID)
	currentSeq := execution.Start(tabID)
	state := &workspace{
		sqlWorkspace: tabs,
		sqlTabStates: map[workspaceui.TabID]sqlTabState{
			tabID: {Status: "当前标签状态"},
		},
		queryExecution: execution,
		queryStatus: "当前查询状态",
		queryColumns: []queryapp.Column{{Name: "current"}},
		queryData: [][]any{{"current"}},
	}
	state.handleQueryResult(queryEvent{
		seq: oldSeq, tab: tabID, done: true, sql: "SELECT 'old'",
		columns: []queryapp.Column{{Name: "old"}},
		batch: [][]any{{"old"}},
	})

	if !state.queryExecution.IsCurrent(currentSeq, tabID) {
		t.Fatal("stale completion must not finish the newer query")
	}
	if state.queryStatus != "当前查询状态" || len(state.queryColumns) != 1 || state.queryColumns[0].Name != "current" {
		t.Fatalf("stale event changed current query state: status=%q columns=%v", state.queryStatus, state.queryColumns)
	}
	if got := state.sqlTabStates[tabID].Status; got != "当前标签状态" {
		t.Fatalf("tab status = %q, want unchanged", got)
	}
}

