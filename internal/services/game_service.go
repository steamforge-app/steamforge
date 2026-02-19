package services

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"steamforge/internal/models"
	"steamforge/internal/settings"
	"steamforge/internal/steam"
)

const headerImageURL = "https://shared.cloudflare.steamstatic.com/store_item_assets/steam/apps/%d/header.jpg"

// knownSoftwareAppIDs contains app IDs for tools, redistributables, and other non-game
// software that should be hidden by default. This list is a hardcoded fallback for apps
// that aren't properly classified by the Steam API's "type" field.
// TODO: Replace with a web endpoint for dynamic classification.
var knownSoftwareAppIDs = map[uint32]bool{
	// Steamworks & SDK
	228980: true, // Steamworks Common Redistributables
	1007:   true, // Steamworks SDK Redist

	// Steam system apps
	5:   true, // Dedicated Server
	7:   true, // Steam Client
	8:   true, // winui2
	753: true, // Steam
	754: true, // Steam Economy
	755: true, // Steam Achievements
	760: true, // Steam Screenshots
	764: true, // Steam Cloud - User Logs
	765: true, // Greenlight
	766: true, // Steam Workshop

	// Proton / Compatibility
	858280:  true, // Proton 3.7
	930400:  true, // Proton 3.16
	961940:  true, // Proton 4.2
	996510:  true, // Proton 4.11
	1054830: true, // Proton 5.0
	1113280: true, // Proton 5.13
	1245040: true, // Proton 6.3
	1420170: true, // Proton 7.0
	1580130: true, // Proton 8.0
	1887720: true, // Proton 9.0
	2348590: true, // Proton Hotfix
	2180100: true, // Proton Experimental

	// Steam Linux Runtime
	1070560: true, // Steam Linux Runtime
	1391110: true, // Steam Linux Runtime - Soldier
	1628350: true, // Steam Linux Runtime - Sniper

	// Source SDK
	215:    true, // Source SDK
	218:    true, // Source SDK Base 2006
	243730: true, // Source SDK Base 2013 SP
	243750: true, // Source SDK Base 2013 MP
	244310: true, // Source Filmmaker

	// Steam Hardware/VR
	250820: true, // SteamVR
	330050: true, // SteamVR Workshop Tools
	353380: true, // Steam Link

	// Steam Deck / Controller
	1675200: true, // Steam Deck Compatibility
}

// isSoftwareAppType returns true for Steam app types that are not games.
func isSoftwareAppType(appType string) bool {
	switch strings.ToLower(appType) {
	case "tool", "application", "demo", "dlc", "driver",
		"config", "hardware", "music", "video", "series",
		"guide", "comic", "beta":
		return true
	}
	return false
}

type GameService struct {
	client *steam.Client
	ctx    context.Context

	mu    sync.RWMutex
	games []models.GameInfo
}

func NewGameService(client *steam.Client) *GameService {
	return &GameService{
		client: client,
	}
}

func (s *GameService) SetContext(ctx context.Context) {
	s.ctx = ctx
}

type xmlGamesList struct {
	XMLName xml.Name `xml:"gamesList"`
	Games   xmlGames `xml:"games"`
}

type xmlGames struct {
	Games []xmlGame `xml:"game"`
}

type xmlGame struct {
	AppID uint32 `xml:"appID"`
	Name  string `xml:"name"`
	Logo  string `xml:"logo"`
}

var httpClient = &http.Client{Timeout: 15 * time.Second}
var longHTTPClient = &http.Client{Timeout: 60 * time.Second}

func (s *GameService) FetchGames() ([]models.GameInfo, error) {
	slog.Info("fetching games")

	cached := settings.LoadGameCache()

	games, err := s.fetchFromSources(cached)
	if err != nil {
		return nil, err
	}

	games = s.mergeWithCache(games, cached)

	sort.Slice(games, func(i, j int) bool {
		return strings.ToLower(games[i].Name) < strings.ToLower(games[j].Name)
	})

	s.enrichMetadata(games)
	s.tagSoftwareApps(games)

	s.mu.Lock()
	s.games = games
	s.mu.Unlock()

	slog.Info("games loaded", "count", len(games))
	return games, nil
}

// fetchFromSources tries each game source in order, falling back to cache.
func (s *GameService) fetchFromSources(cached map[uint32]settings.CachedGame) ([]models.GameInfo, error) {
	type source struct {
		name string
		fn   func() ([]models.GameInfo, error)
	}
	sources := []source{
		{"packageinfo", s.fetchFromPackageInfo},
		{"XML", s.fetchFromXML},
		{"Steam Web API", s.fetchFromSteamWebAPI},
		{"local API", s.fetchFromLocalAPI},
		{"local scan", s.fetchFromLocal},
	}

	for i, src := range sources {
		games, err := src.fn()
		if err == nil {
			return games, nil
		}
		next := "game cache"
		if i < len(sources)-1 {
			next = sources[i+1].name
		}
		slog.Warn(src.name+" fetch failed, trying "+next, "error", err)
	}

	games := s.gamesFromCache(cached)
	if len(games) == 0 {
		return nil, fmt.Errorf("all game sources failed and cache is empty")
	}
	slog.Info("games loaded from cache", "count", len(games))
	return games, nil
}

