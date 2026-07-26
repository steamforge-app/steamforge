package settings

import (
	"path/filepath"
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

var hltbCacheStore = NewJSONCache[uint32, HLTBEntry](hltbCacheFilePath, cacheFlushDelay, "hltb")

func hltbCacheFilePath() string {
	dir := userDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "hltb.json")
}

// LoadHLTBCache returns a snapshot copy of the HLTB cache.
func LoadHLTBCache() map[uint32]HLTBEntry {
	return hltbCacheStore.Load()
}

// GetHLTBEntry retrieves a single HLTB entry by app ID.
func GetHLTBEntry(appID uint32) (HLTBEntry, bool) {
	return hltbCacheStore.Get(appID)
}

// SaveHLTBEntry stores an HLTB entry and schedules a debounced write.
func SaveHLTBEntry(appID uint32, entry HLTBEntry) {
	hltbCacheStore.Set(appID, entry)
}

// FlushHLTBCache forces an immediate write, for use at shutdown so a
// recently-fetched HLTB entry isn't lost if the app closes before the
// debounce timer fires.
func FlushHLTBCache() {
	hltbCacheStore.Flush()
}

func resetHLTBCache() {
	hltbCacheStore.Reset()
}
