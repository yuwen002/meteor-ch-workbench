package gioapp

import (
	"fmt"
	"strings"

	"github.com/yuwen002/meteor-ch-workbench/internal/config"
)

// normalizeTabSnapshot 确保恢复的工作区至少包含一个可用标签，并修正无效活动标签。
func normalizeTabSnapshot(snapshot config.TabsFile, fallbackSQL string) config.TabsFile {
	if len(snapshot.Tabs) > config.TabMaxCount {
		snapshot.Tabs = snapshot.Tabs[:config.TabMaxCount]
	}
	if len(snapshot.Tabs) == 0 {
		snapshot.Tabs = []config.TabState{{ID: "query-1", Title: "查询 1", SQL: fallbackSQL}}
	}
	if activeTabIndex(snapshot.Tabs, snapshot.ActiveTabID) < 0 {
		snapshot.ActiveTabID = snapshot.Tabs[0].ID
	}
	return snapshot
}

// activeTabIndex 返回指定标签在快照中的位置；不存在时返回 -1。
func activeTabIndex(tabs []config.TabState, id string) int {
	for i := range tabs {
		if tabs[i].ID == id {
			return i
		}
	}
	return -1
}

// nextTabState 创建一个新的空白查询标签。
func nextTabState(tabs []config.TabState, id string) config.TabState {
	return config.TabState{
		ID:    id,
		Title: fmt.Sprintf("查询 %d", len(tabs)+1),
		SQL:   "",
	}
}

 
// closeTabState 关闭指定标签，并保证工作区至少保留一个标签。
func closeTabState(tabs []config.TabState, activeID, closeID, fallbackID string) ([]config.TabState, string, bool) {
	index := activeTabIndex(tabs, closeID)
	if index < 0 { return tabs, activeID, false }
	if len(tabs) == 1 {
		return []config.TabState{{ID: fallbackID, Title: "查询 1"}}, fallbackID, true
	}
	updated := make([]config.TabState, 0, len(tabs)-1)
	updated = append(updated, tabs[:index]...)
	updated = append(updated, tabs[index+1:]...)
	if activeID != closeID { return updated, activeID, true }
	next := index
	if next >= len(updated) { next = len(updated)-1 }
	return updated, updated[next].ID, true
}

// renameTabState 更新标签标题；标题去除首尾空白且不能为空。
func renameTabState(tabs []config.TabState, id, title string) ([]config.TabState, bool) {
	title = strings.TrimSpace(title)
	if title == "" { return tabs, false }
	index := activeTabIndex(tabs, id)
	if index < 0 { return tabs, false }
	updated := append([]config.TabState(nil), tabs...)
	updated[index].Title = title
	return updated, true
}

// reorderTabState 将标签移动到目标索引，并保留活动标签 ID。
func reorderTabState(tabs []config.TabState, from, to int) ([]config.TabState, bool) {
	if from < 0 || from >= len(tabs) || to < 0 || to >= len(tabs) || from == to { return tabs, false }
	updated := append([]config.TabState(nil), tabs...)
	moved := updated[from]
	if from < to {
		copy(updated[from:to], updated[from+1:to+1])
	} else {
		copy(updated[to+1:from+1], updated[to:from])
	}
	updated[to] = moved
	return updated, true
}
