// Package app 提供基于新 application 服务的 Gio 原生工作台入口。
package app

import (
	"context"
	"fmt"
	"time"

	gio "gioui.org/app"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/yuwen002/meteor-ch-workbench/internal/bootstrap"
	metadataapp "github.com/yuwen002/meteor-ch-workbench/internal/application/metadata"
	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
	metadataapp "github.com/yuwen002/meteor-ch-workbench/internal/application/metadata"
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
	if services == nil || services.Connections == nil || services.Metadata == nil {
		return fmt.Errorf("新架构应用服务未初始化")
	}
	model, err := connectionui.NewModel(services.Connections)
	if err != nil {
		return err
	}
	state := &workspace{
		window: window,
		services: services,
		connections: model,
		theme: material.NewTheme(),
		checkResults: make(chan connectionCheck, 4),
		objectResults: make(chan objectsResult, 8),
		schemaResults: make(chan schemaResult, 4),
		databaseClicks: make(map[string]*widget.Clickable),
		objectClicks: make(map[string]*widget.Clickable),
		databaseList: widget.List{List: layout.List{Axis: layout.Vertical}},
		objectList: widget.List{List: layout.List{Axis: layout.Vertical}},
		columnList: widget.List{List: layout.List{Axis: layout.Vertical}},
	}
	state.connectionView, err = connectionui.NewView(model, state.reloadConnections, state.selectConnection)
	if err != nil {
		return err
	}
	go state.reloadConnections()

	var ops gio.Ops
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
	objectResults chan objectsResult
	schemaResults chan schemaResult
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
	columnList widget.List
	selectedID string
	selectedName string
	databases []string
	selectedDatabase string
	tables []metadataapp.Object
	views []metadataapp.Object
	loading string
	status string
	requestSeq uint64
	cancelRequest context.CancelFunc
}

func (s *workspace) reloadConnections() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = s.connections.Reload(ctx)
	s.window.Invalidate()
}

func (s *workspace) selectConnection(item connection.Connection) {
	s.selectedID = item.ID
	s.selectedName = item.Name
	s.selectedDatabase = ""
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
	if s.selectedID == "" {
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
		case result := <-s.checkResults:
			if result.seq != s.requestSeq || result.id != s.selectedID {
				continue
			}
			s.loading = ""
			if result.err != nil {
				s.status = "连接测试或数据库读取失败：" + result.err.Error()
				s.databases = nil
			} else {
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
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(s.layoutTopBar),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(14), Right: unit.Dp(14), Top: unit.Dp(12), Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Dp(unit.Dp(250))
						gtx.Constraints.Max.X = gtx.Dp(unit.Dp(290))
						return s.connectionView.Layout(gtx, s.theme)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(14)}.Layout),
					layout.Flexed(1, s.layoutExplorer),
				)
			})
		}),
		layout.Rigid(s.layoutStatusBar),
	)
}

func (s *workspace) layoutTopBar(gtx layout.Context) layout.Dimensions {
	return layout.Inset{Left: unit.Dp(20), Right: unit.Dp(20), Top: unit.Dp(14), Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(material.H6(s.theme, "Meteor CH Workbench").Layout),
					layout.Rigid(material.Caption(s.theme, "ClickHouse 原生数据工作台").Layout),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				label := "未连接"
				if s.selectedName != "" {
					label = "当前连接 · " + s.selectedName
				}
				return material.Body2(s.theme, label).Layout(gtx)
			}),
		)
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

func (s *workspace) layoutExplorer(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						title := "数据库浏览"
						if s.selectedName != "" {
							title = s.selectedName
						}
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(material.H6(s.theme, title).Layout),
							layout.Rigid(material.Caption(s.theme, "选择数据库后查看表、视图及对象信息").Layout),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if s.refreshConnection.Clicked() && s.selectedID != "" {
							for _, item := range s.connections.Snapshot().Items {
								if item.ID == s.selectedID {
									s.selectConnection(item)
									break
								}
							}
						}
						button := material.Button(s.theme, &s.refreshConnection, "刷新连接")
						button.TextSize = unit.Sp(12)
						return button.Layout(gtx)
					}),
				)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(0.34, s.layoutDatabasePanel),
				layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
				layout.Flexed(0.66, s.layoutObjectsPanel),
			)
		}),
	)
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
						if s.refreshObjects.Clicked() && s.selectedDatabase != "" {
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
