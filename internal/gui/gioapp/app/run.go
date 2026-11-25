// Package app 提供基于新 application 服务的 Gio 原生工作台入口。
package app

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"io"
	"strings"
	"time"

	gio "gioui.org/app"
	"gioui.org/gesture"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	gioexplorer "gioui.org/x/explorer"

	metadataapp "github.com/yuwen002/meteor-ch-workbench/internal/application/metadata"
	queryapp "github.com/yuwen002/meteor-ch-workbench/internal/application/query"
	"github.com/yuwen002/meteor-ch-workbench/internal/bootstrap"
	"github.com/yuwen002/meteor-ch-workbench/internal/domain/queryhistory"
	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
	connectionui "github.com/yuwen002/meteor-ch-workbench/internal/gui/gioapp/connection"
	workspaceui "github.com/yuwen002/meteor-ch-workbench/internal/gui/gioapp/workspace"
)

type metadataService interface {
	ListDatabases(context.Context, string) ([]string, error)
	ListTables(context.Context, string, string) ([]metadataapp.Object, error)
	ListViews(context.Context, string, string) ([]metadataapp.Object, error)
	GetTableSchema(context.Context, string, string, string) ([]metadataapp.Column, error)
	GetTableDetails(context.Context, string, string, string) (metadataapp.TableDetails, error)
}

type structObject = metadataapp.Object

type schemaResult struct {
	seq          uint64
	connectionID string
	database     string
	table        string
	columns      []metadataapp.Column
	details      metadataapp.TableDetails
	err          error
}

type queryEvent struct {
	seq     uint64
	tab     workspaceui.TabID
	columns []queryapp.Column
	batch   [][]any
	summary queryapp.Summary
	sql string
	connectionID string
	err     error
	done    bool
}

type queryCellSelection struct {
	row    int
	column queryapp.Column
	value  string
}

type deleteResult struct {
	id  string
	err error
}

type saveResult struct {
	item connection.Connection
	err  error
}

type connectionCheck struct {
	seq       uint64
	id        string
	databases []string
	err       error
}

type objectsResult struct {
	seq          uint64
	connectionID string
	database     string
	tables       []metadataapp.Object
	views        []metadataapp.Object
	err          error
}

// Run 使用新架构服务启动 Gio 窗口，不再依赖旧 internal/app.Service。
func Run(window *gio.Window, services *bootstrap.Services) error {
	if services == nil || services.Connections == nil || services.Folders == nil || services.Metadata == nil || services.Queries == nil || services.History == nil {
		return fmt.Errorf("新架构应用服务未初始化")
	}
	model, err := connectionui.NewModel(services.Connections, services.Folders)
	if err != nil {
		return err
	}
	explorer := gioexplorer.NewExplorer(window)
	state := &workspace{
		window:           window,
		explorer:         explorer,
		services:         services,
		connections:      model,
		theme:            material.NewTheme(),
		checkResults:     make(chan connectionCheck, 4),
		saveResults:      make(chan saveResult, 2),
		deleteResults:    make(chan deleteResult, 4),
		objectResults:    make(chan objectsResult, 8),
		schemaResults:    make(chan schemaResult, 4),
		queryResults:     make(chan queryEvent, 128),
		sqlWorkspace:     workspaceui.NewTabs(),
		sqlTabStates:     map[workspaceui.TabID]sqlTabState{1: {}},
		queryRows:        widget.List{List: layout.List{Axis: layout.Vertical}},
		queryHorizontal:  widget.List{List: layout.List{Axis: layout.Horizontal}},
		queryDetailRows:  widget.List{List: layout.List{Axis: layout.Vertical}},
		queryCellClicks:  make(map[int64]*widget.Clickable),
		resultCellText:   make(map[int64]string),
		exportResults:    make(chan exportResult, 4),
		resultSortColumn: -1,
		databaseClicks:   make(map[string]*widget.Clickable),
		objectClicks:     make(map[string]*widget.Clickable),
		databaseList:     widget.List{List: layout.List{Axis: layout.Vertical}},
		objectList:       widget.List{List: layout.List{Axis: layout.Vertical}},
		objectTreeList:   widget.List{List: layout.List{Axis: layout.Vertical}},
		columnList:       widget.List{List: layout.List{Axis: layout.Vertical}},
	}
	state.connectionView, err = connectionui.NewView(model, state.reloadConnections, state.selectConnection)
	if err != nil {
		return err
	}
	state.connectionView.SetOnSave(state.saveConnection)
	state.connectionView.SetOnDelete(state.deleteConnection)
	state.queryEditor.SingleLine = false
	go state.reloadConnections()

	var ops op.Ops
	for {
		event := window.Event()
		state.explorer.ListenEvents(event)
		switch e := event.(type) {
		case gio.DestroyEvent:
			if state.cancelRequest != nil {
				state.cancelRequest()
			}
			return e.Err
		case gio.FrameEvent:
			gtx := gio.NewContext(&ops, e)
			state.drainResults()
			state.drainExportResults()
			state.layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

type workspace struct {
	window            *gio.Window
	services          *bootstrap.Services
	connections       *connectionui.Model
	connectionView    *connectionui.View
	theme             *material.Theme
	checkResults      chan connectionCheck
	saveResults       chan saveResult
	deleteResults     chan deleteResult
	objectResults     chan objectsResult
	schemaResults     chan schemaResult
	queryResults      chan queryEvent
	queryEditor       widget.Editor
	sqlWorkspace      *workspaceui.Tabs
	sqlTabStates      map[workspaceui.TabID]sqlTabState
	sqlTabClicks      []*widget.Clickable
	sqlTabCloseClicks []*widget.Clickable
	sqlTabList        widget.List
	sqlNewTab         widget.Clickable
	queryRows         widget.List
	queryHorizontal   widget.List
	queryDetailRows   widget.List
	queryCellClicks   map[int64]*widget.Clickable
	selectedQueryCell *queryCellSelection
	queryDetailClose  widget.Clickable
	queryDetailCopy   widget.Clickable
	queryRun          widget.Clickable
	queryCancel       widget.Clickable
	queryExecution    workspaceui.QueryExecution
	queryCancelFunc   context.CancelFunc
	queryColumns      []queryapp.Column
	queryData         [][]any
	queryStatus       string
	queryRunning      bool

	resultView           []resultRowView
	resultViewDirty      bool
	resultCellText       map[int64]string
	resultHeaderChildren []layout.FlexChild
	resultRowChildren    []layout.FlexChild
	resultPixelWidths    []int
	resultFilter         widget.Editor
	resultFilterLast     string
	resultFilterClear    widget.Clickable
	columnFilterEditors  []widget.Editor
	columnFilterLast     []string
	columnFilterClears   []widget.Clickable
	columnFilterClearAll widget.Clickable
	sortClicks           []widget.Clickable
	resultSortColumn     int
	resultSortDescending bool
	columnWidths         []unit.Dp
	columnDrags          []gesture.Drag
	columnDragLastX      []float32
	columnDragActive     []bool

	exportCSV     widget.Clickable
	exportTSV     widget.Clickable
	exportRunning bool
	exportResults chan exportResult
	explorer      *gioexplorer.Explorer

	databaseClicks             map[string]*widget.Clickable
	objectClicks               map[string]*widget.Clickable
	refreshObjects             widget.Clickable
	refreshConnection          widget.Clickable
	selectedObject             string
	selectedTable              string
	columns                    []metadataapp.Column
	tableDetails               metadataapp.TableDetails
	databaseList               widget.List
	objectList                 widget.List
	objectTreeList             widget.List
	columnList                 widget.List
	selectedID                 string
	selectedName               string
	connected                  bool
	connectionSidebarCollapsed bool
	sidebarToggle              widget.Clickable
	retryConnection            widget.Clickable
	databases                  []string
	selectedDatabase           string
	tables                     []metadataapp.Object
	views                      []metadataapp.Object
	loading                    string
	status                     string
	requestSeq                 uint64
	cancelRequest              context.CancelFunc
}

func (s *workspace) saveConnection(item connection.Connection) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		saved, err := s.connections.Save(ctx, item)
		result := saveResult{item: saved, err: err}
		select {
		case s.saveResults <- result:
		default:
		}
		s.window.Invalidate()
	}()
}

func (s *workspace) deleteConnection(id string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := s.connections.Delete(ctx, id)
		select {
		case s.deleteResults <- deleteResult{id: id, err: err}:
		default:
		}
		s.window.Invalidate()
	}()
}

