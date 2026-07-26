package main

import (
	"fmt"

	"steamforge/internal/services"
	"steamforge/internal/settings"
)

// CheckProfileVisibility checks if the user's Steam community profile is public.
// No API key needed — uses the Steam community XML endpoint.
func (a *App) CheckProfileVisibility() (string, error) {
	a.mu.RLock()
	client := a.steamClient
	a.mu.RUnlock()
	if client == nil {
		return "", errNotConnected
	}

	webAPI := services.NewSteamWebAPI(client.SteamID(), a.ctx)
	return webAPI.CheckProfileVisibility()
}

// FetchGlobalPercents returns global unlock percentages for a game.
// Used by the frontend to poll for rarity data independently of achievement loading.
func (a *App) FetchGlobalPercents(appID uint32) (map[string]float32, error) {
	a.mu.RLock()
	client := a.steamClient
	a.mu.RUnlock()
	if client == nil {
		return nil, errNotConnected
	}

	webAPI := services.NewSteamWebAPI(client.SteamID(), a.ctx)
	percents := webAPI.GetGlobalPercents(appID)
	if percents == nil {
		return nil, fmt.Errorf("failed to fetch percentages for app %d", appID)
	}
	return percents, nil
}

// CheckGameEarlyAccess checks if a game is in Early Access via the Steam Store API
// and caches the result in the achievement counts.
func (a *App) CheckGameEarlyAccess(appID uint32) (bool, error) {
	a.mu.RLock()
	client := a.steamClient
	a.mu.RUnlock()
	if client == nil {
		return false, errNotConnected
	}

	webAPI := services.NewSteamWebAPI(client.SteamID(), a.ctx)
	info := webAPI.GetReleaseInfo(appID)

	// Persist the release info in the achievement cache
	cached := settings.LoadAchievementCache()
	if entry, ok := cached[appID]; ok {
		settings.SaveAchievementCountsRelease(appID, entry.Achieved, entry.Total, info.ReleaseDate)
	} else {
		settings.SaveAchievementCountsRelease(appID, 0, 0, info.ReleaseDate)
	}

	return info.EarlyAccess, nil
}
