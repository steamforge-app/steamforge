package settings

import (
	"path/filepath"
	"time"
)

const percentCacheDiskTTL = 24 * time.Hour

type PercentCacheEntry struct {
	Data      map[string]float32 `json:"data"`
	FetchedAt time.Time          `json:"fetchedAt"`
}

var percentCacheStore = NewJSONCache[uint32, PercentCacheEntry](percentCacheFilePath, cacheFlushDelay, "percent")

func percentCacheFilePath() string {
	dir := userDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "percents.json")
}

// LoadPercentEntry returns cached percents for an appID, if any exist on disk.
// found reports whether an entry exists at all; stale reports whether it is
// older than the disk TTL and should be refreshed in the background rather
// than discarded.
func LoadPercentEntry(appID uint32) (data map[string]float32, stale bool, found bool) {
	entry, ok := percentCacheStore.Get(appID)
	if !ok {
		return nil, false, false
	}
	return entry.Data, time.Since(entry.FetchedAt) > percentCacheDiskTTL, true
}

// SavePercentEntry persists percents for an appID to disk (debounced).
func SavePercentEntry(appID uint32, percents map[string]float32) {
	percentCacheStore.Set(appID, PercentCacheEntry{Data: percents, FetchedAt: time.Now()})
}

// FlushPercentCache forces an immediate write of the percent cache to disk.
func FlushPercentCache() {
	percentCacheStore.Flush()
}

func resetPercentCache() {
	percentCacheStore.Reset()
}