func (s *workspace) reloadConnections() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.connections.Reload(ctx); err != nil {
		// 配置读取失败时不能静默显示空侧栏；侧栏空状态直接展示实际路径和错误。
		s.status = fmt.Sprintf("加载连接配置失败 · 配置文件：%s · 原因：%v", s.services.ConfigPath, err)
		s.connectionView.SetDiagnostic(s.status)
		s.window.Invalidate()
		return
	}
	snapshot := s.connections.Snapshot()
	s.status = fmt.Sprintf("配置正常 · %d 个连接 · %d 个分组 · %s", len(snapshot.Items), len(snapshot.Folders), s.services.ConfigPath)
	s.connectionView.SetDiagnostic(s.status)
	s.window.Invalidate()
}

func (s *workspace) selectConnection(item connection.Connection) {
	s.selectedID = item.ID
	s.selectedName = item.Name
	s.connected = false
	s.selectedDatabase = ""
	s.selectedTable = ""
	s.selectedObject = ""
	s.columns = nil
	s.tableDetails = metadataapp.TableDetails{}
	s.databases = nil
	s.tables, s.views = nil, nil
	s.loading = "正在测试连接并读取数据库…"
	if s.cancelRequest != nil {
		s.cancelRequest()
	}
	s.requestSeq++
	seq := s.requestSeq
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	s.cancelRequest = cancel
	go func() {
		defer cancel()
		err := s.connections.Test(ctx, item)
		var databases []string
		if err == nil {
			databases, err = s.services.Metadata.ListDatabases(ctx, item.ID)
		}
		select {
		case s.checkResults <- connectionCheck{seq: seq, id: item.ID, databases: databases, err: err}:
		default:
			// 新请求会使旧结果失效；队列满时丢弃即可，避免窗口退出后阻塞后台 goroutine。
		}
		s.window.Invalidate()
	}()
}

func (s *workspace) selectDatabase(name string) {
	if !s.connected || s.selectedID == "" {
		return
	}
	s.selectedDatabase = name
	s.selectedTable = ""
	s.columns = nil
	s.tableDetails = metadataapp.TableDetails{}
	s.tables, s.views = nil, nil
	s.loading = "正在读取表和视图…"
	if s.cancelRequest != nil {
		s.cancelRequest()
	}
	s.requestSeq++
	seq, connectionID := s.requestSeq, s.selectedID
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	s.cancelRequest = cancel
	go func() {
		defer cancel()
		tables, err := s.services.Metadata.ListTables(ctx, connectionID, name)
		var views []structObject
		if err == nil {
			views, err = s.services.Metadata.ListViews(ctx, connectionID, name)
		}
		result := objectsResult{seq: seq, connectionID: connectionID, database: name, tables: tables, views: views, err: err}
		select {
		case s.objectResults <- result:
		default:
			// 数据库切换时旧结果已失效，队列满时不阻塞后台请求。
		}
		s.window.Invalidate()
	}()
}

func (s *workspace) drainResults() {
	for {
		select {
		case result := <-s.saveResults:
			if result.err != nil {
				s.status = "保存连接失败：" + result.err.Error()
				continue
			}
			s.status = "连接已保存，正在测试连接…"
			s.selectConnection(result.item)
		case result := <-s.deleteResults:
			if result.err != nil {
				s.status = "删除连接失败：" + result.err.Error()
			} else {
				if s.selectedID == result.id {
					s.selectedID = ""
					s.selectedName = ""
					s.connected = false
					s.databases = nil
					s.tables = nil
					s.views = nil
					s.columns = nil
				}
				s.status = "连接已删除"
			}
		case result := <-s.checkResults:
			if result.seq != s.requestSeq || result.id != s.selectedID {
				continue
			}
			s.loading = ""
			if result.err != nil {
				s.connected = false
				s.status = "连接测试或数据库读取失败：" + result.err.Error()
				s.databases = nil
			} else {
				s.connected = true
				s.databases = result.databases
				s.status = fmt.Sprintf("连接成功 · %d 个数据库", len(result.databases))
			}
		case result := <-s.objectResults:
			if result.seq != s.requestSeq || result.connectionID != s.selectedID || result.database != s.selectedDatabase {
				continue
			}
			s.loading = ""
			if result.err != nil {
				s.status = "读取数据库对象失败：" + result.err.Error()
				continue
			}
			s.tables, s.views = result.tables, result.views
			s.status = fmt.Sprintf("%s · %d 张表 · %d 个视图", result.database, len(result.tables), len(result.views))
		case result := <-s.queryResults:
			if !s.queryExecution.IsCurrent(result.seq, result.tab) {
				continue
			}
			_, exists := s.sqlWorkspace.Get(result.tab)
			if !exists {
				continue // 标签页已关闭，丢弃迟到的查询事件。
			}
			if result.done && strings.TrimSpace(result.sql) != "" {
				entry := queryhistory.Entry{
					SQL: result.sql,
					ConnectionID: result.connectionID,
					DurationMs: float64(result.summary.Elapsed) / float64(time.Millisecond),
					Success: result.err == nil,
				}
				go func() { _, _ = s.services.History.Add(context.Background(), entry) }()
			}
			if result.tab != s.sqlWorkspace.ActiveID() {
				state := s.sqlTabStates[result.tab]
				state.Apply(result)
				s.sqlTabStates[result.tab] = state
				continue
			}
			if len(result.columns) > 0 {
				s.queryColumns = result.columns
				s.queryData = nil
				s.clearQueryCellSelection()
				s.resetResultWidgets(result.columns)
			}
			if len(result.batch) > 0 {
				s.queryData = append(s.queryData, result.batch...)
				s.resultViewDirty = true
			}
			if result.done {
				s.queryExecution.Finish(result.seq, result.tab)
				s.queryRunning = false
				s.queryCancelFunc = nil
				if result.err != nil {
					s.queryStatus = "执行失败：" + result.err.Error()
				} else if result.summary.Truncated {
					s.queryStatus = fmt.Sprintf("已返回 %d 行（达到结果上限）· %s", result.summary.RowCount, result.summary.Elapsed.Round(time.Millisecond))
				} else if len(s.queryColumns) > 0 {
					s.queryStatus = fmt.Sprintf("查询完成 · %d 行 · %s", result.summary.RowCount, result.summary.Elapsed.Round(time.Millisecond))
				} else {
					s.queryStatus = fmt.Sprintf("执行完成 · 影响 %d 行 · %s", result.summary.RowsAffected, result.summary.Elapsed.Round(time.Millisecond))
				}
			}
		case result := <-s.schemaResults:
			if result.seq != s.requestSeq || result.connectionID != s.selectedID || result.database != s.selectedDatabase || result.table != s.selectedTable {
				continue
			}
			s.loading = ""
			if result.err != nil {
				s.status = "读取表结构失败：" + result.err.Error()
				continue
			}
			s.columns, s.tableDetails = result.columns, result.details
			s.status = fmt.Sprintf("%s · %d 个字段", result.table, len(result.columns))
		default:
			return
		}
	}
}

