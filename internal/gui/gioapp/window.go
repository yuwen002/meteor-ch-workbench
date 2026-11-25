// Package gioapp 提供 Meteor CH Workbench 的 Gio 原生界面原型。
//
// 当前阶段已接入共享应用服务，支持真实连接测试、数据库列表以及表 / 视图导航；
// SQL 执行、取消和结果展示已接入共享应用服务。
package gioapp

import (
	stdcontext "context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/gesture"
	"gioui.org/io/clipboard"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	gioexplorer "gioui.org/x/explorer"

	appsvc "github.com/yuwen002/meteor-ch-workbench/internal/app"
	"github.com/yuwen002/meteor-ch-workbench/internal/clickhouse"
	"github.com/yuwen002/meteor-ch-workbench/internal/config"
)

type context = layout.Context
type dimensions = layout.Dimensions

const (
	resultPaneQuery = "query"
	resultPaneTable = "table"
)

// Run 运行 Gio 窗口事件循环。
func Run(window *app.Window, service *appsvc.Service) error {
	state := newScreen(window, service, gioexplorer.NewExplorer(window))
	var ops op.Ops
	for {
		evt := window.Event()
		state.explorer.ListenEvents(evt)
		switch event := evt.(type) {
		case app.DestroyEvent:
			state.persistTabs()
			return event.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, event)
			state.drainExportResults()
			state.layout(gtx)
			event.Frame(gtx.Ops)
		}
	}
}

type connectResult struct {
	connectionID string
	sequence     uint64
	databases    []string
	err          error
}
type queryResult struct {
	tabID    string
	queryID  string
	sequence uint64
	result   *clickhouse.QueryResult
	err      error
}
type tabRuntimeState struct {
	result       *clickhouse.QueryResult
	lastSQL      string
	connectionID string
}
type exportResult struct {
	format             string
	err                error
	canceled           bool
	rows               int64
	bytes              int64
	duration           time.Duration
	cleanupErr         error
	cleanupUnsupported bool
}
type databaseResult struct {
	connectionID       string
	connectionSequence uint64
	database           string
	requestID          uint64
	tables             []clickhouse.TableInfo
	views              []clickhouse.TableInfo
	err                error
}
type schemaResult struct {
	connectionID       string
	connectionSequence uint64
	database           string
	table              string
	requestID          uint64
	columns            []clickhouse.ColumnInfo
	details            *clickhouse.TableDetails
	detailsErr         error
	err                error
}
type databaseNode struct {
	click       widget.Clickable
	expanded    bool
	loading     bool
	loaded      bool
	requestID   uint64
	tables      []clickhouse.TableInfo
	views       []clickhouse.TableInfo
	tableClicks map[string]*widget.Clickable
	viewClicks  map[string]*widget.Clickable
	err         error
}

type screen struct {
	window                        *app.Window
	connectResults                chan connectResult
	databaseResults               chan databaseResult
	schemaResults                 chan schemaResult
	schemaSequence                uint64
	schemaLoading                 bool
	schemaError                   string
	selectedSchemaDatabase        string
	selectedSchemaTable           string
	selectedSchema                []clickhouse.ColumnInfo
	selectedTableDetails          *clickhouse.TableDetails
	tableDetailsError             string
	detailsOpen                   bool
	detailsListToggle             widget.Clickable
	detailsShowDDL                bool
	resultPane                    string
	resultTabQuery                widget.Clickable
	resultTabTable                widget.Clickable
	previewTable                  widget.Clickable
	previewPrev                   widget.Clickable
	previewNext                   widget.Clickable
	previewPageSize               widget.Clickable
	previewPage                   int
	previewPageLimit              int
	previewDatabase               string
	previewTableName              string
	previewMode                   bool
	previewTabID                  string
	previewConnectionID           string
	selectedCellLabel             string
	selectedCellValue             string
	cellDetailLines               []string
	cellDetailList                widget.List
	copyCellDetail                widget.Clickable
	closeCellDetail               widget.Clickable
	detailsToggle                 widget.Clickable
	copyDDL                       widget.Clickable
	schemaList                    widget.List
	detailsList                   widget.List
	ddlLines                      []string
	queryResults                  chan queryResult
	exportResults                 chan exportResult
	exportProgress                chan int64
	explorer                      *gioexplorer.Explorer
	exportCSV                     widget.Clickable
	exportTSV                     widget.Clickable
	exportRunning                 bool
	querySequence                 uint64
	runningQueryID                string
	runningTabID                  string
	tabRuntime                    map[string]tabRuntimeState
	queryRunning                  bool
	queryResult                   *clickhouse.QueryResult
	lastExecutedSQL               string
	lastExecutedConnectionID      string
	resultList                    widget.List
	resultColumnHeader            widget.List
	resultColumnRow               widget.List
	resultColumnWidths            []unit.Dp
	resultColumnDrags             []gesture.Drag
	resultColumnDragLastX         []float32
	resultColumnDragActive        []bool
	resultCellClicks              [][]widget.Clickable
	resultSortClicks              []widget.Clickable
	resultSortColumn              int
	resultSortDescending          bool
	resultFilter                  widget.Editor
	resultFilterLast              string
	resultFilterClear             widget.Clickable
	resultColumnFilterEditors     []widget.Editor
	resultColumnFilterLast        []string
	resultColumnFilterClear       widget.Clickable
	resultColumnFilterClearClicks []widget.Clickable
	resultRowCopyClicks           []widget.Clickable
	connectionSequence            uint64
	databaseSequence              uint64
	databaseNames                 []string
	databaseNodes                 map[string]*databaseNode
	treeConnected                 bool
	service                       *appsvc.Service
	historyOpen                   bool
	savedSQLOpen                  bool
	savedSQLToggle                widget.Clickable
	savedSQLEntries               []config.SavedSQL
	savedSQLList                  widget.List
	savedSQLUseClicks             []widget.Clickable
	savedSQLEditClicks            []widget.Clickable
	savedSQLDeleteClicks          []widget.Clickable
	savedSQLSearch                widget.Editor
	savedSQLNameEditor            widget.Editor
	savedSQLEditor                widget.Editor
	savedSQLSaveCurrent           widget.Clickable
	savedSQLSaveEdit              widget.Clickable
	savedSQLCancelEdit            widget.Clickable
	savedSQLClose                 widget.Clickable
	savedSQLRefresh               widget.Clickable
	savedSQLScopeToggle           widget.Clickable
	savedSQLOnlyCurrentConnection bool
	savedSQLFilterState           string
	savedSQLEditingID             string
	historyToggle                 widget.Clickable
	historyEntries                []config.HistoryEntry
	historyList                   widget.List
	historySelectClicks           []widget.Clickable
	historyRunClicks              []widget.Clickable
	historyDeleteClicks           []widget.Clickable
	historyClear                  widget.Clickable
	historyConfirmClear           widget.Clickable
	historyCancelClear            widget.Clickable
	historyClose                  widget.Clickable
	historyRefresh                widget.Clickable
	historyClearConfirm           bool
	pendingHistorySQL             string
	pendingHistoryConnectionID    string
	connections                   []config.Connection
	connectionClicks              []widget.Clickable
	tabs                          []config.TabState
	activeTabID                   string
	tabClicks                     []widget.Clickable
	tabRenameClicks               []widget.Clickable
	tabCloseClicks                []widget.Clickable
	tabDrags                      []gesture.Drag
	tabDragLastX                  []float32
	tabDragAccum                  []float32
	tabDragActive                 []bool
	tabList                       widget.List
	tabTitleEditor                widget.Editor
	renameTabID                   string
	pendingCloseTabID             string
	confirmCloseTab               widget.Clickable
	cancelCloseTab                widget.Clickable
	tabRenameSave                 widget.Clickable
	tabRenameCancel               widget.Clickable
	newTab                        widget.Clickable
	selectedConnectionID          string
	connectFailed                 bool
	pendingPreviewTable           string
	databaseList                  widget.List
	objectList                    widget.List
	theme                         *material.Theme
	sqlEditor                     widget.Editor
	execute                       widget.Clickable
	connect                       widget.Clickable
	stop                          widget.Clickable
	themeToggle                   widget.Clickable
	sidebarToggle                 widget.Clickable
	editorSmaller                 widget.Clickable
	editorLarger                  widget.Clickable
	editorRatio                   float32
	splitDrag                     gesture.Drag
	lastDragY                     float32
	dragActive                    bool
	workspaceHeight               int
	status                        string
	lightTheme                    bool
	sidebarOpen                   bool
}

func newScreen(window *app.Window, service *appsvc.Service, fileExplorer *gioexplorer.Explorer) *screen {
	s := &screen{
		window:          window,
		explorer:        fileExplorer,
		connectResults:  make(chan connectResult, 4),
		databaseResults: make(chan databaseResult, 16),
		schemaResults:   make(chan schemaResult, 8),
		queryResults:    make(chan queryResult, 4),
		exportResults:   make(chan exportResult, 2),
		exportProgress:  make(chan int64, 1),
		databaseNodes:   make(map[string]*databaseNode),
		tabRuntime:      make(map[string]tabRuntimeState),
		service:         service,
		theme:           material.NewTheme(),
		status:          "尚未连接数据库 · 当前为 Gio 界面原型",
		lightTheme:      false,
		sidebarOpen:     true,
		editorRatio:     0.60,
		resultPane:      resultPaneQuery,
	}
	s.savedSQLSearch.SingleLine = true
	s.resultFilter.SingleLine = true
	s.resultSortColumn = -1
	s.savedSQLNameEditor.SingleLine = true
	if service != nil {
		if settings, err := service.GetSettings(); err == nil {
			s.sidebarOpen = !settings.SidebarCollapsed
			s.editorRatio = clampEditorRatio(float32(settings.EditorRatio))
		} else {
			s.status = "读取工作台设置失败：" + err.Error()
		}
		connections, err := service.GetConnections()
		if err != nil {
			s.status = "读取连接配置失败：" + err.Error()
		} else {
			s.connections = connections
			s.connectionClicks = make([]widget.Clickable, len(connections))
			s.status = fmt.Sprintf("已加载 %d 个连接配置 · 请选择一个连接", len(connections))
		}
	}
	const defaultSQL = "SELECT\n    database,\n    table,\n    engine\nFROM system.tables\nORDER BY database, table\nLIMIT 100;"
	var snapshot config.TabsFile
	if service != nil {
		if saved, err := service.GetTabs(); err == nil {
			snapshot = saved
		} else {
			s.status = "读取 SQL 标签失败：" + err.Error()
		}
	}
	snapshot = normalizeTabSnapshot(snapshot, defaultSQL)
	s.tabs = snapshot.Tabs
	s.activeTabID = snapshot.ActiveTabID
	s.resetTabWidgets()
	if index := activeTabIndex(s.tabs, s.activeTabID); index >= 0 {
		s.sqlEditor.SetText(s.tabs[index].SQL)
	}
	s.applyTheme()
	return s
}

// startExport 重新执行最近一次成功查询，并将 ClickHouse 游标结果直接流式写入文件。
func (s *screen) startExport(format string) {
	if s.exportRunning || s.queryRunning || s.queryResult == nil || s.service == nil || s.explorer == nil {
		return
	}
	if s.queryResult.Error != "" || len(s.queryResult.Columns) == 0 || strings.TrimSpace(s.lastExecutedSQL) == "" || s.lastExecutedConnectionID == "" {
		return
	}
	connectionID, sqlText := s.lastExecutedConnectionID, s.lastExecutedSQL
	queryID := fmt.Sprintf("gio-export-%d", time.Now().UnixNano())
	s.exportRunning = true
	s.runningQueryID, s.runningTabID, s.queryRunning = queryID, s.activeTabID, true
	s.status = "正在重新执行当前 SQL，并流式导出完整结果…"
	go func() {
		file, err := s.explorer.CreateFile("query-result." + format)
		if err != nil {
			s.exportResults <- exportResult{format: format, err: err, canceled: errors.Is(err, gioexplorer.ErrUserDecline)}
			s.window.Invalidate()
			return
		}
		startedAt := time.Now()
		writer := &countingWriter{writer: file}
		rows, writeErr := s.service.ExportQueryToWithProgress(writer, connectionID, sqlText, queryID, format, func(count int64) {
			select {
			case s.exportProgress <- count:
			default:
				select {
				case <-s.exportProgress:
				default:
				}
				select {
				case s.exportProgress <- count:
				default:
				}
			}
		})
		closeErr := file.Close()
		if writeErr != nil {
			err = writeErr
		} else if closeErr != nil {
			err = closeErr
		}
		result := exportResult{format: format, err: err, rows: rows, bytes: writer.bytes, duration: time.Since(startedAt), canceled: errors.Is(writeErr, stdcontext.Canceled) || errors.Is(writeErr, stdcontext.DeadlineExceeded)}
		if err != nil {
			supported, removeErr := removeIncompleteExport(file)
			result.cleanupUnsupported = !supported
			result.cleanupErr = removeErr
		}
		s.exportResults <- result
		s.window.Invalidate()
	}()
}

// drainExportResults 在 Gio 事件循环中更新导出状态，避免后台 goroutine 直接修改界面状态。
func (s *screen) drainExportResults() {
	for {
		select {
		case count := <-s.exportProgress:
			if s.exportRunning {
				s.status = fmt.Sprintf("正在流式导出… 已处理 %d 行（每 1000 行更新）", count)
			}
		default:
			goto drainResults
		}
	}

drainResults:
	for {
		select {
		case result := <-s.exportResults:
			s.exportRunning = false
			s.queryRunning, s.runningQueryID, s.runningTabID = false, "", ""
			switch {
			case result.canceled:
				s.status = fmt.Sprintf("已取消导出（已处理 %d 行，已写入 %s）", result.rows, formatExportSize(result.bytes))
			case result.err != nil:
				s.status = fmt.Sprintf("导出中断：%s（已处理 %d 行，已写入 %s）", result.err.Error(), result.rows, formatExportSize(result.bytes))
			default:
				s.status = fmt.Sprintf("已成功导出 %d 行 %s（%s，耗时 %s，平均 %.1f 行/秒；已重新执行 SQL）", result.rows, strings.ToUpper(result.format), formatExportSize(result.bytes), result.duration.Round(time.Millisecond), exportRowsPerSecond(result.rows, result.duration))
			}
			if result.err != nil {
				if result.cleanupErr != nil {
					s.status += "；删除不完整文件失败：" + result.cleanupErr.Error()
				} else if result.cleanupUnsupported {
					s.status += "；当前平台无法自动删除不完整文件，请手动检查目标文件"
				}
			}
		default:
			return
		}
	}
}

