package main

import (
	"fmt"
	"log/slog"
	"time"

	"steamforge/internal/services"
	"steamforge/internal/settings"
	"steamforge/internal/steam"
)

// initClient creates a new Steam client with associated services.
// Returns the client, game service, and achievement service.
func (a *App) initClient(appID uint32) (*steam.Client, *services.GameService, *services.AchievementService, error) {
	client, err := steam.NewClient(appID)
	if err != nil {
		return nil, nil, nil, err
	}
	client.StartCallbackLoop()

	gameService := services.NewGameService(client)
	gameService.SetContext(a.ctx)

	achieveService := services.NewAchievementService(client)
	achieveService.SetContext(a.ctx)

	return client, gameService, achieveService, nil
}

func (a *App) ConnectSteam() (uint64, error) {
	a.mu.RLock()
	existing := a.steamClient
	a.mu.RUnlock()

	if existing != nil {
		return existing.SteamID(), nil
	}

	slog.Info("connecting to Steam")
	client, gameService, achieveService, err := a.initClient(0)
	if err != nil {
		slog.Error("Steam connection failed", "error", err)
		return 0, fmt.Errorf("connect to Steam: %w", err)
	}

	a.mu.Lock()
	a.steamClient = client
	a.gameService = gameService
	a.achieveService = achieveService
	a.mu.Unlock()

	slog.Info("Steam connected", "steamID", client.SteamID())
	settings.SetCurrentUser(client.SteamID())

	a.startInstallWatcher()
	a.startAccountWatcher(client.SteamID())

	return client.SteamID(), nil
}

// GetPersonaName returns the current Steam user's display name.
func (a *App) GetPersonaName() string {
	a.mu.RLock()
	client := a.steamClient
	a.mu.RUnlock()

	if client == nil {
		return ""
	}
	return client.PersonaName()
}

func (a *App) IsConnected() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.steamClient != nil
}

func (a *App) reconnectForGame(appID uint32) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.steamClient != nil && a.steamClient.CurrentAppID == appID {
		return nil
	}

	start := time.Now()
	slog.Info("reconnecting for game", "appID", appID)

	if a.steamClient != nil {
		a.steamClient.Close()
		a.steamClient = nil
		a.achieveService = nil
	}

	var client *steam.Client
	var gameService *services.GameService
	var achieveService *services.AchievementService
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		if attempt > 1 {
			slog.Info("retrying Steam reconnect", "appID", appID, "attempt", attempt)
			time.Sleep(time.Duration(attempt) * 300 * time.Millisecond)
		}
		client, gameService, achieveService, err = a.initClient(appID)
		if err == nil {
			break
		}
		slog.Warn("Steam connect attempt failed", "appID", appID, "attempt", attempt, "error", err)
	}
	if err != nil {
		slog.Error("reconnect failed", "appID", appID, "error", err)
		return fmt.Errorf("reconnect for app %d: %w", appID, err)
	}

	a.steamClient = client
	a.gameService = gameService
	a.achieveService = achieveService

	slog.Info("reconnected", "appID", appID, "elapsed", time.Since(start))
	return nil
}

func (a *App) DisconnectGame() error {
	a.mu.Lock()

	if a.steamClient == nil || a.steamClient.CurrentAppID == 0 {
		a.mu.Unlock()
		return nil
	}

	slog.Info("disconnecting from game", "appID", a.steamClient.CurrentAppID)

	a.steamClient.Close()
	a.steamClient = nil
	a.achieveService = nil
	a.gameService = nil
	a.mu.Unlock()

	// Restore the base connection (appID=0) so Steam no longer thinks
	// we're running the game.
	a.restoreBaseConnection()

	slog.Info("disconnected from game")
	return nil
}

func (a *App) restoreBaseConnection() {
	a.mu.Lock()
	oldClient := a.steamClient
	a.steamClient = nil
	a.achieveService = nil
	a.mu.Unlock()

	if oldClient != nil {
		oldClient.Close()
	}

	client, gameService, _, err := a.initClient(0)
	if err != nil {
		slog.Error("failed to restore base connection", "error", err)
		return
	}

	a.mu.Lock()
	a.steamClient = client
	a.gameService = gameService
	a.mu.Unlock()
}
