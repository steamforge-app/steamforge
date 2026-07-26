package main

import (
	"fmt"
	"log/slog"

	"steamforge/internal/models"
	"steamforge/internal/services"
	"steamforge/internal/settings"
)

// LoadAchievementsFromSchema fetches full achievement data from the Steam community profile.
// No client reconnect needed — works for any public profile.
// Falls back to local schema files if the community endpoint fails.
func (a *App) LoadAchievementsFromSchema(appID uint32) ([]models.Achievement, error) {
	a.mu.RLock()
	client := a.steamClient
	a.mu.RUnlock()

	if client == nil {
		return nil, errNotConnected
	}

	// Try community profile first — has names, descriptions, icons, unlock status
	webAPI := services.NewSteamWebAPI(client.SteamID(), a.ctx)
	achievements, err := webAPI.GetFullAchievements(appID)
	if err == nil && len(achievements) > 0 {
		achieved := 0
		for _, achievement := range achievements {
			if achievement.IsAchieved {
				achieved++
			}
		}
		settings.SaveAchievementCounts(appID, achieved, len(achievements))
		services.MergeSchemaPermissions(appID, achievements)
		return achievements, nil
	}
	if err != nil {
		slog.Debug("community profile fetch failed", "appID", appID, "error", err)
	}

	// Community failed or returned empty — check local schema before falling through to SDK
	if total, hasSchema := services.HasAchievementsFromSchema(appID); hasSchema && total == 0 {
		slog.Debug("schema confirms 0 achievements", "appID", appID)
		settings.SaveAchievementCounts(appID, 0, 0)
		return []models.Achievement{}, nil
	}

	// Neither community nor schema could confirm — error so frontend can try SDK
	return nil, fmt.Errorf("community profile unavailable: %w", err)
}

// LoadAchievements connects to Steam as the game and loads full achievement data.
// This is needed for editing achievements (set/clear).
func (a *App) LoadAchievements(appID uint32) ([]models.Achievement, error) {
	if err := a.reconnectForGame(appID); err != nil {
		return nil, err
	}

	a.mu.RLock()
	achievementService := a.achieveService
	a.mu.RUnlock()

	if achievementService == nil {
		return nil, errNotConnected
	}
	result, err := achievementService.LoadAchievements(appID)
	if err != nil {
		return nil, err
	}

	cacheAchievementCounts(appID, result)
	return result, nil
}

func (a *App) GetAchievements() []models.Achievement {
	a.mu.RLock()
	achievementService := a.achieveService
	a.mu.RUnlock()

	if achievementService == nil {
		return nil
	}
	return achievementService.GetAchievements()
}

func (a *App) SetAchievement(name string) (bool, error) {
	a.mu.RLock()
	achievementService := a.achieveService
	a.mu.RUnlock()

	if achievementService == nil {
		return false, errNotConnected
	}
	return achievementService.SetAchievement(name)
}

func (a *App) ClearAchievement(name string) (bool, error) {
	a.mu.RLock()
	achievementService := a.achieveService
	a.mu.RUnlock()

	if achievementService == nil {
		return false, errNotConnected
	}
	return achievementService.ClearAchievement(name)
}

func (a *App) SetAllAchievements() (int, error) {
	a.mu.RLock()
	achievementService := a.achieveService
	a.mu.RUnlock()

	if achievementService == nil {
		return 0, errNotConnected
	}
	return achievementService.SetAllAchievements()
}

func (a *App) ClearAllAchievements() (int, error) {
	a.mu.RLock()
	achievementService := a.achieveService
	a.mu.RUnlock()

	if achievementService == nil {
		return 0, errNotConnected
	}
	return achievementService.ClearAllAchievements()
}

func (a *App) StoreStats() (bool, error) {
	a.mu.RLock()
	achievementService := a.achieveService
	client := a.steamClient
	a.mu.RUnlock()

	if achievementService == nil {
		return false, errNotConnected
	}
	ok, err := achievementService.StoreStats()
	if err != nil {
		return ok, err
	}

	if ok && client != nil {
		cacheAchievementCounts(client.CurrentAppID, achievementService.GetAchievements())
	}
	return ok, nil
}

func (a *App) GetAchievementCounts() map[uint32]settings.AchievementCount {
	return settings.LoadAchievementCache()
}
