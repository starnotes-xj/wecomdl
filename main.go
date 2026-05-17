package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"wecom-replay-downloader/internal/gui"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	backend := gui.NewBackend()
	err := wails.Run(&options.App{
		Title:     "wecomdl",
		Width:     1180,
		Height:    760,
		MinWidth:  720,
		MinHeight: 560,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: backend.Startup,
		Bind: []any{
			backend,
		},
	})
	if err != nil {
		println("错误:", err.Error())
	}
}
