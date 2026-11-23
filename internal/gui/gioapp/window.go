// Package gioapp 提供 Meteor CH Workbench 的 Gio 原生界面原型。
//
// 当前阶段只验证主窗口布局与基础文本输入；连接、查询执行和结果绑定
// 将在确认 SQL 编辑器能力后接入 internal/app.Service。
package gioapp

import (
	"fmt"
	"image/color"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type context = layout.Context
type dimensions = layout.Dimensions

// Run 运行 Gio 窗口事件循环。
func Run(window *app.Window) error {
	state := newScreen()
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

type screen struct {
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
	status      string
	lightTheme  bool
	sidebarOpen bool
}

func newScreen() *screen {
	s := &screen{
		theme:       material.NewTheme(),
		status:      "尚未连接数据库 · 当前为 Gio 界面原型",
		lightTheme:  false,
		sidebarOpen: true,
		editorRatio: 0.62,
	}
	s.sqlEditor.SetText("SELECT\n    database,\n    table,\n    engine\nFROM system.tables\nORDER BY database, table\nLIMIT 100;")
	s.applyTheme()
	return s
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
	for s.execute.Clicked(gtx) {
		s.status = "SQL 编辑器已就绪；查询执行将在服务装配阶段接入"
	}
	for s.connect.Clicked(gtx) {
		s.status = "连接管理界面正在建设，当前尚未连接数据库"
	}
	for s.stop.Clicked(gtx) {
		s.status = "当前没有正在执行的查询"
	}
	for s.sidebarToggle.Clicked(gtx) {
		s.sidebarOpen = !s.sidebarOpen
	}
	for s.editorSmaller.Clicked(gtx) {
		s.editorRatio = clampEditorRatio(s.editorRatio - 0.05)
	}
	for s.editorLarger.Clicked(gtx) {
		s.editorRatio = clampEditorRatio(s.editorRatio + 0.05)
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
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx context) dimensions {
				return material.H6(s.theme, "连接与数据库").Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
			layout.Rigid(func(gtx context) dimensions {
				return material.Body1(s.theme, "▾ ClickHouse 连接").Layout(gtx)
			}),
			layout.Rigid(func(gtx context) dimensions {
				return material.Body2(s.theme, "　▾ default").Layout(gtx)
			}),
			layout.Rigid(func(gtx context) dimensions {
				return material.Body2(s.theme, "　　▤ system").Layout(gtx)
			}),
			layout.Rigid(func(gtx context) dimensions {
				return material.Body2(s.theme, "　　　▤ tables").Layout(gtx)
			}),
			layout.Rigid(func(gtx context) dimensions {
				return material.Body2(s.theme, "　　　▤ columns").Layout(gtx)
			}),
			layout.Rigid(func(gtx context) dimensions {
				return material.Body2(s.theme, "　　　▤ query_log").Layout(gtx)
			}),
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
			layout.Rigid(func(gtx context) dimensions {
				return material.Body2(s.theme, "最近连接").Layout(gtx)
			}),
		)
	})
}

func (s *screen) layoutEditorAndResults(gtx context) dimensions {
	return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(8), Right: unit.Dp(12)}.Layout(gtx, func(gtx context) dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Flexed(s.editorRatio, s.layoutEditor),
			layout.Rigid(s.layoutSplitControls),
			layout.Flexed(1-s.editorRatio, s.layoutResults),
		)
	})
}

func clampEditorRatio(value float32) float32 {
	if value < 0.35 { return 0.35 }
	if value > 0.80 { return 0.80 }
	return value
}

func (s *screen) layoutSplitControls(gtx context) dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx context) dimensions {
			return material.Caption(s.theme, "编辑器 / 结果区比例").Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx context) dimensions {
			return material.Button(s.theme, &s.editorSmaller, "编辑器 −").Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx context) dimensions {
			return material.Button(s.theme, &s.editorLarger, "编辑器 +").Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
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
