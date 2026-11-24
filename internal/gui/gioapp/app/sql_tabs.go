// Package app 中 SQL 标签页的状态与生命周期操作。
package app

import (
	queryapp "github.com/yuwen002/meteor-ch-workbench/internal/application/query"
	"gioui.org/widget"
)

type sqlTabState struct {
	Text    string
	Columns []queryapp.Column
	Data    [][]any
	Status  string
}

func (s *workspace) closeSQLTab(index int) {
	if index < 0 || index >= len(s.sqlTabs) {
		return
	}
	if len(s.sqlTabClicks) < len(s.sqlTabs) {
		for len(s.sqlTabClicks) < len(s.sqlTabs) {
			s.sqlTabClicks = append(s.sqlTabClicks, new(widget.Clickable))
		}
	}
	if len(s.sqlTabCloseClicks) < len(s.sqlTabs) {
		for len(s.sqlTabCloseClicks) < len(s.sqlTabs) {
			s.sqlTabCloseClicks = append(s.sqlTabCloseClicks, new(widget.Clickable))
		}
	}

	if index == s.activeSQLTab {
		s.cancelActiveQuery("查询已取消（关闭标签页）")
		current := s.sqlTabStates[index]
		current.Text = s.queryEditor.Text()
		current.Columns, current.Data, current.Status = s.queryColumns, s.queryData, s.queryStatus
		s.sqlTabStates[index] = current
		s.sqlTabTexts[index] = current.Text
	}

	if len(s.sqlTabs) == 1 {
		s.sqlTabs[0] = "查询 1"
		s.sqlTabTexts = map[int]string{0: ""}
		s.sqlTabStates = map[int]sqlTabState{0: {Status: "已重置查询标签页"}}
		s.activeSQLTab = 0
		s.queryEditor.SetText("")
		s.queryColumns, s.queryData = nil, nil
		s.queryStatus = "已重置查询标签页"
		s.clearQueryCellSelection()
		return
	}

	oldActive := s.activeSQLTab
	s.sqlTabs = append(s.sqlTabs[:index], s.sqlTabs[index+1:]...)
	s.sqlTabClicks = append(s.sqlTabClicks[:index], s.sqlTabClicks[index+1:]...)
	s.sqlTabCloseClicks = append(s.sqlTabCloseClicks[:index], s.sqlTabCloseClicks[index+1:]...)

	remap := func(source map[int]sqlTabState) map[int]sqlTabState {
		result := make(map[int]sqlTabState, len(source))
		for oldIndex, state := range source {
			if oldIndex == index {
				continue
			}
			newIndex := oldIndex
			if oldIndex > index {
				newIndex--
			}
			result[newIndex] = state
		}
		return result
	}
	s.sqlTabStates = remap(s.sqlTabStates)
	texts := make(map[int]string, len(s.sqlTabTexts))
	for oldIndex, value := range s.sqlTabTexts {
		if oldIndex == index {
			continue
		}
		newIndex := oldIndex
		if oldIndex > index {
			newIndex--
		}
		texts[newIndex] = value
	}
	s.sqlTabTexts = texts

	if oldActive > index {
		s.activeSQLTab = oldActive - 1
	} else if oldActive == index {
		s.activeSQLTab = index
		if s.activeSQLTab >= len(s.sqlTabs) {
			s.activeSQLTab = len(s.sqlTabs) - 1
		}
	}
	if oldActive == index {
		next := s.sqlTabStates[s.activeSQLTab]
		s.queryEditor.SetText(next.Text)
		s.queryColumns, s.queryData, s.queryStatus = next.Columns, next.Data, next.Status
		s.clearQueryCellSelection()
	}
}