func (s *screen) drainConnectResults() {
	for {
		select {
		case result := <-s.connectResults:
			if result.connectionID != s.selectedConnectionID || result.sequence != s.connectionSequence {
				continue
			}
			if result.err != nil {
				if s.pendingHistoryConnectionID == result.connectionID {
					s.pendingHistorySQL, s.pendingHistoryConnectionID = "", ""
				}
				s.treeConnected = false
				s.connectFailed = result.connectionID == s.selectedConnectionID
				s.databaseNames = nil
				s.status = "连接或加载数据库失败：" + result.err.Error()
			} else {
				s.treeConnected = true
				s.connectFailed = false
				s.databaseNames = result.databases
				s.databaseNodes = make(map[string]*databaseNode)
				for _, name := range result.databases {
					s.databaseNodes[name] = &databaseNode{}
				}
				s.status = fmt.Sprintf("连接成功，已加载 %d 个数据库", len(result.databases))
				if s.pendingHistoryConnectionID == result.connectionID && s.pendingHistorySQL != "" {
					s.sqlEditor.SetText(s.pendingHistorySQL)
					s.pendingHistorySQL, s.pendingHistoryConnectionID = "", ""
					s.saveActiveTabText()
					s.executeSQL()
				}
			}
		default:
			return
		}
	}
}

func (s *screen) toggleDatabase(name string, node *databaseNode) {
	node.expanded = !node.expanded
	if !node.expanded || node.loaded || node.loading {
		return
	}
	s.databaseSequence++
	requestID := s.databaseSequence
	node.requestID, node.loading, node.err = requestID, true, nil
	connectionID, connectionSequence := s.selectedConnectionID, s.connectionSequence
	go func() {
		tables, err := s.service.GetTables(connectionID, name)
		var views []clickhouse.TableInfo
		if err == nil {
			views, err = s.service.GetViews(connectionID, name)
		}
		s.databaseResults <- databaseResult{connectionID: connectionID, connectionSequence: connectionSequence, database: name, requestID: requestID, tables: tables, views: views, err: err}
		s.window.Invalidate()
	}()
}

func (s *screen) drainDatabaseResults() {
	for {
		select {
		case result := <-s.databaseResults:
			if result.connectionID != s.selectedConnectionID || result.connectionSequence != s.connectionSequence {
				continue
			}
			node := s.databaseNodes[result.database]
			if node == nil || node.requestID != result.requestID {
				continue
			}
			node.loading, node.err = false, result.err
			if result.err == nil {
				node.tables, node.views, node.loaded = result.tables, result.views, true
				node.tableClicks = make(map[string]*widget.Clickable)
				node.viewClicks = make(map[string]*widget.Clickable)
				for _, table := range result.tables {
					node.tableClicks[table.Name] = &widget.Clickable{}
				}
				for _, view := range result.views {
					node.viewClicks[view.Name] = &widget.Clickable{}
				}
				s.status = fmt.Sprintf("数据库 %s：已加载 %d 张表、%d 个视图", result.database, len(node.tables), len(node.views))
			} else {
				s.status = "加载数据库对象失败：" + result.err.Error()
			}
		default:
			return
		}
	}
}

func (s *screen) selectTable(database, table string) {
	if s.service == nil || !s.treeConnected {
		return
	}
	s.schemaSequence++
	requestID := s.schemaSequence
	connectionID, connectionSequence := s.selectedConnectionID, s.connectionSequence
	s.selectedSchemaDatabase, s.selectedSchemaTable = database, table
	s.detailsOpen = true
	s.selectedSchema = nil
	s.selectedTableDetails = nil
	s.ddlLines = nil
	s.tableDetailsError = ""
	s.schemaError = ""
	s.schemaLoading = true
	s.status = fmt.Sprintf("正在加载表结构：%s.%s", database, table)
	go func() {
		columns, err := s.service.GetTableSchema(connectionID, database, table)
		var details *clickhouse.TableDetails
		var detailsErr error
		if err == nil {
			details, detailsErr = s.service.GetTableDetails(connectionID, database, table)
		}
		s.schemaResults <- schemaResult{
			connectionID:       connectionID,
			connectionSequence: connectionSequence,
			database:           database,
			table:              table,
			requestID:          requestID,
			columns:            columns,
			details:            details,
			detailsErr:         detailsErr,
			err:                err,
		}
		s.window.Invalidate()
	}()
}

func (s *screen) selectView(database string, view clickhouse.TableInfo) {
	if view.Name == "" {
		return
	}
	s.setEditorSQL(buildTablePreviewSQL(database, view.Name, 100))
	s.resultPane = resultPaneQuery
	s.pendingPreviewTable = ""
	s.status = fmt.Sprintf("已将视图 %s.%s 的查询语句填入编辑器", database, view.Name)
}

// setEditorSQL 仅替换当前标签的 SQL 文本，由用户自行决定何时执行。
func (s *screen) setEditorSQL(sqlText string) {
	s.sqlEditor.SetText(sqlText)
	s.saveActiveTabText()
	s.persistTabs()
}

func isCurrentSchemaResult(result schemaResult, connectionID string, connectionSequence, schemaSequence uint64, database, table string) bool {
	return result.connectionID == connectionID &&
		result.connectionSequence == connectionSequence &&
		result.requestID == schemaSequence &&
		result.database == database &&
		result.table == table
}

func (s *screen) drainSchemaResults() {
	for {
		select {
		case result := <-s.schemaResults:
			if !isCurrentSchemaResult(
				result,
				s.selectedConnectionID,
				s.connectionSequence,
				s.schemaSequence,
				s.selectedSchemaDatabase,
				s.selectedSchemaTable,
			) {
				continue
			}
			s.schemaLoading = false
			s.schemaError = ""
			if result.err != nil {
				s.schemaError = result.err.Error()
				s.status = "读取表结构失败：" + result.err.Error()
				continue
			}
			s.selectedSchema = result.columns
			s.selectedTableDetails = result.details
			s.ddlLines = nil
			if result.details != nil {
				s.ddlLines = splitDDLLines(result.details.CreateTableQuery)
			}
			if result.detailsErr != nil {
				s.tableDetailsError = result.detailsErr.Error()
			}
			s.status = fmt.Sprintf("已加载 %s.%s 的 %d 个字段", result.database, result.table, len(result.columns))
		default:
			return
		}
	}
}

func (s *screen) executeSQL() {
	s.persistTabs()
	if s.service == nil {
		s.status = "应用服务尚未初始化"
		return
	}
	if !s.treeConnected || s.selectedConnectionID == "" {
		s.status = "请先连接 ClickHouse"
		return
	}
	sqlText := strings.TrimSpace(s.sqlEditor.Text())
	if sqlText == "" {
		s.status = "SQL 不能为空"
		return
	}
	if s.queryRunning {
		s.status = "已有查询正在执行，请先停止或等待完成"
		return
	}
	s.querySequence++
	queryID := fmt.Sprintf("gio-%d-%d", time.Now().UnixNano(), s.querySequence)
	sequence, connectionID, tabID := s.querySequence, s.selectedConnectionID, s.activeTabID
	s.resultPane = resultPaneQuery
	s.lastExecutedSQL, s.lastExecutedConnectionID = sqlText, connectionID
	s.runningQueryID, s.runningTabID, s.queryRunning, s.queryResult = queryID, tabID, true, nil
	s.tabRuntime[tabID] = tabRuntimeState{lastSQL: sqlText, connectionID: connectionID}
	s.resetResultWidgets(nil)
	s.status = "正在执行 SQL…"
	go func() {
		result, err := s.service.Query(connectionID, sqlText, queryID)
		s.queryResults <- queryResult{tabID: tabID, queryID: queryID, sequence: sequence, result: result, err: err}
		s.window.Invalidate()
	}()
}

func (s *screen) stopQuery() {
	if !s.queryRunning || s.runningQueryID == "" {
		s.status = "当前没有正在执行的查询"
		return
	}
	if s.service.CancelQuery(s.runningQueryID) {
		s.status = "已发送取消请求…"
	} else {
		s.status = "查询已结束，正在接收最终结果…"
	}
}

func (s *screen) drainQueryResults() {
	for {
		select {
		case result := <-s.queryResults:
			if result.queryID != s.runningQueryID || result.sequence != s.querySequence {
				continue
			}
			s.queryRunning, s.runningQueryID, s.runningTabID = false, "", ""
			s.refreshHistory(false)
			state := s.tabRuntime[result.tabID]
			if result.err != nil {
				state.result = nil
				s.tabRuntime[result.tabID] = state
				if result.tabID == s.activeTabID {
					s.queryResult = nil
					s.resetResultWidgets(nil)
					s.status = "查询失败：" + result.err.Error()
				}
				continue
			}
			state.result = result.result
			s.tabRuntime[result.tabID] = state
			if result.tabID != s.activeTabID {
				s.status = "后台标签查询完成：" + result.tabID
				continue
			}
			s.queryResult = result.result
			s.resetResultWidgets(result.result)
			if result.result == nil {
				s.status = "查询未返回结果"
			} else if result.result.Error != "" {
				s.status = "SQL 执行失败：" + result.result.Error
			} else {
				s.status = fmt.Sprintf("查询完成：%d 行 · %.2f ms", result.result.RowCount, result.result.ElapsedMs)
			}
		default:
			return
		}
	}
}

// resultColumnWeights 根据列名和前 30 行样本估算列宽权重，避免所有列等宽导致短列浪费空间。
func resultColumnWeights(columns []clickhouse.ColumnInfo, rows [][]interface{}) []float32 {
	weights := make([]float32, len(columns))
	for i, column := range columns {
		width := len([]rune(column.Name))
		sampleCount := len(rows)
		if sampleCount > 30 {
			sampleCount = 30
		}
		for rowIndex := 0; rowIndex < sampleCount; rowIndex++ {
			if i >= len(rows[rowIndex]) {
				continue
			}
			cellWidth := len([]rune(formatResultCell(rows[rowIndex][i])))
			if cellWidth > width {
				width = cellWidth
			}
		}
		if width < 10 {
			width = 10
		}
		if width > 32 {
			width = 32
		}
		weights[i] = float32(width)
	}
	return weights
}

// resultColumnWidth 将列宽权重转换为固定 dp 宽度，使所有结果行列对齐并可横向滚动。
const (
	minResultColumnWidth          unit.Dp = 80
	maxResultColumnWidth          unit.Dp = 480
	resultColumnResizeHandleWidth unit.Dp = 12
)

func adjustResultColumnWidth(current unit.Dp, delta float32) unit.Dp {
	width := unit.Dp(float32(current) + delta)
	if width < minResultColumnWidth {
		return minResultColumnWidth
	}
	if width > maxResultColumnWidth {
		return maxResultColumnWidth
	}
	return width
}

func (s *screen) layoutResultColumnResizeHandle(gtx context, columnIndex int) dimensions {
	handleWidth := gtx.Dp(resultColumnResizeHandleWidth)
	gtx.Constraints.Min.X, gtx.Constraints.Max.X = handleWidth, handleWidth
	area := clip.Rect(image.Rect(0, 0, handleWidth, gtx.Constraints.Max.Y)).Push(gtx.Ops)
	pointer.CursorColResize.Add(gtx.Ops)
	s.resultColumnDrags[columnIndex].Add(gtx.Ops)
	area.Pop()

	for {
		event, ok := s.resultColumnDrags[columnIndex].Update(gtx.Metric, gtx.Source, gesture.Horizontal)
		if !ok {
			break
		}
		if event.Kind == pointer.Drag {
			if !s.resultColumnDragActive[columnIndex] {
				s.resultColumnDragLastX[columnIndex] = event.Position.X
				s.resultColumnDragActive[columnIndex] = true
			} else {
				current := s.resultColumnWidths[columnIndex]
				if current <= 0 {
					current = minResultColumnWidth
				}
				deltaPx := int(event.Position.X - s.resultColumnDragLastX[columnIndex])
				deltaDp := float32(gtx.Metric.PxToDp(deltaPx))
				s.resultColumnWidths[columnIndex] = adjustResultColumnWidth(current, deltaDp)
				s.resultColumnDragLastX[columnIndex] = event.Position.X
			}
		}
	}
	if !s.resultColumnDrags[columnIndex].Pressed() {
		s.resultColumnDragActive[columnIndex] = false
	}

	return layout.Center.Layout(gtx, func(gtx context) dimensions {
		return material.Caption(s.theme, "⋮").Layout(gtx)
	})
}

func resultColumnWidth(weight float32) unit.Dp {
	if weight < 10 {
		weight = 10
	}
	if weight > 32 {
		weight = 32
	}
	return unit.Dp(weight * 8)
}

// splitDDLLines 将建表语句拆成可独立滚动显示的行，并统一换行符。
func splitDDLLines(ddl string) []string {
	if ddl == "" {
		return nil
	}
	ddl = strings.ReplaceAll(ddl, "\r\n", "\n")
	ddl = strings.ReplaceAll(ddl, "\r", "\n")
	return strings.Split(ddl, "\n")
}

func formatResultRow(row []interface{}) string {
	cells := make([]string, len(row))
	for i, value := range row {
		cell := formatClipboardCell(value)
		if strings.ContainsAny(cell, "\t\r\n\"") {
			cell = "\"" + strings.ReplaceAll(cell, "\"", "\"\"") + "\""
		}
		cells[i] = cell
	}
	return strings.Join(cells, "\t")
}
func formatClipboardCell(value interface{}) string {
	if value == nil {
		return "NULL"
	}
	return fmt.Sprint(value)
}

func splitCellDetailLines(value string) []string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	if value == "" {
		return []string{""}
	}
	return strings.Split(value, "\n")
}

func formatResultCell(value interface{}) string {
	if value == nil {
		return "NULL"
	}
	valueText := fmt.Sprint(value)
	runes := []rune(valueText)
	if len(runes) > 120 {
		return string(runes[:117]) + "..."
	}
	return valueText
}
func (s *screen) refreshHistory(showError bool) {
	if s.service == nil {
		return
	}
	entries, err := s.service.GetQueryHistory()
	if err != nil {
		if showError {
			s.status = "读取查询历史失败：" + err.Error()
		}
		return
	}
	s.historyEntries = entries
	s.historySelectClicks = make([]widget.Clickable, len(entries))
	s.historyRunClicks = make([]widget.Clickable, len(entries))
	s.historyDeleteClicks = make([]widget.Clickable, len(entries))
}

func (s *screen) openHistory() {
	s.historyOpen = true
	s.historyClearConfirm = false
	s.refreshHistory(true)
}

func (s *screen) selectHistoryEntry(entry config.HistoryEntry) {
	s.sqlEditor.SetText(entry.SQL)
	s.persistTabs()
	s.status = "已将查询历史回填到当前 SQL 标签"
}

