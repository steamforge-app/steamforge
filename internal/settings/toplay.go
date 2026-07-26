package settings

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

var (
	toPlayMu     sync.RWMutex
	toPlayList   map[uint32]bool
	toPlayLoaded bool
)

func toPlayFilePath() string {
	dir := userDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "to_play.json")
}

func resetToPlayList() {
	toPlayMu.Lock()
	defer toPlayMu.Unlock()
	toPlayList = nil
	toPlayLoaded = false
}

// LoadToPlayList returns a copy of the current "games to play" set.
func LoadToPlayList() map[uint32]bool {
	toPlayMu.Lock()
	defer toPlayMu.Unlock()

	if !toPlayLoaded {
		toPlayList = make(map[uint32]bool)
		toPlayLoaded = true

		p := toPlayFilePath()
		if p != "" {
			data, err := os.ReadFile(p)
			if err == nil {
				if err := json.Unmarshal(data, &toPlayList); err != nil {
					slog.Warn("failed to parse to-play list", "error", err)
					toPlayList = make(map[uint32]bool)
				}
			}
		}
	}

	result := make(map[uint32]bool, len(toPlayList))
	for k, v := range toPlayList {
		result[k] = v
	}
	return result
}

// SetToPlay adds or removes a game from the "games to play" list and writes
// the change to disk immediately — toggles are rare, deliberate user clicks,
// not a bulk scan, so no debounce is needed here (unlike achievement_cache.go).
func SetToPlay(appID uint32, want bool) {
	toPlayMu.Lock()
	if toPlayList == nil {
		toPlayList = make(map[uint32]bool)
		toPlayLoaded = true
	}
	if want {
		toPlayList[appID] = true
	} else {
		delete(toPlayList, appID)
	}
	snapshot := make(map[uint32]bool, len(toPlayList))
	for k, v := range toPlayList {
		snapshot[k] = v
	}
	toPlayMu.Unlock()

	p := toPlayFilePath()
	if p == "" {
		return
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		slog.Warn("failed to marshal to-play list", "error", err)
		return
	}

	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Warn("failed to create config dir", "error", err)
		return
	}
	if err := atomicWrite(p, data); err != nil {
		slog.Warn("failed to write to-play list", "error", err)
	}
}
