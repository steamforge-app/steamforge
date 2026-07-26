package steam

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// InstalledGame represents a game found via local appmanifest scanning.
type InstalledGame struct {
	AppID uint32
	Name  string
}

// kvPairRe matches top-level "key" "value" lines in ACF/VDF files.
var kvPairRe = regexp.MustCompile(`^\s*"(\w+)"\s+"([^"]*)"`)

// ScanInstalledGames reads installed games from local Steam appmanifest files.
// It checks the primary steamapps directory and all additional library folders.
func ScanInstalledGames() ([]InstalledGame, error) {
	installPath, err := GetInstallPath()
	if err != nil {
		return nil, fmt.Errorf("get Steam install path: %w", err)
	}

	libraryPaths := collectLibraryPaths(installPath)
	slog.Info("scanning Steam libraries", "count", len(libraryPaths))

	var games []InstalledGame
	seen := make(map[uint32]bool)

	for _, libPath := range libraryPaths {
		steamapps := filepath.Join(libPath, "steamapps")
		matches, err := filepath.Glob(filepath.Join(steamapps, "appmanifest_*.acf"))
		if err != nil {
			slog.Warn("glob appmanifests failed", "path", steamapps, "error", err)
			continue
		}

		for _, acfPath := range matches {
			g, err := parseAppManifest(acfPath)
			if err != nil {
				slog.Debug("skip appmanifest", "path", acfPath, "error", err)
				continue
			}
			if g.AppID == 0 || seen[g.AppID] {
				continue
			}
			seen[g.AppID] = true
			games = append(games, g)
		}
	}

	slog.Info("scan complete", "games", len(games))
	return games, nil
}