func (s *screen) runHistoryEntry(entry config.HistoryEntry) {
	if s.queryRunning || s.exportRunning {
		s.status = "当前有查询或导出任务，请完成后再执行历史 SQL"
		return
	}
	var target *config.Connection
	for i := range s.connections {
		if s.connections[i].ID == entry.ConnectionID {
			target = &s.connections[i]
			break
		}
	}
	if target == nil {
		s.status = "历史记录对应的连接已不存在，请重新选择连接"
		return
	}
	s.sqlEditor.SetText(entry.SQL)
	s.persistTabs()
	if s.treeConnected && s.selectedConnectionID == entry.ConnectionID {
		s.executeSQL()
		return
	}
	s.pendingHistorySQL = entry.SQL
	s.pendingHistoryConnectionID = entry.ConnectionID
	s.connectConnection(*target)
	s.status = "正在切换到历史记录对应的连接…"
}

func (s *screen) deleteHistoryEntry(id string) {
	if s.service == nil {
		return
	}
	if err := s.service.DeleteQueryHistory(id); err != nil {
		s.status = "删除查询历史失败：" + err.Error()
		return
	}
	s.refreshHistory(false)
	s.status = "已删除一条查询历史"
}

func (s *screen) clearHistory() {
	if s.service == nil {
		return
	}
	if err := s.service.ClearQueryHistory(); err != nil {
		s.status = "清空查询历史失败：" + err.Error()
		return
	}
	s.historyClearConfirm = false
	s.refreshHistory(false)
	s.status = "查询历史已清空"
}

func historySQLPreview(sql string) string {
	for _, line := range strings.Split(sql, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		runes := []rune(line)
		if len(runes) > 96 {
			return string(runes[:96]) + "…"
		}
		return line
	}
	return "（空 SQL）"
}

func (s *screen) historyConnectionName(id string) string {
	for _, connection := range s.connections {
		if connection.ID == id {
			return connection.Name
		}
	}
	if id == "" {
		return "未知连接"
	}
	return "连接已删除"
}

func (s *screen) connectSelected() {
	for _, connection := range s.connections {
		if connection.ID == s.selectedConnectionID {
			s.connectConnection(connection)
			return
		}
	}
	s.status = "请先在左侧选择一个连接配置"
}

func (s *screen) connectConnection(connection config.Connection) {
	if s.pendingHistoryConnectionID != "" && s.pendingHistoryConnectionID != connection.ID {
		s.pendingHistorySQL, s.pendingHistoryConnectionID = "", ""
	}
	if s.service == nil {
		s.status = "应用服务尚未初始化"
		return
	}
	if s.queryRunning && s.runningQueryID != "" {
		s.service.CancelQuery(s.runningQueryID)
		s.querySequence++
		s.queryRunning = false
		s.runningQueryID = ""
		s.runningTabID = ""
	}
	s.selectedConnectionID = connection.ID
	s.previewMode = false
	s.previewTabID, s.previewConnectionID = "", ""
	s.connectionSequence++
	sequence := s.connectionSequence
	s.treeConnected = false
	s.databaseNames = nil
	s.databaseNodes = make(map[string]*databaseNode)
	s.schemaSequence++
	s.schemaLoading = false
	s.selectedSchema = nil
	s.selectedTableDetails = nil
	s.tableDetailsError = ""
	s.ddlLines = nil
	s.detailsOpen = false
	s.selectedSchemaDatabase, s.selectedSchemaTable, s.schemaError = "", "", ""
	s.status = "正在连接 " + connection.Name + "…"
	go func() {
		err := s.service.TestConnection(connection)
		var databases []string
		if err == nil {
			databases, err = s.service.GetDatabases(connection.ID)
		}
		s.connectResults <- connectResult{connectionID: connection.ID, sequence: sequence, databases: databases, err: err}
		s.window.Invalidate()
	}()
}

func (s *screen) saveWorkspaceLayout() {
	if s.service == nil {
		return
	}
	settings, err := s.service.GetSettings()
	if err != nil {
		s.status = "读取工作台设置失败：" + err.Error()
		return
	}
	settings.SidebarCollapsed = !s.sidebarOpen
	settings.EditorRatio = float64(clampEditorRatio(s.editorRatio))
	if _, err := s.service.SaveSettings(settings); err != nil {
		s.status = "保存工作台布局失败：" + err.Error()
	}
}

func (s *screen) applyTheme() {
	if s.lightTheme {
		s.theme.Palette.Bg = color.NRGBA{R: 248, G: 249, B: 251, A: 255}
		s.theme.Palette.Fg = color.NRGBA{R: 31, G: 41, B: 55, A: 255}
		s.theme.Palette.ContrastBg = color.NRGBA{R: 37, G: 99, B: 235, A: 255}
		s.theme.Palette.ContrastFg = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
		return
	}
	s.theme.Palette.Bg = color.NRGBA{R: 24, G: 28, B: 36, A: 255}
	s.theme.Palette.Fg = color.NRGBA{R: 225, G: 231, B: 239, A: 255}
	s.theme.Palette.ContrastBg = color.NRGBA{R: 59, G: 130, B: 246, A: 255}
	s.theme.Palette.ContrastFg = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
}

func (s *screen) layout(gtx context) dimensions {
	s.drainConnectResults()
	s.drainDatabaseResults()
	s.drainSchemaResults()
	s.drainQueryResults()
	for s.execute.Clicked(gtx) {
		s.executeSQL()
	}
	for s.connect.Clicked(gtx) {
		s.connectSelected()
	}
	for s.stop.Clicked(gtx) {
		s.stopQuery()
	}
	for s.resultTabQuery.Clicked(gtx) {
		s.resultPane = resultPaneQuery
	}
	for s.resultTabTable.Clicked(gtx) {
		if s.selectedSchemaTable != "" {
			s.resultPane = resultPaneTable
			if !s.previewMode {
				s.previewSelectedTable()
			}
		} else {
			s.status = "请先在左侧导航选择一张表"
		}
	}
	for s.sidebarToggle.Clicked(gtx) {
		s.sidebarOpen = !s.sidebarOpen
		s.saveWorkspaceLayout()
	}
	for s.detailsToggle.Clicked(gtx) {
		if s.selectedConnectionID == "" {
			s.status = "请先选择并连接一个连接配置"
		} else {
			s.connectSelected()
		}
	}
	for s.historyToggle.Clicked(gtx) {
		if s.historyOpen {
			s.historyOpen = false
		} else {
			s.detailsOpen = false
			s.savedSQLOpen = false
			s.openHistory()
		}
	}
	for s.savedSQLToggle.Clicked(gtx) {
		if s.savedSQLOpen {
			s.savedSQLOpen = false
		} else {
			s.detailsOpen = false
			s.historyOpen = false
			s.openSavedSQL()
		}
	}
	for s.editorSmaller.Clicked(gtx) {
		s.editorRatio = clampEditorRatio(s.editorRatio - 0.05)
		s.saveWorkspaceLayout()
	}
	for s.editorLarger.Clicked(gtx) {
		s.editorRatio = clampEditorRatio(s.editorRatio + 0.05)
		s.saveWorkspaceLayout()
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(s.layoutToolbar),
		layout.Flexed(1, s.layoutWorkspace),
		layout.Rigid(s.layoutStatus),
	)
}

func (s *screen) layoutToolbar(gtx context) dimensions {
	return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx context) dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx context) dimensions {
				return material.H6(s.theme, "Meteor CH Workbench").Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(24)}.Layout),
			layout.Rigid(func(gtx context) dimensions {
				label := "隐藏导航"
				if !s.sidebarOpen {
					label = "显示导航"
				}
				return material.Button(s.theme, &s.sidebarToggle, label).Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx context) dimensions {
				label := "显示表详情"
				if s.detailsOpen {
					label = "隐藏表详情"
				}
				return material.Button(s.theme, &s.detailsToggle, label).Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx context) dimensions {
				return material.Button(s.theme, &s.connect, "连接").Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx context) dimensions {
				label := "查询历史"
				if s.historyOpen {
					label = "隐藏历史"
				}
				return material.Button(s.theme, &s.historyToggle, label).Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx context) dimensions {
				label := "收藏 SQL"
				if s.savedSQLOpen {
					label = "隐藏收藏"
				}
				return material.Button(s.theme, &s.savedSQLToggle, label).Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
			layout.Flexed(1, func(gtx context) dimensions {
				return layout.Dimensions{}
			}),
			layout.Rigid(func(gtx context) dimensions {
				label := "切换浅色主题"
				if s.lightTheme {
					label = "切换深色主题"
				}
				dims := material.Button(s.theme, &s.themeToggle, label).Layout(gtx)
				for s.themeToggle.Clicked(gtx) {
					s.lightTheme = !s.lightTheme
					s.applyTheme()
				}
				return dims
			}),
		)
	})
}

func (s *screen) layoutWorkspace(gtx context) dimensions {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx context) dimensions {
			if !s.sidebarOpen {
				return dimensions{}
			}
			width := gtx.Dp(232)
			if gtx.Constraints.Max.X < gtx.Dp(900) {
				width = gtx.Dp(212)
			}
			if width > gtx.Constraints.Max.X {
				width = gtx.Constraints.Max.X
			}
			gtx.Constraints.Min.X = width
			gtx.Constraints.Max.X = width
			gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Flexed(1, func(gtx context) dimensions {
					return s.layoutSidebar(gtx)
				}),
				layout.Rigid(s.layoutSidebarFooter),
			)
		}),
		layout.Rigid(func(gtx context) dimensions {
			if !s.sidebarOpen {
				return dimensions{}
			}
			return layout.Spacer{Width: unit.Dp(12)}.Layout(gtx)
		}),
		layout.Flexed(1, s.layoutEditorAndResults),
	}
	if s.historyOpen {
		children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(1)}.Layout))
		children = append(children, layout.Rigid(s.layoutHistoryPanel))
	}
	if s.savedSQLOpen {
		children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(1)}.Layout))
		children = append(children, layout.Rigid(s.layoutSavedSQLPanel))
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
}

func emptyMetadata(value string) string {
	if strings.TrimSpace(value) == "" {
		return "（无）"
	}
	return value
}

func (s *screen) layoutSidebar(gtx context) dimensions {
	return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(4), Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, func(gtx context) dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx context) dimensions {
				return layout.Inset{Left: unit.Dp(4), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx context) dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx context) dimensions {
							label := material.Body2(s.theme, fmt.Sprintf("连接（%d）", len(s.connections)))
							label.Color = s.textSecondary()
							return label.Layout(gtx)
						}),
						layout.Rigid(func(gtx context) dimensions {
							return s.listRefreshButton(gtx, &s.connect, "刷新连接")
						}),
					)
				})
			}),
		}
		if len(s.connections) == 0 {
			children = append(children, layout.Rigid(func(gtx context) dimensions {
				label := material.Caption(s.theme, "暂无连接配置")
				label.Color = s.textSecondary()
				return layout.Inset{Left: unit.Dp(4)}.Layout(gtx, label.Layout)
			}))
		}
		for i := range s.connections {
			connection := s.connections[i]
			active := connection.ID == s.selectedConnectionID
			children = append(children, layout.Rigid(func(gtx context) dimensions {
				return s.connectionListItem(gtx, &s.connectionClicks[i], connection, active)
			}))
		}
		if !s.treeConnected {
			if s.connectFailed {
				children = append(children, layout.Rigid(func(gtx context) dimensions {
					return layout.Inset{Top: unit.Dp(10), Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, func(gtx context) dimensions {
						label := material.Caption(s.theme, "连接失败，请检查连通性或“更多操作”中的配置")
						label.Color = s.dangerColor()
						return label.Layout(gtx)
					})
				}))
			}
			children = append(children, layout.Rigid(func(gtx context) dimensions {
				return layout.Spacer{Height: unit.Dp(10)}.Layout(gtx)
			}))
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		}
		children = append(children, layout.Rigid(func(gtx context) dimensions {
			return layout.Inset{Top: unit.Dp(14), Left: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx context) dimensions {
				name := s.selectedConnectionName()
				label := material.Body2(s.theme, "数据库")
				if name != "" {
					label = material.Body2(s.theme, "数据库 · "+name)
				}
				label.Color = s.textSecondary()
				label.MaxLines = 1
				return label.Layout(gtx)
			})
		}))
		for _, database := range s.databaseNames {
			name := database
			node := s.databaseNodes[name]
			children = append(children, layout.Rigid(func(gtx context) dimensions {
				return s.treeRow(gtx, &node.click, treeRowOptions{
					label:  name,
					icon:   databaseIcon(node.expanded),
					detail: databaseRowDetail(len(node.tables), len(node.views), node.loaded, node.loading),
					active: node.expanded,
				}, func() {
					s.toggleDatabase(name, node)
				})
			}))
			if node.expanded && node.err != nil {
				message := node.err.Error()
				children = append(children, layout.Rigid(func(gtx context) dimensions {
					return s.treeMessage(gtx, "加载失败："+message)
				}))
			}
			if node.expanded && node.loaded {
				for _, table := range node.tables {
					tableName, engine := table.Name, table.Engine
					active := s.selectedSchemaDatabase == name && s.selectedSchemaTable == tableName
					children = append(children, layout.Rigid(func(gtx context) dimensions {
						return s.treeRow(gtx, node.tableClicks[tableName], treeRowOptions{
							label:  tableName,
							icon:   "T",
							detail: engine,
							active: active,
							level:  1,
						}, func() {
							s.selectTable(name, tableName)
						})
					}))
				}
				for _, view := range node.views {
					viewName, engine := view.Name, view.Engine
					click := node.viewClicks[viewName]
					if click == nil {
						continue
					}
					children = append(children, layout.Rigid(func(gtx context) dimensions {
						return s.treeRow(gtx, click, treeRowOptions{
							label:  viewName,
							icon:   "V",
							detail: engine,
							active: s.selectedSchemaDatabase == name && s.selectedSchemaTable == viewName,
							level:  1,
						}, func() {
							s.selectView(name, view)
						})
					}))
				}
				if len(node.tables)+len(node.views) == 0 {
					children = append(children, layout.Rigid(func(gtx context) dimensions {
						return s.treeMessage(gtx, "此数据库暂无表或视图")
					}))
				}
			}
		}
		if len(s.databaseNames) == 0 {
			children = append(children, layout.Rigid(func(gtx context) dimensions {
				return s.treeMessage(gtx, "当前账号没有可见数据库")
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (s *screen) layoutHistoryPanel(gtx context) dimensions {
	width := gtx.Dp(360)
	if width > gtx.Constraints.Max.X {
		width = gtx.Constraints.Max.X
	}
	gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
	return layout.Inset{Top: unit.Dp(12), Bottom: unit.Dp(12), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx context) dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx context) dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx context) dimensions { return material.H6(s.theme, "查询历史").Layout(gtx) }),
					layout.Rigid(func(gtx context) dimensions {
						d := material.Button(s.theme, &s.historyRefresh, "刷新").Layout(gtx)
						if s.historyRefresh.Clicked(gtx) {
							s.refreshHistory(true)
						}
						return d
					}),
					layout.Rigid(func(gtx context) dimensions {
						d := material.Button(s.theme, &s.historyClose, "关闭").Layout(gtx)
						if s.historyClose.Clicked(gtx) {
							s.historyOpen = false
						}
						return d
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		}
		if s.historyClearConfirm {
			children = append(children, layout.Rigid(func(gtx context) dimensions {
				return material.Body2(s.theme, "确认清空全部查询历史？此操作不可撤销。").Layout(gtx)
			}), layout.Rigid(func(gtx context) dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx context) dimensions {
						d := material.Button(s.theme, &s.historyConfirmClear, "确认清空").Layout(gtx)
						if s.historyConfirmClear.Clicked(gtx) {
							s.clearHistory()
						}
						return d
					}),
					layout.Rigid(func(gtx context) dimensions {
						d := material.Button(s.theme, &s.historyCancelClear, "取消").Layout(gtx)
						if s.historyCancelClear.Clicked(gtx) {
							s.historyClearConfirm = false
						}
						return d
					}),
				)
			}))
		} else {
			children = append(children, layout.Rigid(func(gtx context) dimensions {
				d := material.Button(s.theme, &s.historyClear, "清空历史").Layout(gtx)
				if s.historyClear.Clicked(gtx) {
					s.historyClearConfirm = true
				}
				return d
			}))
		}
		if len(s.historyEntries) == 0 {
			children = append(children, layout.Flexed(1, func(gtx context) dimensions {
				return material.Body2(s.theme, "暂无查询历史。执行 SQL 后，记录会自动保存在本地。").Layout(gtx)
			}))
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		}
		children = append(children, layout.Flexed(1, func(gtx context) dimensions {
			s.historyList.Axis = layout.Vertical
			return material.List(s.theme, &s.historyList).Layout(gtx, len(s.historyEntries), func(gtx context, index int) dimensions {
				entry := s.historyEntries[index]
				connectionName := s.historyConnectionName(entry.ConnectionID)
				timestamp := entry.Timestamp.Local().Format("2006-01-02 15:04:05")
				outcome := "成功"
				if !entry.Success {
					outcome = "失败"
				}
				meta := fmt.Sprintf("%s · %s · %.1f ms · %s", timestamp, connectionName, entry.DurationMs, outcome)
				return layout.Inset{Top: unit.Dp(5), Bottom: unit.Dp(5)}.Layout(gtx, func(gtx context) dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx context) dimensions { return material.Body2(s.theme, historySQLPreview(entry.SQL)).Layout(gtx) }),
						layout.Rigid(func(gtx context) dimensions { return material.Caption(s.theme, meta).Layout(gtx) }),
						layout.Rigid(func(gtx context) dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Rigid(func(gtx context) dimensions {
									d := material.Button(s.theme, &s.historySelectClicks[index], "回填").Layout(gtx)
									if s.historySelectClicks[index].Clicked(gtx) {
										s.selectHistoryEntry(entry)
									}
									return d
								}),
								layout.Rigid(func(gtx context) dimensions {
									d := material.Button(s.theme, &s.historyRunClicks[index], "重新执行").Layout(gtx)
									if s.historyRunClicks[index].Clicked(gtx) {
										s.runHistoryEntry(entry)
									}
									return d
								}),
								layout.Rigid(func(gtx context) dimensions {
									d := material.Button(s.theme, &s.historyDeleteClicks[index], "删除").Layout(gtx)
									if s.historyDeleteClicks[index].Clicked(gtx) {
										s.deleteHistoryEntry(entry.ID)
									}
									return d
								}),
							)
						}),
					)
				})
			})
		}))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

