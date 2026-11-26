package app

import (
	"testing"
	"time"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/queryhistory"
)

func TestFilterHistory(t *testing.T) {
	items := []queryhistory.Entry{
		{ID: "1", SQL: "SELECT * FROM events", ConnectionID: "prod-cluster", Timestamp: time.Unix(1, 0)},
		{ID: "2", SQL: "select count() from users", ConnectionID: "test", Timestamp: time.Unix(2, 0)},
		{ID: "3", SQL: "SHOW DATABASES", ConnectionID: "analytics"},
	}

	tests := []struct {
		name    string
		keyword string
		wantIDs []string
	}{
		{name: "empty keyword preserves all entries", wantIDs: []string{"1", "2", "3"}},
		{name: "SQL search ignores case", keyword: "SELECT", wantIDs: []string{"1", "2"}},
		{name: "connection search", keyword: "PROD", wantIDs: []string{"1"}},
		{name: "trims keyword", keyword: "  users  ", wantIDs: []string{"2"}},
		{name: "no match", keyword: "missing", wantIDs: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterHistory(items, tt.keyword)
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("filterHistory() returned %d entries, want %d", len(got), len(tt.wantIDs))
			}
			for i, wantID := range tt.wantIDs {
				if got[i].ID != wantID {
					t.Errorf("filterHistory()[%d].ID = %q, want %q", i, got[i].ID, wantID)
				}
			}
		})
	}
}

func TestFilterHistoryDoesNotMutateInput(t *testing.T) {
	items := []queryhistory.Entry{{ID: "1", SQL: "SELECT 1"}}
	got := filterHistory(items, "")
	if len(got) != 1 {
		t.Fatalf("filterHistory() returned %d entries, want 1", len(got))
	}
	got[0].SQL = "changed"
	if items[0].SQL != "SELECT 1" {
		t.Fatal("filterHistory() returned a slice backed by the input")
	}
}
