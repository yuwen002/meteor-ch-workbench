// Package app 提供基于新 application 服务的 Gio 原生工作台入口。
package app

import (
	"context"
	"fmt"
	"io"
	"image/color"
	"strings"
	"time"

	gio "gioui.org/app"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/yuwen002/meteor-ch-workbench/internal/bootstrap"
	queryapp "github.com/yuwen002/meteor-ch-workbench/internal/application/query"
	metadataapp "github.com/yuwen002/meteor-ch-workbench/internal/application/metadata"
	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
	connectionui "github.com/yuwen002/meteor-ch-workbench/internal/gui/gioapp/connection"
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
	seq uint64
	connectionID string
	database string
	table string
	columns []metadataapp.Column
	details metadataapp.TableDetails
	err error
}

type queryEvent struct {
	seq uint64
	tab int
	columns []queryapp.Column
	batch [][]any
	summary queryapp.Summary
	err error
	done bool
}

type sqlTabState struct {
	Text string
	Columns []queryapp.Column
	Data [][]any
	Status string
}

type queryCellSelection struct {
	row int
	column queryapp.Column
	value string
}

type deleteResult struct { id string; err error }

type saveResult struct {
	item connection.Connection
	err error
}

type connectionCheck struct {
	seq uint64
	id string
	databases []string
	err error
}

type objectsResult struct {
	seq uint64
	connectionID string
	database string
	tables []metadataapp.Object
	views []metadataapp.Object
	err error
}

