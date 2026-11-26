package app

import (
	"testing"
	"time"

	queryapp "github.com/yuwen002/meteor-ch-workbench/internal/application/query"
)

func TestQueryResultStateAppliesColumnsBatchesAndCompletion(t *testing.T) {
	state := queryResultState{
		queryColumns: []queryapp.Column{{Name: "old"}},
		queryData:    [][]any{{"old"}},
		queryStatus:  "旧状态",
	}

	columnsChanged, rowsChanged, finished := state.apply(queryEvent{
		columns: []queryapp.Column{{Name: "value", DatabaseType: "String"}},
	})
	if !columnsChanged || rowsChanged || finished {
		t.Fatalf("column event flags = (%t, %t, %t)", columnsChanged, rowsChanged, finished)
	}
	if len(state.queryData) != 0 || len(state.queryColumns) != 1 || state.queryColumns[0].Name != "value" {
		t.Fatalf("column event did not replace the active result snapshot: %+v", state)
	}

	columnsChanged, rowsChanged, finished = state.apply(queryEvent{batch: [][]any{{"a"}, {"b"}}})
	if columnsChanged || !rowsChanged || finished {
		t.Fatalf("batch event flags = (%t, %t, %t)", columnsChanged, rowsChanged, finished)
	}
	if len(state.queryData) != 2 || state.queryData[1][0] != "b" {
		t.Fatalf("batch rows = %v, want two appended rows", state.queryData)
	}

	columnsChanged, rowsChanged, finished = state.apply(queryEvent{
		done:    true,
		summary: queryapp.Summary{RowCount: 2, Elapsed: 15 * time.Millisecond},
	})
	if columnsChanged || rowsChanged || !finished {
		t.Fatalf("completion event flags = (%t, %t, %t)", columnsChanged, rowsChanged, finished)
	}
	if state.queryStatus == "" || state.queryStatus == "旧状态" {
		t.Fatalf("completion did not update status: %q", state.queryStatus)
	}
}

func TestQueryResultStateCompletionWithoutColumnsPreservesPreviousSnapshot(t *testing.T) {
	state := queryResultState{
		queryColumns: []queryapp.Column{{Name: "existing"}},
		queryData:    [][]any{{"kept"}},
	}
	_, _, finished := state.apply(queryEvent{
		done: true,
		err:  errQueryResultStateTest,
	})
	if !finished {
		t.Fatal("completion should be reported")
	}
	if len(state.queryColumns) != 1 || state.queryColumns[0].Name != "existing" {
		t.Fatalf("columns changed unexpectedly: %v", state.queryColumns)
	}
	if len(state.queryData) != 1 || state.queryData[0][0] != "kept" {
		t.Fatalf("rows changed unexpectedly: %v", state.queryData)
	}
	if state.queryStatus == "" {
		t.Fatal("error completion should set a visible status")
	}
}

var errQueryResultStateTest = testQueryResultError("query failed")

type testQueryResultError string

func (e testQueryResultError) Error() string { return string(e) }