// enrichMetadata adds last played timestamps and installed flags.
func (s *GameService) enrichMetadata(games []models.GameInfo) {
	lastPlayed := steam.ScanLastPlayed(s.client.SteamID())
	if lastPlayed != nil {
		for i := range games {
			if ts, ok := lastPlayed[games[i].AppID]; ok {
				games[i].LastPlayed = ts
			}
		}
	}

	installed, scanErr := steam.ScanInstalledGames()
	if scanErr == nil {
		installedSet := make(map[uint32]bool, len(installed))
		for _, g := range installed {
			installedSet[g.AppID] = true
		}
		for i := range games {
			if installedSet[games[i].AppID] {
				games[i].Installed = true
			}
		}
		slog.Info("tagged installed games", "installed", len(installed), "total", len(games))
	}
}

// tagSoftwareApps marks non-game software using the hardcoded list and Steam API type data.
func (s *GameService) tagSoftwareApps(games []models.GameInfo) {
	softwareCount := 0
	apps001 := s.client.Apps001()
	for i := range games {
		if knownSoftwareAppIDs[games[i].AppID] {
			games[i].IsSoftware = true
			softwareCount++
			continue
		}
		if apps001 != nil {
			appType := apps001.GetAppData(games[i].AppID, "type")
			if appType != "" && isSoftwareAppType(appType) {
				games[i].IsSoftware = true
				softwareCount++
			}
		}
	}
	if softwareCount > 0 {
		slog.Info("tagged software apps", "count", softwareCount)
	}
}

// mergeWithCache merges freshly fetched games with the persistent cache.
// Any game known to the cache but missing from the fresh list is added back.
// The cache is then updated and saved.
func (s *GameService) mergeWithCache(fresh []models.GameInfo, cached map[uint32]settings.CachedGame) []models.GameInfo {
	// Build lookup of fresh games and update the cache with fresh data
	freshSet := make(map[uint32]bool, len(fresh))
	merged := make(map[uint32]settings.CachedGame, len(cached))
	for k, v := range cached {
		merged[k] = v
	}
	for _, g := range fresh {
		freshSet[g.AppID] = true
		merged[g.AppID] = settings.CachedGame{Name: g.Name, LogoURL: g.LogoURL}
	}

	// Add back any cached games missing from the fresh fetch
	var added int
	for appID, cg := range cached {
		if !freshSet[appID] {
			fresh = append(fresh, models.GameInfo{
				AppID:   appID,
				Name:    cg.Name,
				LogoURL: cg.LogoURL,
			})
			added++
		}
	}
	if added > 0 {
		slog.Info("restored games from cache", "added", added)
	}

	settings.SaveGameCache(merged)
	return fresh
}

// gamesFromCache converts the persistent game cache into a GameInfo slice.
func (s *GameService) gamesFromCache(cached map[uint32]settings.CachedGame) []models.GameInfo {
	games := make([]models.GameInfo, 0, len(cached))
	for appID, cg := range cached {
		games = append(games, models.GameInfo{
			AppID:   appID,
			Name:    cg.Name,
			LogoURL: cg.LogoURL,
		})
	}
	return games
}

func (s *GameService) fetchFromPackageInfo() ([]models.GameInfo, error) {
	apps := s.client.Apps()
	apps001 := s.client.Apps001()
	if apps == nil || apps001 == nil {
		return nil, fmt.Errorf("ISteamApps interfaces not available")
	}

	appIDs, err := steam.ScanPackageAppIDs()
	if err != nil {
		return nil, fmt.Errorf("scan packageinfo: %w", err)
	}
	slog.Info("packageinfo candidates", "count", len(appIDs))

	var games []models.GameInfo
	for _, appID := range appIDs {
		if !apps.BIsSubscribedApp(appID) {
			continue
		}
		name := apps001.GetAppData(appID, "name")
		if name == "" {
			continue
		}
		game := models.GameInfo{
			AppID:   appID,
			Name:    name,
			LogoURL: fmt.Sprintf(headerImageURL, appID),
		}
		appType := apps001.GetAppData(appID, "type")
		if appType != "" && isSoftwareAppType(appType) {
			game.IsSoftware = true
		}
		games = append(games, game)
	}

	if len(games) == 0 {
		return nil, fmt.Errorf("packageinfo: 0 owned games from %d candidates", len(appIDs))
	}

	slog.Info("games fetched from packageinfo", "owned", len(games))
	return games, nil
}