// Run 使用新架构服务启动 Gio 窗口，不再依赖旧 internal/app.Service。
func Run(window *gio.Window, services *bootstrap.Services) error {
	if services == nil || services.Connections == nil || services.Folders == nil || services.Metadata == nil || services.Queries == nil {
		return fmt.Errorf("新架构应用服务未初始化")
	}
	model, err := connectionui.NewModel(services.Connections, services.Folders)
	if err != nil {
		return err
	}
	state := &workspace{
		window: window,
		services: services,
		connections: model,
		theme: material.NewTheme(),
		checkResults: make(chan connectionCheck, 4),
		saveResults: make(chan saveResult, 2),
		deleteResults: make(chan deleteResult, 4),
		objectResults: make(chan objectsResult, 8),
		schemaResults: make(chan schemaResult, 4),
		queryResults: make(chan queryEvent, 128),
		sqlTabs: []string{"查询 1"},
		sqlTabTexts: make(map[int]string),
		sqlTabStates: map[int]sqlTabState{0: {}},
		queryRows: widget.List{List: layout.List{Axis: layout.Vertical}},
		queryHorizontal: widget.List{List: layout.List{Axis: layout.Horizontal}},
		queryDetailRows: widget.List{List: layout.List{Axis: layout.Vertical}},
		queryCellClicks: make(map[string]*widget.Clickable),
		databaseClicks: make(map[string]*widget.Clickable),
		objectClicks: make(map[string]*widget.Clickable),
		databaseList: widget.List{List: layout.List{Axis: layout.Vertical}},
		objectList: widget.List{List: layout.List{Axis: layout.Vertical}},
		objectTreeList: widget.List{List: layout.List{Axis: layout.Vertical}},
		columnList: widget.List{List: layout.List{Axis: layout.Vertical}},
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
		switch e := event.(type) {
		case gio.DestroyEvent:
			if state.cancelRequest != nil {
				state.cancelRequest()
			}
			return e.Err
		case gio.FrameEvent:
			gtx := gio.NewContext(&ops, e)
			state.drainResults()
			state.layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

type workspace struct {
	window *gio.Window
	services *bootstrap.Services
	connections *connectionui.Model
	connectionView *connectionui.View
	theme *material.Theme
	checkResults chan connectionCheck
	saveResults chan saveResult
	deleteResults chan deleteResult
	objectResults chan objectsResult
	schemaResults chan schemaResult
	queryResults chan queryEvent
	queryEditor widget.Editor
	sqlTabs []string
	sqlTabTexts map[int]string
	sqlTabStates map[int]sqlTabState
	sqlTabClicks []*widget.Clickable
	sqlNewTab widget.Clickable
	activeSQLTab int
	queryRows widget.List
	queryHorizontal widget.List
	queryDetailRows widget.List
	queryCellClicks map[string]*widget.Clickable
	selectedQueryCell *queryCellSelection
	queryDetailClose widget.Clickable
	queryDetailCopy widget.Clickable
	queryRun widget.Clickable
	queryCancel widget.Clickable
	querySeq uint64
	queryCancelFunc context.CancelFunc
	queryColumns []queryapp.Column
	queryData [][]any
	queryStatus string
	queryRunning bool
	databaseClicks map[string]*widget.Clickable
	objectClicks map[string]*widget.Clickable
	refreshObjects widget.Clickable
	refreshConnection widget.Clickable
	selectedObject string
	selectedTable string
	columns []metadataapp.Column
	tableDetails metadataapp.TableDetails
	databaseList widget.List
	objectList widget.List
	objectTreeList widget.List
	columnList widget.List
	selectedID string
	selectedName string
	connected bool
	retryConnection widget.Clickable
	databases []string
	selectedDatabase string
	tables []metadataapp.Object
	views []metadataapp.Object
	loading string
	status string
	requestSeq uint64
	cancelRequest context.CancelFunc
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
		select { case s.deleteResults <- deleteResult{id:id,err:err}: default: }
		s.window.Invalidate()
	}()
}

func (s *workspace) reloadConnections() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.connections.Reload(ctx); err != nil {
		// 配置读取失败时不能静默显示空侧栏；保留错误，便于定位旧配置未加载的原因。
		s.status = fmt.Sprintf("加载连接配置失败 · 配置文件：%s · 原因：%v", s.services.ConfigPath, err)
		s.window.Invalidate()
		return
	}
	snapshot := s.connections.Snapshot()
	s.status = fmt.Sprintf("配置正常 · %d 个连接 · %d 个分组", len(snapshot.Items), len(snapshot.Folders))
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
			if result.err != nil { s.status = "删除连接失败：" + result.err.Error() } else {
				if s.selectedID == result.id { s.selectedID = ""; s.selectedName = ""; s.connected = false; s.databases=nil; s.tables=nil; s.views=nil; s.columns=nil }
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
			if result.seq != s.querySeq { continue }
			if result.tab != s.activeSQLTab {
				state := s.sqlTabStates[result.tab]
				if len(result.columns) > 0 { state.Columns = result.columns; state.Data = nil }
				if len(result.batch) > 0 { state.Data = append(state.Data, result.batch...) }
				if result.done { state.Status = queryResultStatus(result, state.Columns) }
				s.sqlTabStates[result.tab] = state
				continue
			}
			if len(result.columns) > 0 { s.queryColumns = result.columns; s.queryData = nil; s.clearQueryCellSelection() }
			if len(result.batch) > 0 { s.queryData = append(s.queryData, result.batch...) }
			if result.done {
				s.queryRunning = false
				s.queryCancelFunc = nil
				if result.err != nil { s.queryStatus = "执行失败：" + result.err.Error()
				} else if result.summary.Truncated { s.queryStatus = fmt.Sprintf("已返回 %d 行（达到结果上限）· %s", result.summary.RowCount, result.summary.Elapsed.Round(time.Millisecond))
				} else if len(s.queryColumns) > 0 { s.queryStatus = fmt.Sprintf("查询完成 · %d 行 · %s", result.summary.RowCount, result.summary.Elapsed.Round(time.Millisecond))
				} else { s.queryStatus = fmt.Sprintf("执行完成 · 影响 %d 行 · %s", result.summary.RowsAffected, result.summary.Elapsed.Round(time.Millisecond)) }
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
	return layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, colorAppBackground, clip.Rect{Max: gtx.Constraints.Max}.Op())
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(s.layoutTopBar),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(16), Right: unit.Dp(16), Top: unit.Dp(14), Bottom: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								gtx.Constraints.Min.X = gtx.Dp(unit.Dp(270))
								gtx.Constraints.Max.X = gtx.Dp(unit.Dp(310))
								return s.connectionView.Layout(gtx, s.theme)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(14)}.Layout),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								if !s.connected {
									return s.layoutWelcome(gtx)
								}
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Flexed(0.44, s.layoutExplorer),
									layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
									layout.Flexed(0.56, s.layoutQueryWorkspace),
							)
							}),
						)
					})
				}),
				layout.Rigid(s.layoutStatusBar),
			)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return s.connectionView.Modal(gtx, s.theme)
		}),
	)
}