func (s *workspace) layout(gtx layout.Context) layout.Dimensions {
	connectedID, connectingID := "", ""
	if s.connected {
		connectedID = s.selectedID
	}
	if s.selectedID != "" && s.loading != "" {
		connectingID = s.selectedID
	}
	s.connectionView.SetConnectionStatus(s.selectedID, connectedID, connectingID)
	// 整屏底色直接铺一层不透明矩形，避免任何录制/回放机制带来透明缝隙。
	paint.FillShape(gtx.Ops, colorAppBackground, clip.Rect{Max: gtx.Constraints.Max}.Op())
	outer := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(s.layoutTopBar),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(16), Right: unit.Dp(16), Top: unit.Dp(14), Bottom: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				compact := isCompact(gtx)
				children := make([]layout.FlexChild, 0, 3)
				if !s.connectionSidebarCollapsed {
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						// 统一窄屏阈值：折叠后由顶栏按钮恢复，避免两处断点不一致。
						availableWidth := gtx.Constraints.Max.X
						width := gtx.Dp(unit.Dp(286))
						if compact {
							width = gtx.Dp(unit.Dp(232))
						}
						if width > availableWidth {
							width = availableWidth
						}
						contentHeight := gtx.Constraints.Max.Y
						gtx.Constraints.Min = image.Pt(width, contentHeight)
						gtx.Constraints.Max = image.Pt(width, contentHeight)
						return s.panel(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Top: unit.Dp(12), Bottom: unit.Dp(8), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return s.connectionView.Layout(gtx, s.theme)
							})
						})
					}))
					children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(14)}.Layout))
				}
				children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					if !s.connected {
						return s.layoutWelcome(gtx)
					}
					return s.layoutWorkbench(gtx)
				}))
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
			})
		}),
		layout.Rigid(s.layoutStatusBar),
	)
	return layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions { return outer }),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return s.connectionView.Modal(gtx, s.theme)
		}),
	)
}

func (s *workspace) layoutWelcome(gtx layout.Context) layout.Dimensions {
	return s.card(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					size := image.Pt(gtx.Dp(unit.Dp(56)), gtx.Dp(unit.Dp(56)))
					fillRounded(gtx, size, radiusM, colorAccent)
					return layout.Dimensions{Size: size}
				}),
				layout.Rigid(layout.Spacer{Height: spaceL}.Layout),
				layout.Rigid(styledText(s.theme, "Meteor CH Workbench", unit.Sp(26), colorTextPrimary)),
				layout.Rigid(layout.Spacer{Height: spaceS}.Layout),
				layout.Rigid(styledText(s.theme, "轻量级 ClickHouse 数据库 IDE", unit.Sp(13), colorTextSecondary)),
				layout.Rigid(layout.Spacer{Height: spaceXL}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					message, dot := "尚未连接数据库", colorTextMuted
					if s.selectedName != "" && s.loading != "" {
						message, dot = "正在连接 · "+s.selectedName, colorWarning
					}
					if s.selectedName != "" && s.loading == "" && !s.connected {
						message, dot = "连接失败 · "+s.selectedName, colorWarning
					}
					return s.statusPill(gtx, dot, message, adaptiveMaxLines(gtx))
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if s.status == "" {
						return layout.Dimensions{}
					}
					return layout.Inset{Top: spaceS}.Layout(gtx, styledText(s.theme, s.status, unit.Sp(12), colorTextMuted))
				}),
				layout.Rigid(layout.Spacer{Height: spaceXL}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					var click *widget.Clickable
					label := "刷新连接列表"
					if s.selectedID != "" && s.loading == "" && !s.connected {
						click = &s.retryConnection
						label = "重新连接"
					} else {
						click = &s.refreshConnection
					}
					if click.Clicked(gtx) {
						if s.selectedID != "" && s.loading == "" && !s.connected {
							for _, item := range s.connections.Snapshot().Items {
								if item.ID == s.selectedID {
									s.selectConnection(item)
									break
								}
							}
						} else {
							s.reloadConnections()
						}
					}
					button := material.Button(s.theme, click, label)
					button.CornerRadius = radiusS
					button.Inset = layout.Inset{Left: unit.Dp(18), Right: unit.Dp(18), Top: unit.Dp(8), Bottom: unit.Dp(8)}
					button.TextSize = unit.Sp(12)
					return button.Layout(gtx)
				}),
			)
		})
	})
}
func (s *workspace) layoutTopBar(gtx layout.Context) layout.Dimensions {
	compact := isCompact(gtx)
	inset := layout.Inset{Left: spaceXL, Right: spaceXL, Top: spaceM, Bottom: spaceM}
	if compact {
		inset.Left, inset.Right = spaceM, spaceM
	}
	return s.topSurface(gtx, colorSurface, func(gtx layout.Context) layout.Dimensions {
		return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, 6)
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						size := image.Pt(gtx.Dp(unit.Dp(30)), gtx.Dp(unit.Dp(30)))
						fillRounded(gtx, size, radiusS, colorAccent)
						return layout.Dimensions{Size: size}
					}),
					layout.Rigid(layout.Spacer{Width: spaceM}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						titleText := "Meteor CH Workbench"
						if compact {
							titleText = "Meteor CH"
						}
						widgets := []layout.FlexChild{layout.Rigid(styledText(s.theme, titleText, unit.Sp(15), colorTextPrimary))}
						if !compact {
							widgets = append(widgets, layout.Rigid(styledText(s.theme, "轻量级 ClickHouse 数据库 IDE", unit.Sp(10), colorTextMuted)))
						}
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx, widgets...)
					}),
				)
			}))
			spacerWidth := unit.Dp(32)
			if compact {
				spacerWidth = spaceM
			}
			children = append(children, layout.Rigid(layout.Spacer{Width: spacerWidth}.Layout))
			children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				label, dot := "未连接 · 选择连接开始工作", colorTextMuted
				if s.selectedName != "" && s.loading != "" {
					label = "正在连接 · " + s.selectedName
					dot = colorWarning
				}
				if s.connected {
					label = "已连接 · " + s.selectedName
					dot = colorSuccess
				}
				if s.selectedName != "" && s.loading == "" && !s.connected {
					label = "连接失败 · " + s.selectedName
					dot = colorWarning
				}
				return s.statusPill(gtx, dot, label, adaptiveMaxLines(gtx))
			}))
			children = append(children, layout.Rigid(layout.Spacer{Width: spaceXS}.Layout))
			if compact {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if s.sidebarToggle.Clicked(gtx) {
						s.connectionSidebarCollapsed = !s.connectionSidebarCollapsed
					}
					label := "隐藏连接"
					if s.connectionSidebarCollapsed {
						label = "显示连接"
					}
					return s.smallButton(gtx, &s.sidebarToggle, label, false)
				}))
				children = append(children, layout.Rigid(layout.Spacer{Width: spaceXS}.Layout))
			}
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if s.refreshConnection.Clicked(gtx) {
					s.reloadConnections()
				}
				label := "刷新连接"
				if compact {
					label = "刷新"
				}
				button := material.Button(s.theme, &s.refreshConnection, label)
				button.TextSize = unit.Sp(11)
				button.Inset = layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12), Top: unit.Dp(6), Bottom: unit.Dp(6)}
				button.CornerRadius = radiusS
				return button.Layout(gtx)
			}))
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
		})
	})
}

