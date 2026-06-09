package settings

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const percentCacheDiskTTL = 24 * time.Hour

type PercentCacheEntry struct {
	Data      map[string]float32 `json:"data"`
	FetchedAt time.Time          `json:"fetchedAt"`
}

var (
	percentCacheMu         sync.RWMutex
	percentDiskCache       map[uint32]PercentCacheEntry
	percentCacheLoaded     bool
	percentCacheFlushTimer *time.Timer
)

func percentCacheFilePath() string {
	dir := userDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "percents.json")
}

func ensurePercentCache() {
	if percentCacheLoaded {
		return
	}
	percentDiskCache = make(map[uint32]PercentCacheEntry)
	percentCacheLoaded = true

	p := percentCacheFilePath()
	if p == "" {
		return
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, &percentDiskCache); err != nil {
		slog.Warn("failed to parse percent cache", "error", err)
		percentDiskCache = make(map[uint32]PercentCacheEntry)
	}
}

// LoadPercentEntry returns cached percents for an appID if they exist and are within TTL.
func LoadPercentEntry(appID uint32) (map[string]float32, bool) {
	percentCacheMu.Lock()
	defer percentCacheMu.Unlock()

	ensurePercentCache()

	entry, ok := percentDiskCache[appID]
	if !ok || time.Since(entry.FetchedAt) > percentCacheDiskTTL {
		return nil, false
	}
	return entry.Data, true
}

// SavePercentEntry persists percents for an appID to disk (debounced).
func SavePercentEntry(appID uint32, percents map[string]float32) {
	percentCacheMu.Lock()
	defer percentCacheMu.Unlock()

	ensurePercentCache()
	percentDiskCache[appID] = PercentCacheEntry{Data: percents, FetchedAt: time.Now()}

	if percentCacheFlushTimer != nil {
		percentCacheFlushTimer.Stop()
	}
	percentCacheFlushTimer = time.AfterFunc(cacheFlushDelay, flushPercentCache)
}

// FlushPercentCache forces an immediate write of the percent cache to disk.
func FlushPercentCache() {
	percentCacheMu.Lock()
	if percentCacheFlushTimer != nil {
		percentCacheFlushTimer.Stop()
		percentCacheFlushTimer = nil
	}
	percentCacheMu.Unlock()
	flushPercentCache()
}

func flushPercentCache() {
	percentCacheMu.RLock()
	p := percentCacheFilePath()
	if p == "" || percentDiskCache == nil {
		percentCacheMu.RUnlock()
		return
	}
	data, err := json.Marshal(percentDiskCache)
	percentCacheMu.RUnlock()

	if err != nil {
		slog.Warn("failed to marshal percent cache", "error", err)
		return
	}

	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Warn("failed to create cache dir", "error", err)
		return
	}
	if err := atomicWrite(p, data); err != nil {
		slog.Warn("failed to write percent cache", "error", err)
	}
}
