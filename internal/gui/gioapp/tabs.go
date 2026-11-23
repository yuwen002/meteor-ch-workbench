package gioapp

import (
	"fmt"

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
