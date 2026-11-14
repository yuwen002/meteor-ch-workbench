package main

import (
	"embed"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// assets 将前端构建产物 frontend/dist 整个目录编译进二进制，
// 这样发布时只需要一个可执行文件，无需额外携带前端资源。
//
//go:embed all:frontend/dist
var assets embed.FS

// main 是程序入口，负责创建 Wails 应用、注册后端服务、创建主窗口并进入事件循环。
// 前端通过 Services 注册的 NewApp() 实例调用后端方法（即 app.go 中的导出方法）。
func main() {
	app := application.New(application.Options{
		Name:        "Meteor CH Workbench",
		Description: "A cross-platform ClickHouse GUI tool",
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(assets),
		},
		Services: []application.Service{
			application.NewService(NewApp()),
		},
		OnShutdown: func() {},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Meteor CH Workbench",
		Width:  1280,
		Height: 800,
		Mac: application.MacWindow{
			TitleBar: application.MacTitleBarHiddenInsetUnified,
		},
	})

	err := app.Run()
	if err != nil {
		println("Error:", err.Error())
	}
}
