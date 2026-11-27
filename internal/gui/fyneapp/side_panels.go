package fyneapp

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type SidePanels struct {
	window    fyne.Window
	editor    *widget.Entry
	appTabs   *container.AppTabs

	mainWindow *MainWindow
}

func NewSidePanels(w fyne.Window, editor *widget.Entry, mw *MainWindow) *SidePanels {
	sp := &SidePanels{window: w, editor: editor, mainWindow: mw}
	sp.appTabs = container.NewAppTabs(
		container.NewTabItem("连接", sp.buildConnectionPanel()),
		container.NewTabItem("历史", sp.buildHistoryPanel()),
		container.NewTabItem("收藏", sp.buildSavedPanel()),
		container.NewTabItem("配置", sp.buildConfigPanel()),
	)
	return sp
}

func (sp *SidePanels) Content() *container.AppTabs {
	return sp.appTabs
}

func (sp *SidePanels) buildConnectionPanel() fyne.CanvasObject {
	header := container.NewVBox(
		widget.NewLabelWithStyle("连接", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(
			widget.NewButton("+ 连接", sp.mainWindow.onNewConnection),
			widget.NewButton("+ 文件夹", sp.mainWindow.connDialog.showNewFolder),
		),
	)
	return container.NewBorder(
		header, nil, nil, nil,
		container.NewScroll(sp.mainWindow.sidebarWrapper),
	)
}

func (sp *SidePanels) buildHistoryPanel() fyne.CanvasObject {
	list := widget.NewList(
		func() int {
			entries, _ := Backend().GetQueryHistory()
			return len(entries)
		},
		func() fyne.CanvasObject {
			return container.NewVBox(
				widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				widget.NewLabel(""),
			)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			entries, _ := Backend().GetQueryHistory()
			if i < 0 || i >= len(entries) {
				return
			}
			h := entries[i]
			box := o.(*fyne.Container)
			title := box.Objects[0].(*widget.Label)
			sub := box.Objects[1].(*widget.Label)
			title.SetText(truncateSQL(h.SQL, 50))
			status := "成功"
			if !h.Success {
				status = "失败"
			}
			sub.SetText(fmt.Sprintf("%s · %.0fms · %s", h.Timestamp.Format(time.Stamp), h.DurationMs, status))
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		entries, _ := Backend().GetQueryHistory()
		if id < 0 || id >= len(entries) {
			return
		}
		sp.editor.SetText(entries[id].SQL)
		if entries[id].ConnectionID != "" {
			sp.mainWindow.activeConnID = entries[id].ConnectionID
			sp.mainWindow.sidebar.Refresh()
		}
	}

	clearBtn := widget.NewButton("清空", func() {
		_ = Backend().ClearQueryHistory()
		list.Refresh()
	})

	return container.NewBorder(
		container.NewHBox(widget.NewLabelWithStyle("查询历史", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), clearBtn),
		nil, nil, nil,
		container.NewScroll(list),
	)
}

func (sp *SidePanels) buildSavedPanel() fyne.CanvasObject {
	list := widget.NewList(
		func() int {
			saved, _ := Backend().GetSavedSQL()
			return len(saved)
		},
		func() fyne.CanvasObject {
			return container.NewVBox(
				widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				widget.NewLabel(""),
			)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			saved, _ := Backend().GetSavedSQL()
			if i < 0 || i >= len(saved) {
				return
			}
			s := saved[i]
			box := o.(*fyne.Container)
			title := box.Objects[0].(*widget.Label)
			sub := box.Objects[1].(*widget.Label)
			title.SetText(s.Name)
			sub.SetText(truncateSQL(s.SQL, 50))
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		saved, _ := Backend().GetSavedSQL()
		if id < 0 || id >= len(saved) {
			return
		}
		sp.editor.SetText(saved[id].SQL)
	}

	saveBtn := widget.NewButton("收藏 SQL", func() {
		nameEntry := widget.NewEntry()
		content := container.NewVBox(
			widget.NewLabel("为当前 SQL 起个名字"),
			nameEntry,
		)
		popup := widget.NewModalPopUp(content, sp.window.Canvas())
		nameEntry.OnSubmitted = func(name string) {
			if name == "" {
				popup.Hide()
				return
			}
			_, _ = Backend().SaveSQL(name, sp.editor.Text, sp.mainWindow.activeConnID)
			popup.Hide()
			list.Refresh()
		}
		popup.Show()
	})

	return container.NewBorder(
		container.NewHBox(widget.NewLabelWithStyle("收藏 SQL", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), saveBtn),
		nil, nil, nil,
		container.NewScroll(list),
	)
}

func (sp *SidePanels) buildConfigPanel() fyne.CanvasObject {
	exportFormats := Backend().GetExportFormats()
	var formatNames []string
	for _, f := range exportFormats {
		formatNames = append(formatNames, f.Name)
	}
	formatSelect := widget.NewSelect(formatNames, func(_ string) {})
	if len(formatNames) > 0 {
		formatSelect.Selected = formatNames[0]
	}

	passwordCheck := widget.NewCheck("包含密码", func(_ bool) {})

	exportBtn := widget.NewButton("导出配置", func() {
		idx := 0
		for i, n := range formatNames {
			if n == formatSelect.Selected {
				idx = i
				break
			}
		}
		if idx >= len(exportFormats) {
			return
		}
		format := exportFormats[idx]
		content, _, err := Backend().ExportConfigByFormat(format.ID, passwordCheck.Checked)
		if err != nil {
			dialog.ShowError(err, sp.window)
			return
		}
		textEntry := widget.NewMultiLineEntry()
		textEntry.SetText(content)
		textEntry.Wrapping = fyne.TextWrapWord
		copyBtn := widget.NewButton("复制", func() {
			sp.window.Clipboard().SetContent(content)
		})
		dlg := dialog.NewCustom("导出配置 ("+format.Name+")", "关闭",
			container.NewBorder(copyBtn, nil, nil, nil, container.NewScroll(textEntry)),
			sp.window)
		dlg.Resize(fyne.NewSize(500, 400))
		dlg.Show()
	})

	importEntry := widget.NewMultiLineEntry()
	importEntry.SetPlaceHolder("粘贴配置文件内容...")

	importBtn := widget.NewButton("导入配置", func() {
		if importEntry.Text == "" {
			return
		}
		err := Backend().ImportConfig(importEntry.Text)
		if err != nil {
			dialog.ShowError(err, sp.window)
			return
		}
		dialog.ShowInformation("成功", "配置已导入，请刷新连接列表", sp.window)
		sp.mainWindow.refreshSidebar()
	})

	return container.NewBorder(
		nil, nil, nil, nil,
		container.NewScroll(container.NewVBox(
			widget.NewLabelWithStyle("导出", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			container.NewHBox(widget.NewLabel("格式:"), formatSelect),
			passwordCheck,
			exportBtn,
			widget.NewSeparator(),
			widget.NewLabelWithStyle("导入", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabel("格式自动识别"),
			importEntry,
			importBtn,
		)),
	)
}

func truncateSQL(s string, max int) string {
	if len(s) == 0 {
		return "(空)"
	}
	for _, line := range splitLines(s) {
		line = trimSpace(line)
		if line != "" {
			if len(line) > max {
				return line[:max] + "..."
			}
			return line
		}
	}
	return "(空)"
}