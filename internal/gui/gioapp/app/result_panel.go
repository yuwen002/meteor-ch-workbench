package app

import (
	"fmt"
	"io"
	"strings"

	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

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
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return s.layoutContextMenu(gtx, "results") }),
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
	if s.queryCellClicks == nil {
		s.queryCellClicks = make(map[int64]*widget.Clickable)
	}
	if s.resultCellText == nil {
		s.resultCellText = make(map[int64]string)
	}
	if s.queryCellRightClicks == nil {
		s.queryCellRightClicks = make(map[int64]*rightClickTag)
	}
	if len(s.queryCellClicks) > 4096 {
		clear(s.queryCellClicks)
		clear(s.resultCellText)
	}
	s.resultPixelWidths = s.resultPixelWidths[:0]
	if len(s.resultHeaderRightClicks) != len(s.queryColumns) {
		s.resultHeaderRightClicks = make([]*rightClickTag, len(s.queryColumns))
		for i := range s.resultHeaderRightClicks { s.resultHeaderRightClicks[i] = new(rightClickTag) }
	}
	totalWidth := 0
	for index, width := range s.columnWidths {
		pixels := gtx.Dp(width)
		s.resultPixelWidths = append(s.resultPixelWidths, pixels)
		if !s.hiddenResultColumns[index] { totalWidth += pixels }
	}
	return material.List(s.theme, &s.queryHorizontal).Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		gtx.Constraints.Min.X = totalWidth
		gtx.Constraints.Max.X = totalWidth
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					s.resultHeaderChildren = s.resultHeaderChildren[:0]
					for i, col := range s.queryColumns {
						if s.hiddenResultColumns[i] { continue }
						columnIndex, name, databaseType := i, col.Name, col.DatabaseType
						width := s.resultPixelWidths[columnIndex]
						s.resultHeaderChildren = append(s.resultHeaderChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							s.watchSecondaryClick(gtx, s.resultHeaderRightClicks[columnIndex], func() {
								s.openContextMenu("header", "", "", -1, columnIndex)
							})
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
				dims := material.List(s.theme, &s.queryRows).Layout(gtx, len(s.resultView), func(gtx layout.Context, index int) layout.Dimensions {
					display := s.resultView[index]
					var row []any
					if display.Index >= 0 && display.Index < len(s.queryData) {
						row = s.queryData[display.Index]
					}
					return layout.Inset{Top: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						s.resultRowChildren = s.resultRowChildren[:0]
						for cellIndex := range s.queryColumns {
							if s.hiddenResultColumns[cellIndex] { continue }
							var cell any
							if cellIndex < len(row) {
								cell = row[cellIndex]
							}
							key := resultCellKey(index, cellIndex)
							value, ok := s.resultCellText[key]
							if !ok {
								value = formatResultCell(cell)
								s.resultCellText[key] = value
							}
							click := s.queryCellClicks[key]
							rightTag := s.queryCellRightClicks[key]
							if rightTag == nil { rightTag = new(rightClickTag); s.queryCellRightClicks[key] = rightTag }
							if click == nil {
								click = new(widget.Clickable)
								s.queryCellClicks[key] = click
							}
							s.watchSecondaryClick(gtx, rightTag, func() { s.openContextMenu("cell", "", "", index, cellIndex) })
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
				pruneResultCellCaches(s.queryCellClicks, s.resultCellText, s.queryRows.Position, len(s.resultView), 2)
				return dims
			}),
		)
	})
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
