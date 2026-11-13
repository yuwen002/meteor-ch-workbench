package main

import (
	"embed"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

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
