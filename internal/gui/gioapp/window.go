// Package gioapp 提供 Meteor CH Workbench 的 Gio 原生界面原型。
//
// 当前阶段已接入共享应用服务读取本地连接配置；数据库连接测试、SQL 执行
// 与结果绑定仍在后续阶段实现。
package gioapp

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/app"
	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/op/clip"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	appsvc "github.com/yuwen002/meteor-ch-workbench/internal/app"
	"github.com/yuwen002/meteor-ch-workbench/internal/config"
)

type context = layout.Context
type dimensions = layout.Dimensions

// Run 运行 Gio 窗口事件循环。
func Run(window *app.Window, service *appsvc.Service) error {
	state := newScreen(window, service)
	var ops op.Ops
	for {
		switch event := window.Event().(type) {
		case app.DestroyEvent:
			return event.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, event)
			state.layout(gtx)
			event.Frame(gtx.Ops)
		}
	}
}

type connectResult struct { connectionID string; databases []string; err error }

type screen struct {
	window *app.Window
	connectResults chan connectResult
	databaseNames []string
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

func newScreen(window *app.Window, service *appsvc.Service) *screen {
	s := &screen{
		window: window,
		connectResults: make(chan connectResult, 4),
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

func (s *screen) drainConnectResults() {
	for {
		select {
		case result := <-s.connectResults:
			if result.connectionID != s.selectedConnectionID { continue }
			if result.err != nil {
				s.treeConnected = false
				s.databaseNames = nil
				s.status = "连接或加载数据库失败：" + result.err.Error()
			} else {
				s.treeConnected = true
				s.databaseNames = result.databases
				s.status = fmt.Sprintf("连接成功，已加载 %d 个数据库", len(result.databases))
			}
		default:
			return
		}
	}
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
	s.selectedConnectionID = connection.ID
	s.status = "正在连接 " + connection.Name + "…"
	go func() {
		err := s.service.TestConnection(connection)
		var databases []string
		if err == nil {
			databases, err = s.service.GetDatabases(connection.ID)
		}
		s.connectResults <- connectResult{connectionID: connection.ID, databases: databases, err: err}
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
	for s.execute.Clicked(gtx) {
		s.status = "SQL 编辑器已就绪；查询执行将在服务装配阶段接入"
	}
	for s.connect.Clicked(gtx) {
		s.connectSelected()
	}
	for s.stop.Clicked(gtx) {
		s.status = "当前没有正在执行的查询"
	}
	for s.sidebarToggle.Clicked(gtx) {
		s.sidebarOpen = !s.sidebarOpen
		s.saveWorkspaceLayout()
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
				return material.Button(s.theme, &s.connect, "连接").Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx context) dimensions {
				return material.Button(s.theme, &s.execute, "执行 SQL").Layout(gtx)
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
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
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
	)
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
				children = append(children, layout.Rigid(func(gtx context) dimensions {
					return material.Body2(s.theme, "  ▸ "+name).Layout(gtx)
				}))
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
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx context) dimensions {
				return material.H6(s.theme, "查询结果").Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
			layout.Rigid(func(gtx context) dimensions {
				return material.Body2(s.theme, "数据表格　·　统计信息　·　执行详情").Layout(gtx)
			}),
			layout.Flexed(1, func(gtx context) dimensions {
				return material.Body1(s.theme, "执行查询后，结果将在这里展示").Layout(gtx)
			}),
		)
	})
}

func (s *screen) layoutStatus(gtx context) dimensions {
	return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx context) dimensions {
		return material.Caption(s.theme, s.status).Layout(gtx)
	})
}