// collectLibraryPaths returns all Steam library root paths.
// Always includes the primary install path, plus any from libraryfolders.vdf.
func collectLibraryPaths(installPath string) []string {
	paths := []string{installPath}

	vdfPath := filepath.Join(installPath, "steamapps", "libraryfolders.vdf")
	f, err := os.Open(vdfPath)
	if err != nil {
		// Also try config/libraryfolders.vdf (older Steam layout)
		vdfPath = filepath.Join(installPath, "config", "libraryfolders.vdf")
		f, err = os.Open(vdfPath)
		if err != nil {
			slog.Debug("libraryfolders.vdf not found", "error", err)
			return paths
		}
	}
	defer f.Close()

	seen := make(map[string]bool)
	seen[installPath] = true

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		m := kvPairRe.FindStringSubmatch(scanner.Text())
		if m == nil {
			continue
		}
		key, value := m[1], m[2]
		if key == "path" && value != "" {
			norm := filepath.Clean(value)
			if !seen[norm] {
				seen[norm] = true
				paths = append(paths, norm)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		slog.Warn("error reading libraryfolders.vdf", "error", err)
	}

	return paths
}

// appTimeEntry holds the per-app fields scanned from localconfig.vdf's
// "apps" section in a single pass.
type appTimeEntry struct {
	LastPlayed      uint32
	PlaytimeMinutes uint32
}

// scanAppTimeData parses the user's localconfig.vdf once and returns
// LastPlayed and Playtime (total minutes) for every app that has them.
// Both ScanLastPlayed and ScanPlaytimeHours are thin views over this.
func scanAppTimeData(steamID uint64) map[uint32]appTimeEntry {
	installPath, err := GetInstallPath()
	if err != nil {
		slog.Warn("cannot read app time data: install path", "error", err)
		return nil
	}

	// Account ID is the lower 32 bits of the 64-bit SteamID
	accountID := uint32(steamID & 0xFFFFFFFF)
	vdfPath := filepath.Join(installPath, "userdata", strconv.FormatUint(uint64(accountID), 10), "config", "localconfig.vdf")

	f, err := os.Open(vdfPath)
	if err != nil {
		slog.Warn("cannot read localconfig.vdf", "path", vdfPath, "error", err)
		return nil
	}
	defer f.Close()

	// Simple state machine to parse the nested text VDF.
	// We're looking for entries under "apps" -> "<appid>" -> "LastPlayed"/"Playtime"
	result := make(map[uint32]appTimeEntry)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // localconfig.vdf can be large

	inApps := false
	depth := 0       // nesting depth within the "apps" section
	currentApp := uint32(0)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "{" {
			if inApps {
				depth++
			}
			continue
		}
		if line == "}" {
			if inApps {
				depth--
				if depth == 1 {
					currentApp = 0
				}
				if depth <= 0 {
					inApps = false
					depth = 0
				}
			}
			continue
		}

		m := kvPairRe.FindStringSubmatch(line)
		if m == nil {
			// Could be a section header like "apps"
			trimmed := strings.Trim(line, "\"")
			if strings.EqualFold(trimmed, "apps") {
				inApps = true
				depth = 0
			} else if inApps && depth == 1 {
				// This is an app ID section header
				id, err := strconv.ParseUint(trimmed, 10, 32)
				if err == nil && id > 0 {
					currentApp = uint32(id)
				}
			}
			continue
		}

		// Key-value pair
		if inApps && depth == 2 && currentApp > 0 {
			key := strings.ToLower(m[1])
			switch key {
			case "lastplayed":
				if ts, err := strconv.ParseUint(m[2], 10, 32); err == nil && ts > 0 {
					entry := result[currentApp]
					entry.LastPlayed = uint32(ts)
					result[currentApp] = entry
				}
			case "playtime":
				if minutes, err := strconv.ParseUint(m[2], 10, 32); err == nil {
					entry := result[currentApp]
					entry.PlaytimeMinutes = uint32(minutes)
					result[currentApp] = entry
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Warn("error reading localconfig.vdf", "error", err)
	}

	return result
}

// ScanLastPlayed reads LastPlayed timestamps from the user's localconfig.vdf.
// Returns a map of appID -> unix timestamp.
func ScanLastPlayed(steamID uint64) map[uint32]uint32 {
	data := scanAppTimeData(steamID)
	result := make(map[uint32]uint32, len(data))
	for appID, entry := range data {
		if entry.LastPlayed > 0 {
			result[appID] = entry.LastPlayed
		}
	}
	slog.Info("scanned last played times", "count", len(result))
	return result
}

// ScanPlaytimeHours reads total playtime from the user's localconfig.vdf.
// Returns a map of appID -> hours played, for apps with any recorded time.
func ScanPlaytimeHours(steamID uint64) map[uint32]float64 {
	data := scanAppTimeData(steamID)
	result := make(map[uint32]float64, len(data))
	for appID, entry := range data {
		if entry.PlaytimeMinutes > 0 {
			result[appID] = float64(entry.PlaytimeMinutes) / 60
		}
	}
	slog.Info("scanned playtimes", "count", len(result))
	return result
}

// parseAppManifest extracts AppID and Name from a single ACF file.
func parseAppManifest(path string) (InstalledGame, error) {
	f, err := os.Open(path)
	if err != nil {
		return InstalledGame{}, err
	}
	defer f.Close()

	var g InstalledGame
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		m := kvPairRe.FindStringSubmatch(scanner.Text())
		if m == nil {
			continue
		}
		key := strings.ToLower(m[1])
		switch key {
		case "appid":
			id, err := strconv.ParseUint(m[2], 10, 32)
			if err != nil {
				return InstalledGame{}, fmt.Errorf("invalid appid: %w", err)
			}
			g.AppID = uint32(id)
		case "name":
			g.Name = m[2]
		}
		if g.AppID != 0 && g.Name != "" {
			break // got both fields, stop early
		}
	}
	if err := scanner.Err(); err != nil {
		return InstalledGame{}, fmt.Errorf("read appmanifest: %w", err)
	}

	if g.AppID == 0 {
		return InstalledGame{}, errors.New("no appid found")
	}
	if g.Name == "" {
		g.Name = fmt.Sprintf("App %d", g.AppID)
	}

	return g, nil
}
