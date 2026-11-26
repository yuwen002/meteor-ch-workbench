// Package app 中 SQL 标签页的 UI 状态同步与生命周期操作。
package app

import (
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/yuwen002/meteor-ch-workbench/internal/application/query"
	metadataapp "github.com/yuwen002/meteor-ch-workbench/internal/application/metadata"
	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
	workspaceui "github.com/yuwen002/meteor-ch-workbench/internal/gui/gioapp/workspace"
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
	ColumnTypes    []string
	SortColumn     int
	SortDescending bool
	ColumnWidths   []unit.Dp
	ResultPosition layout.Position
	HorizontalPosition layout.Position
	DetailPosition layout.Position
	SelectedCell *queryCellSelection
}

// saveResultViewState 将结果筛选、排序和列宽归档到指定标签。
func (s *workspace) saveResultViewState(state *sqlTabState) {
	if state == nil {
		return
	}
	state.FilterText = s.resultFilter.Text()
	state.ColumnFilters = make([]string, len(s.columnFilterEditors))
	state.ColumnNames = make([]string, len(s.queryColumns))
	state.ColumnTypes = make([]string, len(s.queryColumns))
	for i := range s.queryColumns {
		state.ColumnNames[i] = s.queryColumns[i].Name
		state.ColumnTypes[i] = s.queryColumns[i].DatabaseType
	}
	for i := range s.columnFilterEditors {
		state.ColumnFilters[i] = s.columnFilterEditors[i].Text()
	}
	state.SortColumn = s.resultSortColumn
	state.SortDescending = s.resultSortDescending
	state.ColumnWidths = append([]unit.Dp(nil), s.columnWidths...)
	state.ResultPosition = s.queryRows.Position
	state.HorizontalPosition = s.queryHorizontal.Position
	state.DetailPosition = s.queryDetailRows.Position
	if s.selectedQueryCell != nil {
		selection := *s.selectedQueryCell
		state.SelectedCell = &selection
	} else {
		state.SelectedCell = nil
	}
}

// restoreResultViewState 恢复指定标签的结果视图设置；列数不匹配时安全使用默认列宽。
func (s *workspace) restoreResultViewState(state sqlTabState) {
	s.resultFilter.SetText(state.FilterText)
	s.resultFilterLast = state.FilterText
	sameColumns := len(state.ColumnNames) == len(s.queryColumns) && len(state.ColumnTypes) == len(s.queryColumns)
	if sameColumns {
		for i := range s.queryColumns {
			if state.ColumnNames[i] != s.queryColumns[i].Name || state.ColumnTypes[i] != s.queryColumns[i].DatabaseType {
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
	s.queryRows.Position = state.ResultPosition
	s.queryHorizontal.Position = state.HorizontalPosition
	s.queryDetailRows.Position = state.DetailPosition
	if sameColumns && state.SelectedCell != nil && resultColumnExists(s.queryColumns, state.SelectedCell.column) {
		selection := *state.SelectedCell
		s.selectedQueryCell = &selection
		s.queryDetailRows.Position = state.DetailPosition
	} else {
		s.selectedQueryCell = nil
		s.queryDetailRows.Position = layout.Position{}
	}
}

func shouldPreserveTabDatabase(currentConnectionID, targetConnectionID string, explicitlyPreserve bool) bool {
	return explicitlyPreserve || (currentConnectionID != "" && currentConnectionID == targetConnectionID)
}

func resultColumnExists(columns []query.Column, target query.Column) bool {
	for _, column := range columns {
		if column.Name == target.Name && column.DatabaseType == target.DatabaseType {
			return true
		}
	}
	return false
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


// databaseForConnectedTab 只从当前活动标签读取其自己的数据库上下文，避免异步连接结果串用其他标签的数据库。
func databaseForConnectedTab(tab sqlTabState, connectionID string, databases []string) (string, bool) {
	if tab.ConnectionID != connectionID || tab.Database == "" {
		return "", false
	}
	if !databaseNameExists(databases, tab.Database) {
		return "", true
	}
	return tab.Database, false
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
		s.resetConnectionContext()
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
		s.restoreSQLTabConnection(nextID)
		next = s.sqlTabStates[nextID]
		s.queryStatus = next.Status
	}
}

func (s *workspace) switchSQLTab(tabID workspaceui.TabID) {
	activeID := s.sqlWorkspace.ActiveID()
	if tabID == activeID {
		return
	}
	s.cancelActiveQuery("查询已取消（切换标签页）")
	current := s.sqlTabStates[activeID]
	s.sqlWorkspace.UpdateSQL(activeID, s.queryEditor.Text())
	current.Columns, current.Data, current.Status = s.queryColumns, s.queryData, s.queryStatus
	s.saveResultViewState(&current)
	s.sqlTabStates[activeID] = current
	s.sqlWorkspace.Activate(tabID)
	next := s.sqlTabStates[tabID]
	nextTab, _ := s.sqlWorkspace.Get(tabID)
	s.restoreSQLTabConnection(tabID)
	next = s.sqlTabStates[tabID]
	s.queryEditor.SetText(nextTab.Editor.Text())
	s.queryColumns, s.queryData, s.queryStatus = next.Columns, next.Data, next.Status
	s.clearQueryCellSelection()
	s.resetResultWidgets(s.queryColumns)
	s.restoreResultViewState(next)
}

// restoreSQLTabConnection 让活动标签页独占当前连接上下文，避免空标签继承其他标签的连接。
func (s *workspace) restoreSQLTabConnection(tabID workspaceui.TabID) {
	next := s.sqlTabStates[tabID]
	if next.ConnectionID == "" {
		s.resetConnectionContext()
		return
	}
	if next.ConnectionID != s.selectedID {
		var target *connection.Connection
		if s.connections != nil {
			for _, item := range s.connections.Snapshot().Items {
				if item.ID == next.ConnectionID {
					copy := item
					target = &copy
					break
				}
			}
		}
		if target == nil {
			s.resetConnectionContext()
			next.Status = "此查询标签页关联的连接已不存在，请重新选择连接"
			s.sqlTabStates[tabID] = next
			return
		}
		s.selectConnectionForTab(*target, true)
		return
	}
	if s.connected {
		if next.Database != "" {
			if next.Database != s.selectedDatabase {
				s.selectDatabase(next.Database)
			}
		} else if s.selectedDatabase != "" {
			s.resetDatabaseContext()
		}
		return
	}
	if s.loading == "" && s.connections != nil {
		for _, item := range s.connections.Snapshot().Items {
			if item.ID == next.ConnectionID {
				s.selectConnectionForTab(item, true)
				break
			}
		}
	}
}

// resetDatabaseContext 取消旧数据库请求并清理当前连接下的数据库对象状态。
func (s *workspace) resetDatabaseContext() {
	if s.cancelRequest != nil {
		s.cancelRequest()
		s.cancelRequest = nil
	}
	s.requestSeq++
	s.selectedDatabase = ""
	s.selectedTable = ""
	s.selectedObject = ""
	s.tables, s.views, s.columns = nil, nil, nil
	s.tableDetails = metadataapp.TableDetails{}
	s.loading = ""
}

// resetConnectionContext 使迟到的连接检测结果失效，并清空当前活动标签的连接上下文。
func (s *workspace) resetConnectionContext() {
	if s.cancelRequest != nil {
		s.cancelRequest()
		s.cancelRequest = nil
	}
	s.requestSeq++
	s.selectedID, s.selectedName = "", ""
	s.connected = false
	s.databases = nil
	s.resetDatabaseContext()
}
