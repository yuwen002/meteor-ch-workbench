package main

import (
	"log"

	gio "gioui.org/app"
	"gioui.org/unit"

	"github.com/yuwen002/meteor-ch-workbench/internal/bootstrap"
	gioapp "github.com/yuwen002/meteor-ch-workbench/internal/gui/gioapp/app"
)

// 初始窗口按逻辑尺寸给出，保证在 125% / 150% 缩放的常见笔记本屏幕上也能完整显示；
// 窗口本身可自由缩放与最大化，大屏用户可自行放大。
const (
	initialWindowWidth  = unit.Dp(1080)
	initialWindowHeight = unit.Dp(660)
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
			gio.Size(initialWindowWidth, initialWindowHeight),
		)
		if err := gioapp.Run(window, services); err != nil {
			log.Printf("Gio 窗口退出：%v", err)
		}
	}()
	gio.Main()
}
