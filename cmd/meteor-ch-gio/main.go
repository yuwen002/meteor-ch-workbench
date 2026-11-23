package main

import (
	"log"

	"gioui.org/app"
	"gioui.org/unit"

	appsvc "github.com/yuwen002/meteor-ch-workbench/internal/app"
	"github.com/yuwen002/meteor-ch-workbench/internal/gui/gioapp"
)

func main() {
	go func() {
		service := appsvc.NewDefaultService()
		defer service.Close()

		window := new(app.Window)
		window.Option(
			app.Title("Meteor CH Workbench"),
			app.Size(unit.Dp(1440), unit.Dp(900)),
		)
		if err := gioapp.Run(window, service); err != nil {
			log.Printf("Gio 窗口退出：%v", err)
		}
	}()
	app.Main()
}
