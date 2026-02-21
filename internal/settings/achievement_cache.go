package settings

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type AchievementCount struct {
	Achieved    int  `json:"achieved"`
	Total       int  `json:"total"`
	EarlyAccess bool `json:"earlyAccess,omitempty"`
}

const cacheFlushDelay = 500 * time.Millisecond

var (
	cacheMu         sync.RWMutex
	achCache        map[uint32]AchievementCount
	cacheLoaded     bool
	cacheFlushTimer *time.Timer
)

func cacheFilePath() string {
	dir := userDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "achievements.json")
}

func resetAchievementCache() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if cacheFlushTimer != nil {
		cacheFlushTimer.Stop()
		cacheFlushTimer = nil
	}
	achCache = nil
	cacheLoaded = false
}

func LoadAchievementCache() map[uint32]AchievementCount {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	if !cacheLoaded {
		achCache = make(map[uint32]AchievementCount)
		cacheLoaded = true

		p := cacheFilePath()
		if p != "" {
			data, err := os.ReadFile(p)
			if err == nil {
				if err := json.Unmarshal(data, &achCache); err != nil {
					slog.Warn("failed to parse achievement cache", "error", err)
					achCache = make(map[uint32]AchievementCount)
				}
			}
		}
	}

	result := make(map[uint32]AchievementCount, len(achCache))
	for k, v := range achCache {
		result[k] = v
	}
	return result
}


func ClearAchievementCache() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if cacheFlushTimer != nil {
		cacheFlushTimer.Stop()
		cacheFlushTimer = nil
	}
	achCache = make(map[uint32]AchievementCount)
	cacheLoaded = true

	p := cacheFilePath()
	if p != "" {
		os.Remove(p)
	}
}

// ClearNonPerfectedCache removes all entries except:
//   - Perfected games (achieved == total, total > 0)
//   - Released games with no achievements (total == 0, not early access)
func ClearNonPerfectedCache() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if cacheFlushTimer != nil {
		cacheFlushTimer.Stop()
		cacheFlushTimer = nil
	}

	if achCache == nil {
		achCache = make(map[uint32]AchievementCount)
		cacheLoaded = true
		return
	}

	for appID, entry := range achCache {
		isPerfected := entry.Total > 0 && entry.Achieved == entry.Total
		isReleasedNoAchievements := entry.Total == 0 && !entry.EarlyAccess
		if !isPerfected && !isReleasedNoAchievements {
			delete(achCache, appID)
		}
	}

	if cacheFlushTimer != nil {
		cacheFlushTimer.Stop()
	}
	cacheFlushTimer = time.AfterFunc(cacheFlushDelay, flushAchievementCache)
}

// SaveAchievementCounts updates the in-memory cache and schedules a debounced disk write.
// During scans this avoids writing the full JSON file on every single game.
func SaveAchievementCounts(appID uint32, achieved, total int) {
	saveAchievementEntry(appID, AchievementCount{Achieved: achieved, Total: total})
}

// SaveAchievementCountsEarlyAccess saves counts and marks the game as early access.
func SaveAchievementCountsEarlyAccess(appID uint32, achieved, total int, earlyAccess bool) {
	saveAchievementEntry(appID, AchievementCount{Achieved: achieved, Total: total, EarlyAccess: earlyAccess})
}

func saveAchievementEntry(appID uint32, entry AchievementCount) {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	if achCache == nil {
		achCache = make(map[uint32]AchievementCount)
		cacheLoaded = true
	}

	achCache[appID] = entry

	// Debounce: reset the timer on each call, flush after the delay
	if cacheFlushTimer != nil {
		cacheFlushTimer.Stop()
	}
	cacheFlushTimer = time.AfterFunc(cacheFlushDelay, flushAchievementCache)
}

// FlushAchievementCache forces an immediate write of the achievement cache to disk.
// Call this at scan completion or shutdown to ensure no data is lost.
func FlushAchievementCache() {
	cacheMu.Lock()
	if cacheFlushTimer != nil {
		cacheFlushTimer.Stop()
		cacheFlushTimer = nil
	}
	cacheMu.Unlock()
	flushAchievementCache()
}

func flushAchievementCache() {
	cacheMu.RLock()
	p := cacheFilePath()
	if p == "" || achCache == nil {
		cacheMu.RUnlock()
		return
	}
	data, err := json.Marshal(achCache)
	cacheMu.RUnlock()

	if err != nil {
		slog.Warn("failed to marshal achievement cache", "error", err)
		return
	}

	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Warn("failed to create cache dir", "error", err)
		return
	}
	if err := atomicWrite(p, data); err != nil {
		slog.Warn("failed to write achievement cache", "error", err)
	}
}
