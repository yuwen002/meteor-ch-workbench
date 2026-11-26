// Package app 提供 Gio 原生工作台的对象导航视图。
package app

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	metadataapp "github.com/yuwen002/meteor-ch-workbench/internal/application/metadata"
)

// Explorer 对象导航视图。连接与元数据状态仍由 workspace 持有，视图只负责绘制和触发既有用例。

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
			s.resetDatabaseContext()
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
			s.explorerSelection.SetSelectedObject(key)
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

func (s *workspace) treeGroupLabel(gtx layout.Context, title string) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(2), Left: unit.Dp(26), Right: unit.Dp(8)}.Layout(gtx, styledText(s.theme, title, unit.Sp(10), colorTextMuted))
}

