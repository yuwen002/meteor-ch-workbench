// Package gioapp 提供 Meteor CH Workbench 的 Gio 原生界面原型。
//
// 当前阶段已接入共享应用服务，支持真实连接测试、数据库列表以及表 / 视图导航；
// SQL 执行、取消和结果展示已接入共享应用服务。
package gioapp

import (
	"errors"
	"fmt"
	"os"
	"image"
	"image/color"
	"io"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/gesture"
	"gioui.org/io/clipboard"
	"gioui.org/io/pointer"
	"gioui.org/op/clip"
	"gioui.org/layout"
	"gioui.org/op"
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

// Run 运行 Gio 窗口事件循环。
func Run(window *app.Window, service *appsvc.Service) error {
	state := newScreen(window, service, gioexplorer.NewExplorer(window))
	var ops op.Ops
	for {
		evt := window.Event()
		state.explorer.ListenEvents(evt)
		switch event := evt.(type) {
		case app.DestroyEvent:
			return event.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, event)
			state.drainExportResults()
			state.layout(gtx)
			event.Frame(gtx.Ops)
		}
	}
}

type connectResult struct { connectionID string; sequence uint64; databases []string; err error }
type queryResult struct { queryID string; sequence uint64; result *clickhouse.QueryResult; err error }
type exportResult struct { format string; err error; canceled bool; rows int64; cleanupErr error; cleanupUnsupported bool }
type databaseResult struct { connectionID string; connectionSequence uint64; database string; requestID uint64; tables []clickhouse.TableInfo; views []clickhouse.TableInfo; err error }
type schemaResult struct { connectionID string; connectionSequence uint64; database string; table string; requestID uint64; columns []clickhouse.ColumnInfo; details *clickhouse.TableDetails; detailsErr error; err error }
type databaseNode struct { click widget.Clickable; expanded bool; loading bool; loaded bool; requestID uint64; tables []clickhouse.TableInfo; views []clickhouse.TableInfo; tableClicks map[string]*widget.Clickable; err error }

type screen struct {
	window *app.Window
	connectResults chan connectResult
	databaseResults chan databaseResult
	schemaResults chan schemaResult
	schemaSequence uint64
	schemaLoading bool
	schemaError string
	selectedSchemaDatabase string
	selectedSchemaTable string
	selectedSchema []clickhouse.ColumnInfo
	selectedTableDetails *clickhouse.TableDetails
	tableDetailsError string
	detailsOpen bool
	detailsToggle widget.Clickable
	copyDDL widget.Clickable
	schemaList widget.List
	detailsList widget.List
	ddlLines []string
	queryResults chan queryResult
	exportResults chan exportResult
	exportProgress chan int64
	explorer *gioexplorer.Explorer
	exportCSV widget.Clickable
	exportTSV widget.Clickable
	exportRunning bool
	querySequence uint64
	runningQueryID string
	queryRunning bool
	queryResult *clickhouse.QueryResult
	lastExecutedSQL string
	lastExecutedConnectionID string
	resultList widget.List
	resultColumnHeader widget.List
	resultColumnRow widget.List
	resultColumnWidths []unit.Dp
	resultColumnDrags []gesture.Drag
	resultColumnDragLastX []float32
	resultColumnDragActive []bool
	resultCellClicks [][]widget.Clickable
	resultRowCopyClicks []widget.Clickable
	connectionSequence uint64
	databaseSequence uint64
	databaseNames []string
	databaseNodes map[string]*databaseNode
	treeConnected bool
	service *appsvc.Service
	connections        []config.Connection
	connectionClicks   []widget.Clickable
	selectedConnectionID string
	theme       *material.Theme
	sqlEditor   widget.Editor
	execute     widget.Clickable
	connect     widget.Clickable
	stop        widget.Clickable
	themeToggle widget.Clickable
	sidebarToggle widget.Clickable
	editorSmaller widget.Clickable
	editorLarger widget.Clickable
	editorRatio float32
	splitDrag gesture.Drag
	lastDragY float32
	dragActive bool
	workspaceHeight int
	status      string
	lightTheme  bool
	sidebarOpen bool
}

