package gioapp

import (
	"testing"

	"github.com/yuwen002/meteor-ch-workbench/internal/clickhouse"
	"github.com/yuwen002/meteor-ch-workbench/internal/config"
)

func TestNormalizeTabSnapshot(t *testing.T) {
	tests := []struct {
		name     string
		snapshot config.TabsFile
		wantID   string
		wantSQL  string
	}{
		{name: "空快照创建默认标签", snapshot: config.TabsFile{}, wantID: "query-1", wantSQL: "SELECT 1"},
		{name: "保留有效活动标签", snapshot: config.TabsFile{Tabs: []config.TabState{{ID: "a"}, {ID: "b"}}, ActiveTabID: "b"}, wantID: "b"},
		{name: "无效活动标签回退到首个", snapshot: config.TabsFile{Tabs: []config.TabState{{ID: "a"}, {ID: "b"}}, ActiveTabID: "missing"}, wantID: "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeTabSnapshot(tt.snapshot, "SELECT 1")
			if got.ActiveTabID != tt.wantID {
				t.Fatalf("ActiveTabID = %q, want %q", got.ActiveTabID, tt.wantID)
			}
			index := activeTabIndex(got.Tabs, got.ActiveTabID)
			if index < 0 {
				t.Fatal("活动标签未能定位")
			}
			if got.Tabs[index].SQL != tt.wantSQL {
				t.Fatalf("SQL = %q, want %q", got.Tabs[index].SQL, tt.wantSQL)
			}
		})
	}
}

func TestActiveTabIndex(t *testing.T) {
	tabs := []config.TabState{{ID: "first"}, {ID: "second"}}
	if got := activeTabIndex(tabs, "second"); got != 1 {
		t.Fatalf("activeTabIndex = %d, want 1", got)
	}
	if got := activeTabIndex(tabs, "missing"); got != -1 {
		t.Fatalf("activeTabIndex = %d, want -1", got)
	}
}

func TestNextTabState(t *testing.T) {
	got := nextTabState([]config.TabState{{ID: "first"}, {ID: "second"}}, "third")
	if got.ID != "third" || got.Title != "查询 3" || got.SQL != "" {
		t.Fatalf("unexpected new tab: %#v", got)
	}
}

func TestTabRuntimeStateIsRestoredIndependently(t *testing.T) {
	firstResult := &clickhouse.QueryResult{RowCount: 3}
	secondResult := &clickhouse.QueryResult{RowCount: 8}
	s := &screen{
		activeTabID: "first",
		tabRuntime: map[string]tabRuntimeState{
			"second": {result: secondResult, lastSQL: "SELECT 2", connectionID: "conn-2"},
		},
		queryResult: firstResult,
		lastExecutedSQL: "SELECT 1",
		lastExecutedConnectionID: "conn-1",
	}
	s.saveActiveTabRuntime()
	s.activeTabID = "second"
	s.queryResult = nil
	s.lastExecutedSQL = ""
	s.lastExecutedConnectionID = ""
	s.restoreActiveTabRuntime()
	if s.queryResult != secondResult {
		t.Fatal("切换标签后没有恢复该标签自己的查询结果")
	}
	if s.lastExecutedSQL != "SELECT 2" || s.lastExecutedConnectionID != "conn-2" {
		t.Fatalf("恢复了错误的 SQL 上下文：SQL=%q connection=%q", s.lastExecutedSQL, s.lastExecutedConnectionID)
	}
	if got := s.tabRuntime["first"]; got.result != firstResult || got.lastSQL != "SELECT 1" || got.connectionID != "conn-1" {
		t.Fatalf("原标签状态未独立保存：%#v", got)
	}
}
