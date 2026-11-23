package gioapp

import (
	"strings"
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

func TestCloseTabState(t *testing.T) {
	tests := []struct {
		name string
		tabs []config.TabState
		active, close string
		wantIDs []string
		wantActive string
		wantOK bool
	}{
		{name: "关闭非活动标签", tabs: []config.TabState{{ID:"a"},{ID:"b"},{ID:"c"}}, active:"b", close:"a", wantIDs:[]string{"b","c"}, wantActive:"b", wantOK:true},
		{name: "关闭活动标签选择右侧", tabs: []config.TabState{{ID:"a"},{ID:"b"},{ID:"c"}}, active:"b", close:"b", wantIDs:[]string{"a","c"}, wantActive:"c", wantOK:true},
		{name: "关闭末尾活动标签选择左侧", tabs: []config.TabState{{ID:"a"},{ID:"b"}}, active:"b", close:"b", wantIDs:[]string{"a"}, wantActive:"a", wantOK:true},
		{name: "最后一个标签替换为空白标签", tabs: []config.TabState{{ID:"a"}}, active:"a", close:"a", wantIDs:[]string{"fresh"}, wantActive:"fresh", wantOK:true},
		{name: "不存在的标签不修改", tabs: []config.TabState{{ID:"a"}}, active:"a", close:"missing", wantIDs:[]string{"a"}, wantActive:"a", wantOK:false},
	}
	for _, tt := range tests { t.Run(tt.name, func(t *testing.T) {
		got, active, ok := closeTabState(tt.tabs, tt.active, tt.close, "fresh")
		if ok != tt.wantOK || active != tt.wantActive || len(got) != len(tt.wantIDs) { t.Fatalf("got=%#v active=%q ok=%v", got, active, ok) }
		for i, id := range tt.wantIDs { if got[i].ID != id { t.Fatalf("tabs[%d].ID=%q want %q", i, got[i].ID, id) } }
	}) }
}

func TestRenameTabState(t *testing.T) {
	tabs := []config.TabState{{ID:"a",Title:"旧标题"},{ID:"b",Title:"查询 2"}}
	got, ok := renameTabState(tabs, "a", "  业务查询  ")
	if !ok || got[0].Title != "业务查询" || got[1].Title != "查询 2" { t.Fatalf("unexpected rename result: %#v, ok=%v", got, ok) }
	if _, ok := renameTabState(tabs, "a", "   "); ok { t.Fatal("空标题不应允许保存") }
	if _, ok := renameTabState(tabs, "missing", "新标题"); ok { t.Fatal("不存在的标签不应允许重命名") }
}

func TestReorderTabState(t *testing.T) {
	tabs := []config.TabState{{ID:"a"},{ID:"b"},{ID:"c"},{ID:"d"}}
	got, ok := reorderTabState(tabs, 0, 2)
	if !ok || got[0].ID != "b" || got[1].ID != "c" || got[2].ID != "a" || got[3].ID != "d" { t.Fatalf("unexpected reorder: %#v", got) }
	got, ok = reorderTabState(got, 3, 1)
	if !ok || got[0].ID != "b" || got[1].ID != "d" || got[2].ID != "c" || got[3].ID != "a" { t.Fatalf("unexpected reverse reorder: %#v", got) }
	if _, ok := reorderTabState(tabs, -1, 0); ok { t.Fatal("越界索引不应成功") }
}

func TestTabHasUnsavedChanges(t *testing.T) {
	tabs := []config.TabState{{ID:"active", SQL:"SELECT 1"},{ID:"other", SQL:"SELECT 2"}}
	if !tabHasUnsavedChanges(tabs, "active", "active", "SELECT 3") { t.Fatal("活动标签编辑器内容变化时应提示未保存") }
	if tabHasUnsavedChanges(tabs, "active", "active", "SELECT 1") { t.Fatal("内容未变化时不应提示未保存") }
	if tabHasUnsavedChanges(tabs, "active", "other", "SELECT 3") { t.Fatal("非活动标签不应使用当前编辑器内容判断未保存") }
	if tabHasUnsavedChanges(tabs, "active", "missing", "SELECT 3") { t.Fatal("不存在的标签不应提示未保存") }
}

func TestHistorySQLPreview(t *testing.T) {
	tests := []struct {
		name string
		sql string
		want string
	}{
		{name: "跳过空行", sql: "\n  \n SELECT 1\nFROM system.one", want: "SELECT 1"},
		{name: "空 SQL", sql: " \n\t", want: "（空 SQL）"},
		{name: "截断超长首行", sql: strings.Repeat("中", 100), want: strings.Repeat("中", 96) + "…"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := historySQLPreview(tt.sql); got != tt.want { t.Fatalf("historySQLPreview() = %q, want %q", got, tt.want) }
		})
	}
}

func TestSavedSQLMatches(t *testing.T) {
	entry := config.SavedSQL{Name: "日常巡检", SQL: "SELECT count() FROM system.tables", ConnectionID: "conn-a"}
	tests := []struct {
		name string
		keyword string
		connectionID string
		connectionName string
		onlyCurrent bool
		want bool
	}{
		{name: "空关键字匹配", want: true},
		{name: "名称不区分大小写", keyword: "巡检", want: true},
		{name: "SQL 内容搜索", keyword: "SYSTEM.TABLES", want: true},
		{name: "连接名称搜索", keyword: "生产库", connectionName: "生产库", want: true},
		{name: "当前连接匹配", connectionID: "conn-a", onlyCurrent: true, want: true},
		{name: "其他连接排除", connectionID: "conn-b", onlyCurrent: true, want: false},
		{name: "未选连接不匹配当前连接筛选", onlyCurrent: true, want: false},
		{name: "无关关键字排除", keyword: "不存在", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := savedSQLMatches(entry, tt.keyword, tt.connectionID, tt.connectionName, tt.onlyCurrent); got != tt.want {
				t.Fatalf("savedSQLMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}