type savedSQLListItem struct {
	entry config.SavedSQL
	index int
}

func (s *screen) refreshSavedSQL(showError bool) {
	if s.service == nil {
		return
	}
	entries, err := s.service.GetSavedSQL()
	if err != nil {
		if showError {
			s.status = "读取收藏 SQL 失败：" + err.Error()
		}
		return
	}
	s.savedSQLEntries = entries
	s.savedSQLUseClicks = make([]widget.Clickable, len(entries))
	s.savedSQLEditClicks = make([]widget.Clickable, len(entries))
	s.savedSQLDeleteClicks = make([]widget.Clickable, len(entries))
}

func (s *screen) openSavedSQL() {
	s.savedSQLOpen = true
	s.refreshSavedSQL(true)
}

func savedSQLMatches(entry config.SavedSQL, keyword, connectionID, connectionName string, onlyCurrentConnection bool) bool {
	if onlyCurrentConnection && (connectionID == "" || entry.ConnectionID != connectionID) {
		return false
	}
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return true
	}
	searchable := strings.ToLower(entry.Name + "\n" + entry.SQL + "\n" + connectionName)
	return strings.Contains(searchable, keyword)
}

func (s *screen) filteredSavedSQL() []savedSQLListItem {
	keyword := s.savedSQLSearch.Text()
	filterState := fmt.Sprintf("%s|%s|%t", strings.ToLower(strings.TrimSpace(keyword)), s.selectedConnectionID, s.savedSQLOnlyCurrentConnection)
	if filterState != s.savedSQLFilterState {
		s.savedSQLList.Position = layout.Position{}
		s.savedSQLFilterState = filterState
	}
	filtered := make([]savedSQLListItem, 0, len(s.savedSQLEntries))
	for i, entry := range s.savedSQLEntries {
		if !savedSQLMatches(entry, keyword, s.selectedConnectionID, s.historyConnectionName(entry.ConnectionID), s.savedSQLOnlyCurrentConnection) {
			continue
		}
		filtered = append(filtered, savedSQLListItem{entry: entry, index: i})
	}
	return filtered
}

func (s *screen) saveCurrentSQL() {
	if s.service == nil {
		s.status = "应用服务尚未初始化"
		return
	}
	sqlText := strings.TrimSpace(s.sqlEditor.Text())
	if sqlText == "" {
		s.status = "SQL 不能为空，无法收藏"
		return
	}
	item, err := s.service.SaveSQL(s.savedSQLNameEditor.Text(), sqlText, s.selectedConnectionID)
	if err != nil {
		if errors.Is(err, config.ErrSavedSQLDuplicate) {
			s.status = "这条 SQL 已经收藏过了"
		} else {
			s.status = "收藏 SQL 失败：" + err.Error()
		}
		return
	}
	s.savedSQLNameEditor.SetText("")
	s.refreshSavedSQL(false)
	s.status = "已收藏 SQL：" + item.Name
}

func (s *screen) useSavedSQL(entry config.SavedSQL) {
	s.sqlEditor.SetText(entry.SQL)
	s.persistTabs()
	s.status = "已将收藏 SQL 回填到当前标签"
}

func (s *screen) beginEditSavedSQL(entry config.SavedSQL) {
	s.savedSQLEditingID = entry.ID
	s.savedSQLNameEditor.SetText(entry.Name)
	s.savedSQLEditor.SetText(entry.SQL)
	s.status = "正在编辑收藏 SQL：" + entry.Name
}

func (s *screen) saveEditedSQL() {
	if s.service == nil || s.savedSQLEditingID == "" {
		return
	}
	item, err := s.service.UpdateSavedSQL(s.savedSQLEditingID, s.savedSQLNameEditor.Text(), s.savedSQLEditor.Text())
	if err != nil {
		if errors.Is(err, config.ErrSavedSQLDuplicate) {
			s.status = "这条 SQL 已经收藏过了"
		} else {
			s.status = "更新收藏 SQL 失败：" + err.Error()
		}
		return
	}
	s.savedSQLEditingID = ""
	s.savedSQLNameEditor.SetText("")
	s.savedSQLEditor.SetText("")
	s.refreshSavedSQL(false)
	s.status = "已更新收藏 SQL：" + item.Name
}

func (s *screen) cancelEditSavedSQL() {
	s.savedSQLEditingID = ""
	s.savedSQLNameEditor.SetText("")
	s.savedSQLEditor.SetText("")
}

func (s *screen) deleteSavedSQL(id string) {
	if s.service == nil {
		return
	}
	if err := s.service.DeleteSavedSQL(id); err != nil {
		s.status = "删除收藏 SQL 失败：" + err.Error()
		return
	}
	if s.savedSQLEditingID == id {
		s.cancelEditSavedSQL()
	}
	// 保留当前帧的点击控件切片，避免 Gio 正在布局旧列表快照时索引失效；
	// 下一次显式刷新或重新打开面板时会重建控件。
	filtered := s.savedSQLEntries[:0]
	for _, entry := range s.savedSQLEntries {
		if entry.ID != id {
			filtered = append(filtered, entry)
		}
	}
	s.savedSQLEntries = filtered
	s.savedSQLList.Position = layout.Position{}
	s.status = "已删除一条收藏 SQL"
}

func (s *screen) layoutSavedSQLPanel(gtx context) dimensions {
	width := gtx.Dp(380)
	if width > gtx.Constraints.Max.X {
		width = gtx.Constraints.Max.X
	}
	gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
	return layout.Inset{Top: unit.Dp(12), Bottom: unit.Dp(12), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx context) dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx context) dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx context) dimensions { return material.H6(s.theme, "收藏 SQL").Layout(gtx) }),
					layout.Rigid(func(gtx context) dimensions {
						d := material.Button(s.theme, &s.savedSQLRefresh, "刷新").Layout(gtx)
						if s.savedSQLRefresh.Clicked(gtx) {
							s.refreshSavedSQL(true)
						}
						return d
					}),
					layout.Rigid(func(gtx context) dimensions {
						d := material.Button(s.theme, &s.savedSQLClose, "关闭").Layout(gtx)
						if s.savedSQLClose.Clicked(gtx) {
							s.savedSQLOpen = false
						}
						return d
					}),
				)
			}),
			layout.Rigid(func(gtx context) dimensions {
				return material.Editor(s.theme, &s.savedSQLSearch, "搜索名称、SQL 或连接").Layout(gtx)
			}),
			layout.Rigid(func(gtx context) dimensions {
				label := "仅当前连接"
				if s.savedSQLOnlyCurrentConnection {
					label = "显示全部连接"
				}
				d := material.Button(s.theme, &s.savedSQLScopeToggle, label).Layout(gtx)
				if s.savedSQLScopeToggle.Clicked(gtx) {
					s.savedSQLOnlyCurrentConnection = !s.savedSQLOnlyCurrentConnection
				}
				return d
			}),
		}
		if s.savedSQLEditingID != "" {
			children = append(children,
				layout.Rigid(func(gtx context) dimensions { return material.Body2(s.theme, "编辑收藏").Layout(gtx) }),
				layout.Rigid(func(gtx context) dimensions {
					return material.Editor(s.theme, &s.savedSQLNameEditor, "收藏名称").Layout(gtx)
				}),
				layout.Flexed(0.35, func(gtx context) dimensions {
					return material.Editor(s.theme, &s.savedSQLEditor, "SQL 内容").Layout(gtx)
				}),
				layout.Rigid(func(gtx context) dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx context) dimensions {
							d := material.Button(s.theme, &s.savedSQLSaveEdit, "保存修改").Layout(gtx)
							if s.savedSQLSaveEdit.Clicked(gtx) {
								s.saveEditedSQL()
							}
							return d
						}),
						layout.Rigid(func(gtx context) dimensions {
							d := material.Button(s.theme, &s.savedSQLCancelEdit, "取消").Layout(gtx)
							if s.savedSQLCancelEdit.Clicked(gtx) {
								s.cancelEditSavedSQL()
							}
							return d
						}),
					)
				}),
			)
		} else {
			children = append(children,
				layout.Rigid(func(gtx context) dimensions {
					return material.Editor(s.theme, &s.savedSQLNameEditor, "收藏名称（可选）").Layout(gtx)
				}),
				layout.Rigid(func(gtx context) dimensions {
					d := material.Button(s.theme, &s.savedSQLSaveCurrent, "收藏当前 SQL").Layout(gtx)
					if s.savedSQLSaveCurrent.Clicked(gtx) {
						s.saveCurrentSQL()
					}
					return d
				}),
			)
		}
		items := s.filteredSavedSQL()
		if len(items) == 0 {
			message := "暂无收藏 SQL。可先在编辑器中编写 SQL，再点击「收藏当前 SQL」。"
			if len(s.savedSQLEntries) > 0 {
				message = "没有符合搜索条件的收藏 SQL。"
			}
			children = append(children, layout.Flexed(1, func(gtx context) dimensions { return material.Body2(s.theme, message).Layout(gtx) }))
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		}
		children = append(children, layout.Flexed(1, func(gtx context) dimensions {
			s.savedSQLList.Axis = layout.Vertical
			return material.List(s.theme, &s.savedSQLList).Layout(gtx, len(items), func(gtx context, index int) dimensions {
				item := items[index]
				entry, original := item.entry, item.index
				meta := s.historyConnectionName(entry.ConnectionID) + " · 更新于 " + entry.UpdatedAt.Local().Format("01-02 15:04")
				return layout.Inset{Top: unit.Dp(5), Bottom: unit.Dp(5)}.Layout(gtx, func(gtx context) dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx context) dimensions { return material.Body1(s.theme, entry.Name).Layout(gtx) }),
						layout.Rigid(func(gtx context) dimensions {
							return material.Caption(s.theme, historySQLPreview(entry.SQL)).Layout(gtx)
						}),
						layout.Rigid(func(gtx context) dimensions { return material.Caption(s.theme, meta).Layout(gtx) }),
						layout.Rigid(func(gtx context) dimensions {
							return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
								layout.Rigid(func(gtx context) dimensions {
									d := material.Button(s.theme, &s.savedSQLUseClicks[original], "回填").Layout(gtx)
									if s.savedSQLUseClicks[original].Clicked(gtx) {
										s.useSavedSQL(entry)
									}
									return d
								}),
								layout.Rigid(func(gtx context) dimensions {
									d := material.Button(s.theme, &s.savedSQLEditClicks[original], "编辑").Layout(gtx)
									if s.savedSQLEditClicks[original].Clicked(gtx) {
										s.beginEditSavedSQL(entry)
									}
									return d
								}),
								layout.Rigid(func(gtx context) dimensions {
									d := material.Button(s.theme, &s.savedSQLDeleteClicks[original], "删除").Layout(gtx)
									if s.savedSQLDeleteClicks[original].Clicked(gtx) {
										s.deleteSavedSQL(entry.ID)
									}
									return d
								}),
							)
						}),
					)
				})
			})
		}))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (s *screen) previewSelectedTable() {
	if s.selectedSchemaDatabase == "" || s.selectedSchemaTable == "" {
		s.status = "请先选择一张表"
		return
	}
	if s.queryRunning || s.exportRunning {
		s.status = "当前有查询或导出任务正在执行，请稍后再预览"
		return
	}
	if s.previewMode && (s.previewDatabase != s.selectedSchemaDatabase || s.previewTableName != s.selectedSchemaTable) {
		s.previewPage = 0
	}
	if s.previewPageLimit <= 0 {
		s.previewPageLimit = 100
	}
	s.previewMode = true
	s.previewTabID, s.previewConnectionID = s.activeTabID, s.selectedConnectionID
	s.previewDatabase, s.previewTableName = s.selectedSchemaDatabase, s.selectedSchemaTable
	s.resultPane = resultPaneTable
	s.runTablePreviewPage()
}