func newScreen(window *app.Window, service *appsvc.Service, fileExplorer *gioexplorer.Explorer) *screen {
	s := &screen{
		window: window,
		explorer: fileExplorer,
		connectResults: make(chan connectResult, 4),
		databaseResults: make(chan databaseResult, 16),
		schemaResults: make(chan schemaResult, 8),
		queryResults: make(chan queryResult, 4),
		exportResults: make(chan exportResult, 2),
		exportProgress: make(chan int64, 1),
		databaseNodes: make(map[string]*databaseNode),
		service:     service,
		theme:       material.NewTheme(),
		status:      "尚未连接数据库 · 当前为 Gio 界面原型",
		lightTheme:  false,
		sidebarOpen: true,
		editorRatio: 0.62,
	}
	if service != nil {
		if settings, err := service.GetSettings(); err == nil {
			s.sidebarOpen = !settings.SidebarCollapsed
			s.editorRatio = float32(settings.EditorRatio)
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
	s.sqlEditor.SetText("SELECT\n    database,\n    table,\n    engine\nFROM system.tables\nORDER BY database, table\nLIMIT 100;")
	s.applyTheme()
	return s
}

// startExport 重新执行最近一次成功查询，并将 ClickHouse 游标结果直接流式写入文件。
func (s *screen) startExport(format string) {
	if s.exportRunning || s.queryResult == nil || s.service == nil || s.explorer == nil {
		return
	}
	if s.queryResult.Error != "" || len(s.queryResult.Columns) == 0 || strings.TrimSpace(s.lastExecutedSQL) == "" || s.lastExecutedConnectionID == "" {
		return
	}
	connectionID, sqlText := s.lastExecutedConnectionID, s.lastExecutedSQL
	queryID := fmt.Sprintf("gio-export-%d", time.Now().UnixNano())
	s.exportRunning = true
	s.runningQueryID, s.queryRunning = queryID, true
	s.status = "正在重新执行当前 SQL，并流式导出完整结果…"
	go func() {
		file, err := s.explorer.CreateFile("query-result." + format)
		if err != nil {
			s.exportResults <- exportResult{format: format, err: err, canceled: errors.Is(err, gioexplorer.ErrUserDecline)}
			s.window.Invalidate()
			return
		}
		rows, writeErr := s.service.ExportQueryToWithProgress(file, connectionID, sqlText, queryID, format, func(count int64) {
			select {
			case s.exportProgress <- count:
			default:
				select { case <-s.exportProgress: default: }
				select { case s.exportProgress <- count: default: }
			}
		})
		closeErr := file.Close()
		if writeErr != nil {
			err = writeErr
		} else if closeErr != nil {
			err = closeErr
		}
		result := exportResult{format: format, err: err, rows: rows}
		if err != nil {
			if localFile, ok := file.(*os.File); ok {
				if removeErr := os.Remove(localFile.Name()); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
					result.cleanupErr = removeErr
				}
			} else {
				result.cleanupUnsupported = true
			}
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
				s.status = fmt.Sprintf("正在流式导出… 已写入 %d 行（每 1000 行更新）", count)
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
			s.queryRunning, s.runningQueryID = false, ""
			switch {
			case result.canceled:
				s.status = fmt.Sprintf("已取消导出（已处理 %d 行）", result.rows)
			case result.err != nil:
				s.status = fmt.Sprintf("导出中断：%s（已处理 %d 行）", result.err.Error(), result.rows)
			}
			if result.cleanupErr != nil {
				s.status += "；删除不完整文件失败：" + result.cleanupErr.Error()
			} else if result.cleanupUnsupported && result.err != nil {
				s.status += "；当前平台无法自动删除不完整文件，请手动检查目标文件"
			}
			switch {
			case result.canceled:
				// 取消状态已在上方设置。
			case result.err != nil:
				// 错误状态已在上方设置。
			default:
				s.status = fmt.Sprintf("已成功流式导出 %d 行 %s 查询结果（已重新执行 SQL）", result.rows, strings.ToUpper(result.format))
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
			if result.connectionID != s.selectedConnectionID || result.sequence != s.connectionSequence { continue }
			if result.err != nil {
				s.treeConnected = false
				s.databaseNames = nil
				s.status = "连接或加载数据库失败：" + result.err.Error()
			} else {
				s.treeConnected = true
				s.databaseNames = result.databases
				s.databaseNodes = make(map[string]*databaseNode)
				for _, name := range result.databases { s.databaseNodes[name] = &databaseNode{} }
				s.status = fmt.Sprintf("连接成功，已加载 %d 个数据库", len(result.databases))
			}
		default:
			return
		}
	}
}

func (s *screen) toggleDatabase(name string, node *databaseNode) {
	node.expanded = !node.expanded
	if !node.expanded || node.loaded || node.loading { return }
	s.databaseSequence++
	requestID := s.databaseSequence
	node.requestID, node.loading, node.err = requestID, true, nil
	connectionID, connectionSequence := s.selectedConnectionID, s.connectionSequence
	go func() {
		tables, err := s.service.GetTables(connectionID, name)
		var views []clickhouse.TableInfo
		if err == nil { views, err = s.service.GetViews(connectionID, name) }
		s.databaseResults <- databaseResult{connectionID: connectionID, connectionSequence: connectionSequence, database: name, requestID: requestID, tables: tables, views: views, err: err}
		s.window.Invalidate()
	}()
}

func (s *screen) drainDatabaseResults() {
	for {
		select {
		case result := <-s.databaseResults:
			if result.connectionID != s.selectedConnectionID || result.connectionSequence != s.connectionSequence { continue }
			node := s.databaseNodes[result.database]
			if node == nil || node.requestID != result.requestID { continue }
			node.loading, node.err = false, result.err
			if result.err == nil {
				node.tables, node.views, node.loaded = result.tables, result.views, true
				node.tableClicks = make(map[string]*widget.Clickable)
				for _, table := range result.tables { node.tableClicks[table.Name] = &widget.Clickable{} }
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
			connectionID: connectionID,
			connectionSequence: connectionSequence,
			database: database,
			table: table,
			requestID: requestID,
			columns: columns,
			details: details,
			detailsErr: detailsErr,
			err: err,
		}
		s.window.Invalidate()
	}()
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
			if result.detailsErr != nil { s.tableDetailsError = result.detailsErr.Error() }
			s.status = fmt.Sprintf("已加载 %s.%s 的 %d 个字段", result.database, result.table, len(result.columns))
		default:
			return
		}
	}
}

func (s *screen) executeSQL() {
	if s.service == nil { s.status = "应用服务尚未初始化"; return }
	if !s.treeConnected || s.selectedConnectionID == "" { s.status = "请先连接 ClickHouse"; return }
	sqlText := strings.TrimSpace(s.sqlEditor.Text())
	if sqlText == "" { s.status = "SQL 不能为空"; return }
	if s.queryRunning { s.status = "已有查询正在执行，请先停止或等待完成"; return }
	s.querySequence++
	queryID := fmt.Sprintf("gio-%d-%d", time.Now().UnixNano(), s.querySequence)
	sequence, connectionID := s.querySequence, s.selectedConnectionID
	s.lastExecutedSQL, s.lastExecutedConnectionID = sqlText, connectionID
	s.runningQueryID, s.queryRunning, s.queryResult = queryID, true, nil
	s.resultColumnHeader.Position = layout.Position{}
	s.resultColumnRow.Position = layout.Position{}
	s.resultCellClicks = nil
	s.resultRowCopyClicks = nil
	s.status = "正在执行 SQL…"
	go func() {
		result, err := s.service.Query(connectionID, sqlText, queryID)
		s.queryResults <- queryResult{queryID: queryID, sequence: sequence, result: result, err: err}
		s.window.Invalidate()
	}()
}

func (s *screen) stopQuery() {
	if !s.queryRunning || s.runningQueryID == "" { s.status = "当前没有正在执行的查询"; return }
	if s.service.CancelQuery(s.runningQueryID) { s.status = "已发送取消请求…" } else { s.status = "查询已结束，正在接收最终结果…" }
}

func (s *screen) drainQueryResults() {
	for {
		select {
		case result := <-s.queryResults:
			if result.queryID != s.runningQueryID || result.sequence != s.querySequence { continue }
			s.queryRunning, s.runningQueryID = false, ""
			if result.err != nil { s.queryResult = nil; s.status = "查询失败：" + result.err.Error(); continue }
			s.queryResult = result.result
			s.resultColumnWidths = nil
			s.resultColumnDrags = nil
			s.resultColumnDragLastX = nil
			s.resultColumnDragActive = nil
			s.resultCellClicks = nil
			s.resultRowCopyClicks = nil
			if result.result != nil {
				s.resultColumnWidths = make([]unit.Dp, len(result.result.Columns))
				for columnIndex, weight := range resultColumnWeights(result.result.Columns, result.result.Rows) {
					s.resultColumnWidths[columnIndex] = resultColumnWidth(weight)
				}
				s.resultColumnDrags = make([]gesture.Drag, len(result.result.Columns))
				s.resultColumnDragLastX = make([]float32, len(result.result.Columns))
				s.resultColumnDragActive = make([]bool, len(result.result.Columns))
				s.resultCellClicks = make([][]widget.Clickable, len(result.result.Rows))
				s.resultRowCopyClicks = make([]widget.Clickable, len(result.result.Rows))
				for rowIndex := range result.result.Rows {
					s.resultCellClicks[rowIndex] = make([]widget.Clickable, len(result.result.Columns))
				}
			}
			if result.result == nil { s.status = "查询未返回结果" } else if result.result.Error != "" { s.status = "SQL 执行失败：" + result.result.Error } else { s.status = fmt.Sprintf("查询完成：%d 行 · %.2f ms", result.result.RowCount, result.result.ElapsedMs) }
		default: return
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
	minResultColumnWidth unit.Dp = 80
	maxResultColumnWidth unit.Dp = 480
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
	if s.service == nil {
		s.status = "应用服务尚未初始化"
		return
	}
	if s.queryRunning && s.runningQueryID != "" {
		s.service.CancelQuery(s.runningQueryID)
		s.querySequence++
		s.queryRunning = false
		s.runningQueryID = ""
	}
	s.queryResult = nil
	s.selectedConnectionID = connection.ID
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
	for s.sidebarToggle.Clicked(gtx) {
		s.sidebarOpen = !s.sidebarOpen
		s.saveWorkspaceLayout()
	}
	for s.detailsToggle.Clicked(gtx) {
		if s.selectedSchemaTable != "" {
			s.detailsOpen = !s.detailsOpen
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
				if !s.sidebarOpen { label = "显示导航" }
				return material.Button(s.theme, &s.sidebarToggle, label).Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx context) dimensions {
				label := "表详情"
				if s.detailsOpen { label = "隐藏详情" }
				return material.Button(s.theme, &s.detailsToggle, label).Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx context) dimensions {
				return material.Button(s.theme, &s.connect, "连接").Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx context) dimensions {
				label := "执行 SQL"
				if s.queryRunning { label = "执行中…" }
				return material.Button(s.theme, &s.execute, label).Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx context) dimensions {
				return material.Button(s.theme, &s.stop, "停止").Layout(gtx)
			}),
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
			width := gtx.Dp(248)
			gtx.Constraints.Min.X = width
			gtx.Constraints.Max.X = width
			return s.layoutSidebar(gtx)
		}),
		layout.Flexed(1, s.layoutEditorAndResults),
	}
	if s.detailsOpen && s.selectedSchemaTable != "" {
		children = append(children, layout.Rigid(s.layoutTableDetails))
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
}

func emptyMetadata(value string) string {
	if strings.TrimSpace(value) == "" { return "（无）" }
	return value
}

func (s *screen) layoutSidebar(gtx context) dimensions {
	return layout.Inset{Top: unit.Dp(12), Bottom: unit.Dp(12), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx context) dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx context) dimensions {
				return material.H6(s.theme, "已保存连接").Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		}
		if len(s.connections) == 0 {
			children = append(children, layout.Rigid(func(gtx context) dimensions {
				return material.Body2(s.theme, "暂无连接配置").Layout(gtx)
			}))
		}
		for i := range s.connections {
			connection := s.connections[i]
			if s.connectionClicks[i].Clicked(gtx) {
				s.connectConnection(connection)
			}
			label := fmt.Sprintf("%s  ·  %s:%d", connection.Name, connection.Host, connection.Port)
			children = append(children, layout.Rigid(func(gtx context) dimensions {
				return material.Button(s.theme, &s.connectionClicks[i], label).Layout(gtx)
			}))
		}
		if s.treeConnected {
			children = append(children, layout.Rigid(func(gtx context) dimensions {
				return material.H6(s.theme, "数据库").Layout(gtx)
			}))
			for _, database := range s.databaseNames {
				name := database
				node := s.databaseNodes[name]
				children = append(children, layout.Rigid(func(gtx context) dimensions {
					label := "  ▸ " + name
					if node.expanded { label = "  ▾ " + name }
					if node.loading { label += "（加载中）" }
					dims := material.Button(s.theme, &node.click, label).Layout(gtx)
					for node.click.Clicked(gtx) { s.toggleDatabase(name, node) }
					return dims
				}))
				if node.expanded && node.err != nil {
					message := node.err.Error()
					children = append(children, layout.Rigid(func(gtx context) dimensions { return material.Caption(s.theme, "加载失败："+message).Layout(gtx) }))
				}
				if node.expanded && node.loaded {
					children = append(children, layout.Rigid(func(gtx context) dimensions { return material.Caption(s.theme, fmt.Sprintf("表（%d）", len(node.tables))).Layout(gtx) }))
					for _, table := range node.tables {
						tableName := table.Name
						tableClick := node.tableClicks[tableName]
						label := tableName + " · " + table.Engine
						children = append(children, layout.Rigid(func(gtx context) dimensions {
							dims := material.Button(s.theme, tableClick, label).Layout(gtx)
							for tableClick.Clicked(gtx) { s.selectTable(name, tableName) }
							return dims
						}))
					}
					children = append(children, layout.Rigid(func(gtx context) dimensions { return material.Caption(s.theme, fmt.Sprintf("视图（%d）", len(node.views))).Layout(gtx) }))
					for _, view := range node.views { label := "    "+view.Name; children = append(children, layout.Rigid(func(gtx context) dimensions { return material.Caption(s.theme, label).Layout(gtx) })) }
					if len(node.tables)+len(node.views)==0 { children = append(children, layout.Rigid(func(gtx context) dimensions { return material.Caption(s.theme, "    此数据库暂无表或视图").Layout(gtx) })) }
				}
			}
			if len(s.databaseNames) == 0 {
				children = append(children, layout.Rigid(func(gtx context) dimensions {
					return material.Caption(s.theme, "当前账号没有可见数据库").Layout(gtx)
				}))
			}
		}
		children = append(children,
			layout.Rigid(layout.Spacer{Height: unit.Dp(20)}.Layout),
			layout.Rigid(func(gtx context) dimensions {
				return material.Body1(s.theme, "常用功能").Layout(gtx)
			}),
			layout.Rigid(func(gtx context) dimensions {
				return material.Body2(s.theme, "查询历史").Layout(gtx)
			}),
			layout.Rigid(func(gtx context) dimensions {
				return material.Body2(s.theme, "收藏 SQL").Layout(gtx)
			}),
		)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (s *screen) layoutTableDetails(gtx context) dimensions {
	width := gtx.Dp(360)
	gtx.Constraints.Min.X = width
	gtx.Constraints.Max.X = width
	return layout.Inset{Top: unit.Dp(12), Bottom: unit.Dp(12), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx context) dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx context) dimensions { return material.H6(s.theme, "表详情").Layout(gtx) }),
			layout.Rigid(func(gtx context) dimensions { return material.Caption(s.theme, s.selectedSchemaDatabase+"."+s.selectedSchemaTable).Layout(gtx) }),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		}
		if s.schemaLoading {
			children = append(children, layout.Rigid(func(gtx context) dimensions { return material.Body2(s.theme, "正在加载表结构与存储信息…").Layout(gtx) }))
		} else if s.schemaError != "" {
			message := s.schemaError
			children = append(children, layout.Rigid(func(gtx context) dimensions { return material.Body2(s.theme, "表结构读取失败："+message).Layout(gtx) }))
		} else {
			children = append(children, layout.Flexed(0.35, func(gtx context) dimensions {
				itemCount := len(s.selectedSchema)
				if itemCount == 0 {
					itemCount = 1
				}
				s.schemaList.Axis = layout.Vertical
				return material.List(s.theme, &s.schemaList).Layout(gtx, itemCount, func(gtx context, index int) dimensions {
					if len(s.selectedSchema) == 0 {
						return material.Caption(s.theme, "该表没有可见字段").Layout(gtx)
					}
					column := s.selectedSchema[index]
					line := column.Name+"  "+column.Type
					if column.IsPrimary { line += "  [主键]" }
					if column.HasDefault { line += "  [默认值]" }
					if column.Comment != "" { line += "  · "+column.Comment }
					return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx context) dimensions {
						return material.Caption(s.theme, line).Layout(gtx)
					})
				})
			}))
		}
		if s.tableDetailsError != "" {
			message := s.tableDetailsError
			children = append(children, layout.Rigid(func(gtx context) dimensions { return material.Caption(s.theme, "存储详情读取失败："+message).Layout(gtx) }))
		} else if details := s.selectedTableDetails; details != nil {
			metadata := []string{
				"引擎："+emptyMetadata(details.Engine),
				"注释："+emptyMetadata(details.Comment),
				"PARTITION BY："+emptyMetadata(details.PartitionKey),
				"ORDER BY："+emptyMetadata(details.SortingKey),
				"PRIMARY KEY："+emptyMetadata(details.PrimaryKey),
				"TTL："+emptyMetadata(details.TTLExpression),
			}
			for _, item := range metadata {
				line := item
				children = append(children, layout.Rigid(func(gtx context) dimensions { return material.Caption(s.theme, line).Layout(gtx) }))
			}
			if details.CreateTableQuery != "" {
				children = append(children,
					layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
					layout.Rigid(func(gtx context) dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx context) dimensions { return material.Body2(s.theme, "建表语句（可滚动）").Layout(gtx) }),
							layout.Rigid(func(gtx context) dimensions {
								dims := material.Button(s.theme, &s.copyDDL, "复制 DDL").Layout(gtx)
								for s.copyDDL.Clicked(gtx) {
									gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(details.CreateTableQuery))})
									s.status = "已将 "+s.selectedSchemaDatabase+"."+s.selectedSchemaTable+" 的 DDL 复制到剪贴板"
								}
								return dims
							}),
						)
					}),
				)
				children = append(children, layout.Flexed(1, func(gtx context) dimensions {
					s.detailsList.Axis = layout.Vertical
					return material.List(s.theme, &s.detailsList).Layout(gtx, len(s.ddlLines), func(gtx context, index int) dimensions {
						line := s.ddlLines[index]
						return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx context) dimensions {
							return material.Caption(s.theme, line).Layout(gtx)
						})
					})
				}))
			} else {
				children = append(children, layout.Rigid(func(gtx context) dimensions { return material.Caption(s.theme, "该表没有可用的建表语句").Layout(gtx) }))
			}
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (s *screen) layoutEditorAndResults(gtx context) dimensions {
	s.workspaceHeight = gtx.Constraints.Max.Y
	return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(8), Right: unit.Dp(12)}.Layout(gtx, func(gtx context) dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Flexed(s.editorRatio, s.layoutEditor),
			layout.Rigid(s.layoutSplitControls),
			layout.Flexed(1-s.editorRatio, s.layoutResults),
		)
	})
}

