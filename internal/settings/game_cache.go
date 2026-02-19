package settings

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

type CachedGame struct {
	Name    string `json:"name"`
	LogoURL string `json:"logoUrl"`
}

var (
	gameCacheMu     sync.RWMutex
	gameCache       map[uint32]CachedGame
	gameCacheLoaded bool
)

func gameCacheFilePath() string {
	dir := userDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "games.json")
}

func resetGameCache() {
	gameCacheMu.Lock()
	defer gameCacheMu.Unlock()
	gameCache = nil
	gameCacheLoaded = false
}

func LoadGameCache() map[uint32]CachedGame {
	gameCacheMu.Lock()
	defer gameCacheMu.Unlock()

	if !gameCacheLoaded {
		gameCache = make(map[uint32]CachedGame)
		gameCacheLoaded = true

		p := gameCacheFilePath()
		if p != "" {
			data, err := os.ReadFile(p)
			if err == nil {
				if err := json.Unmarshal(data, &gameCache); err != nil {
					slog.Warn("failed to parse game cache", "error", err)
					gameCache = make(map[uint32]CachedGame)
				}
			}
		}
	}

	result := make(map[uint32]CachedGame, len(gameCache))
	for k, v := range gameCache {
		result[k] = v
	}
	return result
}

func SaveGameCache(games map[uint32]CachedGame) {
	gameCacheMu.Lock()
	defer gameCacheMu.Unlock()

	gameCache = games
	gameCacheLoaded = true

	p := gameCacheFilePath()
	if p == "" {
		return
	}

	data, err := json.Marshal(games)
	if err != nil {
		slog.Warn("failed to marshal game cache", "error", err)
		return
	}

	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Warn("failed to create cache dir", "error", err)
		return
	}
	if err := atomicWrite(p, data); err != nil {
		slog.Warn("failed to write game cache", "error", err)
	}
}
