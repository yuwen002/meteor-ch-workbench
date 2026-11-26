package main

import (
	"log"
	"os"

	gio "gioui.org/app"
	"github.com/yuwen002/meteor-ch-workbench/internal/bootstrap"
	gioapp "github.com/yuwen002/meteor-ch-workbench/internal/gui/gioapp/app"
)

func main() {
	services, err := bootstrap.NewDefault()
	if err != nil {
		log.Fatalf("初始化应用服务失败：%v", err)
	}

	// Gio 的 app.Main 在 Windows 上进入不会返回的主线程消息等待。
	// 窗口事件循环退出后必须主动结束进程，否则主 goroutine 会永久停在
	// app.Main 的 select{} 中，最终触发 Go runtime 的 deadlock fatal error。
	go func() {
		window := new(gio.Window)
		width, height := initialWindowSize()
		window.Option(
			gio.Title("Meteor CH Workbench"),
			gio.Size(width, height),
		)

		runErr := gioapp.Run(window, services)
		if runErr != nil {
			log.Printf("Gio 窗口退出：%v", runErr)
		}
		if closeErr := services.Close(); closeErr != nil {
			log.Printf("关闭应用服务失败：%v", closeErr)
			if runErr == nil {
				runErr = closeErr
			}
		}
		if runErr != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}()

	gio.Main()
}