func (s *workspace) layoutWelcome(gtx layout.Context) layout.Dimensions {
	return s.panel(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(material.H4(s.theme, "欢迎使用 Meteor CH Workbench").Layout),
				layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
				layout.Rigid(material.Body1(s.theme, "选择左侧已保存的连接，或新建连接开始使用").Layout),
				layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					message := "尚未连接数据库"
					if s.selectedName != "" && s.loading != "" { message = "正在连接 · " + s.selectedName }
					if s.selectedName != "" && s.loading == "" && !s.connected { message = "连接失败 · " + s.selectedName }
					return material.Body2(s.theme, message).Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if s.status == "" { return layout.Dimensions{} }
					return material.Caption(s.theme, s.status).Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(18)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if s.selectedID != "" && s.loading == "" && !s.connected {
						if s.retryConnection.Clicked(gtx) {
							for _, item := range s.connections.Snapshot().Items {
								if item.ID == s.selectedID { s.selectConnection(item); break }
							}
						}
						button := material.Button(s.theme, &s.retryConnection, "重新连接")
						return button.Layout(gtx)
					}
					if s.refreshConnection.Clicked(gtx) { s.reloadConnections() }
					button := material.Button(s.theme, &s.refreshConnection, "刷新连接列表")
					return button.Layout(gtx)
				}),
			)
		})
	})
}
func (s *workspace) layoutTopBar(gtx layout.Context) layout.Dimensions {
	return s.panel(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(20), Right: unit.Dp(20), Top: unit.Dp(10), Bottom: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(material.H6(s.theme, "Meteor CH Workbench").Layout),
						layout.Rigid(material.Caption(s.theme, "轻量级 ClickHouse 数据库 IDE").Layout),
					)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(28)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					label := "未连接 · 选择左侧连接开始工作"
					if s.selectedName != "" && s.loading != "" { label = "◌  正在连接 · " + s.selectedName }
					if s.connected { label = "●  已连接 · " + s.selectedName }
					if s.selectedName != "" && s.loading == "" && !s.connected { label = "连接失败 · " + s.selectedName }
					return material.Body2(s.theme, label).Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if s.refreshConnection.Clicked(gtx) { s.reloadConnections() }
					button := material.Button(s.theme, &s.refreshConnection, "刷新连接")
					button.TextSize = unit.Sp(12)
					return button.Layout(gtx)
				}),
			)
		})
	})
}


func queryResultStatus(result queryEvent, columns []queryapp.Column) string {
	if result.err != nil { return "执行失败：" + result.err.Error() }
	if result.summary.Truncated { return fmt.Sprintf("已返回 %d 行（达到结果上限）· %s", result.summary.RowCount, result.summary.Elapsed.Round(time.Millisecond)) }
	if len(columns) > 0 { return fmt.Sprintf("查询完成 · %d 行 · %s", result.summary.RowCount, result.summary.Elapsed.Round(time.Millisecond)) }
	return fmt.Sprintf("执行完成 · 影响 %d 行 · %s", result.summary.RowsAffected, result.summary.Elapsed.Round(time.Millisecond))
}
func (s *workspace) executeQuery() {
	if !s.connected || s.selectedID == "" { s.queryStatus = "请先连接 ClickHouse"; return }
	sqlText := strings.TrimSpace(s.queryEditor.Text())
	if sqlText == "" { s.queryStatus = "请输入 SQL 语句"; return }
	if s.queryCancelFunc != nil { s.queryCancelFunc() }
	s.querySeq++
	seq := s.querySeq
	tab := s.activeSQLTab
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	s.queryCancelFunc = cancel
	s.queryRunning = true
	s.queryColumns, s.queryData = nil, nil
	s.clearQueryCellSelection()
	s.queryStatus = "正在执行 SQL…"
	request := queryapp.Request{ConnectionID:s.selectedID, SQL:sqlText, Mode:classifySQLMode(sqlText), MaxRows:10000, BatchSize:128, Timeout:30*time.Second}
	go func() {
		defer cancel()
		summary, err := s.services.Queries.Execute(ctx, request,
			func(columns []queryapp.Column) error {
				select { case s.queryResults <- queryEvent{seq:seq, tab:tab, columns:columns}: return nil; case <-ctx.Done(): return ctx.Err() }
			},
			func(rows [][]any) error {
				batch := make([][]any,len(rows))
				for i := range rows { batch[i] = append([]any(nil),rows[i]...) }
				select { case s.queryResults <- queryEvent{seq:seq,tab:tab,batch:batch}: return nil; case <-ctx.Done(): return ctx.Err() }
			},
		)
		s.queryResults <- queryEvent{seq:seq,tab:tab,summary:summary,err:err,done:true}
		s.window.Invalidate()
	}()
}


