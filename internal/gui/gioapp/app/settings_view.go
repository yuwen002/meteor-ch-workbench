package app

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"strconv"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"

	settingsdomain "github.com/yuwen002/meteor-ch-workbench/internal/domain/settings"
)

func (s *workspace) openSettings() {
	s.settingsVisible = true
	s.settingsStatus = ""
	s.settingsRowLimitEditor.SetText(strconv.Itoa(s.queryRowLimit))
	s.settingsRatioEditor.SetText(strconv.FormatFloat(s.settingsPreferences.EditorRatio, 'f', 2, 64))
}

func (s *workspace) saveSettings() {
	value, err := parseSettingsRowLimit(s.settingsRowLimitEditor.Text())
	if err != nil {
		s.settingsStatus = err.Error()
		return
	}
	ratio, err := parseSettingsEditorRatio(s.settingsRatioEditor.Text())
	if err != nil {
		s.settingsStatus = err.Error()
		return
	}
	preferences := s.settingsPreferences
	preferences.QueryRowLimit = value
	preferences.SidebarCollapsed = s.connectionSidebarCollapsed
	preferences.EditorRatio = ratio
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	saved, err := s.services.Settings.Save(ctx, preferences)
	if err != nil {
		s.settingsStatus = "保存设置失败：" + err.Error()
		return
	}
	s.settingsPreferences = saved
	s.queryRowLimit = saved.QueryRowLimit
	s.settingsStatus = "设置已保存；新执行的查询将使用新的行数上限"
	s.status = s.settingsStatus
}

func (s *workspace) layoutSettingsPanel(gtx layout.Context) layout.Dimensions {
	width := gtx.Dp(unit.Dp(460))
	maxWidth := gtx.Constraints.Max.X - gtx.Dp(unit.Dp(32))
	if width > maxWidth { width = maxWidth }
	if width < gtx.Dp(unit.Dp(260)) { width = maxWidth }
	height := gtx.Dp(unit.Dp(400))
	if height > gtx.Constraints.Max.Y-gtx.Dp(unit.Dp(32)) { height = gtx.Constraints.Max.Y-gtx.Dp(unit.Dp(32)) }
	if width < 1 || height < 1 { return layout.Dimensions{} }
	gtx.Constraints.Min = image.Pt(width, height)
	gtx.Constraints.Max = image.Pt(width, height)
	return s.card(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: spaceL, Bottom: spaceL, Left: spaceL, Right: spaceL}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(styledText(s.theme, "工作台设置", unit.Sp(17), colorTextPrimary)),
				layout.Rigid(layout.Spacer{Height: spaceS}.Layout),
				layout.Rigid(styledText(s.theme, "查询结果行数上限", unit.Sp(12), colorTextSecondary)),
				layout.Rigid(layout.Spacer{Height: spaceXS}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					s.settingsRowLimitEditor.SingleLine = true
					editor := material.Editor(s.theme, &s.settingsRowLimitEditor, "例如 10000")
					editor.TextSize = unit.Sp(13)
					return editor.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: spaceXS}.Layout),
				layout.Rigid(styledText(s.theme, "范围：0 表示不限制；100～1,000,000 表示最大返回行数。仅影响新执行的查询。", unit.Sp(10), colorTextMuted)),
				layout.Rigid(layout.Spacer{Height: spaceS}.Layout),
				layout.Rigid(styledText(s.theme, "界面主题", unit.Sp(12), colorTextSecondary)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := "当前：浅色主题"
					if s.settingsPreferences.Theme == settingsdomain.ThemeDark { label = "当前：深色主题" }
					if s.settingsThemeToggle.Clicked(gtx) { s.toggleThemePreference() }
					return s.smallButton(gtx, &s.settingsThemeToggle, label+" · 切换", false)
				}),
				layout.Rigid(layout.Spacer{Height: spaceS}.Layout),
				layout.Rigid(styledText(s.theme, "编辑器高度占比（0.35～0.80）", unit.Sp(12), colorTextSecondary)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					s.settingsRatioEditor.SingleLine = true
					editor := material.Editor(s.theme, &s.settingsRatioEditor, "例如 0.45")
					editor.TextSize = unit.Sp(13)
					return editor.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: spaceXS}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := "侧栏：展开"
					if s.connectionSidebarCollapsed { label = "侧栏：收起" }
					if s.settingsSidebarToggle.Clicked(gtx) { s.toggleSidebarPreference() }
					return s.smallButton(gtx, &s.settingsSidebarToggle, label, false)
				}),
				layout.Rigid(layout.Spacer{Height: spaceS}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if s.settingsStatus == "" { return layout.Dimensions{} }
					return styledText(s.theme, s.settingsStatus, unit.Sp(10), colorTextSecondary)(gtx)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{} }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							if s.settingsCancel.Clicked(gtx) { s.settingsVisible = false }
							return s.smallButton(gtx, &s.settingsCancel, "关闭", false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if s.settingsSave.Clicked(gtx) { s.saveSettings() }
							return s.primaryButton(gtx, &s.settingsSave, "保存设置", false)
						}),
					)
				}),
			)
		})
	})
}

func (s *workspace) toggleSidebarPreference() {
	s.connectionSidebarCollapsed = !s.connectionSidebarCollapsed
	preferences := s.settingsPreferences
	preferences.SidebarCollapsed = s.connectionSidebarCollapsed
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	saved, err := s.services.Settings.Save(ctx, preferences)
	if err != nil {
		s.settingsStatus = "保存侧栏状态失败：" + err.Error()
		return
	}
	s.settingsPreferences = saved
}

func parseSettingsRowLimit(text string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil { return 0, fmt.Errorf("请输入有效整数；输入 0 表示不限制") }
	if value != settingsdomain.RowLimitUnlimited && (value < settingsdomain.RowLimitMin || value > settingsdomain.RowLimitMax) {
		return 0, fmt.Errorf("行数上限须为 0 或 %d～%d", settingsdomain.RowLimitMin, settingsdomain.RowLimitMax)
	}
	return value, nil
}

func parseSettingsEditorRatio(text string) (float64, error) {
	value, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || value < settingsdomain.MinEditorRatio || value > settingsdomain.MaxEditorRatio {
		return 0, fmt.Errorf("编辑器占比须为 %.2f～%.2f", settingsdomain.MinEditorRatio, settingsdomain.MaxEditorRatio)
	}
	return value, nil
}

func (s *workspace) applyTheme(mode string) {
	if mode != settingsdomain.ThemeDark { mode = settingsdomain.ThemeLight }
	setPalette(mode)
	if mode == settingsdomain.ThemeDark {
		s.theme.Palette.Bg = colorSurface
		s.theme.Palette.Fg = colorTextPrimary
		s.theme.Palette.ContrastBg = colorAccent
		s.theme.Palette.ContrastFg = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
		return
	}
	s.theme.Palette.Bg = colorSurface
	s.theme.Palette.Fg = colorTextPrimary
	s.theme.Palette.ContrastBg = colorAccent
	s.theme.Palette.ContrastFg = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
}

func (s *workspace) toggleThemePreference() {
	preferences := s.settingsPreferences
	if preferences.Theme == settingsdomain.ThemeDark {
		preferences.Theme = settingsdomain.ThemeLight
	} else {
		preferences.Theme = settingsdomain.ThemeDark
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	saved, err := s.services.Settings.Save(ctx, preferences)
	if err != nil {
		s.settingsStatus = "保存主题失败：" + err.Error()
		return
	}
	s.settingsPreferences = saved
	s.applyTheme(saved.Theme)
	s.settingsStatus = "主题已切换并保存"
	s.status = s.settingsStatus
}
