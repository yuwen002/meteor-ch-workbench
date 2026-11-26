package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/queryhistory"
)

func filterHistory(items []queryhistory.Entry, keyword string) []queryhistory.Entry {
	needle := strings.ToLower(strings.TrimSpace(keyword))
	if needle == "" {
		return append([]queryhistory.Entry(nil), items...)
	}
	filtered := make([]queryhistory.Entry, 0, len(items))
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.SQL), needle) ||
			strings.Contains(strings.ToLower(item.ConnectionID), needle) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (s *workspace) reloadHistory() {
	s.historyListSeq++
	seq := s.historyListSeq
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		items, err := s.services.History.List(ctx)
		select {
		case s.historyListResults <- historyListResult{seq: seq, items: items, err: err}:
		case <-ctx.Done():
		}
		s.window.Invalidate()
	}()
}

func (s *workspace) mutateHistory(action, id string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var err error
		if action == "删除查询历史" {
			err = s.services.History.Delete(ctx, id)
		} else {
			err = s.services.History.Clear(ctx)
		}
		select {
		case s.historyResults <- historySaveResult{action: action, err: err}:
		case <-ctx.Done():
		}
		s.window.Invalidate()
	}()
}

func (s *workspace) layoutHistoryPanel(gtx layout.Context) layout.Dimensions {
	filtered := filterHistory(s.historyItems, s.historySearch.Text())
	originalIndex := make(map[string]int, len(s.historyItems))
	for i, item := range s.historyItems {
		originalIndex[item.ID] = i
	}

	height := gtx.Dp(unit.Dp(190))
	gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = height, height
	return s.surface(gtx, colorSurfaceSubtle, radiusS, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: spaceXS, Bottom: spaceXS, Left: spaceM, Right: spaceM}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(styledText(s.theme, fmt.Sprintf("查询历史（%d）", len(s.historyItems)), unit.Sp(11), colorTextPrimary)),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							s.historySearch.SingleLine = true
							editor := material.Editor(s.theme, &s.historySearch, "搜索 SQL 或连接 ID")
							editor.TextSize = unit.Sp(11)
							return layout.Inset{Left: spaceM, Right: spaceM}.Layout(gtx, editor.Layout)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if s.historyRefresh.Clicked(gtx) {
								s.reloadHistory()
							}
							return s.smallButton(gtx, &s.historyRefresh, "刷新", false)
						}),
						layout.Rigid(layout.Spacer{Width: spaceXS}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if s.historyClear.Clicked(gtx) && len(s.historyItems) > 0 {
								s.mutateHistory("清空查询历史", "")
							}
							return s.smallButton(gtx, &s.historyClear, "清空", len(s.historyItems) == 0)
						}),
					)
				}),
				layout.Rigid(s.divider),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					if len(filtered) == 0 {
						message := "暂无查询历史"
						if strings.TrimSpace(s.historySearch.Text()) != "" {
							message = "没有匹配的查询历史"
						}
						return layout.Center.Layout(gtx, styledText(s.theme, message, unit.Sp(11), colorTextMuted))
					}
					s.historyList.Axis = layout.Vertical
					return material.List(s.theme, &s.historyList).Layout(gtx, len(filtered), func(gtx layout.Context, index int) layout.Dimensions {
						item := filtered[index]
						rowIndex := originalIndex[item.ID]
						click := &s.historyClicks[rowIndex]
						remove := &s.historyDeleteClicks[rowIndex]
						if click.Clicked(gtx) {
							s.queryEditor.SetText(item.SQL)
							s.queryStatus = "已载入查询历史"
						}
						if remove.Clicked(gtx) {
							s.mutateHistory("删除查询历史", item.ID)
						}
						preview := strings.ReplaceAll(strings.TrimSpace(item.SQL), "\n", " ")
						if len(preview) > 150 {
							preview = preview[:147] + "..."
						}
						resultLabel := "失败"
						if item.Success {
							resultLabel = "成功"
						}
						detail := fmt.Sprintf("%s · %s · %.0f ms · %s", item.Timestamp.Local().Format("2006-01-02 15:04:05"), item.ConnectionID, item.DurationMs, resultLabel)
						return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
											layout.Rigid(styledText(s.theme, preview, unit.Sp(11), colorTextPrimary)),
											layout.Rigid(styledText(s.theme, detail, unit.Sp(10), colorTextMuted)),
										)
									})
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return s.smallButton(gtx, remove, "删除", false)
								}),
							)
						})
					})
				}),
			)
		})
	})
}

var _ = widget.List{}
