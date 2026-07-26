package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"steamforge/internal/services"
	"steamforge/internal/settings"
	"steamforge/internal/steam"
)

// Version is set via ldflags at build time (e.g. -X main.Version=v1.0.0).
var Version = "dev"

var errNotConnected = errors.New("not connected to Steam")

type App struct {
	ctx context.Context

	mu                   sync.RWMutex
	steamClient          *steam.Client
	gameService          *services.GameService
	achieveService       *services.AchievementService
	imageService         *services.ImageService
	hltbService          *services.HLTBService
	installedUpdateTimes map[uint32]uint32 // appID -> appmanifest LastUpdated, kept fresh by the install watcher

	scanCancel context.CancelFunc
	scanDone   chan struct{}

	installWatcher *steam.InstallWatcher
	accountWatcher *steam.AccountWatcher
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	settings.Load()

	cacheDir := filepath.Join(os.TempDir(), "steamforge", "cache")
	imageService, err := services.NewImageService(cacheDir)
	if err != nil {
		slog.Warn("image service init failed", "error", err)
	} else {
		imageService.SetContext(ctx)
		a.mu.Lock()
		a.imageService = imageService
		a.mu.Unlock()
	}

	slog.Info("startup complete")
}

func (a *App) shutdown(ctx context.Context) {
	a.stopScan()
	a.stopAccountWatcher()
	a.stopInstallWatcher()

	settings.FlushHLTBCache()
	settings.FlushPercentCache()

	a.mu.RLock()
	client := a.steamClient
	a.mu.RUnlock()

	if client != nil {
		done := make(chan struct{})
		go func() {
			client.Close()
			close(done)
		}()
		select {
		case <-done:
			slog.Info("Steam client closed")
		case <-ctx.Done():
			slog.Warn("shutdown timed out waiting for Steam client close")
		case <-time.After(5 * time.Second):
			slog.Warn("Steam client close timed out after 5s")
		}
	}
}
