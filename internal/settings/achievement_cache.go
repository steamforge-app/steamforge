package settings

import (
	"os"
	"path/filepath"
	"time"
)

type AchievementCount struct {
	Achieved    int    `json:"achieved"`
	Total       int    `json:"total"`
	EarlyAccess bool   `json:"earlyAccess,omitempty"`
	ReleaseDate string `json:"releaseDate,omitempty"`
	Protected   bool   `json:"protected,omitempty"`
}

var achCacheStore = NewJSONCache[uint32, AchievementCount](cacheFilePath, cacheFlushDelay, "achievement")

func cacheFilePath() string {
	dir := userDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "achievements.json")
}

func resetAchievementCache() {
	achCacheStore.Reset()
}

// LoadAchievementCache returns a snapshot copy of the achievement cache.
func LoadAchievementCache() map[uint32]AchievementCount {
	return achCacheStore.Load()
}

// ClearAchievementCache clears all achievement data and deletes the cache file.
func ClearAchievementCache() {
	achCacheStore.mu.Lock()
	defer achCacheStore.mu.Unlock()
	if achCacheStore.flushTimer != nil {
		achCacheStore.flushTimer.Stop()
		achCacheStore.flushTimer = nil
	}
	achCacheStore.data = make(map[uint32]AchievementCount)
	achCacheStore.loaded = true

	p := cacheFilePath()
	if p != "" {
		os.Remove(p)
	}
}

// ClearNonPerfectedCache removes all entries except:
//   - Perfected games (achieved == total, total > 0)
//   - Released games with no achievements (total == 0, not early access)
func ClearNonPerfectedCache() {
	achCacheStore.mu.Lock()
	defer achCacheStore.mu.Unlock()
	if achCacheStore.flushTimer != nil {
		achCacheStore.flushTimer.Stop()
		achCacheStore.flushTimer = nil
	}

	if achCacheStore.data == nil {
		achCacheStore.data = make(map[uint32]AchievementCount)
		achCacheStore.loaded = true
		return
	}

	for appID, entry := range achCacheStore.data {
		isPerfected := entry.Total > 0 && entry.Achieved == entry.Total
		isOldReleaseNoAchievements := entry.Total == 0 &&
			entry.ReleaseDate != "" && entry.ReleaseDate != "unreleased"
		if !isPerfected && !isOldReleaseNoAchievements {
			delete(achCacheStore.data, appID)
		}
	}

	achCacheStore.flushTimer = time.AfterFunc(cacheFlushDelay, achCacheStore.Flush)
}

// SaveAchievementCounts updates the in-memory cache and schedules a debounced disk write.
// During scans this avoids writing the full JSON file on every single game.
func SaveAchievementCounts(appID uint32, achieved, total int) {
	SaveAchievementEntry(appID, AchievementCount{Achieved: achieved, Total: total})
}

// SaveAchievementCountsEarlyAccess saves counts and marks the game as early access.
func SaveAchievementCountsEarlyAccess(appID uint32, achieved, total int, earlyAccess bool) {
	SaveAchievementEntry(appID, AchievementCount{Achieved: achieved, Total: total, EarlyAccess: earlyAccess})
}

// SaveAchievementCountsRelease saves counts with a release date.
// EarlyAccess is derived: true when releaseDate is "unreleased".
func SaveAchievementCountsRelease(appID uint32, achieved, total int, releaseDate string) {
	SaveAchievementEntry(appID, AchievementCount{
		Achieved:    achieved,
		Total:       total,
		EarlyAccess: releaseDate == "unreleased",
		ReleaseDate: releaseDate,
	})
}

// SaveAchievementEntry updates a single cache entry and schedules a debounced flush.
func SaveAchievementEntry(appID uint32, entry AchievementCount) {
	achCacheStore.Set(appID, entry)
}

// FlushAchievementCache forces an immediate write of the achievement cache to disk.
// Call this at scan completion or shutdown to ensure no data is lost.
func FlushAchievementCache() {
	achCacheStore.Flush()
}
