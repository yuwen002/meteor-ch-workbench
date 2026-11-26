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

	savedsqldomain "github.com/yuwen002/meteor-ch-workbench/internal/domain/savedsql"
)

// filterSavedSQL 按收藏名称或 SQL 内容进行不区分大小写的子串筛选。
func filterSavedSQL(items []savedsqldomain.Entry, keyword string) []savedsqldomain.Entry {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return append([]savedsqldomain.Entry(nil), items...)
	}
	filtered := make([]savedsqldomain.Entry, 0, len(items))
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Name), keyword) ||
			strings.Contains(strings.ToLower(item.SQL), keyword) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}


type savedSQLItemControls struct {
	open   widget.Clickable
	remove widget.Clickable
	rename widget.Clickable
}

// syncSavedSQLControls 按收藏 ID 维护控件，筛选和排序不会造成按钮错位。
func (s *workspace) syncSavedSQLControls(items []savedsqldomain.Entry) {
	if s.savedSQLControls == nil {
		s.savedSQLControls = make(map[string]*savedSQLItemControls, len(items))
	}
	active := make(map[string]struct{}, len(items))
	for _, item := range items {
		active[item.ID] = struct{}{}
		if s.savedSQLControls[item.ID] == nil {
			s.savedSQLControls[item.ID] = &savedSQLItemControls{}
		}
	}
	for id := range s.savedSQLControls {
		if _, ok := active[id]; !ok {
			delete(s.savedSQLControls, id)
		}
	}
}

func (s *workspace) reloadSavedSQL() {
	s.savedSQLListSeq++
	seq := s.savedSQLListSeq
	s.goWorker(func() {
		ctx, cancel := context.WithTimeout(s.lifecycleParent(), 5*time.Second)
		defer cancel()
		items, err := s.services.SavedSQL.List(ctx)
		select {
		case s.savedSQLListResults <- savedSQLListResult{seq: seq, items: items, err: err}:
		case <-ctx.Done():
		}
		s.window.Invalidate()
	})
}

func (s *workspace) layoutSavedSQLPanel(gtx layout.Context) layout.Dimensions {
	filtered := filterSavedSQL(s.savedSQLItems, s.savedSQLSearch.Text())
	needle := strings.TrimSpace(s.savedSQLSearch.Text())
	height := gtx.Dp(unit.Dp(174))
	gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = height, height
	return s.surface(gtx, colorSurfaceSubtle, radiusS, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: spaceXS, Bottom: spaceXS, Left: spaceM, Right: spaceM}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(styledText(s.theme, fmt.Sprintf("收藏 SQL（%d）", len(s.savedSQLItems)), unit.Sp(11), colorTextPrimary)),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: spaceM, Right: spaceM}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								s.savedSQLSearch.SingleLine = true
								editor := material.Editor(s.theme, &s.savedSQLSearch, "搜索名称或 SQL")
							editor.TextSize = unit.Sp(11)
								return editor.Layout(gtx)
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if s.savedSQLRefresh.Clicked(gtx) { s.reloadSavedSQL() }
							return s.smallButton(gtx, &s.savedSQLRefresh, "刷新", false)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if s.savedSQLRenameID == "" { return layout.Dimensions{} }
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							s.savedSQLRenameEditor.SingleLine = true
							editor := material.Editor(s.theme, &s.savedSQLRenameEditor, "输入新的收藏名称")
							editor.TextSize = unit.Sp(11)
							return editor.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if s.savedSQLRenameSave.Clicked(gtx) { s.renameSavedSQL(s.savedSQLRenameID) }
							return s.smallButton(gtx, &s.savedSQLRenameSave, "保存名称", strings.TrimSpace(s.savedSQLRenameEditor.Text()) == "")
						}),
					)
				}),
				layout.Rigid(s.divider),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					if len(filtered) == 0 {
						message := "暂无收藏 SQL"
						if needle != "" { message = "没有匹配的收藏 SQL" }
						return layout.Center.Layout(gtx, styledText(s.theme, message, unit.Sp(11), colorTextMuted))
					}
					s.savedSQLList.Axis = layout.Vertical
					return material.List(s.theme, &s.savedSQLList).Layout(gtx, len(filtered), func(gtx layout.Context, index int) layout.Dimensions {
						item := filtered[index]
						controls := s.savedSQLControls[item.ID]
						if controls == nil {
							controls = &savedSQLItemControls{}
							s.savedSQLControls[item.ID] = controls
						}
						click := &controls.open
						remove := &controls.remove
						if click.Clicked(gtx) {
							s.queryEditor.SetText(item.SQL)
							s.queryStatus = "已载入收藏 SQL：" + item.Name
						}
						if remove.Clicked(gtx) { s.deleteSavedSQL(item.ID) }
						rename := &controls.rename
						if rename.Clicked(gtx) { s.savedSQLRenameID = item.ID; s.savedSQLRenameEditor.SetText(item.Name) }
						return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
											layout.Rigid(styledText(s.theme, item.Name, unit.Sp(11), colorTextPrimary)),
											layout.Rigid(styledText(s.theme, strings.ReplaceAll(strings.TrimSpace(item.SQL), "\n", " "), unit.Sp(10), colorTextMuted)),
										)
									})
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return s.smallButton(gtx, rename, "改名", false)
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

func (s *workspace) renameSavedSQL(id string) {
	name := strings.TrimSpace(s.savedSQLRenameEditor.Text())
	if name == "" { return }
	var item savedsqldomain.Entry
	for _, saved := range s.savedSQLItems {
		if saved.ID == id { item = saved; break }
	}
	if item.ID == "" { return }
	s.goWorker(func() {
		ctx, cancel := context.WithTimeout(s.lifecycleParent(), 5*time.Second)
		defer cancel()
		updated, err := s.services.SavedSQL.Update(ctx, id, name, item.SQL)
		select {
		case s.savedSQLResults <- savedSQLResult{name: updated.Name, action: "重命名收藏", err: err}:
		case <-ctx.Done():
		}
		s.window.Invalidate()
	})
}

func (s *workspace) deleteSavedSQL(id string) {
	s.goWorker(func() {
		ctx, cancel := context.WithTimeout(s.lifecycleParent(), 5*time.Second)
		defer cancel()
		err := s.services.SavedSQL.Delete(ctx, id)
		select {
		case s.savedSQLResults <- savedSQLResult{action: "删除收藏", err: err}:
		case <-ctx.Done():
		}
		s.window.Invalidate()
	})
}

func (s *workspace) saveCurrentSQL() {
	sqlText := strings.TrimSpace(s.queryEditor.Text())
	if sqlText == "" || !s.connected || s.selectedID == "" {
		return
	}
	connectionID := s.selectedID
	s.goWorker(func() {
		ctx, cancel := context.WithTimeout(s.lifecycleParent(), 5*time.Second)
		defer cancel()
		item, err := s.services.SavedSQL.Add(ctx, savedsqldomain.Entry{
			SQL:          sqlText,
			ConnectionID: connectionID,
		})
		select {
		case s.savedSQLResults <- savedSQLResult{name: item.Name, action: "收藏 SQL", err: err}:
		case <-ctx.Done():
		}
		s.window.Invalidate()
	})
}
