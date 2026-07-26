package settings

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// cacheFlushDelay is the debounce period for cache writes. Used by achievement,
// HLTB, and percent caches to batch multiple writes together.
const cacheFlushDelay = 500 * time.Millisecond

// JSONCache is a lazily-loaded, in-memory map backed by a single JSON file,
// shared by every per-user cache in this package (achievements, HLTB times,
// global percents, game list, to-play list). Callers get a snapshot copy on
// Load/Get and never touch the internal map directly, so all locking lives
// here instead of being reimplemented per cache.
type JSONCache[K comparable, V any] struct {
	mu          sync.RWMutex
	data        map[K]V
	loaded      bool
	filePath    func() string
	flushDelay  time.Duration // 0 means write synchronously on every Set
	flushTimer  *time.Timer
	logLabel    string
}

// NewJSONCache creates a new cache instance for a given file path and flush delay.
func NewJSONCache[K comparable, V any](filePath func() string, flushDelay time.Duration, logLabel string) *JSONCache[K, V] {
	return &JSONCache[K, V]{filePath: filePath, flushDelay: flushDelay, logLabel: logLabel}
}

// Load returns a snapshot copy of the entire cache, loading from disk first if needed.
func (c *JSONCache[K, V]) Load() map[K]V {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureLoadedLocked()

	result := make(map[K]V, len(c.data))
	for k, v := range c.data {
		result[k] = v
	}
	return result
}

// Get returns a single entry, loading from disk first if needed.
func (c *JSONCache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureLoadedLocked()
	v, ok := c.data[key]
	return v, ok
}

// Set stores an entry and persists it — immediately if flushDelay is 0,
// otherwise via a debounced timer.
func (c *JSONCache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	c.ensureLoadedLocked()
	c.data[key] = value
	if c.flushDelay == 0 {
		data := c.snapshotLocked()
		c.mu.Unlock()
		c.writeToDisk(data)
		return
	}
	if c.flushTimer != nil {
		c.flushTimer.Stop()
	}
	c.flushTimer = time.AfterFunc(c.flushDelay, c.Flush)
	c.mu.Unlock()
}

// Flush forces an immediate write, canceling any pending debounce timer.
// Safe to call even when flushDelay is 0 (no-op timer state, still writes).
func (c *JSONCache[K, V]) Flush() {
	c.mu.Lock()
	if c.flushTimer != nil {
		c.flushTimer.Stop()
		c.flushTimer = nil
	}
	data := c.snapshotLocked()
	c.mu.Unlock()
	c.writeToDisk(data)
}

// Reset clears the in-memory cache (e.g. on Steam account switch) without
// touching the on-disk file for the previous user.
func (c *JSONCache[K, V]) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.flushTimer != nil {
		c.flushTimer.Stop()
		c.flushTimer = nil
	}
	c.data = nil
	c.loaded = false
}

func (c *JSONCache[K, V]) ensureLoadedLocked() {
	if c.loaded {
		return
	}
	c.data = make(map[K]V)
	c.loaded = true
	p := c.filePath()
	if p == "" {
		return
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return
	}
	if err := json.Unmarshal(raw, &c.data); err != nil {
		slog.Warn("failed to parse cache", "cache", c.logLabel, "error", err)
		c.data = make(map[K]V)
	}
}

func (c *JSONCache[K, V]) snapshotLocked() map[K]V {
	result := make(map[K]V, len(c.data))
	for k, v := range c.data {
		result[k] = v
	}
	return result
}

func (c *JSONCache[K, V]) writeToDisk(data map[K]V) {
	p := c.filePath()
	if p == "" {
		return
	}
	marshaled, err := json.Marshal(data)
	if err != nil {
		slog.Warn("failed to marshal cache", "cache", c.logLabel, "error", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		slog.Warn("failed to create cache dir", "cache", c.logLabel, "error", err)
		return
	}
	if err := atomicWrite(p, marshaled); err != nil {
		slog.Warn("failed to write cache", "cache", c.logLabel, "error", err)
	}
}
