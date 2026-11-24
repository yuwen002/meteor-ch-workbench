package main

import (
	"log"

	"gioui.org/app"
	"gioui.org/unit"

	"github.com/yuwen002/meteor-ch-workbench/internal/bootstrap"
	gioapp "github.com/yuwen002/meteor-ch-workbench/internal/gui/gioapp/app"
)

func main() {
	go func() {
		services, err := bootstrap.NewDefault()
		if err != nil {
			log.Printf("初始化应用服务失败：%v", err)
			return
		}
		defer func() {
			if err := services.Close(); err != nil {
				log.Printf("关闭应用服务失败：%v", err)
			}
		}()

		window := new(app.Window)
		window.Option(
			app.Title("Meteor CH Workbench"),
			app.Size(unit.Dp(1440), unit.Dp(900)),
		)
		if err := gioapp.Run(window, services); err != nil {
			log.Printf("Gio 窗口退出：%v", err)
		}
	}()
	app.Main()
}