func (s *screen) runTablePreviewPage() {
	if s.previewPage < 0 {
		s.previewPage = 0
	}
	if s.previewPageLimit <= 0 {
		s.previewPageLimit = 100
	}
	sqlText := buildTablePreviewPageSQL(s.previewDatabase, s.previewTableName, s.previewPageLimit, s.previewPage)
	s.sqlEditor.SetText(sqlText)
	s.persistTabs()
	s.executeSQL()
}

func (s *screen) isCurrentTablePreview() bool {
	if !s.previewMode || s.previewDatabase == "" || s.previewTableName == "" {
		return false
	}
	want := buildTablePreviewPageSQL(s.previewDatabase, s.previewTableName, s.previewPageLimit, s.previewPage)
	return s.activeTabID == s.previewTabID &&
		s.selectedConnectionID == s.previewConnectionID &&
		s.lastExecutedConnectionID == s.previewConnectionID &&
		s.selectedSchemaDatabase == s.previewDatabase &&
		s.selectedSchemaTable == s.previewTableName &&
		strings.TrimSpace(s.lastExecutedSQL) == want
}

func (s *screen) changePreviewPage(delta int) {
	if !s.isCurrentTablePreview() || s.queryRunning || s.exportRunning {
		return
	}
	next := s.previewPage + delta
	if next < 0 {
		return
	}
	if delta > 0 && (s.queryResult == nil || s.queryResult.Error != "" || len(s.queryResult.Rows) < s.previewPageLimit) {
		return
	}
	s.previewPage = next
	s.runTablePreviewPage()
}

func (s *screen) cyclePreviewPageSize() {
	if !s.isCurrentTablePreview() || s.queryRunning || s.exportRunning {
		return
	}
	switch s.previewPageLimit {
	case 50:
		s.previewPageLimit = 100
	case 100:
		s.previewPageLimit = 200
	default:
		s.previewPageLimit = 50
	}
	s.previewPage = 0
	s.runTablePreviewPage()
}

func (s *screen) layoutTableDetails(gtx context) dimensions {
	return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10), Left: unit.Dp(12), Right: unit.Dp(8)}.Layout(gtx, func(gtx context) dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx context) dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx context) dimensions { return s.sectionTitle(gtx, "表结构 / DDL") }),
					layout.Rigid(func(gtx context) dimensions {
						if s.previewTable.Clicked(gtx) {
							s.previewSelectedTable()
						}
						return s.smallButton(gtx, &s.previewTable, "预览数据", false)
					}),
				)
			}),
			layout.Rigid(func(gtx context) dimensions {
				return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx context) dimensions {
					label := material.Caption(s.theme, s.selectedSchemaDatabase+"."+s.selectedSchemaTable)
					label.Color = s.textMuted()
					label.MaxLines = 1
					return label.Layout(gtx)
				})
			}),
			layout.Rigid(func(gtx context) dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx context) dimensions {
						label := material.Caption(s.theme, fmt.Sprintf("字段（%d）", len(s.selectedSchema)))
						label.Color = s.textMuted()
						return label.Layout(gtx)
					}),
					layout.Rigid(func(gtx context) dimensions {
						label := "显示 DDL"
						if s.detailsShowDDL {
							label = "仅字段"
						}
						d := s.smallButton(gtx, &s.detailsListToggle, label, false)
						for s.detailsListToggle.Clicked(gtx) {
							s.detailsShowDDL = !s.detailsShowDDL
						}
						return d
					}),
				)
			}),
		}
		if s.schemaLoading {
			children = append(children, layout.Rigid(func(gtx context) dimensions { return material.Body2(s.theme, "正在加载…").Layout(gtx) }))
		} else if s.schemaError != "" {
			message := s.schemaError
			children = append(children, layout.Rigid(func(gtx context) dimensions {
				return material.Body2(s.theme, "表结构读取失败："+message).Layout(gtx)
			}))
		}
		children = append(children,
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Flexed(1, func(gtx context) dimensions {
				itemCount := len(s.selectedSchema)
				if itemCount == 0 {
					itemCount = 1
				}
				s.schemaList.Axis = layout.Vertical
				return material.List(s.theme, &s.schemaList).Layout(gtx, itemCount, func(gtx context, index int) dimensions {
					if len(s.selectedSchema) == 0 {
						if s.schemaError != "" {
							return dimensions{}
						}
						return material.Caption(s.theme, "该表没有可见字段").Layout(gtx)
					}
					column := s.selectedSchema[index]
					return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx context) dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx context) dimensions {
								name := column.Name
								if column.IsPrimary {
									name += " ●"
								}
								return material.Body2(s.theme, name).Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
							layout.Flexed(1, func(gtx context) dimensions {
								return material.Caption(s.theme, column.Type).Layout(gtx)
							}),
						)
					})
				})
			}),
		)
		details := s.selectedTableDetails
		if s.tableDetailsError != "" {
			message := s.tableDetailsError
			children = append(children, layout.Rigid(func(gtx context) dimensions {
				return material.Caption(s.theme, "存储详情读取失败："+message).Layout(gtx)
			}))
		} else if details != nil && details.CreateTableQuery != "" && s.detailsShowDDL {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx context) dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx context) dimensions { return material.Body2(s.theme, "DDL").Layout(gtx) }),
						layout.Rigid(func(gtx context) dimensions {
							dims := material.Button(s.theme, &s.copyDDL, "复制").Layout(gtx)
							for s.copyDDL.Clicked(gtx) {
								gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(details.CreateTableQuery))})
								s.status = "已将 " + s.selectedSchemaDatabase + "." + s.selectedSchemaTable + " 的 DDL 复制到剪贴板"
							}
							return dims
						}),
					)
				}),
				layout.Flexed(1, func(gtx context) dimensions {
					s.detailsList.Axis = layout.Vertical
					return material.List(s.theme, &s.detailsList).Layout(gtx, len(s.ddlLines), func(gtx context, index int) dimensions {
						line := s.ddlLines[index]
						return layout.Inset{Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx context) dimensions {
							return material.Caption(s.theme, line).Layout(gtx)
						})
					})
				}),
			)
		} else if details != nil {
			metadata := []string{
				"引擎：" + emptyMetadata(details.Engine),
				"注释：" + emptyMetadata(details.Comment),
				"PARTITION BY：" + emptyMetadata(details.PartitionKey),
				"ORDER BY：" + emptyMetadata(details.SortingKey),
				"PRIMARY KEY：" + emptyMetadata(details.PrimaryKey),
				"TTL：" + emptyMetadata(details.TTLExpression),
			}
			children = append(children,
				layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx context) dimensions { return material.Body2(s.theme, "存储信息").Layout(gtx) }),
			)
			for _, item := range metadata {
				line := item
				children = append(children, layout.Rigid(func(gtx context) dimensions { return material.Caption(s.theme, line).Layout(gtx) }))
			}
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (s *screen) layoutTableData(gtx context) dimensions {
	return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx context) dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx context) dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx context) dimensions {
						return s.sectionTitle(gtx, "表数据 · "+s.selectedSchemaDatabase+"."+s.selectedSchemaTable)
					}),
					layout.Flexed(1, func(gtx context) dimensions { return dimensions{} }),
					layout.Rigid(func(gtx context) dimensions {
						d := s.smallButton(gtx, &s.previewTable, "刷新数据", false)
						if s.previewTable.Clicked(gtx) {
							s.previewSelectedTable()
						}
						return d
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx context) dimensions {
						d := material.Button(s.theme, &s.resultTabQuery, "查看查询结果").Layout(gtx)
						if s.resultTabQuery.Clicked(gtx) {
							s.resultPane = resultPaneQuery
						}
						return d
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
			layout.Flexed(1, func(gtx context) dimensions {
				return s.layoutResultsBody(gtx, true)
			}),
		)
	})
}

// layoutEditorAndResults 按 DBeaver / Navicat 的分栏组织右侧工作区：
// 左半为资源列（数据库 → 表与视图）与表结构，右半为 SQL 编辑器 + 查询结果。
func (s *screen) layoutEditorAndResults(gtx context) dimensions {
	s.workspaceHeight = gtx.Constraints.Max.Y
	children := []layout.FlexChild{
		layout.Flexed(1, s.layoutQueryArea),
	}
	if s.selectedSchemaTable != "" {
		children = append(children,
			layout.Rigid(func(gtx context) dimensions {
				return layout.Spacer{Width: unit.Dp(12)}.Layout(gtx)
			}),
			layout.Rigid(func(gtx context) dimensions {
				width := gtx.Dp(340)
				if gtx.Constraints.Max.X < gtx.Dp(1000) {
					width = gtx.Dp(300)
				}
				if width > gtx.Constraints.Max.X/2 {
					width = gtx.Constraints.Max.X / 2
				}
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
				gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Flexed(1, s.layoutDatabaseColumn),
					layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
					layout.Flexed(1, s.layoutTableDetails),
				)
			}),
		)
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
}

func (s *screen) layoutQueryArea(gtx context) dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(s.editorRatio, s.layoutEditor),
		layout.Rigid(s.layoutSplitControls),
		layout.Flexed(1-s.editorRatio, s.layoutResultPane),
	)
}

// layoutDatabaseColumn 呈现当前连接的数据库列表，可点击展开并加载表与视图。
func (s *screen) layoutDatabaseColumn(gtx context) dimensions {
	return s.sectionCard(gtx, func(gtx context) dimensions {
		children := []layout.FlexChild{
			layout.Rigid(s.databaseColumnHeader),
			layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		}
		switch {
		case !s.treeConnected:
			children = append(children, layout.Flexed(1, func(gtx context) dimensions {
				return s.emptyHint(gtx, "连接成功后在此选择数据库，查看表与视图")
			}))
		case len(s.databaseNames) == 0:
			children = append(children, layout.Flexed(1, func(gtx context) dimensions {
				return s.emptyHint(gtx, "当前账号没有可见数据库")
			}))
		default:
			children = append(children, layout.Flexed(1, func(gtx context) dimensions {
				return s.layoutDatabaseRows(gtx)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (s *screen) databaseColumnHeader(gtx context) dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx context) dimensions {
			return s.sectionTitle(gtx, "数据库")
		}),
		layout.Flexed(1, func(gtx context) dimensions { return dimensions{} }),
		layout.Rigid(func(gtx context) dimensions {
			label := material.Caption(s.theme, fmt.Sprintf("%d", len(s.databaseNames)))
			label.Color = s.textMuted()
			return label.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx context) dimensions {
			if s.selectedConnectionID == "" {
				return dimensions{}
			}
			if s.connect.Clicked(gtx) {
				s.connectSelected()
			}
			return s.smallButton(gtx, &s.connect, "重新加载", false)
		}),
	)
}

func (s *screen) layoutDatabaseRows(gtx context) dimensions {
	s.databaseList.Axis = layout.Vertical
	return material.List(s.theme, &s.databaseList).Layout(gtx, len(s.databaseNames), func(gtx context, index int) dimensions {
		name := s.databaseNames[index]
		node := s.databaseNodes[name]
		if node == nil {
			return dimensions{}
		}
		rows := []layout.FlexChild{
			layout.Rigid(func(gtx context) dimensions {
				return s.treeRow(gtx, &node.click, treeRowOptions{
					label:  name,
					icon:   databaseIcon(node.expanded),
					detail: databaseRowDetail(len(node.tables), len(node.views), node.loaded, node.loading),
					active: node.expanded,
				}, func() {
					s.toggleDatabase(name, node)
				})
			}),
		}
		if node.expanded && node.err != nil {
			rows = append(rows, layout.Rigid(func(gtx context) dimensions {
				return s.treeMessage(gtx, "加载失败："+node.err.Error())
			}))
		}
		if node.expanded && node.loaded {
			rows = append(rows, layout.Rigid(func(gtx context) dimensions {
				return s.nodeChildren(gtx, name, node)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
	})
}

// nodeChildren 展开数据库下的表与视图；数据量大时改为分节标题 + 独立滚动列表，
// 避免把整棵树的每个节点都塞进同一帧。
func (s *screen) nodeChildren(gtx context, database string, node *databaseNode) dimensions {
	if len(node.tables)+len(node.views) == 0 {
		return s.treeMessage(gtx, "此数据库暂无表或视图")
	}
	children := make([]layout.FlexChild, 0, 4)
	if len(node.tables) > 0 {
		children = append(children, layout.Rigid(func(gtx context) dimensions {
			return s.sectionLabel(gtx, fmt.Sprintf("表（%d）", len(node.tables)))
		}))
		for _, table := range node.tables {
			tableName, engine := table.Name, table.Engine
			click := node.tableClicks[tableName]
			if click == nil {
				continue
			}
			active := s.selectedSchemaDatabase == database && s.selectedSchemaTable == tableName
			children = append(children, layout.Rigid(func(gtx context) dimensions {
				return s.treeRow(gtx, click, treeRowOptions{
					label:  tableName,
					icon:   "T",
					detail: engine,
					active: active,
					level:  1,
				}, func() {
					s.selectTable(database, tableName)
				})
			}))
		}
	}
	if len(node.views) > 0 {
		children = append(children, layout.Rigid(func(gtx context) dimensions {
			return s.sectionLabel(gtx, fmt.Sprintf("视图（%d）", len(node.views)))
		}))
		for _, view := range node.views {
			viewName, engine := view.Name, view.Engine
			click := node.viewClicks[viewName]
			if click == nil {
				continue
			}
			children = append(children, layout.Rigid(func(gtx context) dimensions {
				return s.treeRow(gtx, click, treeRowOptions{
					label:  viewName,
					icon:   "V",
					detail: engine,
					level:  1,
				}, func() {
					s.selectView(database, view)
				})
			}))
		}
	}
	s.objectList.Axis = layout.Vertical
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx context) dimensions {
			height := gtx.Constraints.Max.Y
			if height > gtx.Dp(420) {
				height = gtx.Dp(420)
			}
			if height < 0 {
				height = 0
			}
			gtx.Constraints.Min.Y = height
			gtx.Constraints.Max.Y = height
			return material.List(s.theme, &s.objectList).Layout(gtx, 1, func(gtx context, _ int) dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		}),
	)
}

func (s *screen) emptyHint(gtx context, message string) dimensions {
	return layout.Center.Layout(gtx, func(gtx context) dimensions {
		label := material.Caption(s.theme, message)
		label.Color = s.textMuted()
		return label.Layout(gtx)
	})
}

func (s *screen) sectionTitle(gtx context, title string) dimensions {
	label := material.Body2(s.theme, title)
	label.Color = s.theme.Palette.Fg
	label.TextSize = unit.Sp(13)
	return label.Layout(gtx)
}

func (s *screen) sectionLabel(gtx context, title string) dimensions {
	label := material.Caption(s.theme, title)
	label.Color = s.textMuted()
	return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(2), Left: unit.Dp(6)}.Layout(gtx, label.Layout)
}

// sectionCard 为工作区各栏提供统一内边距，视觉上对齐连接侧栏的紧凑节奏。
func (s *screen) sectionCard(gtx context, child layout.Widget) dimensions {
	return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10), Left: unit.Dp(12), Right: unit.Dp(8)}.Layout(gtx, child)
}

func (s *screen) layoutResultPane(gtx context) dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(s.layoutResultPaneTabs),
		layout.Flexed(1, func(gtx context) dimensions {
			switch s.resultPane {
			case resultPaneTable:
				if s.selectedSchemaTable != "" {
					return s.layoutTableData(gtx)
				}
				s.resultPane = resultPaneQuery
			}
			return s.layoutQueryResults(gtx)
		}),
	)
}

