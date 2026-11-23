package main

import (
	"log"

	"gioui.org/app"

	"github.com/yuwen002/meteor-ch-workbench/internal/gui/gioapp"
)

func main() {
	go func() {
		window := new(app.Window)
		window.Option(
			app.Title("Meteor CH Workbench"),
			app.Size(1440, 900),
		)
		if err := gioapp.Run(window); err != nil {
			log.Printf("Gio 窗口退出：%v", err)
		}
	}()
	app.Main()
}