func queryResultStatus(result queryEvent, columns []queryapp.Column) string {
	if result.err != nil {
		return "执行失败：" + result.err.Error()
	}
	if result.summary.Truncated {
		return fmt.Sprintf("已返回 %d 行（达到结果上限）· %s", result.summary.RowCount, result.summary.Elapsed.Round(time.Millisecond))
	}
	if len(columns) > 0 {
		return fmt.Sprintf("查询完成 · %d 行 · %s", result.summary.RowCount, result.summary.Elapsed.Round(time.Millisecond))
	}
	return fmt.Sprintf("执行完成 · 影响 %d 行 · %s", result.summary.RowsAffected, result.summary.Elapsed.Round(time.Millisecond))
}
func (s *workspace) executeQuery() {
	if !s.connected || s.selectedID == "" {
		s.queryStatus = "请先连接 ClickHouse"
		return
	}
	sqlText := strings.TrimSpace(s.queryEditor.Text())
	if sqlText == "" {
		s.queryStatus = "请输入 SQL 语句"
		return
	}
	s.sqlWorkspace.UpdateSQL(s.sqlWorkspace.ActiveID(), s.queryEditor.Text())
	if s.queryCancelFunc != nil {
		s.queryCancelFunc()
	}
	tab := s.sqlWorkspace.ActiveID()
	seq := s.queryExecution.Start(tab)
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	s.queryCancelFunc = cancel
	s.queryRunning = true
	s.queryColumns, s.queryData = nil, nil
	s.clearQueryCellSelection()
	s.resetResultWidgets(nil)
	s.queryStatus = "正在执行 SQL…"
	request := queryapp.Request{ConnectionID: s.selectedID, SQL: sqlText, Mode: classifySQLMode(sqlText), MaxRows: 10000, BatchSize: 128, Timeout: 30 * time.Second}
	go func() {
		defer cancel()
		summary, err := s.services.Queries.Execute(ctx, request,
			func(columns []queryapp.Column) error {
				select {
				case s.queryResults <- queryEvent{seq: seq, tab: tab, columns: columns}:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			},
			func(rows [][]any) error {
				batch := make([][]any, len(rows))
				for i := range rows {
					batch[i] = append([]any(nil), rows[i]...)
				}
				select {
				case s.queryResults <- queryEvent{seq: seq, tab: tab, batch: batch}:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			},
		)
		select {
		case s.queryResults <- queryEvent{seq: seq, tab: tab, summary: summary, sql: sqlText, connectionID: s.selectedID, err: err, done: true}:
		case <-ctx.Done():
			// 标签页切换或窗口退出后不再等待无人消费的结果事件。
		}
		s.window.Invalidate()
	}()
}

// cancelActiveQuery 在离开当前 SQL 标签页时取消仍在执行的请求，并使其后续事件失效。
func (s *workspace) cancelActiveQuery(reason string) {
	if !s.queryRunning {
		return
	}
	if s.queryCancelFunc != nil {
		s.queryCancelFunc()
		s.queryCancelFunc = nil
	}
	s.queryExecution.Cancel()
	s.queryRunning = false
	s.queryStatus = reason
}

func (s *workspace) clearQueryCellSelection() {
	s.selectedQueryCell = nil
	clear(s.queryCellClicks)
	clear(s.resultCellText)
	s.queryDetailRows.Position.First = 0
}

func (s *workspace) layoutQueryCellDetail(gtx layout.Context) layout.Dimensions {
	selection := s.selectedQueryCell
	if selection == nil {
		return layout.Dimensions{}
	}
	if s.queryDetailClose.Clicked(gtx) {
		s.clearQueryCellSelection()
		return layout.Dimensions{}
	}
	if s.queryDetailCopy.Clicked(gtx) {
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(selection.value))})
		s.queryStatus = "已请求复制完整单元格内容"
	}
	lines := strings.Split(selection.value, "\n")
	return layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, material.Body2(s.theme, fmt.Sprintf("完整值 · 第 %d 行 · %s · %s", selection.row, selection.column.Name, selection.column.DatabaseType)).Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						button := material.Button(s.theme, &s.queryDetailCopy, "复制完整值")
						button.TextSize = unit.Sp(12)
						return layout.Inset{Right: unit.Dp(6)}.Layout(gtx, button.Layout)
					}),
					layout.Rigid(material.Button(s.theme, &s.queryDetailClose, "关闭").Layout),
				)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return material.List(s.theme, &s.queryDetailRows).Layout(gtx, len(lines), func(gtx layout.Context, index int) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, material.Body2(s.theme, lines[index]).Layout)
				})
			}),
		)
	})
}

// closeSQLTab 关闭指定 SQL 标签；最后一个标签关闭时重置为空白查询页。
// layoutSchemaStrip 是右列上方的表结构摘要条，选中表后横向展示字段，避免与编辑器争抢横向空间。
func (s *workspace) layoutSchemaStrip(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: spaceM, Bottom: spaceXS, Left: spaceL, Right: spaceL}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						title := s.selectedTable
						if s.selectedDatabase != "" && title != "" {
							title = s.selectedDatabase + "." + title
						}
						label := material.Label(s.theme, unit.Sp(12), title)
						label.Color = colorTextPrimary
						label.MaxLines = adaptiveMaxLines(gtx)
						return label.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						label := material.Label(s.theme, unit.Sp(11), s.tableDetails.Engine)
						label.Color = colorTextSecondary
						return layout.Inset{Right: spaceS}.Layout(gtx, label.Layout)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						label := material.Label(s.theme, unit.Sp(11), fmt.Sprintf("字段 %d", len(s.columns)))
						label.Color = colorTextMuted
						return label.Layout(gtx)
					}),
				)
			})
		}),
		layout.Rigid(s.divider),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(s.columns) == 0 {
				message := "选择左侧表对象以查看字段"
				if s.loading != "" {
					message = s.loading
				}
				return layout.Inset{Top: spaceS, Left: spaceL, Right: spaceL}.Layout(gtx, styledText(s.theme, message, unit.Sp(11), colorTextMuted))
			}
			s.columnList.Axis = layout.Vertical
			return layout.Inset{Top: spaceXS, Bottom: spaceXS, Left: spaceS, Right: spaceS}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.List(s.theme, &s.columnList).Layout(gtx, len(s.columns), func(gtx layout.Context, index int) layout.Dimensions {
					column := s.columns[index]
					detail := column.Type
					if column.IsPrimary {
						detail += " · PK"
					}
					if column.HasDefault {
						detail += " · 默认值"
					}
					return layout.Inset{Top: unit.Dp(3), Bottom: unit.Dp(3), Left: spaceXS, Right: spaceXS}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(0.4, func(gtx layout.Context) layout.Dimensions {
								label := material.Label(s.theme, unit.Sp(11), column.Name)
								label.Color = colorTextPrimary
								label.MaxLines = 1
								return label.Layout(gtx)
							}),
							layout.Flexed(0.6, func(gtx layout.Context) layout.Dimensions {
								label := material.Label(s.theme, unit.Sp(11), detail)
								label.Color = colorTextMuted
								label.MaxLines = 1
								return label.Layout(gtx)
							}),
						)
					})
				})
			})
		}),
	)
}