func (s *workspace) clearQueryCellSelection() {
	s.selectedQueryCell = nil
	s.queryCellClicks = make(map[string]*widget.Clickable)
	s.queryDetailRows.Position.First = 0
}

func (s *workspace) layoutQueryCellDetail(gtx layout.Context) layout.Dimensions {
	selection := s.selectedQueryCell
	if selection == nil { return layout.Dimensions{} }
	if s.queryDetailClose.Clicked(gtx) { s.clearQueryCellSelection(); return layout.Dimensions{} }
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

func (s *workspace) layoutQueryWorkspace(gtx layout.Context) layout.Dimensions {
	return s.panel(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top:unit.Dp(14),Bottom:unit.Dp(12),Left:unit.Dp(16),Right:unit.Dp(16)}.Layout(gtx,func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis:layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis:layout.Horizontal,Alignment:layout.Middle}.Layout(gtx,
						layout.Flexed(1,material.Subtitle1(s.theme,"SQL 查询").Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if s.queryRun.Clicked(gtx) && !s.queryRunning { s.executeQuery() }
							button:=material.Button(s.theme,&s.queryRun,"执行 SQL"); button.TextSize=unit.Sp(13)
							return layout.Inset{Right:unit.Dp(8)}.Layout(gtx,button.Layout)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if s.queryCancel.Clicked(gtx) && s.queryCancelFunc!=nil { s.queryCancelFunc(); s.queryStatus="正在取消…" }
							button:=material.Button(s.theme,&s.queryCancel,"停止"); button.TextSize=unit.Sp(13)
							return button.Layout(gtx)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis:layout.Horizontal, Alignment:layout.Middle}.Layout(gtx,
						layout.Flexed(1,func(gtx layout.Context) layout.Dimensions {
							if len(s.sqlTabClicks)<len(s.sqlTabs) {
								for len(s.sqlTabClicks)<len(s.sqlTabs) { s.sqlTabClicks=append(s.sqlTabClicks,new(widget.Clickable)) }
							}
							children:=make([]layout.FlexChild,0,len(s.sqlTabs))
							for i,name:=range s.sqlTabs {
								index,label:=i,name
								children=append(children,layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									click:=s.sqlTabClicks[index]
									if click.Clicked(gtx) && index!=s.activeSQLTab {
										current := s.sqlTabStates[s.activeSQLTab]
										current.Text = s.queryEditor.Text()
										current.Columns, current.Data, current.Status = s.queryColumns, s.queryData, s.queryStatus
										s.sqlTabStates[s.activeSQLTab] = current
										s.sqlTabTexts[s.activeSQLTab] = current.Text
										s.activeSQLTab=index
										next := s.sqlTabStates[index]
										s.queryEditor.SetText(next.Text)
										s.queryColumns, s.queryData, s.queryStatus = next.Columns, next.Data, next.Status
										s.queryRunning = false
										s.clearQueryCellSelection()
										s.queryCancelFunc = nil
									}
									if index==s.activeSQLTab { label="● "+name }
									button:=material.Button(s.theme,click,label); button.TextSize=unit.Sp(12)
									return layout.Inset{Right:unit.Dp(6)}.Layout(gtx,button.Layout)
								}))
							}
							return layout.Flex{Axis:layout.Horizontal}.Layout(gtx,children...)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if s.sqlNewTab.Clicked(gtx) {
								current := s.sqlTabStates[s.activeSQLTab]
								current.Text = s.queryEditor.Text()
								current.Columns, current.Data, current.Status = s.queryColumns, s.queryData, s.queryStatus
								s.sqlTabStates[s.activeSQLTab] = current
								s.sqlTabTexts[s.activeSQLTab] = current.Text
								s.sqlTabs=append(s.sqlTabs,fmt.Sprintf("查询 %d",len(s.sqlTabs)+1))
								s.activeSQLTab=len(s.sqlTabs)-1
								s.sqlTabStates[s.activeSQLTab] = sqlTabState{Status:"新查询标签页已创建"}
								s.queryEditor.SetText("")
								s.queryColumns, s.queryData = nil, nil
								s.clearQueryCellSelection()
								s.queryStatus="新查询标签页已创建"
							}
							button:=material.Button(s.theme,&s.sqlNewTab,"＋"); button.TextSize=unit.Sp(12)
							return button.Layout(gtx)
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Height:unit.Dp(6)}.Layout),
				layout.Flexed(0.42,func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top:unit.Dp(10), Bottom:unit.Dp(10), Left:unit.Dp(10), Right:unit.Dp(10)}.Layout(gtx,func(gtx layout.Context) layout.Dimensions {
						paint.FillShape(gtx.Ops, color.NRGBA{R: 247, G: 249, B: 252, A: 255}, clip.Rect{Max: gtx.Constraints.Max}.Op())
						return layout.Inset{Top:unit.Dp(10),Bottom:unit.Dp(10),Left:unit.Dp(12),Right:unit.Dp(12)}.Layout(gtx,material.Editor(s.theme,&s.queryEditor,"输入 SQL，例如 SELECT 1").Layout)
					})
				}),
				layout.Rigid(layout.Spacer{Height:unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					message:=s.queryStatus; if message=="" { message="结果将在这里显示" }
					return material.Caption(s.theme,message).Layout(gtx)
				}),
				layout.Flexed(0.58,func(gtx layout.Context) layout.Dimensions {
					if len(s.queryColumns)==0 { return material.Body2(s.theme,"暂无查询结果").Layout(gtx) }
					columnWeights := resultColumnFlexWeights(s.queryColumns, s.queryData, 40)
					if len(s.queryCellClicks) > 4096 { s.queryCellClicks = make(map[string]*widget.Clickable) }
					columnWidths := make([]int, len(columnWeights))
					totalWidth := 0
					for i, weight := range columnWeights {
						width := int(weight * 9)
						if width < 112 { width = 112 }
						if width > 320 { width = 320 }
						columnWidths[i] = gtx.Dp(unit.Dp(width))
						totalWidth += columnWidths[i]
					}
					return material.List(s.theme,&s.queryHorizontal).Layout(gtx,1,func(gtx layout.Context,_ int) layout.Dimensions {
						gtx.Constraints.Min.X = totalWidth
						gtx.Constraints.Max.X = totalWidth
						return layout.Flex{Axis:layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Top:unit.Dp(4),Bottom:unit.Dp(6)}.Layout(gtx,func(gtx layout.Context) layout.Dimensions {
									children:=make([]layout.FlexChild,0,len(s.queryColumns))
									for i,col:=range s.queryColumns {
										name,databaseType,width:=col.Name,col.DatabaseType,columnWidths[i]
										children=append(children,layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											gtx.Constraints.Min.X,gtx.Constraints.Max.X=width,width
											return layout.Flex{Axis:layout.Vertical}.Layout(gtx,
												layout.Rigid(material.Body2(s.theme,name).Layout),
												layout.Rigid(material.Caption(s.theme,databaseType).Layout),
											)
										}))
									}
									return layout.Flex{Axis:layout.Horizontal}.Layout(gtx,children...)
								})
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if s.selectedQueryCell == nil { return layout.Dimensions{} }
							gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = gtx.Dp(unit.Dp(112)), gtx.Dp(unit.Dp(112))
							return s.layoutQueryCellDetail(gtx)
						}),
							layout.Flexed(1,func(gtx layout.Context) layout.Dimensions {
								return material.List(s.theme,&s.queryRows).Layout(gtx,len(s.queryData),func(gtx layout.Context,index int) layout.Dimensions {
									row:=s.queryData[index]
									return layout.Inset{Top:unit.Dp(3),Bottom:unit.Dp(3)}.Layout(gtx,func(gtx layout.Context) layout.Dimensions {
										children:=make([]layout.FlexChild,0,len(s.queryColumns))
										for i:=range s.queryColumns {
										var cell any
										if i<len(row) { cell=row[i] }
										value,width:=formatResultCell(cell),columnWidths[i]
										cellIndex := i
										cellKey := fmt.Sprintf("%d:%d", index, cellIndex)
										click := s.queryCellClicks[cellKey]
										if click == nil { click = new(widget.Clickable); s.queryCellClicks[cellKey] = click }
										if click.Clicked(gtx) {
											column := s.queryColumns[cellIndex]
											s.selectedQueryCell = &queryCellSelection{row:index+1,column:column,value:resultCellRawValue(cell)}
										}
										children=append(children,layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											gtx.Constraints.Min.X,gtx.Constraints.Max.X=width,width
											return click.Layout(gtx,func(gtx layout.Context) layout.Dimensions {
												return material.Caption(s.theme,value).Layout(gtx)
											})
										}))
									}
									return layout.Flex{Axis:layout.Horizontal}.Layout(gtx,children...)
								})
								})
							}),
						)
					})
				}),
			)
		})
	})
}

