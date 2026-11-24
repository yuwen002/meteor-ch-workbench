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
	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
	connectionui "github.com/yuwen002/meteor-ch-workbench/internal/gui/gioapp/connection"
)

type metadataService interface {
	ListDatabases(context.Context, string) ([]string, error)
	ListTables(context.Context, string, string) ([]structObject, error)
	ListViews(context.Context, string, string) ([]structObject, error)
}

type structObject = struct {
	Name string
	Engine string
	Comment string
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
	tables []structObject
	views []structObject
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
		databaseClicks: make(map[string]*widget.Clickable),
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
	databaseClicks map[string]*widget.Clickable
	databaseList widget.List
	objectList widget.List
	selectedID string
	selectedName string
	databases []string
	selectedDatabase string
	tables []structObject
	views []structObject
	loading string
	status string
	requestSeq uint64
}

func (s *workspace) reloadConnections() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.connections.Reload(ctx); err != nil {
		s.status = "读取连接列表失败：" + err.Error()
	}
	s.window.Invalidate()
}

func (s *workspace) selectConnection(item connection.Connection) {
	s.selectedID = item.ID
	s.selectedName = item.Name
	s.selectedDatabase = ""
	s.databases = nil
	s.tables, s.views = nil, nil
	s.loading = "正在测试连接并读取数据库…"
	s.requestSeq++
	seq := s.requestSeq
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err := s.connections.Test(ctx, item)
		var databases []string
		if err == nil {
			databases, err = s.services.Metadata.ListDatabases(ctx, item.ID)
		}
		select {
		case s.checkResults <- connectionCheck{seq: seq, id: item.ID, databases: databases, err: err}:
		default:
			s.checkResults <- connectionCheck{seq: seq, id: item.ID, databases: databases, err: err}
		}
		_ = seq
		s.window.Invalidate()
	}()
}

func (s *workspace) selectDatabase(name string) {
	if s.selectedID == "" {
		return
	}
	s.selectedDatabase = name
	s.tables, s.views = nil, nil
	s.loading = "正在读取表和视图…"
	s.requestSeq++
	seq, connectionID := s.requestSeq, s.selectedID
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
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
			s.objectResults <- result
		}
		_ = seq
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
		default:
			return
		}
	}
}

func (s *workspace) layout(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: 16, Right: 16, Top: 12, Bottom: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.H6(s.theme, "Meteor CH Workbench").Layout(gtx)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: 12, Right: 12, Bottom: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Constrain(gtx, func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.X = gtx.Dp(unit.Dp(260))
							gtx.Constraints.Max.X = gtx.Dp(unit.Dp(320))
							return s.connectionView.Layout(gtx, s.theme)
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Spacer{Width: unit.Dp(1)}.Layout(gtx)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: 20, Right: 20, Top: 12, Bottom: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(material.Subtitle1(s.theme, "数据库浏览").Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								label := "选择左侧连接以浏览数据库"
								if s.selectedName != "" {
									label = "连接：" + s.selectedName
								}
								return material.Body2(s.theme, label).Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if s.loading != "" {
									return material.Caption(s.theme, s.loading).Layout(gtx)
								}
								return layout.Dimensions{}
							}),
							layout.Flexed(1, s.layoutDatabaseAndObjects),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return material.Caption(s.theme, s.status).Layout(gtx)
							}),
						)
					})
				}),
			)
		}),
	)
}

func (s *workspace) layoutDatabaseAndObjects(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Flexed(0.38, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(material.Body1(s.theme, "数据库").Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
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
						return layout.Inset{Top: 3, Bottom: 3, Left: 4, Right: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								label := name
								if name == s.selectedDatabase {
									label = "● " + label
								}
								return material.Body2(s.theme, label).Layout(gtx)
							})
						})
					})
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(16)}.Layout),
		layout.Flexed(0.62, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(material.Body1(s.theme, "表与视图").Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					items := make([]string, 0, len(s.tables)+len(s.views))
					for _, item := range s.tables {
						items = append(items, "表  ·  "+item.Name+"  ("+item.Engine+")")
					}
					for _, item := range s.views {
						items = append(items, "视图  ·  "+item.Name)
					}
					return material.List(s.theme, &s.objectList).Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
						return layout.Inset{Top: 4, Bottom: 4}.Layout(gtx, material.Body2(s.theme, items[index]).Layout)
					})
				}),
			)
		}),
	)
}
