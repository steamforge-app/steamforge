package settings

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type HLTBEntry struct {
	Main          float32 `json:"main"`
	MainExtra     float32 `json:"mainExtra"`
	Completionist float32 `json:"completionist"`
	// CheckedAt is a unix timestamp, consulted as a time-based freshness check
	// when a more precise signal (GameUpdatedAt) isn't available — i.e. for
	// "not found" entries, and for found entries on games that aren't installed.
	CheckedAt int64 `json:"checkedAt"`
	// GameUpdatedAt is the installed game's appmanifest LastUpdated timestamp
	// at the time this entry was cached (0 if the game wasn't installed then).
	// A found entry is re-checked once the game's current LastUpdated moves
	// past this value, since HLTB times can change after updates/DLC.
	GameUpdatedAt int64 `json:"gameUpdatedAt"`
}

var (
	hltbMu         sync.RWMutex
	hltbCache      map[uint32]HLTBEntry
	hltbLoaded     bool
	hltbFlushTimer *time.Timer
)

func hltbCacheFilePath() string {
	dir := userDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "hltb.json")
}

func LoadHLTBCache() map[uint32]HLTBEntry {
	hltbMu.Lock()
	defer hltbMu.Unlock()

	if !hltbLoaded {
		hltbCache = make(map[uint32]HLTBEntry)
		hltbLoaded = true

		p := hltbCacheFilePath()
		if p != "" {
			data, err := os.ReadFile(p)
			if err == nil {
				if err := json.Unmarshal(data, &hltbCache); err != nil {
					slog.Warn("failed to parse hltb cache", "error", err)
					hltbCache = make(map[uint32]HLTBEntry)
				}
			}
		}
	}

	result := make(map[uint32]HLTBEntry, len(hltbCache))
	for k, v := range hltbCache {
		result[k] = v
	}
	return result
}

func GetHLTBEntry(appID uint32) (HLTBEntry, bool) {
	hltbMu.RLock()
	defer hltbMu.RUnlock()

	if !hltbLoaded {
		hltbMu.RUnlock()
		LoadHLTBCache()
		hltbMu.RLock()
	}

	entry, ok := hltbCache[appID]
	return entry, ok
}

func SaveHLTBEntry(appID uint32, entry HLTBEntry) {
	hltbMu.Lock()
	defer hltbMu.Unlock()

	if hltbCache == nil {
		hltbCache = make(map[uint32]HLTBEntry)
		hltbLoaded = true
	}

	hltbCache[appID] = entry

	if hltbFlushTimer != nil {
		hltbFlushTimer.Stop()
	}
	hltbFlushTimer = time.AfterFunc(cacheFlushDelay, flushHLTBCache)
}

func flushHLTBCache() {
	hltbMu.RLock()
	p := hltbCacheFilePath()
	if p == "" || hltbCache == nil {
		hltbMu.RUnlock()
		return
	}
	data, err := json.Marshal(hltbCache)
	hltbMu.RUnlock()

	if err != nil {
		slog.Warn("failed to marshal hltb cache", "error", err)
		return
	}

	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Warn("failed to create hltb cache dir", "error", err)
		return
	}
	if err := atomicWrite(p, data); err != nil {
		slog.Warn("failed to write hltb cache", "error", err)
	}
}