func (s *workspace) layoutQueryWorkspace(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if s.selectedTable == "" && len(s.columns) == 0 {
				return layout.Dimensions{}
			}
			height := gtx.Dp(unit.Dp(150))
			if height > gtx.Constraints.Max.Y/3 {
				height = gtx.Constraints.Max.Y / 3
			}
			gtx.Constraints.Min.Y = height
			gtx.Constraints.Max.Y = height
			return s.panel(gtx, s.layoutSchemaStrip)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if s.selectedTable == "" && len(s.columns) == 0 {
				return layout.Dimensions{}
			}
			return layout.Spacer{Height: unit.Dp(10)}.Layout(gtx)
		}),
		layout.Flexed(1, s.layoutQueryPanel),
	)
}

func (s *workspace) layoutQueryPanel(gtx layout.Context) layout.Dimensions {
	return s.panel(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(s.layoutQueryToolbar),
			layout.Rigid(s.divider),
			layout.Rigid(s.layoutTabStrip),
			layout.Rigid(s.divider),
			layout.Flexed(0.45, s.layoutSQLEditor),
			layout.Rigid(s.divider),
			layout.Flexed(0.55, s.layoutResultsPanel),
		)
	})
}

// layoutQueryToolbar 是编辑器上方的一行紧凑工具条：左侧标题与运行状态，右侧为执行 / 停止操作。
func (s *workspace) layoutQueryToolbar(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: spaceS, Bottom: spaceS, Left: spaceL, Right: spaceM}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				label := material.Label(s.theme, unit.Sp(12), "SQL 编辑器")
				label.Color = colorTextPrimary
				return label.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: spaceS}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				if !s.queryRunning {
					return layout.Dimensions{}
				}
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						size := image.Pt(gtx.Dp(unit.Dp(6)), gtx.Dp(unit.Dp(6)))
						fillRounded(gtx, size, unit.Dp(3), colorWarning)
						return layout.Dimensions{Size: size}
					}),
					layout.Rigid(layout.Spacer{Width: spaceXS}.Layout),
					layout.Rigid(styledText(s.theme, "执行中…", unit.Sp(11), colorTextMuted)),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if s.queryRun.Clicked(gtx) && !s.queryRunning {
					s.executeQuery()
				}
				return s.primaryButton(gtx, &s.queryRun, "执行 SQL", s.queryRunning)
			}),
			layout.Rigid(layout.Spacer{Width: spaceS}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if s.queryCancel.Clicked(gtx) {
					s.cancelActiveQuery("查询已取消")
				}
				return s.smallButton(gtx, &s.queryCancel, "停止", !s.queryRunning)
			}),
		)
	})
}

// layoutTabStrip 是查询标签页栏：标签之间用细缝分隔，末尾固定一个新建按钮。
func (s *workspace) layoutTabStrip(gtx layout.Context) layout.Dimensions {
	items := s.sqlWorkspace.Items()
	for len(s.sqlTabClicks) < len(items) {
		s.sqlTabClicks = append(s.sqlTabClicks, new(widget.Clickable))
	}
	for len(s.sqlTabCloseClicks) < len(items) {
		s.sqlTabCloseClicks = append(s.sqlTabCloseClicks, new(widget.Clickable))
	}
	return layout.Inset{Top: spaceXS, Bottom: spaceXS, Left: spaceS, Right: spaceM}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				s.sqlTabList.Axis = layout.Horizontal
				return material.List(s.theme, &s.sqlTabList).Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
					return layout.Inset{Right: spaceXS}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return s.layoutSQLTab(gtx, items[index], index)
					})
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if s.sqlNewTab.Clicked(gtx) {
					if len(items) >= 12 {
						s.queryStatus = "查询标签页最多支持 12 个"
					} else {
						s.cancelActiveQuery("查询已取消（新建标签页）")
						activeID := s.sqlWorkspace.ActiveID()
						current := s.sqlTabStates[activeID]
						s.sqlWorkspace.UpdateSQL(activeID, s.queryEditor.Text())
						current.Columns, current.Data, current.Status = s.queryColumns, s.queryData, s.queryStatus
						s.sqlTabStates[activeID] = current
						created := s.sqlWorkspace.Create()
						s.sqlTabStates[created.ID] = sqlTabState{Status: "新查询标签页已创建"}
						s.queryEditor.SetText("")
						s.queryColumns, s.queryData = nil, nil
						s.clearQueryCellSelection()
						s.resetResultWidgets(nil)
						s.queryStatus = "新查询标签页已创建"

					}
				}
				return s.smallButton(gtx, &s.sqlNewTab, "＋ 新建查询", false)
			}),
		)
	})
}

// layoutSQLTab 绘制单个查询标签页：整块可点击切换，关闭按钮嵌在标签内部。
func (s *workspace) layoutSQLTab(gtx layout.Context, tab workspaceui.Tab, index int) layout.Dimensions {
	active := tab.ID == s.sqlWorkspace.ActiveID()
	click := s.sqlTabClicks[index]
	if click.Clicked(gtx) && !active {
		s.switchSQLTab(tab.ID)
	}
	closeClick := s.sqlTabCloseClicks[index]
	marker := colorTextMuted
	if active {
		marker = colorAccent
	}
	return s.surface(gtx, func() color.NRGBA {
		if active {
			return colorAccentSoft
		}
		return color.NRGBA{}
	}(), radiusS, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(5), Bottom: unit.Dp(5), Left: spaceS, Right: spaceXS}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								size := image.Pt(gtx.Dp(unit.Dp(6)), gtx.Dp(unit.Dp(6)))
								fillRounded(gtx, size, unit.Dp(3), marker)
								return layout.Dimensions{Size: size}
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								label := material.Label(s.theme, unit.Sp(12), tab.Title)
								label.Color = colorTextSecondary
								if active {
									label.Color = colorTextPrimary
								}
								label.MaxLines = 1
								return label.Layout(gtx)
							}),
						)
					})
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if closeClick.Clicked(gtx) {
					s.closeSQLTab(index)
				}
				return closeClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(5), Bottom: unit.Dp(5), Left: unit.Dp(2), Right: spaceS}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						label := material.Label(s.theme, unit.Sp(11), "×")
						label.Color = colorTextMuted
						return label.Layout(gtx)
					})
				})
			}),
		)
	})
}

