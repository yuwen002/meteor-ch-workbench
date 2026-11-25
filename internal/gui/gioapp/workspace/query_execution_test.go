package workspace

import "testing"

func TestQueryExecutionRejectsStaleGeneration(t *testing.T) {
	var execution QueryExecution
	first := execution.Start(1)
	second := execution.Start(2)
	if execution.IsCurrent(first, 1) { t.Fatal("previous query must become stale after a new query starts") }
	if !execution.IsCurrent(second, 2) { t.Fatal("latest query should be current") }
	if execution.Finish(first, 1) { t.Fatal("stale query must not finish the latest query") }
	if !execution.IsCurrent(second, 2) { t.Fatal("stale completion must not invalidate the latest query") }
	if !execution.Finish(second, 2) || execution.IsCurrent(second, 2) { t.Fatal("finished query must no longer be current") }
}

func TestQueryExecutionCancelInvalidatesPendingEvents(t *testing.T) {
	var execution QueryExecution
	generation := execution.Start(7)
	execution.Cancel()
	if execution.IsCurrent(generation, 7) { t.Fatal("cancelled query events must be rejected") }
	if execution.Finish(generation, 7) { t.Fatal("cancelled query must not finish") }
	if next := execution.Start(7); next <= generation { t.Fatalf("generation after cancel = %d, want greater than %d", next, generation) }
}

func TestQueryExecutionRejectsWrongTab(t *testing.T) {
	var execution QueryExecution
	generation := execution.Start(3)
	if execution.IsCurrent(generation, 4) { t.Fatal("query event from another tab must be rejected") }
}
