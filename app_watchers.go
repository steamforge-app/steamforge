package main

import (
	"log/slog"
	"strconv"
	"time"

	"steamforge/internal/services"
	"steamforge/internal/settings"
	"steamforge/internal/steam"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) startInstallWatcher() {
	// Seed the game service (and update-time cache) with the current
	// installed state before the watcher starts, so the first FetchGames()
	// doesn't need to rescan.
	if initial, err := steam.ScanInstalledGames(); err == nil {
		initialMap := make(map[uint32]bool, len(initial))
		updateTimes := make(map[uint32]uint32, len(initial))
		for _, g := range initial {
			initialMap[g.AppID] = true
			updateTimes[g.AppID] = g.LastUpdated
		}
		a.mu.Lock()
		gs := a.gameService
		a.installedUpdateTimes = updateTimes
		a.mu.Unlock()
		if gs != nil {
			gs.SetInstalledApps(initialMap)
		}
	}

	watcher, err := steam.NewInstallWatcher(func(installed map[uint32]bool, updateTimes map[uint32]uint32) {
		// Update the game service cache so subsequent FetchGames() calls
		// don't need to rescan the filesystem.
		a.mu.Lock()
		gs := a.gameService
		a.installedUpdateTimes = updateTimes
		a.mu.Unlock()
		if gs != nil {
			gs.SetInstalledApps(installed)
		}

		// Convert uint32 keys to strings for JSON serialization.
		payload := make(map[string]bool, len(installed))
		for appID, v := range installed {
			payload[strconv.FormatUint(uint64(appID), 10)] = v
		}
		wailsRuntime.EventsEmit(a.ctx, "games-install-changed", payload)
	})
	if err != nil {
		slog.Warn("failed to start install watcher", "error", err)
		return
	}

	a.mu.Lock()
	a.installWatcher = watcher
	a.mu.Unlock()
}

func (a *App) stopInstallWatcher() {
	a.mu.Lock()
	watcher := a.installWatcher
	a.installWatcher = nil
	a.mu.Unlock()

	if watcher != nil {
		watcher.Close()
	}
}

func (a *App) startAccountWatcher(steamID uint64) {
	a.stopAccountWatcher()

	watcher, err := steam.NewAccountWatcher(steamID, a.handleAccountChange)
	if err != nil {
		slog.Warn("failed to start account watcher", "error", err)
		return
	}

	a.mu.Lock()
	a.accountWatcher = watcher
	a.mu.Unlock()
}

func (a *App) stopAccountWatcher() {
	a.mu.Lock()
	watcher := a.accountWatcher
	a.accountWatcher = nil
	a.mu.Unlock()

	if watcher != nil {
		watcher.Close()
	}
}

func (a *App) handleAccountChange(newSteamID uint64) {
	slog.Info("account switch detected", "newSteamID", newSteamID)

	// Tear down current state
	a.stopScan()
	a.stopInstallWatcher()

	a.mu.Lock()
	oldClient := a.steamClient
	a.steamClient = nil
	a.gameService = nil
	a.achieveService = nil
	a.mu.Unlock()

	if oldClient != nil {
		oldClient.Close()
	}

	// Steam needs time to finish the account switch before we can reconnect.
	// Retry for up to 30 seconds.
	var client *steam.Client
	var gameService *services.GameService
	var achieveService *services.AchievementService
	for attempt := range 10 {
		select {
		case <-time.After(3 * time.Second):
		case <-a.ctx.Done():
			slog.Warn("account switch reconnect cancelled during shutdown")
			wailsRuntime.EventsEmit(a.ctx, "steam-disconnected", nil)
			return
		}
		var err error
		client, gameService, achieveService, err = a.initClient(0)
		if err == nil {
			break
		}
		slog.Debug("account switch reconnect attempt", "attempt", attempt+1, "error", err)
	}

	if client == nil {
		slog.Error("failed to reconnect after account switch")
		wailsRuntime.EventsEmit(a.ctx, "steam-disconnected", nil)
		return
	}

	a.mu.Lock()
	a.steamClient = client
	a.gameService = gameService
	a.achieveService = achieveService
	a.mu.Unlock()

	settings.SetCurrentUser(client.SteamID())
	a.startInstallWatcher()

	personaName := client.PersonaName()

	slog.Info("reconnected after account switch", "steamID", client.SteamID(), "persona", personaName)
	wailsRuntime.EventsEmit(a.ctx, "account-changed", map[string]any{
		"steamId":     strconv.FormatUint(client.SteamID(), 10),
		"personaName": personaName,
	})
}