// layoutSQLEditor 是 SQL 输入区：圆角内凹背景 + 等宽字体，把编辑区与结果区明确分开。
func (s *workspace) layoutSQLEditor(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: spaceS, Bottom: spaceS, Left: spaceM, Right: spaceM}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return s.surface(gtx, colorSurfaceSubtle, radiusS, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(spaceM).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				editor := material.Editor(s.theme, &s.queryEditor, "输入 SQL，例如 SELECT 1")
				editor.TextSize = unit.Sp(13)
				editor.Color = colorTextPrimary
				editor.HintColor = colorTextMuted
				return editor.Layout(gtx)
			})
		})
	})
}

func (s *workspace) layoutResultsPanel(gtx layout.Context) layout.Dimensions {
	s.refreshResultView()
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: spaceS, Bottom: spaceXS, Left: spaceL, Right: spaceL}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						label := material.Label(s.theme, unit.Sp(12), "查询结果")
						label.Color = colorTextPrimary
						return label.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: spaceS}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						message := s.queryStatus
						if message == "" {
							if len(s.queryColumns) == 0 {
								message = "尚未执行查询"
							} else {
								message = fmt.Sprintf("%d 行 · %d 列", len(s.queryData), len(s.queryColumns))
							}
						}
						label := material.Label(s.theme, unit.Sp(11), message)
						label.Color = statusForeground(message)
						label.MaxLines = adaptiveMaxLines(gtx)
						return label.Layout(gtx)
					}),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if len(s.queryColumns) == 0 {
				return layout.Dimensions{}
			}
			if s.resultFilterClear.Clicked(gtx) {
				s.resultFilter.SetText("")
				s.resultFilterLast = ""
				s.resultViewDirty = true
				s.queryRows.Position = layout.Position{}
			}
			if s.columnFilterClearAll.Clicked(gtx) {
				for i := range s.columnFilterEditors {
					s.columnFilterEditors[i].SetText("")
				}
				for i := range s.columnFilterLast {
					s.columnFilterLast[i] = ""
				}
				s.resultViewDirty = true
				s.queryRows.Position = layout.Position{}
			}
			if s.exportCSV.Clicked(gtx) {
				s.startExport("csv")
			}
			if s.exportTSV.Clicked(gtx) {
				s.startExport("tsv")
			}
			return layout.Inset{Top: spaceXS, Bottom: spaceXS, Left: spaceL, Right: spaceL}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						editor := material.Editor(s.theme, &s.resultFilter, "筛选当前结果（匹配任意列）")
						editor.TextSize = unit.Sp(11)
						return editor.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: spaceXS}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return s.smallButton(gtx, &s.resultFilterClear, "清除关键字", s.resultFilter.Text() == "")
					}),
					layout.Rigid(layout.Spacer{Width: spaceXS}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						hasColumnFilter := false
						for i := range s.columnFilterEditors {
							if s.columnFilterEditors[i].Text() != "" {
								hasColumnFilter = true
								break
							}
						}
						return s.smallButton(gtx, &s.columnFilterClearAll, "清除列条件", !hasColumnFilter)
					}),
					layout.Rigid(layout.Spacer{Width: spaceXS}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return styledText(s.theme, fmt.Sprintf("显示 %d / %d", len(s.resultView), len(s.queryData)), unit.Sp(10), colorTextMuted)(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: spaceXS}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return s.smallButton(gtx, &s.exportCSV, "CSV", s.exportRunning)
					}),
					layout.Rigid(layout.Spacer{Width: spaceXS}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return s.smallButton(gtx, &s.exportTSV, "TSV", s.exportRunning)
					}),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if len(s.queryColumns) == 0 {
				return layout.Dimensions{}
			}
			return layout.Inset{Left: spaceL, Right: spaceL, Bottom: spaceXS}.Layout(gtx, styledText(s.theme,
				"列条件：包含 / !不包含 / = / != / > / < / >= / <= / between:下界..上界 / is:null",
				unit.Sp(10), colorTextMuted))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if len(s.queryColumns) == 0 {
				return layout.Dimensions{}
			}
			filters := make([]string, len(s.columnFilterEditors))
			for i := range s.columnFilterEditors {
				filters[i] = s.columnFilterEditors[i].Text()
			}
			summaries := resultColumnFilterSummary(s.queryColumns, filters)
			if len(summaries) == 0 {
				return layout.Dimensions{}
			}
			return layout.Inset{Left: spaceL, Right: spaceL, Bottom: spaceXS}.Layout(gtx, styledText(s.theme,
				"生效条件："+strings.Join(summaries, "；"), unit.Sp(10), colorAccent))
		}),
		layout.Rigid(s.divider),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(s.queryColumns) == 0 {
				return layout.Inset{Top: spaceL, Left: spaceL, Right: spaceL}.Layout(gtx, styledText(s.theme, "执行 SQL 后结果将显示在此处", unit.Sp(11), colorTextMuted))
			}
			return layout.Inset{Top: spaceXS}.Layout(gtx, s.layoutResultGrid)
		}),
	)
}

