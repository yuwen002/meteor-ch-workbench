package main

import (
	"log"

	gio "gioui.org/app"
	"gioui.org/unit"

	"github.com/yuwen002/meteor-ch-workbench/internal/bootstrap"
	gioapp "github.com/yuwen002/meteor-ch-workbench/internal/gui/gioapp/app"
)

func main() {
	services, err := bootstrap.NewDefault()
	if err != nil {
		log.Fatalf("初始化应用服务失败：%v", err)
	}
	defer func() {
		if err := services.Close(); err != nil {
			log.Printf("关闭应用服务失败：%v", err)
		}
	}()

	go func() {
		window := new(gio.Window)
		window.Option(
			gio.Title("Meteor CH Workbench"),
			gio.Size(unit.Dp(1440), unit.Dp(900)),
		)
		if err := gioapp.Run(window, services); err != nil {
			log.Printf("Gio 窗口退出：%v", err)
		}
	}()
	gio.Main()
}