func (s *workspace) layoutStatusBar(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Left: unit.Dp(18), Right: unit.Dp(18), Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		message := s.status
		if s.loading != "" {
			message = s.loading
		}
		if message == "" {
			message = "就绪 · 选择连接并打开数据库"
		}
		return material.Caption(s.theme, message).Layout(gtx)
	})
}

// panel 为数据库浏览的各列提供统一的浅色面板底色。
func (s *workspace) panel(gtx layout.Context, child layout.Widget) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, colorSurface, clip.Rect{Max: gtx.Constraints.Max}.Op())
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}),
		layout.Stacked(child),
	)
}

// layoutExplorer 使用单一对象树呈现数据库、表和视图，右侧显示选中对象结构。
func (s *workspace) layoutExplorer(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Flexed(0.40, func(gtx layout.Context) layout.Dimensions {
			return s.panel(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Flexed(0.42, s.layoutDatabasePanel),
					layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
					layout.Flexed(0.58, s.layoutTablePanel),
				)
			})
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
		layout.Flexed(0.60, func(gtx layout.Context) layout.Dimensions {
			return s.panel(gtx, s.layoutSchemaPanel)
		}),
	)
}

func (s *workspace) layoutTablePanel(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(8), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, material.Subtitle1(s.theme, "表与视图").Layout),
					layout.Rigid(material.Caption(s.theme, fmt.Sprintf("%d 张表 · %d 个视图", len(s.tables), len(s.views))).Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if s.refreshObjects.Clicked(gtx) && s.selectedDatabase != "" { s.selectDatabase(s.selectedDatabase) }
						button := material.Button(s.theme, &s.refreshObjects, "刷新")
						button.TextSize = unit.Sp(12)
						return button.Layout(gtx)
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				if s.selectedDatabase == "" { return material.Body2(s.theme, "选择数据库以加载表和视图").Layout(gtx) }
				items := make([]structObject, 0, len(s.tables)+len(s.views))
				items = append(items, s.tables...)
				items = append(items, s.views...)
				if len(items) == 0 && s.loading == "" { return material.Body2(s.theme, "该数据库没有可显示的表或视图").Layout(gtx) }
				return material.List(s.theme, &s.objectList).Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
					item := items[index]
					key := s.selectedDatabase + "." + item.Name
					click := s.objectClicks[key]
					if click == nil { click = new(widget.Clickable); s.objectClicks[key] = click }
					if click.Clicked(gtx) { s.selectedObject = key; s.loadTableSchema(item.Name) }
					return layout.Inset{Top: unit.Dp(5), Bottom: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							kind := "表"
							if index >= len(s.tables) { kind = "视图" }
							title := item.Name
							if key == s.selectedObject { title = "● " + title }
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(material.Body1(s.theme, title).Layout),
								layout.Rigid(material.Caption(s.theme, kind+" · "+item.Engine).Layout),
							)
						})
					})
				})
			}),
		)
	})
}

