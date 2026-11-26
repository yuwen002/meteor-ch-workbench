// Package app 提供基于新 application 服务的 Gio 原生工作台入口。
package app

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"

	workspaceui "github.com/yuwen002/meteor-ch-workbench/internal/gui/gioapp/workspace"
)

// SQL 工作区的布局与交互视图。状态仍由 workspace 统一持有，避免改变标签状态生命周期。

func (s *workspace) clearQueryCellSelection() {
	s.selectedQueryCell = nil
	clear(s.queryCellClicks)
	clear(s.resultCellText)
	s.queryDetailRows.Position.First = 0
}

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
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !s.savedSQLVisible { return layout.Dimensions{} }
				return s.layoutSavedSQLPanel(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !s.historyVisible { return layout.Dimensions{} }
				return s.layoutHistoryPanel(gtx)
			}),
			layout.Rigid(s.divider),
			layout.Rigid(s.layoutTabStrip),
			layout.Rigid(s.divider),
			layout.Flexed(float32(s.settingsPreferences.EditorRatio), s.layoutSQLEditor),
			layout.Rigid(s.divider),
			layout.Flexed(float32(1-s.settingsPreferences.EditorRatio), s.layoutResultsPanel),
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
				if s.historyToggle.Clicked(gtx) {
					s.historyVisible = !s.historyVisible
					if s.historyVisible { s.reloadHistory() }
				}
				label := "查询历史"
				if s.historyVisible { label = "收起历史" }
				return s.smallButton(gtx, &s.historyToggle, label, false)
			}),
			layout.Rigid(layout.Spacer{Width: spaceS}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if s.savedSQLToggle.Clicked(gtx) {
					s.savedSQLVisible = !s.savedSQLVisible
					if s.savedSQLVisible { s.reloadSavedSQL() }
				}
				label := "收藏 SQL"
				if s.savedSQLVisible { label = "收起收藏" }
				return s.smallButton(gtx, &s.savedSQLToggle, label, false)
			}),
			layout.Rigid(layout.Spacer{Width: spaceS}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if s.saveSQL.Clicked(gtx) && !s.queryRunning {
					s.saveCurrentSQL()
				}
				return s.smallButton(gtx, &s.saveSQL, "收藏 SQL", s.queryRunning || strings.TrimSpace(s.queryEditor.Text()) == "" || !s.connected)
			}),
			layout.Rigid(layout.Spacer{Width: spaceS}.Layout),
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
	s.ensureSQLTabClickables(items)
	return layout.Inset{Top: spaceXS, Bottom: spaceXS, Left: spaceS, Right: spaceM}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				s.sqlTabList.Axis = layout.Horizontal
				return material.List(s.theme, &s.sqlTabList).Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
					return layout.Inset{Right: spaceXS}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return s.layoutSQLTab(gtx, items[index])
					})
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if s.sqlNewTab.Clicked(gtx) {
					if len(items) >= 12 {
						s.queryStatus = "查询标签页最多支持 12 个"
					} else {
						s.cancelActiveQuery("查询已取消（新建标签页）")
						s.saveActiveSQLTabState()
						created := s.sqlWorkspace.Create()
						s.sqlTabStates[created.ID] = sqlTabState{ConnectionID: s.selectedID, ConnectionName: s.selectedName, Database: s.selectedDatabase, Status: "新查询标签页已创建", SortColumn: -1}
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

func (s *workspace) layoutSQLTab(gtx layout.Context, tab workspaceui.Tab) layout.Dimensions {
	active := tab.ID == s.sqlWorkspace.ActiveID()
	click := s.sqlTabClicks[tab.ID]
	if click.Clicked(gtx) && !active {
		s.switchSQLTab(tab.ID)
	}
	closeClick := s.sqlTabCloseClicks[tab.ID]
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
					s.closeSQLTab(tab.ID)
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