func (s *GameService) fetchFromXML() ([]models.GameInfo, error) {
	steamID := s.client.SteamID()
	url := fmt.Sprintf("https://steamcommunity.com/profiles/%d/games?xml=1", steamID)
	slog.Info("fetching games from XML", "url", url)

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch games list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("XML endpoint returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read games response: %w", err)
	}

	var gamesList xmlGamesList
	if err := xml.Unmarshal(body, &gamesList); err != nil {
		return nil, fmt.Errorf("parse games XML (len=%d): %w", len(body), err)
	}

	if len(gamesList.Games.Games) == 0 {
		return nil, fmt.Errorf("XML returned 0 games (profile may be private, steamID=%d)", steamID)
	}

	games := make([]models.GameInfo, 0, len(gamesList.Games.Games))
	for _, g := range gamesList.Games.Games {
		if g.AppID == 0 {
			continue
		}

		logoURL := g.Logo
		if logoURL == "" {
			logoURL = fmt.Sprintf(headerImageURL, g.AppID)
		}
		games = append(games, models.GameInfo{
			AppID:   g.AppID,
			Name:    g.Name,
			LogoURL: logoURL,
		})
	}

	if len(games) == 0 {
		return nil, fmt.Errorf("XML returned games but all had appID=0")
	}

	slog.Info("games fetched from XML", "count", len(games))
	return games, nil
}

type steamAppListResponse struct {
	AppList struct {
		Apps []struct {
			AppID uint32 `json:"appid"`
			Name  string `json:"name"`
		} `json:"apps"`
	} `json:"applist"`
}

func (s *GameService) fetchFromSteamWebAPI() ([]models.GameInfo, error) {
	apps := s.client.Apps()
	if apps == nil {
		return nil, fmt.Errorf("ISteamApps not available")
	}

	slog.Info("fetching full app list from Steam Web API")
	resp, err := longHTTPClient.Get("https://api.steampowered.com/ISteamApps/GetAppList/v2/")
	if err != nil {
		return nil, fmt.Errorf("fetch app list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("app list returned HTTP %d", resp.StatusCode)
	}

	var apiResp steamAppListResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("parse app list: %w", err)
	}

	slog.Info("checking ownership", "candidates", len(apiResp.AppList.Apps))

	var games []models.GameInfo
	for _, app := range apiResp.AppList.Apps {
		if app.AppID == 0 || app.Name == "" {
			continue
		}
		if !apps.BIsSubscribedApp(app.AppID) {
			continue
		}
		games = append(games, models.GameInfo{
			AppID:   app.AppID,
			Name:    app.Name,
			LogoURL: fmt.Sprintf(headerImageURL, app.AppID),
		})
	}

	if len(games) == 0 {
		return nil, fmt.Errorf("Steam Web API: 0 owned games found")
	}

	slog.Info("games fetched from Steam Web API", "owned", len(games))
	return games, nil
}

func (s *GameService) fetchFromLocalAPI() ([]models.GameInfo, error) {
	apps := s.client.Apps()
	apps001 := s.client.Apps001()
	if apps == nil || apps001 == nil {
		return nil, fmt.Errorf("ISteamApps interfaces not available")
	}

	candidates := make(map[uint32]bool)

	lastPlayed := steam.ScanLastPlayed(s.client.SteamID())
	for appID := range lastPlayed {
		candidates[appID] = true
	}

	installed, _ := steam.ScanInstalledGames()
	for _, g := range installed {
		candidates[g.AppID] = true
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no candidate app IDs found locally")
	}

	var games []models.GameInfo
	for appID := range candidates {
		if !apps.BIsSubscribedApp(appID) {
			continue
		}
		name := apps001.GetAppData(appID, "name")
		if name == "" {
			name = fmt.Sprintf("App %d", appID)
		}
		games = append(games, models.GameInfo{
			AppID:   appID,
			Name:    name,
			LogoURL: fmt.Sprintf(headerImageURL, appID),
		})
	}

	if len(games) == 0 {
		return nil, fmt.Errorf("local API returned 0 owned games from %d candidates", len(candidates))
	}

	slog.Info("games fetched from local API", "candidates", len(candidates), "owned", len(games))
	return games, nil
}

func (s *GameService) fetchFromLocal() ([]models.GameInfo, error) {
	slog.Info("scanning local appmanifest files")

	installed, err := steam.ScanInstalledGames()
	if err != nil {
		return nil, fmt.Errorf("scan installed games: %w", err)
	}

	games := make([]models.GameInfo, 0, len(installed))
	for _, g := range installed {
		games = append(games, models.GameInfo{
			AppID:   g.AppID,
			Name:    g.Name,
			LogoURL: fmt.Sprintf(headerImageURL, g.AppID),
		})
	}

	slog.Info("games fetched from local scan", "count", len(games))
	return games, nil
}

func (s *GameService) SearchGames(query string) []models.GameInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if query == "" {
		return s.games
	}

	query = strings.ToLower(query)
	var result []models.GameInfo
	for _, g := range s.games {
		if strings.Contains(strings.ToLower(g.Name), query) {
			result = append(result, g)
		}
	}
	return result
}