func (s *workspace) layoutSchemaPanel(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(8), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(material.Subtitle1(s.theme, "表结构 / DDL").Layout),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				if s.selectedTable == "" { return material.Body2(s.theme, "选择一张表查看字段、引擎和建表语句").Layout(gtx) }
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(material.Caption(s.theme, fmt.Sprintf("引擎：%s · 主键：%s", s.tableDetails.Engine, s.tableDetails.PrimaryKey)).Layout),
					layout.Rigid(material.Caption(s.theme, fmt.Sprintf("分区：%s · 排序：%s", s.tableDetails.PartitionKey, s.tableDetails.SortingKey)).Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return material.List(s.theme, &s.columnList).Layout(gtx, len(s.columns), func(gtx layout.Context, index int) layout.Dimensions {
							column := s.columns[index]
							marker := ""
							if column.IsPrimary { marker += " · 主键" }
							if column.HasDefault { marker += " · 默认值" }
							return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, material.Body2(s.theme, fmt.Sprintf("%s  %s%s", column.Name, column.Type, marker)).Layout)
						})
					}),
					layout.Rigid(material.Caption(s.theme, s.tableDetails.TTLExpression).Layout),
				)
			}),
		)
	})
}

func (s *workspace) layoutDatabasePanel(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(8), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, material.Subtitle1(s.theme, "数据库").Layout),
					layout.Rigid(material.Caption(s.theme, fmt.Sprintf("%d", len(s.databases))).Layout),
				)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				if s.selectedID == "" {
					return material.Body2(s.theme, "先从左侧选择一个连接").Layout(gtx)
				}
				if len(s.databases) == 0 && s.loading == "" {
					return material.Body2(s.theme, "没有数据库，或尚未加载").Layout(gtx)
				}
				return material.List(s.theme, &s.databaseList).Layout(gtx, len(s.databases), func(gtx layout.Context, index int) layout.Dimensions {
					name := s.databases[index]
					click := s.databaseClicks[name]
					if click == nil {
						click = new(widget.Clickable)
						s.databaseClicks[name] = click
					}
					if click.Clicked(gtx) {
						s.selectDatabase(name)
					}
					return layout.Inset{Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							label := "▸  " + name
							if name == s.selectedDatabase {
								label = "▾  " + name
							}
							return material.Body2(s.theme, label).Layout(gtx)
						})
					})
				})
			}),
		)
	})
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

