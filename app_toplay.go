package main

import (
	"steamforge/internal/settings"
)

// GetToPlayList returns the appIDs currently on the user's "games to play" list.
// Pure local state — no Steam client connection required.
func (a *App) GetToPlayList() []uint32 {
	list := settings.LoadToPlayList()
	appIDs := make([]uint32, 0, len(list))
	for appID := range list {
		appIDs = append(appIDs, appID)
	}
	return appIDs
}

// SetToPlay adds or removes a game from the "games to play" list.
func (a *App) SetToPlay(appID uint32, want bool) error {
	settings.SetToPlay(appID, want)
	return nil
}
