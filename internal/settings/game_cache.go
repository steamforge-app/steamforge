package settings

import (
	"path/filepath"
)

type CachedGame struct {
	Name    string `json:"name"`
	LogoURL string `json:"logoUrl"`
}

var gameCacheStore = NewJSONCache[uint32, CachedGame](gameCacheFilePath, 0, "game")

func gameCacheFilePath() string {
	dir := userDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "games.json")
}

func resetGameCache() {
	gameCacheStore.Reset()
}

// LoadGameCache returns a snapshot copy of the game cache.
func LoadGameCache() map[uint32]CachedGame {
	return gameCacheStore.Load()
}

// SaveGameCache stores the entire game cache and writes it synchronously to disk.
func SaveGameCache(games map[uint32]CachedGame) {
	gameCacheStore.mu.Lock()
	gameCacheStore.ensureLoadedLocked()
	gameCacheStore.data = games
	snapshot := gameCacheStore.snapshotLocked()
	gameCacheStore.mu.Unlock()
	gameCacheStore.writeToDisk(snapshot)
}