func (s *workspace) layoutObjectsPanel(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(8), Left: unit.Dp(14), Right: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, material.Subtitle1(s.theme, "表与视图").Layout),
					layout.Rigid(material.Caption(s.theme, fmt.Sprintf("%d 张表 · %d 个视图", len(s.tables), len(s.views))).Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if s.refreshObjects.Clicked(gtx) && s.selectedDatabase != "" {
							s.selectDatabase(s.selectedDatabase)
						}
						button := material.Button(s.theme, &s.refreshObjects, "刷新")
						button.TextSize = unit.Sp(12)
						return button.Layout(gtx)
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Flexed(0.48, func(gtx layout.Context) layout.Dimensions {
				if s.selectedDatabase == "" {
					return material.Body2(s.theme, "选择左侧数据库以加载对象").Layout(gtx)
				}
				items := make([]structObject, 0, len(s.tables)+len(s.views))
				items = append(items, s.tables...)
				items = append(items, s.views...)
				if len(items) == 0 && s.loading == "" {
					return material.Body2(s.theme, "该数据库没有可显示的表或视图").Layout(gtx)
				}
				return material.List(s.theme, &s.objectList).Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
					item := items[index]
					key := s.selectedDatabase + "." + item.Name
					click := s.objectClicks[key]
					if click == nil {
						click = new(widget.Clickable)
						s.objectClicks[key] = click
					}
					if click.Clicked(gtx) {
						s.selectedObject = key
						s.loadTableSchema(item.Name)
					}
					return layout.Inset{Top: unit.Dp(5), Bottom: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							kind := "表"
							if index >= len(s.tables) {
								kind = "视图"
							}
							title := item.Name
							if key == s.selectedObject {
								title = "● " + title
							}
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(material.Body1(s.theme, title).Layout),
								layout.Rigid(material.Caption(s.theme, kind+" · "+item.Engine).Layout),
							)
						})
					})
				})
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
			layout.Rigid(material.Subtitle1(s.theme, "表结构 / DDL").Layout),
			layout.Flexed(0.52, func(gtx layout.Context) layout.Dimensions {
				if s.selectedTable == "" {
					return material.Body2(s.theme, "选择一张表查看字段、引擎和建表语句").Layout(gtx)
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(material.Caption(s.theme, fmt.Sprintf("引擎：%s · 主键：%s", s.tableDetails.Engine, s.tableDetails.PrimaryKey)).Layout),
					layout.Rigid(material.Caption(s.theme, fmt.Sprintf("分区：%s · 排序：%s", s.tableDetails.PartitionKey, s.tableDetails.SortingKey)).Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return material.List(s.theme, &s.columnList).Layout(gtx, len(s.columns), func(gtx layout.Context, index int) layout.Dimensions {
							column := s.columns[index]
							marker := ""
							if column.IsPrimary { marker += " · 主键" }
							if column.HasDefault { marker += " · 默认值" }
							return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx,
								material.Body2(s.theme, fmt.Sprintf("%s  %s%s", column.Name, column.Type, marker)).Layout)
						})
					}),
					layout.Rigid(material.Caption(s.theme, s.tableDetails.TTLExpression).Layout),
				)
			}),
		)
	})
}