func (s *workspace) layoutResultGrid(gtx layout.Context) layout.Dimensions {
	if len(s.queryCellClicks) > 4096 {
		clear(s.queryCellClicks)
		clear(s.resultCellText)
	}
	s.resultPixelWidths = s.resultPixelWidths[:0]
	totalWidth := 0
	for _, width := range s.columnWidths {
		pixels := gtx.Dp(width)
		s.resultPixelWidths = append(s.resultPixelWidths, pixels)
		totalWidth += pixels
	}
	return material.List(s.theme, &s.queryHorizontal).Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		gtx.Constraints.Min.X = totalWidth
		gtx.Constraints.Max.X = totalWidth
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					s.resultHeaderChildren = s.resultHeaderChildren[:0]
					for i, col := range s.queryColumns {
						columnIndex, name, databaseType := i, col.Name, col.DatabaseType
						width := s.resultPixelWidths[columnIndex]
						s.resultHeaderChildren = append(s.resultHeaderChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
										layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
											dims := s.sortClicks[columnIndex].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
												label := material.Label(s.theme, unit.Sp(11), resultColumnLabel(s.queryColumns, columnIndex, s.resultSortColumn, s.resultSortDescending))
												label.Color = colorTextPrimary
												label.MaxLines = 1
												return label.Layout(gtx)
											})
											if s.sortClicks[columnIndex].Clicked(gtx) {
												if s.resultSortColumn == columnIndex {
													s.resultSortDescending = !s.resultSortDescending
												} else {
													s.resultSortColumn, s.resultSortDescending = columnIndex, false
												}
												s.resultViewDirty = true
												s.queryRows.Position = layout.Position{}
											}
											return dims
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return s.layoutResultColumnResizeHandle(gtx, columnIndex)
										}),
									)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									label := material.Label(s.theme, unit.Sp(9), databaseType)
									label.Color = colorTextMuted
									label.MaxLines = 1
									return label.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
										layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
											editor := material.Editor(s.theme, &s.columnFilterEditors[columnIndex], "筛选 "+name)
											editor.TextSize = unit.Sp(10)
											return editor.Layout(gtx)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											if s.columnFilterClears[columnIndex].Clicked(gtx) {
												s.columnFilterEditors[columnIndex].SetText("")
												s.columnFilterLast[columnIndex] = ""
												s.resultViewDirty = true
												s.queryRows.Position = layout.Position{}
											}
											return s.smallButton(gtx, &s.columnFilterClears[columnIndex], "×", s.columnFilterEditors[columnIndex].Text() == "")
										}),
									)
								}),
							)
						}))
					}
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, s.resultHeaderChildren...)
				})
			}),
			layout.Rigid(s.divider),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if s.selectedQueryCell == nil {
					return layout.Dimensions{}
				}
				gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = gtx.Dp(unit.Dp(112)), gtx.Dp(unit.Dp(112))
				return s.layoutQueryCellDetail(gtx)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return material.List(s.theme, &s.queryRows).Layout(gtx, len(s.resultView), func(gtx layout.Context, index int) layout.Dimensions {
					display := s.resultView[index]
					return layout.Inset{Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						s.resultRowChildren = s.resultRowChildren[:0]
						for cellIndex := range s.queryColumns {
							var cell any
							if cellIndex < len(display.Values) {
								cell = display.Values[cellIndex]
							}
							key := resultCellKey(index, cellIndex)
							value, ok := s.resultCellText[key]
							if !ok {
								value = formatResultCell(cell)
								s.resultCellText[key] = value
							}
							click := s.queryCellClicks[key]
							if click == nil {
								click = new(widget.Clickable)
								s.queryCellClicks[key] = click
							}
							if click.Clicked(gtx) {
								column := s.queryColumns[cellIndex]
								s.selectedQueryCell = &queryCellSelection{row: display.Index + 1, column: column, value: resultCellRawValue(cell)}
							}
							width := s.resultPixelWidths[cellIndex]
							s.resultRowChildren = append(s.resultRowChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
								return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return material.Caption(s.theme, value).Layout(gtx)
								})
							}))
						}
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, s.resultRowChildren...)
					})
				})
			}),
		)
	})
}

func (s *workspace) layoutStatusBar(gtx layout.Context) layout.Dimensions {
	return s.topSurface(gtx, colorSurface, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: spaceXL, Right: spaceXL, Top: spaceS, Bottom: spaceS}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			message := s.status
			if s.loading != "" {
				message = s.loading
			}
			if message == "" {
				message = "就绪 · 选择连接并打开数据库"
			}
			label := material.Label(s.theme, unit.Sp(11), message)
			label.Color = statusForeground(message)
			// 宽屏保持单行密度；窄屏允许两行，避免关键错误信息被截断。
			label.MaxLines = adaptiveMaxLines(gtx)
			return label.Layout(gtx)
		})
	})
}

// panel 为数据库浏览的各列提供统一的圆角面板底色。
func (s *workspace) panel(gtx layout.Context, child layout.Widget) layout.Dimensions {
	size := gtx.Constraints.Max
	if gtx.Constraints.Min.X > 0 {
		size.X = gtx.Constraints.Min.X
	}
	if gtx.Constraints.Min.Y > 0 {
		size.Y = gtx.Constraints.Min.Y
	}
	// 圆角半径不得超过短边一半，避免 clip.RRect 在小尺寸下生成退化路径。
	r := gtx.Dp(radiusM)
	if limit := min(size.X, size.Y) / 2; r > limit {
		r = limit
	}
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			fillRounded(gtx, size, radiusM, colorSurface)
			return layout.Dimensions{Size: size}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			if r > 0 {
				defer clip.RRect{Rect: image.Rectangle{Max: size}, NE: r, NW: r, SE: r, SW: r}.Push(gtx.Ops).Pop()
			}
			gtx.Constraints.Min = image.Point{}
			gtx.Constraints.Max = size
			return child(gtx)
		}),
	)
}

// layoutWorkbench 按 DBeaver / Navicat 的主界面组织右侧工作区：
// 左列是 Navigator 对象树（数据库 → 表 / 视图），右列是 SQL 编辑器与查询结果。
func (s *workspace) layoutWorkbench(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			width := gtx.Dp(unit.Dp(300))
			if isCompact(gtx) {
				width = gtx.Dp(unit.Dp(232))
			}
			if width > gtx.Constraints.Max.X {
				width = gtx.Constraints.Max.X
			}
			gtx.Constraints.Min = image.Pt(width, gtx.Constraints.Max.Y)
			gtx.Constraints.Max = image.Pt(width, gtx.Constraints.Max.Y)
			return s.panel(gtx, s.layoutNavigator)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
		layout.Flexed(1, s.layoutQueryWorkspace),
	)
}

// layoutNavigator 是单一对象树：连接下的数据库可展开，展开后按“表 / 视图”分组列出对象。
func (s *workspace) layoutNavigator(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: spaceM, Bottom: spaceS, Left: spaceM, Right: spaceM}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						label := material.Label(s.theme, unit.Sp(12), "对象导航")
						label.Color = colorTextPrimary
						return label.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if s.refreshObjects.Clicked(gtx) && s.selectedDatabase != "" {
							s.selectDatabase(s.selectedDatabase)
						}
						return s.smallButton(gtx, &s.refreshObjects, "刷新", s.selectedDatabase == "")
					}),
				)
			})
		}),
		layout.Rigid(s.divider),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(s.databases) == 0 {
				message := "该连接没有可见数据库"
				if s.loading != "" {
					message = s.loading
				}
				return layout.Inset{Top: spaceM, Left: spaceM, Right: spaceM}.Layout(gtx, styledText(s.theme, message, unit.Sp(11), colorTextMuted))
			}
			return layout.Inset{Top: spaceXS, Bottom: spaceXS}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				s.databaseList.Axis = layout.Vertical
				return material.List(s.theme, &s.databaseList).Layout(gtx, len(s.databases), func(gtx layout.Context, index int) layout.Dimensions {
					return s.layoutDatabaseNode(gtx, s.databases[index])
				})
			})
		}),
	)
}

