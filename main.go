package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/lang"

	fyneui "github.com/yuwen002/meteor-ch-workbench/internal/gui/fyneapp"
)

func main() {
	zhHans := []byte(`{
		"(Select one)": "",
		"(Select All)": "全选"
	}`)
	_ = lang.AddTranslationsForLocale(zhHans, fyne.Locale("zh-Hans"))
	_ = lang.AddTranslationsForLocale(zhHans, fyne.Locale("zh_Hans"))
	_ = lang.AddTranslationsForLocale(zhHans, fyne.Locale("zh"))

	f := app.NewWithID("meteor-ch-workbench")
	f.Lifecycle().SetOnStopped(func() {
		fyneui.Shutdown()
	})

	w := f.NewWindow("Meteor CH Workbench")
	w.Resize(fyne.NewSize(1280, 800))
	w.SetContent(fyneui.NewMainWindow(w))
	w.ShowAndRun()
}