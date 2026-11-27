package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	fyneui "github.com/yuwen002/meteor-ch-workbench/internal/gui/fyneapp"
)

func main() {
	f := app.NewWithID("meteor-ch-workbench")
	f.Lifecycle().SetOnStopped(func() {
		fyneui.Shutdown()
	})

	w := f.NewWindow("Meteor CH Workbench")
	w.Resize(fyne.NewSize(1280, 800))
	w.SetContent(fyneui.NewMainWindow(w))
	w.ShowAndRun()
}