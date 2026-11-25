package app

import (
	"errors"
	"testing"
	"time"

	"github.com/yuwen002/meteor-ch-workbench/internal/application/query"
)

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

func TestSQLTabStateRecordsFailure(t *testing.T) {
	state := sqlTabState{}
	state.Apply(queryEvent{done: true, err: errors.New("connection lost")})
	if state.Status != "执行失败：connection lost" {
		t.Fatalf("failure status = %q", state.Status)
	}
}