func (s *screen) layoutQueryResults(gtx context) dimensions {
	return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx context) dimensions {
		return s.layoutResultsBody(gtx, false)
	})
}

func (s *screen) layoutResultPaneTabs(gtx context) dimensions {
	return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(2), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx context) dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx context) dimensions {
				label := "查询结果"
				if s.queryRunning {
					label = "查询结果 · 执行中…"
				}
				return s.tabButton(gtx, &s.resultTabQuery, label, s.resultPane == resultPaneQuery)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
			layout.Rigid(func(gtx context) dimensions {
				if s.selectedSchemaTable == "" {
					return dimensions{}
				}
				return s.tabButton(gtx, &s.resultTabTable, "表数据 · "+s.selectedSchemaTable, s.resultPane == resultPaneTable)
			}),
			layout.Flexed(1, func(gtx context) dimensions { return dimensions{} }),
			layout.Rigid(func(gtx context) dimensions {
				if s.databaseNames == nil && s.selectedSchemaTable == "" {
					return dimensions{}
				}
				return s.smallButton(gtx, &s.detailsToggle, "刷新数据库列表", false)
			}),
		)
	})
}

func (s *screen) tabButton(gtx context, click *widget.Clickable, label string, active bool) dimensions {
	style := material.Button(s.theme, click, label)
	style.TextSize = unit.Sp(12)
	style.Inset = layout.Inset{Left: unit.Dp(10), Right: unit.Dp(10), Top: unit.Dp(4), Bottom: unit.Dp(4)}
	style.CornerRadius = unit.Dp(4)
	if !active {
		style.Background = color.NRGBA{}
		style.Color = s.textSecondary()
	}
	return style.Layout(gtx)
}

func (s *screen) textSecondary() color.NRGBA {
	foreground := s.theme.Palette.Fg
	background := s.theme.Palette.Bg
	if luminance(foreground) < 0.5 {
		return mixColor(foreground, background, 0.38)
	}
	return mixColor(foreground, background, 0.42)
}

func (s *screen) textMuted() color.NRGBA {
	foreground := s.theme.Palette.Fg
	background := s.theme.Palette.Bg
	if luminance(foreground) < 0.5 {
		return mixColor(foreground, background, 0.62)
	}
	return mixColor(foreground, background, 0.58)
}

func (s *screen) accentColor() color.NRGBA {
	return s.theme.Palette.ContrastBg
}

func (s *screen) accentSoftColor() color.NRGBA {
	background := s.theme.Palette.Bg
	if luminance(background) < 0.5 {
		return mixColor(background, s.theme.Palette.ContrastBg, 0.18)
	}
	return mixColor(background, s.theme.Palette.ContrastBg, 0.08)
}

func (s *screen) hoverColor() color.NRGBA {
	background := s.theme.Palette.Bg
	if luminance(background) < 0.5 {
		return mixColor(background, color.NRGBA{R: 255, G: 255, B: 255, A: 255}, 0.06)
	}
	return mixColor(background, color.NRGBA{R: 15, G: 23, B: 42, A: 255}, 0.04)
}

func (s *screen) borderColor() color.NRGBA {
	background := s.theme.Palette.Bg
	if luminance(background) < 0.5 {
		return mixColor(background, color.NRGBA{R: 255, G: 255, B: 255, A: 255}, 0.12)
	}
	return mixColor(background, color.NRGBA{R: 15, G: 23, B: 42, A: 255}, 0.10)
}

// surfaceMuted 用于小按钮与次级面板底色，深浅主题下都只比背景略微偏移。
func (s *screen) surfaceMuted() color.NRGBA {
	background := s.theme.Palette.Bg
	if luminance(background) < 0.5 {
		return mixColor(background, color.NRGBA{R: 255, G: 255, B: 255, A: 255}, 0.08)
	}
	return mixColor(background, color.NRGBA{R: 15, G: 23, B: 42, A: 255}, 0.05)
}

func (s *screen) successColor() color.NRGBA {
	if luminance(s.theme.Palette.Bg) < 0.5 {
		return color.NRGBA{R: 74, G: 222, B: 148, A: 255}
	}
	return color.NRGBA{R: 22, G: 138, B: 94, A: 255}
}

func (s *screen) warningColor() color.NRGBA {
	if luminance(s.theme.Palette.Bg) < 0.5 {
		return color.NRGBA{R: 246, G: 190, B: 92, A: 255}
	}
	return color.NRGBA{R: 183, G: 118, B: 20, A: 255}
}

func (s *screen) dangerColor() color.NRGBA {
	if luminance(s.theme.Palette.Bg) < 0.5 {
		return color.NRGBA{R: 248, G: 113, B: 113, A: 255}
	}
	return color.NRGBA{R: 197, G: 60, B: 60, A: 255}
}

func luminance(value color.NRGBA) float32 {
	return (0.299*float32(value.R) + 0.587*float32(value.G) + 0.114*float32(value.B)) / 255
}

// mixColor 按比例把 overlay 混入 base，ratio 为 overlay 的占比。
func mixColor(base, overlay color.NRGBA, ratio float32) color.NRGBA {
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	blend := func(a, b uint8) uint8 {
		return uint8(float32(a)*(1-ratio) + float32(b)*ratio)
	}
	return color.NRGBA{R: blend(base.R, overlay.R), G: blend(base.G, overlay.G), B: blend(base.B, overlay.B), A: 255}
}

func (s *screen) selectedConnectionName() string {
	for _, connection := range s.connections {
		if connection.ID == s.selectedConnectionID {
			return connection.Name
		}
	}
	return ""
}

// connectionStatus 返回连接状态文字与颜色：已连接、连接中、失败、未连接。
func (s *screen) connectionStatus(connection config.Connection) (string, color.NRGBA) {
	if !s.treeConnected {
		if connection.ID == s.selectedConnectionID {
			return "连接中…", s.warningColor()
		}
		return "未连接", s.textMuted()
	}
	if connection.ID == s.selectedConnectionID {
		return "已连接", s.successColor()
	}
	return "未连接", s.textMuted()
}

// connectionListItem 渲染连接列表中的一行：状态圆点、名称、目标地址，连接成功后追加库表数量。
func (s *screen) connectionListItem(gtx context, click *widget.Clickable, connection config.Connection, active bool) dimensions {
	if click.Clicked(gtx) {
		s.connectConnection(connection)
	}
	status, statusColor := s.connectionStatus(connection)
	target := fmt.Sprintf("%s:%d · %s", connection.Host, connection.Port, connection.Protocol)
	if connection.Protocol == "" {
		target = fmt.Sprintf("%s:%d", connection.Host, connection.Port)
	}
	return click.Layout(gtx, func(gtx context) dimensions {
		background := color.NRGBA{}
		if active {
			background = s.accentSoftColor()
		} else if click.Hovered() {
			background = s.hoverColor()
		}
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx context) dimensions {
				size := gtx.Constraints.Min
				fillRoundedRect(gtx, size, unit.Dp(6), background)
				return dimensions{Size: size}
			}),
			layout.Stacked(func(gtx context) dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6), Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, func(gtx context) dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx context) dimensions {
							return statusDot(gtx, statusColor)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						layout.Flexed(1, func(gtx context) dimensions {
							name := material.Body2(s.theme, connection.Name)
							name.Color = s.theme.Palette.Fg
							name.MaxLines = 1
							detail := material.Caption(s.theme, target)
							detail.Color = s.textMuted()
							detail.MaxLines = 1
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(name.Layout),
								layout.Rigid(detail.Layout),
							)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
						layout.Rigid(func(gtx context) dimensions {
							label := material.Caption(s.theme, status)
							label.Color = statusColor
							return label.Layout(gtx)
						}),
					)
				})
			}),
		)
	})
}

func (s *screen) listRefreshButton(gtx context, click *widget.Clickable, label string) dimensions {
	if click.Clicked(gtx) {
		if label == "刷新连接" {
			s.connectSelected()
		}
	}
	return s.smallButton(gtx, click, label, false)
}

func (s *screen) smallButton(gtx context, click *widget.Clickable, label string, disabled bool) dimensions {
	style := material.Button(s.theme, click, label)
	style.TextSize = unit.Sp(11)
	style.Inset = layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(3), Bottom: unit.Dp(3)}
	style.CornerRadius = unit.Dp(4)
	style.Background = s.surfaceMuted()
	style.Color = s.textSecondary()
	if disabled {
		style.Color = s.textMuted()
	}
	return style.Layout(gtx)
}

// primaryButton 用于主操作（执行查询），用强调色区分于普通小按钮。
func (s *screen) primaryButton(gtx context, click *widget.Clickable, label string, disabled bool) dimensions {
	style := material.Button(s.theme, click, label)
	style.TextSize = unit.Sp(12)
	style.Inset = layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12), Top: unit.Dp(5), Bottom: unit.Dp(5)}
	style.CornerRadius = unit.Dp(4)
	style.Background = s.accentColor()
	style.Color = s.theme.Palette.ContrastFg
	if disabled {
		style.Background = s.surfaceMuted()
		style.Color = s.textSecondary()
	}
	return style.Layout(gtx)
}

// treeRowOptions 描述资源树中一行的展示内容。
type treeRowOptions struct {
	label  string
	icon   string
	detail string
	active bool
	level  int
}

// treeRow 渲染导航树单行：图标、名称、次要信息，选中行使用强调底色，
// 行内文字全部单行截断，避免长表名把侧栏撑破。
func (s *screen) treeRow(gtx context, click *widget.Clickable, options treeRowOptions, onClick func()) dimensions {
	return click.Layout(gtx, func(gtx context) dimensions {
		background := color.NRGBA{}
		if options.active {
			background = s.accentSoftColor()
		} else if click.Hovered() {
			background = s.hoverColor()
		}
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx context) dimensions {
				size := gtx.Constraints.Min
				fillRoundedRect(gtx, size, unit.Dp(4), background)
				return dimensions{Size: size}
			}),
			layout.Stacked(func(gtx context) dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				indent := unit.Dp(4 + options.level*14)
				return layout.Inset{Top: unit.Dp(3), Bottom: unit.Dp(3), Left: indent, Right: unit.Dp(6)}.Layout(gtx, func(gtx context) dimensions {
					children := make([]layout.FlexChild, 0, 4)
					if options.icon != "" {
						children = append(children, layout.Rigid(func(gtx context) dimensions {
							return treeIcon(gtx, s.theme, options.icon, options.active, s)
						}), layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout))
					}
					children = append(children, layout.Rigid(func(gtx context) dimensions {
						label := material.Body2(s.theme, options.label)
						label.Color = s.theme.Palette.Fg
						if options.level > 0 {
							label.TextSize = unit.Sp(12)
							label.Color = s.textSecondary()
						}
						label.MaxLines = 1
						return label.Layout(gtx)
					}))
					if options.detail != "" {
						children = append(children, layout.Flexed(1, func(gtx context) dimensions {
							detail := material.Caption(s.theme, options.detail)
							detail.Color = s.textMuted()
							detail.MaxLines = 1
							detail.Alignment = end
							return detail.Layout(gtx)
						}))
					}
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
				})
			}),
		)
	})
}

func (s *screen) treeMessage(gtx context, message string) dimensions {
	label := material.Caption(s.theme, message)
	label.Color = s.textMuted()
	label.MaxLines = 1
	return layout.Inset{Top: unit.Dp(3), Bottom: unit.Dp(3), Left: unit.Dp(18), Right: unit.Dp(6)}.Layout(gtx, label.Layout)
}

func (s *screen) layoutSidebarFooter(gtx context) dimensions {
	return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(4), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx context) dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx context) dimensions {
				return s.smallButton(gtx, &s.historyToggle, "查询历史", false)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx context) dimensions {
				return s.smallButton(gtx, &s.savedSQLToggle, "收藏 SQL", false)
			}),
		)
	})
}

func databaseIcon(expanded bool) string {
	if expanded {
		return "▾"
	}
	return "▸"
}

func databaseRowDetail(tables, views int, loaded, loading bool) string {
	if loading && !loaded {
		return "加载中…"
	}
	if !loaded {
		return ""
	}
	return fmt.Sprintf("%d 表 · %d 视图", tables, views)
}

// statusDot 绘制连接状态圆点，用于区分已连接、连接中与未连接。
func statusDot(gtx context, fill color.NRGBA) dimensions {
	size := gtx.Dp(6)
	area := clip.Ellipse(image.Rect(0, 0, size, size)).Push(gtx.Ops)
	paint.Fill(gtx.Ops, fill)
	area.Pop()
	return dimensions{Size: image.Pt(size, size)}
}

var end = text.End

func treeIcon(gtx context, theme *material.Theme, icon string, active bool, s *screen) dimensions {
	label := material.Caption(theme, icon)
	if active {
		label.Color = s.accentColor()
	} else {
		label.Color = s.textMuted()
	}
	return label.Layout(gtx)
}

func fillRoundedRect(gtx context, size image.Point, radius unit.Dp, background color.NRGBA) {
	if background.A == 0 || size.X <= 0 || size.Y <= 0 {
		return
	}
	r := gtx.Dp(radius)
	if limit := min(size.X, size.Y) / 2; r > limit {
		r = limit
	}
	if r <= 0 {
		rect := clip.Rect{Max: size}.Push(gtx.Ops)
		paint.Fill(gtx.Ops, background)
		rect.Pop()
		return
	}
	rr := clip.RRect{Rect: image.Rectangle{Max: size}, NE: r, NW: r, SE: r, SW: r}
	area := rr.Push(gtx.Ops)
	paint.Fill(gtx.Ops, background)
	area.Pop()
}

