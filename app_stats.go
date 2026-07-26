package main

import (
	"fmt"

	"steamforge/internal/steam"
)

// GetAllPlaytimes returns hours played for every app with recorded playtime,
// via a single parse of localconfig.vdf. Requires a connected Steam client.
func (a *App) GetAllPlaytimes() (map[uint32]float64, error) {
	a.mu.RLock()
	client := a.steamClient
	a.mu.RUnlock()
	if client == nil {
		return nil, errNotConnected
	}

	return steam.ScanPlaytimeHours(client.SteamID()), nil
}

// GetPlaytime returns this account's total hours played for a game, read
// from the local Steam client's localconfig.vdf. Steam no longer exposes
// this over any public, unauthenticated network endpoint, so this reads the
// same local file Steam's own client uses (steam.ScanLastPlayed already
// reads this file for LastPlayed). Returns an error if the game has no
// recorded playtime.
func (a *App) GetPlaytime(appID uint32) (float64, error) {
	a.mu.RLock()
	client := a.steamClient
	a.mu.RUnlock()
	if client == nil {
		return 0, errNotConnected
	}

	hours, found := steam.ScanPlaytimeHours(client.SteamID())[appID]
	if !found {
		return 0, fmt.Errorf("no playtime found for app %d", appID)
	}
	return hours, nil
}
