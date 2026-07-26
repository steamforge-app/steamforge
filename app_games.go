package main

import (
	"steamforge/internal/models"
)

func (a *App) FetchGames() ([]models.GameInfo, error) {
	if a.stopScan() {
		a.restoreBaseConnection()
	}

	a.mu.RLock()
	gameService := a.gameService
	a.mu.RUnlock()

	if gameService == nil {
		return nil, errNotConnected
	}
	return gameService.FetchGames()
}

func (a *App) SearchGames(query string) []models.GameInfo {
	a.mu.RLock()
	gameService := a.gameService
	a.mu.RUnlock()

	if gameService == nil {
		return nil
	}
	return gameService.SearchGames(query)
}