// sidebarFooter 固定在导航栏底部，显示当前连接摘要，替代原先散落在内容区的提示。
func (s *screen) sidebarFooter(gtx context) dimensions {
	return layout.Inset{Top: unit.Dp(8), Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, func(gtx context) dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx context) dimensions {
				size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(1))
				fillRoundedRect(gtx, size, unit.Dp(0), s.borderColor())
				return dimensions{Size: size}
			}),
			layout.Rigid(func(gtx context) dimensions {
				return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx context) dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx context) dimensions {
							return statusDot(gtx, s.connectionDotColor())
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
						layout.Flexed(1, func(gtx context) dimensions {
							label := material.Caption(s.theme, s.sidebarSummary())
							label.Color = s.textMuted()
							label.MaxLines = 1
							return label.Layout(gtx)
						}),
					)
				})
			}),
		)
	})
}

func (s *screen) connectionDotColor() color.NRGBA {
	if s.treeConnected {
		return s.successColor()
	}
	if s.selectedConnectionID == "" {
		return s.textMuted()
	}
	return s.dangerColor()
}

func (s *screen) sidebarSummary() string {
	if s.selectedConnectionName() == "" {
		return "未选择连接"
	}
	if !s.treeConnected {
		if s.connectFailed {
			return "连接失败"
		}
		return "未连接"
	}
	return fmt.Sprintf("已连接 · %d 个数据库", len(s.databaseNames))
}

func adjustEditorRatio(current, delta float32, workspaceHeight int) float32 {
	if workspaceHeight <= 0 {
		return clampEditorRatio(current)
	}
	return clampEditorRatio(current + delta/float32(workspaceHeight))
}

func clampEditorRatio(value float32) float32 {
	if value < 0.35 {
		return 0.35
	}
	if value > 0.80 {
		return 0.80
	}
	return value
}

func (s *screen) layoutSplitControls(gtx context) dimensions {
	// 用固定高度的命中区域承载 Gio 拖拽手势，拖动增量按工作区高度换算为比例。
	height := gtx.Dp(22)
	gtx.Constraints.Min.Y = height
	gtx.Constraints.Max.Y = height
	area := clip.Rect(image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)).Push(gtx.Ops)
	pointer.CursorRowResize.Add(gtx.Ops)
	s.splitDrag.Add(gtx.Ops)
	area.Pop()

	for {
		event, ok := s.splitDrag.Update(gtx.Metric, gtx.Source, gesture.Vertical)
		if !ok {
			break
		}
		if event.Kind == pointer.Drag {
			if !s.dragActive {
				// Drag 手势首次达到阈值时建立基准点，后续帧累计位移。
				s.lastDragY = event.Position.Y
				s.dragActive = true
			} else if s.workspaceHeight > 0 {
				delta := event.Position.Y - s.lastDragY
				s.editorRatio = adjustEditorRatio(s.editorRatio, delta, s.workspaceHeight)
				s.lastDragY = event.Position.Y
			}
		}
	}
	if !s.splitDrag.Pressed() {
		if s.dragActive {
			s.dragActive = false
			s.saveWorkspaceLayout()
		}
	}

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx context) dimensions {
			return material.Caption(s.theme, "⠿ 拖动调整编辑器 / 结果区比例").Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
		layout.Rigid(func(gtx context) dimensions {
			return material.Caption(s.theme, fmt.Sprintf("%.0f%% / %.0f%%", s.editorRatio*100, (1-s.editorRatio)*100)).Layout(gtx)
		}),
	)
}

func (s *screen) layoutEditor(gtx context) dimensions {
	return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(8), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx context) dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx context) dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx context) dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx context) dimensions {
								return s.sectionTitle(gtx, "SQL 编辑器")
							}),
							layout.Rigid(func(gtx context) dimensions {
								if s.activeTabID == "" {
									return dimensions{}
								}
								index := activeTabIndex(s.tabs, s.activeTabID)
								if index < 0 {
									return dimensions{}
								}
								label := material.Caption(s.theme, s.tabs[index].Title)
								label.Color = s.textMuted()
								label.MaxLines = 1
								return label.Layout(gtx)
							}),
						)
					}),
					layout.Flexed(1, func(gtx context) dimensions { return dimensions{} }),
					layout.Rigid(func(gtx context) dimensions {
						label := "执行 SQL"
						if s.queryRunning {
							label = "执行中…"
						}
						return s.primaryButton(gtx, &s.execute, label, s.queryRunning)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx context) dimensions {
						return s.smallButton(gtx, &s.stop, "停止", !s.queryRunning)
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
			layout.Rigid(s.layoutTabBar),
			layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
			layout.Flexed(1, func(gtx context) dimensions {
				return material.Editor(s.theme, &s.sqlEditor, "输入 ClickHouse SQL...").Layout(gtx)
			}),
			layout.Rigid(func(gtx context) dimensions {
				label := material.Caption(s.theme, "基础文本编辑验证：语法高亮、补全和大 SQL 性能尚未实现")
				label.Color = s.textMuted()
				return label.Layout(gtx)
			}),
		)
	})
}

func (s *screen) layoutTabBar(gtx context) dimensions {
	s.tabList.Axis = layout.Horizontal
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx context) dimensions {
			return material.List(s.theme, &s.tabList).Layout(gtx, len(s.tabs), func(gtx context, index int) dimensions {
				tab := s.tabs[index]
				return layout.Inset{Right: unit.Dp(5)}.Layout(gtx, func(gtx context) dimensions {
					if s.renameTabID == tab.ID {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx context) dimensions {
								width := gtx.Dp(140)
								if width > gtx.Constraints.Max.X {
									width = gtx.Constraints.Max.X
								}
								gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
								return material.Editor(s.theme, &s.tabTitleEditor, "标签名称").Layout(gtx)
							}),
							layout.Rigid(func(gtx context) dimensions {
								d := material.Button(s.theme, &s.tabRenameSave, "保存").Layout(gtx)
								for s.tabRenameSave.Clicked(gtx) {
									s.commitTabRename()
								}
								return d
							}),
							layout.Rigid(func(gtx context) dimensions {
								d := material.Button(s.theme, &s.tabRenameCancel, "取消").Layout(gtx)
								for s.tabRenameCancel.Clicked(gtx) {
									s.renameTabID = ""
								}
								return d
							}),
						)
					}
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx context) dimensions {
							label := tab.Title
							if tab.ID == s.activeTabID {
								label = "● " + label
							}
							area := clip.Rect(image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)).Push(gtx.Ops)
							pointer.CursorGrab.Add(gtx.Ops)
							s.tabDrags[index].Add(gtx.Ops)
							area.Pop()
							d := material.Button(s.theme, &s.tabClicks[index], label).Layout(gtx)
							for s.tabClicks[index].Clicked(gtx) {
								s.activateTab(tab.ID)
							}
							if s.updateTabDrag(gtx, index) {
								return d
							}
							return d
						}),
						layout.Rigid(func(gtx context) dimensions {
							d := material.Button(s.theme, &s.tabRenameClicks[index], "改名").Layout(gtx)
							for s.tabRenameClicks[index].Clicked(gtx) {
								s.beginTabRename(tab.ID)
							}
							return d
						}),
						layout.Rigid(func(gtx context) dimensions {
							d := material.Button(s.theme, &s.tabCloseClicks[index], "关闭").Layout(gtx)
							if s.tabCloseClicks[index].Clicked(gtx) {
								s.closeTab(tab.ID)
							}
							return d
						}),
					)
				})
			})
		}),
		layout.Rigid(func(gtx context) dimensions {
			if s.pendingCloseTabID == "" {
				return dimensions{}
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx context) dimensions {
					d := material.Button(s.theme, &s.confirmCloseTab, "确认丢弃并关闭").Layout(gtx)
					if s.confirmCloseTab.Clicked(gtx) {
						id := s.pendingCloseTabID
						s.pendingCloseTabID = ""
						s.closeTabNow(id)
					}
					return d
				}),
				layout.Rigid(func(gtx context) dimensions {
					d := material.Button(s.theme, &s.cancelCloseTab, "取消").Layout(gtx)
					if s.cancelCloseTab.Clicked(gtx) {
						s.pendingCloseTabID = ""
						s.status = "已取消关闭标签"
					}
					return d
				}),
			)
		}),
		layout.Rigid(func(gtx context) dimensions {
			d := material.Button(s.theme, &s.newTab, "+ 新建").Layout(gtx)
			for s.newTab.Clicked(gtx) {
				s.createTab()
			}
			return d
		}),
	)
}

func (s *screen) resetTabWidgets() {
	s.tabClicks = make([]widget.Clickable, len(s.tabs))
	s.tabRenameClicks = make([]widget.Clickable, len(s.tabs))
	s.tabCloseClicks = make([]widget.Clickable, len(s.tabs))
	s.tabDrags = make([]gesture.Drag, len(s.tabs))
	s.tabDragLastX = make([]float32, len(s.tabs))
	s.tabDragAccum = make([]float32, len(s.tabs))
	s.tabDragActive = make([]bool, len(s.tabs))
}

func (s *screen) beginTabRename(id string) {
	index := activeTabIndex(s.tabs, id)
	if index < 0 {
		return
	}
	s.renameTabID = id
	s.tabTitleEditor.SetText(s.tabs[index].Title)
}

func (s *screen) commitTabRename() {
	tabs, ok := renameTabState(s.tabs, s.renameTabID, s.tabTitleEditor.Text())
	if !ok {
		s.status = "标签名称不能为空"
		return
	}
	s.tabs = tabs
	s.renameTabID = ""
	s.persistTabs()
	s.status = "标签名称已更新"
}

func (s *screen) closeTab(id string) {
	if s.queryRunning && s.runningTabID == id {
		s.status = "该标签仍有查询或导出任务，完成或取消后才能关闭"
		return
	}
	if tabHasUnsavedChanges(s.tabs, s.activeTabID, id, s.sqlEditor.Text()) {
		s.pendingCloseTabID = id
		s.status = "当前 SQL 有未保存修改，请确认丢弃后关闭"
		return
	}
	s.closeTabNow(id)
}

func (s *screen) closeTabNow(id string) {
	if s.queryRunning && s.runningTabID == id {
		s.status = "该标签仍有查询或导出任务，完成或取消后才能关闭"
		return
	}
	s.saveActiveTabText()
	s.saveActiveTabRuntime()
	wasActive := s.activeTabID == id
	newID := fmt.Sprintf("query-%d", time.Now().UnixNano())
	tabs, activeID, ok := closeTabState(s.tabs, s.activeTabID, id, newID)
	if !ok {
		return
	}
	delete(s.tabRuntime, id)
	s.tabs, s.activeTabID = tabs, activeID
	s.resetTabWidgets()
	if wasActive {
		index := activeTabIndex(s.tabs, s.activeTabID)
		s.sqlEditor.SetText(s.tabs[index].SQL)
		s.restoreActiveTabRuntime()
	}
	if s.renameTabID == id {
		s.renameTabID = ""
	}
	if s.pendingCloseTabID == id {
		s.pendingCloseTabID = ""
	}
	s.persistTabs()
	s.status = "已关闭 SQL 标签"
}

func (s *screen) moveTab(from, to int) {
	tabs, ok := reorderTabState(s.tabs, from, to)
	if !ok {
		return
	}
	s.tabs = tabs
	s.resetTabWidgets()
	s.persistTabs()
}

func (s *screen) updateTabDrag(gtx context, index int) bool {
	if index < 0 || index >= len(s.tabDrags) {
		return false
	}
	moved := false
	for {
		event, ok := s.tabDrags[index].Update(gtx.Metric, gtx.Source, gesture.Horizontal)
		if !ok {
			break
		}
		if event.Kind != pointer.Drag {
			continue
		}
		if !s.tabDragActive[index] {
			s.tabDragLastX[index] = event.Position.X
			s.tabDragAccum[index] = 0
			s.tabDragActive[index] = true
			continue
		}
		delta := event.Position.X - s.tabDragLastX[index]
		s.tabDragAccum[index] += delta
		s.tabDragLastX[index] = event.Position.X
		threshold := float32(gtx.Dp(72))
		if s.tabDragAccum[index] >= threshold && index+1 < len(s.tabs) {
			s.moveTab(index, index+1)
			moved = true
			break
		}
		if s.tabDragAccum[index] <= -threshold && index > 0 {
			s.moveTab(index, index-1)
			moved = true
			break
		}
	}
	if index < len(s.tabDragActive) && !s.tabDrags[index].Pressed() {
		s.tabDragActive[index] = false
	}
	return moved
}

func (s *screen) saveActiveTabText() {
	if index := activeTabIndex(s.tabs, s.activeTabID); index >= 0 {
		s.tabs[index].SQL = s.sqlEditor.Text()
	}
}

func (s *screen) persistTabs() {
	if s.service == nil {
		return
	}
	s.saveActiveTabText()
	if err := s.service.SaveTabs(config.TabsFile{Tabs: s.tabs, ActiveTabID: s.activeTabID}); err != nil {
		s.status = "保存 SQL 标签失败：" + err.Error()
	}
}

func (s *screen) saveActiveTabRuntime() {
	if s.tabRuntime == nil || s.activeTabID == "" {
		return
	}
	s.tabRuntime[s.activeTabID] = tabRuntimeState{result: s.queryResult, lastSQL: s.lastExecutedSQL, connectionID: s.lastExecutedConnectionID}
}

func (s *screen) restoreActiveTabRuntime() {
	state := s.tabRuntime[s.activeTabID]
	s.queryResult, s.lastExecutedSQL, s.lastExecutedConnectionID = state.result, state.lastSQL, state.connectionID
	s.resetResultWidgets(state.result)
}

func (s *screen) resetResultWidgets(result *clickhouse.QueryResult) {
	s.resultList.Position = layout.Position{}
	s.resultColumnHeader.Position = layout.Position{}
	s.resultColumnRow.Position = layout.Position{}
	s.resultCellClicks, s.resultRowCopyClicks = nil, nil
	s.resultSortClicks = nil
	s.resultSortColumn, s.resultSortDescending = -1, false
	s.resultFilter.SetText("")
	s.resultFilterLast = ""
	s.resultColumnFilterEditors, s.resultColumnFilterLast = nil, nil
	s.resultColumnFilterClearClicks = nil
	s.resultColumnWidths, s.resultColumnDrags = nil, nil
	s.resultColumnDragLastX, s.resultColumnDragActive = nil, nil
	s.selectedCellLabel, s.selectedCellValue, s.cellDetailLines = "", "", nil
	s.cellDetailList.Position = layout.Position{}
	if result == nil {
		return
	}
	s.resultColumnWidths = make([]unit.Dp, len(result.Columns))
	for i, weight := range resultColumnWeights(result.Columns, result.Rows) {
		s.resultColumnWidths[i] = resultColumnWidth(weight)
	}
	s.resultColumnDrags = make([]gesture.Drag, len(result.Columns))
	s.resultColumnDragLastX = make([]float32, len(result.Columns))
	s.resultColumnDragActive = make([]bool, len(result.Columns))
	s.resultSortClicks = make([]widget.Clickable, len(result.Columns))
	s.resultColumnFilterEditors = make([]widget.Editor, len(result.Columns))
	s.resultColumnFilterClearClicks = make([]widget.Clickable, len(result.Columns))
	s.resultColumnFilterLast = make([]string, len(result.Columns))
	for i := range s.resultColumnFilterEditors {
		s.resultColumnFilterEditors[i].SingleLine = true
	}
	s.resultCellClicks = make([][]widget.Clickable, len(result.Rows))
	s.resultRowCopyClicks = make([]widget.Clickable, len(result.Rows))
	for i := range result.Rows {
		s.resultCellClicks[i] = make([]widget.Clickable, len(result.Columns))
	}
}