// layoutDatabaseNode 渲染一个可折叠的数据库节点及其下属表与视图。
func (s *workspace) layoutDatabaseNode(gtx layout.Context, name string) layout.Dimensions {
	click := s.databaseClicks[name]
	if click == nil {
		click = new(widget.Clickable)
		s.databaseClicks[name] = click
	}
	if click.Clicked(gtx) {
		if name == s.selectedDatabase {
			s.selectedDatabase = ""
			s.selectedTable = ""
			s.selectedObject = ""
			s.tables, s.views = nil, nil
			s.columns = nil
			s.tableDetails = metadataapp.TableDetails{}
		} else {
			s.selectDatabase(name)
		}
	}
	expanded := name == s.selectedDatabase
	dot := colorTextMuted
	if expanded {
		dot = colorAccent
	}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.treeRow(gtx, click, dot, treeRow{label: name, expandable: true, expanded: expanded})
		}),
	}
	if expanded {
		if s.loading != "" && len(s.tables)+len(s.views) == 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: spaceXL, Right: spaceM, Top: spaceXS, Bottom: spaceXS}.Layout(gtx, styledText(s.theme, "正在加载对象…", unit.Sp(11), colorTextMuted))
			}))
		}
		if len(s.tables) > 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return s.treeGroupLabel(gtx, fmt.Sprintf("表（%d）", len(s.tables)))
			}))
			for _, table := range s.tables {
				children = append(children, layout.Rigid(s.layoutObjectNode(gtx, name, table, "T")))
			}
		}
		if len(s.views) > 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return s.treeGroupLabel(gtx, fmt.Sprintf("视图（%d）", len(s.views)))
			}))
			for _, view := range s.views {
				children = append(children, layout.Rigid(s.layoutObjectNode(gtx, name, view, "V")))
			}
		}
		if s.loading == "" && len(s.tables)+len(s.views) == 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: spaceXL, Right: spaceM, Top: spaceXS, Bottom: spaceXS}.Layout(gtx, styledText(s.theme, "此数据库没有表或视图", unit.Sp(11), colorTextMuted))
			}))
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// layoutObjectNode 渲染树中的表 / 视图行，点击即加载结构并同步到表结构面板。
func (s *workspace) layoutObjectNode(gtx layout.Context, database string, object metadataapp.Object, kind string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		key := database + "." + object.Name
		click := s.objectClicks[key]
		if click == nil {
			click = new(widget.Clickable)
			s.objectClicks[key] = click
		}
		if click.Clicked(gtx) {
			s.selectedObject = key
			s.loadTableSchema(object.Name)
		}
		active := key == s.selectedObject
		dot := colorTextMuted
		if active {
			dot = colorAccent
		}
		return s.treeRow(gtx, click, dot, treeRow{label: object.Name, badge: kind, detail: object.Engine, active: active, indent: 1})
	}
}

// treeRow 描述资源树中一行需要展示的信息。
type treeRow struct {
	label      string
	badge      string
	detail     string
	active     bool
	expandable bool
	expanded   bool
	indent     int
}

// treeRow 绘制资源树的一行：缩进 + 类型标记 + 名称 + 次要信息。
func (s *workspace) treeRow(gtx layout.Context, click *widget.Clickable, marker color.NRGBA, row treeRow) layout.Dimensions {
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		background := color.NRGBA{}
		if row.active {
			background = colorSurfaceSelected
		}
		return s.surface(gtx, background, radiusS, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(5), Bottom: unit.Dp(5), Left: unit.Dp(6) + unit.Dp(14*row.indent), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						glyph := "•"
						size := unit.Sp(10)
						switch {
						case row.badge != "":
							glyph = row.badge
						case row.expandable && row.expanded:
							glyph = "▾"
							size = unit.Sp(11)
						case row.expandable:
							glyph = "▸"
							size = unit.Sp(11)
						}
						label := material.Label(s.theme, size, glyph)
						label.Color = marker
						slot := gtx
						slot.Constraints.Min = image.Pt(gtx.Dp(unit.Dp(14)), gtx.Dp(unit.Dp(16)))
						slot.Constraints.Max = slot.Constraints.Min
						return layout.W.Layout(slot, label.Layout)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						size := unit.Sp(12)
						if row.expandable {
							size = unit.Sp(13)
						}
						foreground := colorTextSecondary
						if row.active || row.expanded {
							foreground = colorTextPrimary
						}
						label := material.Label(s.theme, size, row.label)
						label.Color = foreground
						label.MaxLines = adaptiveMaxLines(gtx)
						return label.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if row.detail == "" {
							return layout.Dimensions{}
						}
						label := material.Label(s.theme, unit.Sp(10), row.detail)
						label.Color = colorTextMuted
						label.MaxLines = 1
						return layout.Inset{Left: unit.Dp(6)}.Layout(gtx, label.Layout)
					}),
				)
			})
		})
	})
}

// primaryButton 用于主操作（执行查询），用强调色与次要按钮区分开。
func (s *workspace) primaryButton(gtx layout.Context, click *widget.Clickable, label string, disabled bool) layout.Dimensions {
	button := material.Button(s.theme, click, label)
	button.TextSize = unit.Sp(12)
	button.Inset = layout.Inset{Left: unit.Dp(14), Right: unit.Dp(14), Top: unit.Dp(5), Bottom: unit.Dp(5)}
	button.CornerRadius = radiusS
	button.Background = colorAccent
	button.Color = s.theme.Palette.ContrastFg
	if disabled {
		button.Background = colorSurfaceSubtle
		button.Color = colorTextSecondary
	}
	return button.Layout(gtx)
}

// switchSQLTab 在标签之间切换时保存当前编辑内容与查询结果，并载入目标标签的状态。
func (s *workspace) switchSQLTab(tabID workspaceui.TabID) {
	s.cancelActiveQuery("查询已取消（切换标签页）")
	activeID := s.sqlWorkspace.ActiveID()
	current := s.sqlTabStates[activeID]
	s.sqlWorkspace.UpdateSQL(activeID, s.queryEditor.Text())
	current.Columns, current.Data, current.Status = s.queryColumns, s.queryData, s.queryStatus
	s.sqlTabStates[activeID] = current
	s.sqlWorkspace.Activate(tabID)
	next := s.sqlTabStates[tabID]
	nextTab, _ := s.sqlWorkspace.Get(tabID)
	s.queryEditor.SetText(nextTab.Editor.Text())
	s.queryColumns, s.queryData, s.queryStatus = next.Columns, next.Data, next.Status
	s.clearQueryCellSelection()
	s.resetResultWidgets(s.queryColumns)
}
func (s *workspace) treeGroupLabel(gtx layout.Context, title string) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(2), Left: unit.Dp(26), Right: unit.Dp(8)}.Layout(gtx, styledText(s.theme, title, unit.Sp(10), colorTextMuted))
}

// smallButton 统一小尺寸次要按钮，避免各处手写字号与内边距。
func (s *workspace) smallButton(gtx layout.Context, click *widget.Clickable, label string, disabled bool) layout.Dimensions {
	button := material.Button(s.theme, click, label)
	button.TextSize = unit.Sp(11)
	button.Inset = layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(3), Bottom: unit.Dp(3)}
	button.CornerRadius = radiusS
	button.Background = colorSurfaceSubtle
	button.Color = colorTextSecondary
	if disabled {
		button.Color = colorTextMuted
	}
	return button.Layout(gtx)
}

func (s *workspace) loadTableSchema(table string) {
	if s.selectedID == "" || s.selectedDatabase == "" || table == "" {
		return
	}
	if s.cancelRequest != nil {
		s.cancelRequest()
	}
	s.requestSeq++
	seq, connectionID, database := s.requestSeq, s.selectedID, s.selectedDatabase
	s.selectedTable = table
	s.columns = nil
	s.tableDetails = metadataapp.TableDetails{}
	s.loading = "正在读取表结构与 DDL…"
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	s.cancelRequest = cancel
	go func() {
		defer cancel()
		columns, err := s.services.Metadata.GetTableSchema(ctx, connectionID, database, table)
		var details metadataapp.TableDetails
		if err == nil {
			details, err = s.services.Metadata.GetTableDetails(ctx, connectionID, database, table)
		}
		select {
		case s.schemaResults <- schemaResult{seq: seq, connectionID: connectionID, database: database, table: table, columns: columns, details: details, err: err}:
		default:
		}
		s.window.Invalidate()
	}()
}