func adjustEditorRatio(current, delta float32, workspaceHeight int) float32 {
	if workspaceHeight <= 0 {
		return clampEditorRatio(current)
	}
	return clampEditorRatio(current + delta/float32(workspaceHeight))
}

func clampEditorRatio(value float32) float32 {
	if value < 0.35 { return 0.35 }
	if value > 0.80 { return 0.80 }
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
	return layout.Inset{Top: unit.Dp(12), Bottom: unit.Dp(12), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx context) dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx context) dimensions {
				return material.H6(s.theme, "SQL 编辑器").Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
			layout.Flexed(1, func(gtx context) dimensions {
				return material.Editor(s.theme, &s.sqlEditor, "输入 ClickHouse SQL...").Layout(gtx)
			}),
			layout.Rigid(func(gtx context) dimensions {
				return material.Caption(s.theme, "基础文本编辑验证：语法高亮、补全和大 SQL 性能尚未实现").Layout(gtx)
			}),
		)
	})
}

func (s *screen) layoutResults(gtx context) dimensions {
	return layout.Inset{Top: unit.Dp(12), Bottom: unit.Dp(12), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx context) dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx context) dimensions { return material.H6(s.theme, "查询结果").Layout(gtx) }),
			layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		}
		if s.queryRunning { children = append(children, layout.Rigid(func(gtx context) dimensions { return material.Body2(s.theme, "查询正在执行，可点击停止取消。").Layout(gtx) })) }
		if s.queryResult == nil {
			children = append(children, layout.Flexed(1, func(gtx context) dimensions {
				message := "执行查询后，结果将在这里展示"
				if !s.treeConnected { message = "请先连接 ClickHouse，再执行 SQL" }
				return material.Body1(s.theme, message).Layout(gtx)
			}))
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		}
		result := s.queryResult
		meta := fmt.Sprintf("%d 行 · %.2f ms · 上限 %d 行", result.RowCount, result.ElapsedMs, result.RowLimit)
		if result.Truncated { meta += " · 结果已截断 · 导出仅包含当前已加载行" }
		if result.Error != "" { children = append(children, layout.Rigid(func(gtx context) dimensions { return material.Body2(s.theme, "执行错误："+result.Error).Layout(gtx) })) }
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
						if s.exportRunning { label = "导出中…" }
						return material.Button(s.theme, &s.exportCSV, label).Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx context) dimensions {
						label := "导出 TSV"
						if s.exportRunning { label = "导出中…" }
						return material.Button(s.theme, &s.exportTSV, label).Layout(gtx)
					}),
				)
			}))
		} else {
			children = append(children, layout.Rigid(func(gtx context) dimensions { return material.Caption(s.theme, meta).Layout(gtx) }))
		}
		if len(result.Columns) == 0 {
			children = append(children, layout.Flexed(1, func(gtx context) dimensions { return material.Body2(s.theme, "没有可展示的结果列").Layout(gtx) }))
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		}
		columnWeights := resultColumnWeights(result.Columns, result.Rows)
		children = append(children, layout.Rigid(func(gtx context) dimensions {
			s.resultColumnHeader.Axis = layout.Horizontal
			return material.List(s.theme, &s.resultColumnHeader).Layout(gtx, len(result.Columns)+1, func(gtx context, index int) dimensions {
				if index == 0 {
					gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(80), gtx.Dp(80)
					return material.Body2(s.theme, "操作").Layout(gtx)
				}
				columnIndex := index - 1
				column := result.Columns[columnIndex]
				width := resultColumnWidth(columnWeights[columnIndex])
				if columnIndex < len(s.resultColumnWidths) && s.resultColumnWidths[columnIndex] > 0 {
					width = s.resultColumnWidths[columnIndex]
				}
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(width), gtx.Dp(width)
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx context) dimensions {
						return material.Body2(s.theme, column.Name).Layout(gtx)
					}),
					layout.Rigid(func(gtx context) dimensions {
						return s.layoutResultColumnResizeHandle(gtx, columnIndex)
					}),
				)
			})
		}))
		children = append(children, layout.Flexed(1, func(gtx context) dimensions {
			s.resultList.Axis = layout.Vertical
			return material.List(s.theme, &s.resultList).Layout(gtx, len(result.Rows), func(gtx context, index int) dimensions {
				row := result.Rows[index]
				s.resultColumnRow.Axis = layout.Horizontal
				s.resultColumnRow.Position = s.resultColumnHeader.Position
				dims := s.resultColumnRow.List.Layout(gtx, len(result.Columns)+1, func(gtx context, columnIndex int) dimensions {
					if columnIndex == 0 {
						gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(80), gtx.Dp(80)
						return s.resultRowCopyClicks[index].Layout(gtx, func(gtx context) dimensions {
							return material.Caption(s.theme, "复制行").Layout(gtx)
						})
					}
					cellIndex := columnIndex - 1
					cellText := "NULL"
					if cellIndex < len(row) { cellText = formatResultCell(row[cellIndex]) }
					width := resultColumnWidth(columnWeights[cellIndex])
					if cellIndex < len(s.resultColumnWidths) && s.resultColumnWidths[cellIndex] > 0 {
						width = s.resultColumnWidths[cellIndex]
					}
					gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(width), gtx.Dp(width)
					return s.resultCellClicks[index][cellIndex].Layout(gtx, func(gtx context) dimensions {
						return layout.Inset{Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx context) dimensions {
							return material.Caption(s.theme, cellText).Layout(gtx)
						})
					})
				})
				for s.resultRowCopyClicks[index].Clicked(gtx) {
					gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(formatResultRow(row)))})
					s.status = fmt.Sprintf("已复制第 %d 行（制表符分隔）", index+1)
				}
				for cellIndex := range row {
					if cellIndex < len(s.resultCellClicks[index]) && s.resultCellClicks[index][cellIndex].Clicked(gtx) {
						gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(formatClipboardCell(row[cellIndex])))})
						s.status = fmt.Sprintf("已复制第 %d 行第 %d 列", index+1, cellIndex+1)
					}
				}
				s.resultColumnHeader.Position = s.resultColumnRow.Position
				return dims
			})
		}))
		if len(result.Rows) == 0 && result.Error == "" { children = append(children, layout.Rigid(func(gtx context) dimensions { return material.Caption(s.theme, "查询成功，但没有返回数据行").Layout(gtx) })) }
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}
func (s *screen) layoutStatus(gtx context) dimensions {
	return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx context) dimensions {
		return material.Caption(s.theme, s.status).Layout(gtx)
	})
}
