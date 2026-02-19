package settings

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

// Settings holds user preferences that persist across sessions.
type Settings struct {
	ViewMode           string `json:"viewMode"`
	ShowLabels         bool   `json:"showLabels"`
	SortBy             string `json:"sortBy"`
	SortOrder          string `json:"sortOrder"`
	InstalledOpen      bool   `json:"installedOpen"`
	OtherOpen          bool   `json:"otherOpen"`
	AutoStore          bool   `json:"autoStore"`
	AllowLock          bool   `json:"allowLock"`
	ShowUnlockDates    bool   `json:"showUnlockDates"`
	AchievementSort    string `json:"achievementSort"`
	AchievementSortDir string `json:"achievementSortDir"`
	ShowSoftware       bool   `json:"showSoftware"`
	ShowCardButtons    bool   `json:"showCardButtons"`
	CardMinWidth       int    `json:"cardMinWidth"`
	WindowWidth        int    `json:"windowWidth"`
	WindowHeight       int    `json:"windowHeight"`
	LastScanTime       int64  `json:"lastScanTime,omitempty"`
}

// Defaults returns settings with sensible default values.
func Defaults() Settings {
	return Settings{
		ViewMode:           "grid",
		SortBy:             "name",
		SortOrder:          "asc",
		InstalledOpen:      true,
		OtherOpen:          true,
		ShowUnlockDates:    true,
		AchievementSort:    "unlockTime",
		AchievementSortDir: "asc",
		ShowCardButtons:    true,
		CardMinWidth:       200,
		WindowWidth:        1280,
		WindowHeight:       800,
	}
}

var (
	mu       sync.RWMutex
	current  Settings
	filePath string
	loaded   bool

	currentUserMu sync.RWMutex
	currentUserID uint64
)

// SetCurrentUser stores the active Steam user ID and resets in-memory caches
// and settings so they reload from the new user's directory on next access.
func SetCurrentUser(steamID uint64) {
	currentUserMu.Lock()
	currentUserID = steamID
	currentUserMu.Unlock()

	resetAchievementCache()
	resetGameCache()
	reloadSettings()

	slog.Info("current user set", "steamID", steamID)
}

// userDir returns the per-user data directory: {configDir}/users/{steamID}/
// Returns "" if no user has been set or configDir is unavailable.
func userDir() string {
	currentUserMu.RLock()
	id := currentUserID
	currentUserMu.RUnlock()

	if id == 0 {
		return ""
	}

	dir, err := configDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "users", fmt.Sprintf("%d", id))
}

// configDir returns the user-specific configuration directory for SteamForge.
func configDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find user config dir: %w", err)
	}
	return filepath.Join(dir, "steamforge"), nil
}

// DataDir returns the path to the active data directory.
// Returns the per-user directory when a user is set, otherwise the shared config directory.
func DataDir() string {
	if dir := userDir(); dir != "" {
		return dir
	}
	dir, err := configDir()
	if err != nil {
		return ""
	}
	return dir
}

// Load reads settings from disk. Returns defaults if file doesn't exist.
func Load() Settings {
	mu.Lock()
	defer mu.Unlock()

	if loaded {
		return current
	}

	current = Defaults()
	loaded = true

	dir, err := configDir()
	if err != nil {
		slog.Warn("config dir not available", "error", err)
		return current
	}

	filePath = filepath.Join(dir, "settings.json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("failed to read settings", "path", filePath, "error", err)
		}
		return current
	}

	if err := json.Unmarshal(data, &current); err != nil {
		slog.Warn("failed to parse settings", "path", filePath, "error", err)
		current = Defaults()
		return current
	}

	validateSettings(&current)
	slog.Info("settings loaded", "path", filePath)
	return current
}

// Get returns the current settings.
func Get() Settings {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// settingsFilePath returns the per-user settings path when a user is set,
// falling back to the shared config path.
func settingsFilePath() string {
	if dir := userDir(); dir != "" {
		return filepath.Join(dir, "settings.json")
	}
	dir, err := configDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "settings.json")
}

// reloadSettings switches to per-user settings after SetCurrentUser.
// If a per-user file exists, it replaces the in-memory settings.
// If not, the current settings are kept (inherited from shared) and will be
// saved to the per-user path on the next Save.
func reloadSettings() {
	mu.Lock()
	defer mu.Unlock()

	filePath = settingsFilePath()
	if filePath == "" {
		return
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		// No per-user file yet — keep current settings, save will create it
		return
	}

	if err := json.Unmarshal(data, &current); err != nil {
		slog.Warn("failed to parse per-user settings", "path", filePath, "error", err)
		return
	}

	validateSettings(&current)
	slog.Info("per-user settings loaded", "path", filePath)
}

// Save writes settings to disk.
func Save(s Settings) error {
	mu.Lock()
	current = s
	path := settingsFilePath()
	mu.Unlock()

	if path == "" {
		return fmt.Errorf("settings path not available")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	if err := atomicWrite(path, data); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}

	slog.Debug("settings saved", "path", path)
	return nil
}

// validateSettings ensures all settings fields contain valid values.
func validateSettings(s *Settings) {
	if s.ViewMode != "grid" && s.ViewMode != "list" {
		s.ViewMode = "grid"
	}
	if s.SortBy != "name" && s.SortBy != "appId" && s.SortBy != "lastPlayed" && s.SortBy != "achievements" {
		s.SortBy = "name"
	}
	if s.SortOrder != "asc" && s.SortOrder != "desc" {
		s.SortOrder = "asc"
	}
	if s.AchievementSort != "default" && s.AchievementSort != "name" && s.AchievementSort != "unlockTime" && s.AchievementSort != "percent" {
		s.AchievementSort = "unlockTime"
	}
	if s.AchievementSortDir != "asc" && s.AchievementSortDir != "desc" {
		s.AchievementSortDir = "asc"
	}
	if s.CardMinWidth < 150 || s.CardMinWidth > 400 {
		s.CardMinWidth = 200
	}
}

// atomicWrite writes data to a temp file and renames it to the target path,
// ensuring the file is never left in a partially-written state.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0600); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}
