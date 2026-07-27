package settings

import (
	"path/filepath"
)

var toPlayCacheStore = NewJSONCache[uint32, bool](toPlayFilePath, 0, "toplay")

func toPlayFilePath() string {
	dir := userDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "to_play.json")
}

func resetToPlayList() {
	toPlayCacheStore.Reset()
}

// LoadToPlayList returns a copy of the current "games to play" set.
func LoadToPlayList() map[uint32]bool {
	return toPlayCacheStore.Load()
}

// SetToPlay adds or removes a game from the "games to play" list and writes
// the change to disk immediately — toggles are rare, deliberate user clicks,
// not a bulk scan, so no debounce is needed here (unlike achievement_cache.go).
func SetToPlay(appID uint32, want bool) {
	toPlayCacheStore.SetOrDelete(appID, true, !want)
}
