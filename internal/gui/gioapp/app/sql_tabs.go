// Package app 中 SQL 标签页的 UI 状态同步与生命周期操作。
package app

import (
	"gioui.org/widget"
	"github.com/yuwen002/meteor-ch-workbench/internal/application/query"
)

type sqlTabState struct {
	Columns []query.Column
	Data    [][]any
	Status  string
}

func (s *workspace) closeSQLTab(index int) {
	items := s.sqlWorkspace.Items()
	if index < 0 || index >= len(items) {
		return
	}
	tabID := items[index].ID
	activeID := s.sqlWorkspace.ActiveID()
	if tabID == activeID {
		s.cancelActiveQuery("查询已取消（关闭标签页）")
		current := s.sqlTabStates[tabID]
		s.sqlWorkspace.UpdateSQL(tabID, s.queryEditor.Text())
		current.Columns, current.Data, current.Status = s.queryColumns, s.queryData, s.queryStatus
		s.sqlTabStates[tabID] = current
	}

	if len(items) == 1 {
		s.sqlWorkspace.UpdateSQL(tabID, "")
		s.sqlWorkspace.MarkSaved(tabID)
		s.sqlWorkspace.Rename(tabID, "查询 1")
		s.sqlTabStates[tabID] = sqlTabState{Status: "已重置查询标签页"}
		s.queryEditor.SetText("")
		s.queryColumns, s.queryData = nil, nil
		s.queryStatus = "已重置查询标签页"
		s.clearQueryCellSelection()
		return
	}

	if _, ok := s.sqlWorkspace.Close(tabID); !ok {
		return
	}
	delete(s.sqlTabStates, tabID)
	s.sqlTabClicks = append(s.sqlTabClicks[:index], s.sqlTabClicks[index+1:]...)
	s.sqlTabCloseClicks = append(s.sqlTabCloseClicks[:index], s.sqlTabCloseClicks[index+1:]...)

	if tabID == activeID {
		nextID := s.sqlWorkspace.ActiveID()
		nextTab, _ := s.sqlWorkspace.Get(nextID)
		next := s.sqlTabStates[nextID]
		s.queryEditor.SetText(nextTab.SQL)
		s.queryColumns, s.queryData, s.queryStatus = next.Columns, next.Data, next.Status
		s.clearQueryCellSelection()
	}
}
