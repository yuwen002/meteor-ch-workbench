// Package app 中 SQL 标签页的 UI 状态同步与生命周期操作。
package app

import (
	"gioui.org/unit"

	"github.com/yuwen002/meteor-ch-workbench/internal/application/query"
)

type sqlTabState struct {
	ConnectionID   string
	ConnectionName string
	Database       string
	Columns        []query.Column
	Data           [][]any
	Status         string
	FilterText     string
	ColumnFilters  []string
	ColumnNames    []string
	SortColumn     int
	SortDescending bool
	ColumnWidths   []unit.Dp
}

// saveResultViewState 将结果筛选、排序和列宽归档到指定标签。
func (s *workspace) saveResultViewState(state *sqlTabState) {
	if state == nil {
		return
	}
	state.FilterText = s.resultFilter.Text()
	state.ColumnFilters = make([]string, len(s.columnFilterEditors))
	state.ColumnNames = make([]string, len(s.queryColumns))
	for i := range s.queryColumns {
		state.ColumnNames[i] = s.queryColumns[i].Name
	}
	for i := range s.columnFilterEditors {
		state.ColumnFilters[i] = s.columnFilterEditors[i].Text()
	}
	state.SortColumn = s.resultSortColumn
	state.SortDescending = s.resultSortDescending
	state.ColumnWidths = append([]unit.Dp(nil), s.columnWidths...)
}

// restoreResultViewState 恢复指定标签的结果视图设置；列数不匹配时安全使用默认列宽。
func (s *workspace) restoreResultViewState(state sqlTabState) {
	s.resultFilter.SetText(state.FilterText)
	s.resultFilterLast = state.FilterText
	sameColumns := len(state.ColumnNames) == len(s.queryColumns)
	if sameColumns {
		for i := range s.queryColumns {
			if state.ColumnNames[i] != s.queryColumns[i].Name {
				sameColumns = false
				break
			}
		}
	}
	if sameColumns && len(state.ColumnFilters) == len(s.columnFilterEditors) {
		for i := range s.columnFilterEditors {
			s.columnFilterEditors[i].SetText(state.ColumnFilters[i])
		}
	}
	if sameColumns && state.SortColumn >= 0 && state.SortColumn < len(s.queryColumns) {
		s.resultSortColumn = state.SortColumn
		s.resultSortDescending = state.SortDescending
	}
	if sameColumns && len(state.ColumnWidths) == len(s.queryColumns) {
		s.columnWidths = append([]unit.Dp(nil), state.ColumnWidths...)
	}
	s.resultViewDirty = true
}

// Apply 将一个查询事件归档到所属标签页，不依赖当前活动标签。
func (s *sqlTabState) Apply(result queryEvent) {
	if s == nil {
		return
	}
	if len(result.columns) > 0 {
		s.Columns = append([]query.Column(nil), result.columns...)
		s.Data = nil
	}
	if len(result.batch) > 0 {
		s.Data = append(s.Data, result.batch...)
	}
	if result.done {
		s.Status = queryResultStatus(result, s.Columns)
	}
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
		s.saveResultViewState(&current)
		s.sqlTabStates[tabID] = current
	}

	if len(items) == 1 {
		s.sqlWorkspace.UpdateSQL(tabID, "")
		s.sqlWorkspace.MarkSaved(tabID)
		s.sqlWorkspace.Rename(tabID, "查询 1")
		s.sqlTabStates[tabID] = sqlTabState{Status: "已重置查询标签页", SortColumn: -1}
		s.queryEditor.SetText("")
		s.queryColumns, s.queryData = nil, nil
		s.queryStatus = "已重置查询标签页"
		s.clearQueryCellSelection()
		s.resetResultWidgets(nil)
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
		s.queryEditor.SetText(nextTab.Editor.Text())
		s.queryColumns, s.queryData, s.queryStatus = next.Columns, next.Data, next.Status
		s.clearQueryCellSelection()
		s.resetResultWidgets(s.queryColumns)
		s.restoreResultViewState(next)
	}
}