func (s *screen) activateTab(id string) {
	if id == s.activeTabID {
		return
	}
	s.saveActiveTabText()
	s.saveActiveTabRuntime()
	index := activeTabIndex(s.tabs, id)
	if index < 0 {
		return
	}
	s.activeTabID = id
	s.sqlEditor.SetText(s.tabs[index].SQL)
	s.restoreActiveTabRuntime()
	s.persistTabs()
	s.status = "已切换到 " + s.tabs[index].Title
}

func (s *screen) createTab() {
	if len(s.tabs) >= config.TabMaxCount {
		s.status = fmt.Sprintf("标签页最多支持 %d 个", config.TabMaxCount)
		return
	}
	s.saveActiveTabText()
	s.saveActiveTabRuntime()
	tab := nextTabState(s.tabs, fmt.Sprintf("query-%d", time.Now().UnixNano()))
	s.tabs = append(s.tabs, tab)
	s.resetTabWidgets()
	s.activeTabID = tab.ID
	s.sqlEditor.SetText("")
	s.queryResult = nil
	s.lastExecutedSQL = ""
	s.lastExecutedConnectionID = ""
	s.previewMode = false
	s.previewTabID, s.previewConnectionID = "", ""
	s.tabRuntime[tab.ID] = tabRuntimeState{}
	s.resetResultWidgets(nil)
	s.persistTabs()
	s.status = "已新建空白 SQL 标签"
}

func (s *screen) layoutResultsBody(gtx context, isTableMode bool) dimensions {
	children := []layout.FlexChild{
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
	}
	if s.queryRunning && s.runningTabID == s.activeTabID {
		children = append(children, layout.Rigid(func(gtx context) dimensions {
			return material.Body2(s.theme, "查询正在执行，可点击停止取消。").Layout(gtx)
		}))
	}
	if s.isCurrentTablePreview() {
		children = append(children, layout.Rigid(func(gtx context) dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx context) dimensions {
					d := material.Button(s.theme, &s.previewPrev, "上一页").Layout(gtx)
					if s.previewPrev.Clicked(gtx) {
						s.changePreviewPage(-1)
					}
					return d
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx context) dimensions {
					label := fmt.Sprintf("第 %d 页", s.previewPage+1)
					return material.Body2(s.theme, label).Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx context) dimensions {
					d := material.Button(s.theme, &s.previewNext, "下一页").Layout(gtx)
					if s.previewNext.Clicked(gtx) {
						s.changePreviewPage(1)
					}
					return d
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx context) dimensions {
					d := material.Button(s.theme, &s.previewPageSize, fmt.Sprintf("每页 %d 行", s.previewPageLimit)).Layout(gtx)
					if s.previewPageSize.Clicked(gtx) {
						s.cyclePreviewPageSize()
					}
					return d
				}),
			)
		}))
	}
	if s.queryResult == nil {
		children = append(children, layout.Flexed(1, func(gtx context) dimensions {
			message := "在左侧选择一张表，或用「表数据」页查看已预览的数据"
			if !s.treeConnected {
				message = "请先连接 ClickHouse，再执行 SQL"
			}
			if isTableMode {
				message = "在左侧导航中选择一张表并点击「预览数据」"
			}
			return material.Body1(s.theme, message).Layout(gtx)
		}))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	result := s.queryResult
	meta := fmt.Sprintf("%d 行 · %.2f ms · 上限 %d 行", result.RowCount, result.ElapsedMs, result.RowLimit)
	if result.Truncated {
		meta += " · 结果已截断 · 导出仅包含当前已加载行"
	}
	if result.Error != "" {
		children = append(children, layout.Rigid(func(gtx context) dimensions {
			return material.Body2(s.theme, "执行错误："+result.Error).Layout(gtx)
		}))
	}
	if len(result.Columns) > 0 && result.Error == "" {
		if s.exportCSV.Clicked(gtx) {
			s.startExport("csv")
		}
		if s.exportTSV.Clicked(gtx) {
			s.startExport("tsv")
		}
		children = append(children, layout.Rigid(func(gtx context) dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx context) dimensions { return material.Caption(s.theme, meta).Layout(gtx) }),
				layout.Rigid(func(gtx context) dimensions {
					label := "导出 CSV"
					if s.exportRunning {
						label = "导出中…"
					}
					return material.Button(s.theme, &s.exportCSV, label).Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx context) dimensions {
					label := "导出 TSV"
					if s.exportRunning {
						label = "导出中…"
					}
					return material.Button(s.theme, &s.exportTSV, label).Layout(gtx)
				}),
			)
		}))
	} else {
		children = append(children, layout.Rigid(func(gtx context) dimensions { return material.Caption(s.theme, meta).Layout(gtx) }))
	}
	if len(result.Columns) == 0 {
		children = append(children, layout.Flexed(1, func(gtx context) dimensions {
			return material.Body2(s.theme, "没有可展示的结果列").Layout(gtx)
		}))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	columnWeights := resultColumnWeights(result.Columns, result.Rows)
	filterText := s.resultFilter.Text()
	if filterText != s.resultFilterLast {
		s.resultFilterLast = filterText
		s.resultList.Position = layout.Position{}
	}
	columnFilters := make([]string, len(s.resultColumnFilterEditors))
	for i := range s.resultColumnFilterEditors {
		columnFilters[i] = s.resultColumnFilterEditors[i].Text()
		if i >= len(s.resultColumnFilterLast) || columnFilters[i] != s.resultColumnFilterLast[i] {
			if i < len(s.resultColumnFilterLast) {
				s.resultColumnFilterLast[i] = columnFilters[i]
			}
			s.resultList.Position = layout.Position{}
		}
	}
	displayRows := buildResultRowViewWithColumnFilters(result.Rows, filterText, columnFilters, s.resultSortColumn, s.resultSortDescending)
	children = append(children, layout.Rigid(func(gtx context) dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx context) dimensions {
				return material.Editor(s.theme, &s.resultFilter, "筛选当前结果（匹配任意列）").Layout(gtx)
			}),
			layout.Rigid(func(gtx context) dimensions {
				d := material.Button(s.theme, &s.resultFilterClear, "清除关键字").Layout(gtx)
				for s.resultFilterClear.Clicked(gtx) {
					s.resultFilter.SetText("")
				}
				return d
			}),
			layout.Rigid(func(gtx context) dimensions {
				d := material.Button(s.theme, &s.resultColumnFilterClear, "清除列条件").Layout(gtx)
				for s.resultColumnFilterClear.Clicked(gtx) {
					for i := range s.resultColumnFilterEditors {
						s.resultColumnFilterEditors[i].SetText("")
						s.resultColumnFilterLast[i] = ""
					}
					s.resultList.Position = layout.Position{}
				}
				return d
			}),
			layout.Rigid(func(gtx context) dimensions {
				return material.Caption(s.theme, fmt.Sprintf("显示 %d / %d 行", len(displayRows), len(result.Rows))).Layout(gtx)
			}),
		)
	}))
	children = append(children, layout.Rigid(func(gtx context) dimensions {
		return material.Caption(s.theme, "列条件：默认包含；!不包含；= 等于；!= 不等于；>、<、>=、<= 数值或时间比较；between:下界..上界 闭区间；时间可用 YYYY-MM-DD / DateTime / RFC3339；is:null / is:not-null 判断空值").Layout(gtx)
	}))
	if summaries := resultColumnFilterSummary(result.Columns, columnFilters); len(summaries) > 0 {
		children = append(children, layout.Rigid(func(gtx context) dimensions {
			return material.Caption(s.theme, "生效条件："+strings.Join(summaries, "；")).Layout(gtx)
		}))
	}
	children = append(children, layout.Rigid(func(gtx context) dimensions {
		s.resultColumnHeader.Axis = layout.Horizontal
		return material.List(s.theme, &s.resultColumnHeader).Layout(gtx, len(result.Columns)+1, func(gtx context, index int) dimensions {
			if index == 0 {
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(80), gtx.Dp(80)
				return material.Body2(s.theme, "操作").Layout(gtx)
			}
			columnIndex := index - 1
			width := resultColumnWidth(columnWeights[columnIndex])
			if columnIndex < len(s.resultColumnWidths) && s.resultColumnWidths[columnIndex] > 0 {
				width = s.resultColumnWidths[columnIndex]
			}
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(width), gtx.Dp(width)
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx context) dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx context) dimensions {
							d := s.resultSortClicks[columnIndex].Layout(gtx, func(gtx context) dimensions {
								return material.Body2(s.theme, resultColumnLabel(result.Columns, columnIndex, s.resultSortColumn, s.resultSortDescending)).Layout(gtx)
							})
							for s.resultSortClicks[columnIndex].Clicked(gtx) {
								if s.resultSortColumn == columnIndex {
									s.resultSortDescending = !s.resultSortDescending
								} else {
									s.resultSortColumn, s.resultSortDescending = columnIndex, false
								}
								s.resultList.Position = layout.Position{}
							}
							return d
						}),
						layout.Rigid(func(gtx context) dimensions { return s.layoutResultColumnResizeHandle(gtx, columnIndex) }),
					)
				}),
				layout.Rigid(func(gtx context) dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx context) dimensions {
							return material.Editor(s.theme, &s.resultColumnFilterEditors[columnIndex], "筛选 "+result.Columns[columnIndex].Name).Layout(gtx)
						}),
						layout.Rigid(func(gtx context) dimensions {
							d := material.Button(s.theme, &s.resultColumnFilterClearClicks[columnIndex], "清除").Layout(gtx)
							for s.resultColumnFilterClearClicks[columnIndex].Clicked(gtx) {
								s.resultColumnFilterEditors[columnIndex].SetText("")
								if columnIndex < len(s.resultColumnFilterLast) {
									s.resultColumnFilterLast[columnIndex] = ""
								}
								s.resultList.Position = layout.Position{}
							}
							return d
						}),
					)
				}),
			)
		})
	}))
	children = append(children, layout.Flexed(1, func(gtx context) dimensions {
		s.resultList.Axis = layout.Vertical
		return material.List(s.theme, &s.resultList).Layout(gtx, len(displayRows), func(gtx context, index int) dimensions {
			rowIndex := displayRows[index].Index
			row := result.Rows[rowIndex]
			s.resultColumnRow.Axis = layout.Horizontal
			s.resultColumnRow.Position = s.resultColumnHeader.Position
			dims := s.resultColumnRow.List.Layout(gtx, len(result.Columns)+1, func(gtx context, columnIndex int) dimensions {
				if columnIndex == 0 {
					gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(80), gtx.Dp(80)
					return s.resultRowCopyClicks[rowIndex].Layout(gtx, func(gtx context) dimensions {
						return material.Caption(s.theme, "复制行").Layout(gtx)
					})
				}
				cellIndex := columnIndex - 1
				cellText := "NULL"
				if cellIndex < len(row) {
					cellText = formatResultCell(row[cellIndex])
				}
				width := resultColumnWidth(columnWeights[cellIndex])
				if cellIndex < len(s.resultColumnWidths) && s.resultColumnWidths[cellIndex] > 0 {
					width = s.resultColumnWidths[cellIndex]
				}
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(width), gtx.Dp(width)
				return s.resultCellClicks[rowIndex][cellIndex].Layout(gtx, func(gtx context) dimensions {
					return layout.Inset{Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx context) dimensions {
						return material.Caption(s.theme, cellText).Layout(gtx)
					})
				})
			})
			for s.resultRowCopyClicks[rowIndex].Clicked(gtx) {
				gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(formatResultRow(row)))})
				s.status = fmt.Sprintf("已复制第 %d 行（制表符分隔）", rowIndex+1)
			}
			for cellIndex := range row {
				if cellIndex < len(s.resultCellClicks[rowIndex]) && s.resultCellClicks[rowIndex][cellIndex].Clicked(gtx) {
					gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(formatClipboardCell(row[cellIndex])))})
					s.selectedCellLabel = fmt.Sprintf("第 %d 行 · %s", rowIndex+1, result.Columns[cellIndex].Name)
					s.selectedCellValue = formatClipboardCell(row[cellIndex])
					s.cellDetailLines = splitCellDetailLines(s.selectedCellValue)
					s.cellDetailList.Position = layout.Position{}
					s.status = fmt.Sprintf("已复制第 %d 行第 %d 列，并显示完整内容", rowIndex+1, cellIndex+1)
				}
			}
			s.resultColumnHeader.Position = s.resultColumnRow.Position
			return dims
		})
	}))
	if len(result.Rows) == 0 && result.Error == "" {
		children = append(children, layout.Rigid(func(gtx context) dimensions {
			return material.Caption(s.theme, "查询成功，但没有返回数据行").Layout(gtx)
		}))
	}
	if s.selectedCellLabel != "" {
		children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout))
		children = append(children, layout.Rigid(func(gtx context) dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx context) dimensions {
					return material.Body2(s.theme, "单元格详情 · "+s.selectedCellLabel).Layout(gtx)
				}),
				layout.Rigid(func(gtx context) dimensions {
					d := material.Button(s.theme, &s.copyCellDetail, "复制完整内容").Layout(gtx)
					for s.copyCellDetail.Clicked(gtx) {
						gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(s.selectedCellValue))})
						s.status = "已复制单元格完整内容"
					}
					return d
				}),
				layout.Rigid(func(gtx context) dimensions {
					d := material.Button(s.theme, &s.closeCellDetail, "关闭").Layout(gtx)
					for s.closeCellDetail.Clicked(gtx) {
						s.selectedCellLabel, s.selectedCellValue, s.cellDetailLines = "", "", nil
						s.cellDetailList.Position = layout.Position{}
					}
					return d
				}),
			)
		}))
		children = append(children, layout.Flexed(0.35, func(gtx context) dimensions {
			s.cellDetailList.Axis = layout.Vertical
			return material.List(s.theme, &s.cellDetailList).Layout(gtx, len(s.cellDetailLines), func(gtx context, index int) dimensions {
				return layout.Inset{Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx context) dimensions { return material.Caption(s.theme, s.cellDetailLines[index]).Layout(gtx) })
			})
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (s *screen) layoutStatus(gtx context) dimensions {
	return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx context) dimensions {
		return material.Caption(s.theme, s.status).Layout(gtx)
	})
}
