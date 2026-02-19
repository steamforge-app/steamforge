package main

import (
	"context"
	"embed"
	"log/slog"
	"os"
	"time"

	"steamforge/internal/logging"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	logPath, logCleanup, err := logging.Setup()
	if err != nil {
		println("Warning: logging setup failed:", err.Error())
	} else {
		defer logCleanup()
		slog.Info("SteamForge starting", "logPath", logPath)
	}

	app := NewApp()

	err = wails.Run(&options.App{
		Title:     "SteamForge",
		Width:     1280,
		Height:    800,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "steamforge-80a3f9d2-4e7b-4c1a-b5e6-9f2d3a8c1b7e",
			OnSecondInstanceLaunch: func(data options.SecondInstanceData) {
				slog.Info("second instance attempted, focusing existing window")
				wailsRuntime.Show(app.ctx)
				wailsRuntime.WindowUnminimise(app.ctx)
			},
		},
		BackgroundColour: &options.RGBA{R: 27, G: 40, B: 56, A: 255},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		OnBeforeClose: func(ctx context.Context) (prevent bool) {
			// Start a hard-exit timer so the process never hangs
			go func() {
				time.Sleep(8 * time.Second)
				slog.Warn("hard exit: shutdown took too long")
				os.Exit(0)
			}()
			return false
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		slog.Error("wails.Run failed", "error", err)
		println("Error:", err.Error())
	}

	slog.Info("SteamForge shutdown")
	os.Exit(0)
}
