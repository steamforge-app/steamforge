package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"steamforge/internal/logging"
	"steamforge/internal/models"
	"steamforge/internal/services"
	"steamforge/internal/settings"
	"steamforge/internal/steam"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Version is set via ldflags at build time (e.g. -X main.Version=v1.0.0).
var Version = "dev"

var errNotConnected = errors.New("not connected to Steam")

type App struct {
	ctx context.Context

	mu             sync.RWMutex
	steamClient    *steam.Client
	gameService    *services.GameService
	achieveService *services.AchievementService
	imageService   *services.ImageService

	scanCancel context.CancelFunc
	scanDone   chan struct{}

	installWatcher *steam.InstallWatcher
	accountWatcher *steam.AccountWatcher
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	settings.Load()

	cacheDir := filepath.Join(os.TempDir(), "steamforge", "cache")
	imageService, err := services.NewImageService(cacheDir)
	if err != nil {
		slog.Warn("image service init failed", "error", err)
	} else {
		imageService.SetContext(ctx)
		a.mu.Lock()
		a.imageService = imageService
		a.mu.Unlock()
	}

	slog.Info("startup complete")
}

func (a *App) shutdown(ctx context.Context) {
	a.stopScan()
	a.stopAccountWatcher()
	a.stopInstallWatcher()

	a.mu.RLock()
	client := a.steamClient
	a.mu.RUnlock()

	if client != nil {
		done := make(chan struct{})
		go func() {
			client.Close()
			close(done)
		}()
		select {
		case <-done:
			slog.Info("Steam client closed")
		case <-ctx.Done():
			slog.Warn("shutdown timed out waiting for Steam client close")
		case <-time.After(5 * time.Second):
			slog.Warn("Steam client close timed out after 5s")
		}
	}
}

// initClient creates a new Steam client with associated services.
// Returns the client, game service, and achievement service.
func (a *App) initClient(appID uint32) (*steam.Client, *services.GameService, *services.AchievementService, error) {
	client, err := steam.NewClient(appID)
	if err != nil {
		return nil, nil, nil, err
	}
	client.StartCallbackLoop()

	gameService := services.NewGameService(client)
	gameService.SetContext(a.ctx)

	achieveService := services.NewAchievementService(client)
	achieveService.SetContext(a.ctx)

	return client, gameService, achieveService, nil
}

func (a *App) ConnectSteam() (uint64, error) {
	a.mu.RLock()
	existing := a.steamClient
	a.mu.RUnlock()

	if existing != nil {
		return existing.SteamID(), nil
	}

	slog.Info("connecting to Steam")
	client, gameService, achieveService, err := a.initClient(0)
	if err != nil {
		slog.Error("Steam connection failed", "error", err)
		return 0, fmt.Errorf("connect to Steam: %w", err)
	}

	a.mu.Lock()
	a.steamClient = client
	a.gameService = gameService
	a.achieveService = achieveService
	a.mu.Unlock()

	slog.Info("Steam connected", "steamID", client.SteamID())
	settings.SetCurrentUser(client.SteamID())

	a.startInstallWatcher()
	a.startAccountWatcher(client.SteamID())

	return client.SteamID(), nil
}

// GetPersonaName returns the current Steam user's display name.
func (a *App) GetPersonaName() string {
	a.mu.RLock()
	client := a.steamClient
	a.mu.RUnlock()

	if client == nil {
		return ""
	}
	return client.PersonaName()
}

func (a *App) IsConnected() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.steamClient != nil
}

func (a *App) reconnectForGame(appID uint32) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.steamClient != nil && a.steamClient.CurrentAppID == appID {
		return nil
	}

	start := time.Now()
	slog.Info("reconnecting for game", "appID", appID)

	if a.steamClient != nil {
		a.steamClient.Close()
		a.steamClient = nil
		a.achieveService = nil
	}

	var client *steam.Client
	var gameService *services.GameService
	var achieveService *services.AchievementService
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		if attempt > 1 {
			slog.Info("retrying Steam reconnect", "appID", appID, "attempt", attempt)
			time.Sleep(time.Duration(attempt) * 300 * time.Millisecond)
		}
		client, gameService, achieveService, err = a.initClient(appID)
		if err == nil {
			break
		}
		slog.Warn("Steam connect attempt failed", "appID", appID, "attempt", attempt, "error", err)
	}
	if err != nil {
		slog.Error("reconnect failed", "appID", appID, "error", err)
		return fmt.Errorf("reconnect for app %d: %w", appID, err)
	}

	a.steamClient = client
	a.gameService = gameService
	a.achieveService = achieveService

	slog.Info("reconnected", "appID", appID, "elapsed", time.Since(start))
	return nil
}

func (a *App) DisconnectGame() error {
	a.mu.Lock()

	if a.steamClient == nil || a.steamClient.CurrentAppID == 0 {
		a.mu.Unlock()
		return nil
	}

	slog.Info("disconnecting from game", "appID", a.steamClient.CurrentAppID)

	a.steamClient.Close()
	a.steamClient = nil
	a.achieveService = nil
	a.gameService = nil
	a.mu.Unlock()

	// Restore the base connection (appID=0) so Steam no longer thinks
	// we're running the game.
	a.restoreBaseConnection()

	slog.Info("disconnected from game")
	return nil
}

func (a *App) FetchGames() ([]models.GameInfo, error) {
	if a.stopScan() {
		a.restoreBaseConnection()
	}

	a.mu.RLock()
	gameService := a.gameService
	a.mu.RUnlock()

	if gameService == nil {
		return nil, errNotConnected
	}
	return gameService.FetchGames()
}

func (a *App) SearchGames(query string) []models.GameInfo {
	a.mu.RLock()
	gameService := a.gameService
	a.mu.RUnlock()

	if gameService == nil {
		return nil
	}
	return gameService.SearchGames(query)
}

// LoadAchievementsFromSchema fetches full achievement data from the Steam community profile.
// No client reconnect needed — works for any public profile.
// Falls back to local schema files if the community endpoint fails.
func (a *App) LoadAchievementsFromSchema(appID uint32) ([]models.Achievement, error) {
	a.mu.RLock()
	client := a.steamClient
	a.mu.RUnlock()

	if client == nil {
		return nil, errNotConnected
	}

	// Try community profile first — has names, descriptions, icons, unlock status
	webAPI := services.NewSteamWebAPI(client.SteamID(), a.ctx)
	achievements, err := webAPI.GetFullAchievements(appID)
	if err == nil && len(achievements) > 0 {
		achieved := 0
		for _, achievement := range achievements {
			if achievement.IsAchieved {
				achieved++
			}
		}
		settings.SaveAchievementCounts(appID, achieved, len(achievements))
		services.MergeSchemaPermissions(appID, achievements)
		return achievements, nil
	}
	if err != nil {
		slog.Debug("community profile fetch failed", "appID", appID, "error", err)
	}

	// Community failed or returned empty — check local schema before falling through to SDK
	if total, hasSchema := services.HasAchievementsFromSchema(appID); hasSchema && total == 0 {
		slog.Debug("schema confirms 0 achievements", "appID", appID)
		settings.SaveAchievementCounts(appID, 0, 0)
		return []models.Achievement{}, nil
	}

	// Neither community nor schema could confirm — error so frontend can try SDK
	return nil, fmt.Errorf("community profile unavailable: %w", err)
}

// LoadAchievements connects to Steam as the game and loads full achievement data.
// This is needed for editing achievements (set/clear).
func (a *App) LoadAchievements(appID uint32) ([]models.Achievement, error) {
	if err := a.reconnectForGame(appID); err != nil {
		return nil, err
	}

	a.mu.RLock()
	achievementService := a.achieveService
	a.mu.RUnlock()

	if achievementService == nil {
		return nil, errNotConnected
	}
	result, err := achievementService.LoadAchievements(appID)
	if err != nil {
		return nil, err
	}

	cacheAchievementCounts(appID, result)
	return result, nil
}

func (a *App) GetAchievements() []models.Achievement {
	a.mu.RLock()
	achievementService := a.achieveService
	a.mu.RUnlock()

	if achievementService == nil {
		return nil
	}
	return achievementService.GetAchievements()
}

func (a *App) SetAchievement(name string) (bool, error) {
	a.mu.RLock()
	achievementService := a.achieveService
	a.mu.RUnlock()

	if achievementService == nil {
		return false, errNotConnected
	}
	return achievementService.SetAchievement(name)
}

func (a *App) ClearAchievement(name string) (bool, error) {
	a.mu.RLock()
	achievementService := a.achieveService
	a.mu.RUnlock()

	if achievementService == nil {
		return false, errNotConnected
	}
	return achievementService.ClearAchievement(name)
}

func (a *App) SetAllAchievements() (int, error) {
	a.mu.RLock()
	achievementService := a.achieveService
	a.mu.RUnlock()

	if achievementService == nil {
		return 0, errNotConnected
	}
	return achievementService.SetAllAchievements()
}

func (a *App) ClearAllAchievements() (int, error) {
	a.mu.RLock()
	achievementService := a.achieveService
	a.mu.RUnlock()

	if achievementService == nil {
		return 0, errNotConnected
	}
	return achievementService.ClearAllAchievements()
}

func (a *App) StoreStats() (bool, error) {
	a.mu.RLock()
	achievementService := a.achieveService
	client := a.steamClient
	a.mu.RUnlock()

	if achievementService == nil {
		return false, errNotConnected
	}
	ok, err := achievementService.StoreStats()
	if err != nil {
		return ok, err
	}

	if ok && client != nil {
		cacheAchievementCounts(client.CurrentAppID, achievementService.GetAchievements())
	}
	return ok, nil
}

func (a *App) GetAchievementCounts() map[uint32]settings.AchievementCount {
	return settings.LoadAchievementCache()
}

// GetToPlayList returns the appIDs currently on the user's "games to play" list.
// Pure local state — no Steam client connection required.
func (a *App) GetToPlayList() []uint32 {
	list := settings.LoadToPlayList()
	appIDs := make([]uint32, 0, len(list))
	for appID := range list {
		appIDs = append(appIDs, appID)
	}
	return appIDs
}

// SetToPlay adds or removes a game from the "games to play" list.
func (a *App) SetToPlay(appID uint32, want bool) error {
	settings.SetToPlay(appID, want)
	return nil
}

// GetHLTBTimes returns completion time estimates for a game from HowLongToBeat.
// Returns cached data immediately if available; otherwise fetches live and caches the result.
func (a *App) GetHLTBTimes(appID uint32, gameName string) (*services.HLTBTimes, error) {
	if entry, ok := settings.GetHLTBEntry(appID); ok {
		return &services.HLTBTimes{
			Main:          entry.Main,
			MainExtra:     entry.MainExtra,
			Completionist: entry.Completionist,
		}, nil
	}

	svc := services.NewHLTBService(a.ctx)
	times, err := svc.Search(gameName)
	if err != nil {
		slog.Warn("hltb search failed", "appID", appID, "game", gameName, "error", err)
		return nil, err
	}
	if times == nil {
		return nil, nil
	}

	settings.SaveHLTBEntry(appID, settings.HLTBEntry{
		Main:          times.Main,
		MainExtra:     times.MainExtra,
		Completionist: times.Completionist,
	})
	return times, nil
}

// GetAllPlaytimes returns hours played for every app with recorded playtime,
// via a single parse of localconfig.vdf. Requires a connected Steam client.
func (a *App) GetAllPlaytimes() (map[uint32]float64, error) {
	a.mu.RLock()
	client := a.steamClient
	a.mu.RUnlock()
	if client == nil {
		return nil, errNotConnected
	}

	return steam.ScanPlaytimeHours(client.SteamID()), nil
}

// GetAllCachedHLTB returns every HLTB entry already cached to disk.
// Never performs a live HLTB search — that only happens via GetHLTBTimes,
// called when a game is actually opened.
func (a *App) GetAllCachedHLTB() map[uint32]settings.HLTBEntry {
	return settings.LoadHLTBCache()
}

func (a *App) GetSettings() settings.Settings {
	return settings.Get()
}

func (a *App) SaveSettings(newSettings settings.Settings) error {
	return settings.Save(newSettings)
}

func (a *App) GetDataDir() string {
	return settings.DataDir()
}

// OpenDataDir opens the data directory in the OS file manager.
func (a *App) OpenDataDir() error {
	dir := settings.DataDir()
	if dir == "" {
		return errors.New("data directory not set")
	}
	switch runtime.GOOS {
	case "windows":
		return exec.Command("explorer", dir).Start()
	case "darwin":
		return exec.Command("open", dir).Start()
	default:
		return exec.Command("xdg-open", dir).Start()
	}
}

// GetLogContent returns the contents of the current session's log file.
func (a *App) GetLogContent() (string, error) {
	return logging.ReadLog()
}

// CheckProfileVisibility checks if the user's Steam community profile is public.
// No API key needed — uses the Steam community XML endpoint.
func (a *App) CheckProfileVisibility() (string, error) {
	a.mu.RLock()
	client := a.steamClient
	a.mu.RUnlock()
	if client == nil {
		return "", errNotConnected
	}

	webAPI := services.NewSteamWebAPI(client.SteamID(), a.ctx)
	return webAPI.CheckProfileVisibility()
}

// FetchGlobalPercents returns global unlock percentages for a game.
// Used by the frontend to poll for rarity data independently of achievement loading.
func (a *App) FetchGlobalPercents(appID uint32) (map[string]float32, error) {
	a.mu.RLock()
	client := a.steamClient
	a.mu.RUnlock()
	if client == nil {
		return nil, errNotConnected
	}

	webAPI := services.NewSteamWebAPI(client.SteamID(), a.ctx)
	percents := webAPI.GetGlobalPercents(appID)
	if percents == nil {
		return nil, fmt.Errorf("failed to fetch percentages for app %d", appID)
	}
	return percents, nil
}

// GetPlaytime returns this account's total hours played for a game, read
// from the local Steam client's localconfig.vdf. Steam no longer exposes
// this over any public, unauthenticated network endpoint, so this reads the
// same local file Steam's own client uses (steam.ScanLastPlayed already
// reads this file for LastPlayed). Returns an error if the game has no
// recorded playtime.
func (a *App) GetPlaytime(appID uint32) (float64, error) {
	a.mu.RLock()
	client := a.steamClient
	a.mu.RUnlock()
	if client == nil {
		return 0, errNotConnected
	}

	hours, found := steam.ScanPlaytimeHours(client.SteamID())[appID]
	if !found {
		return 0, fmt.Errorf("no playtime found for app %d", appID)
	}
	return hours, nil
}

// CheckGameEarlyAccess checks if a game is in Early Access via the Steam Store API
// and caches the result in the achievement counts.
func (a *App) CheckGameEarlyAccess(appID uint32) (bool, error) {
	a.mu.RLock()
	client := a.steamClient
	a.mu.RUnlock()
	if client == nil {
		return false, errNotConnected
	}

	webAPI := services.NewSteamWebAPI(client.SteamID(), a.ctx)
	info := webAPI.GetReleaseInfo(appID)

	// Persist the release info in the achievement cache
	cached := settings.LoadAchievementCache()
	if entry, ok := cached[appID]; ok {
		settings.SaveAchievementCountsRelease(appID, entry.Achieved, entry.Total, info.ReleaseDate)
	} else {
		settings.SaveAchievementCountsRelease(appID, 0, 0, info.ReleaseDate)
	}

	return info.EarlyAccess, nil
}

func (a *App) GetImageBase64(url string) (string, error) {
	a.mu.RLock()
	imageService := a.imageService
	a.mu.RUnlock()

	if imageService == nil {
		return "", errors.New("image service not available")
	}
	return imageService.GetImageBase64(url)
}

func (a *App) ScanAchievementCounts() {
	a.stopScan()

	a.mu.RLock()
	gameService := a.gameService
	a.mu.RUnlock()
	if gameService == nil {
		return
	}

	allGames := gameService.SearchGames("")
	cached := settings.LoadAchievementCache()
	lastScanTime := settings.Get().LastScanTime
	var toScan []models.GameInfo
	for _, game := range allGames {
		if game.IsSoftware {
			continue
		}
		entry, ok := cached[game.AppID]
		if !ok {
			// Never scanned
			toScan = append(toScan, game)
		} else if lastScanTime > 0 && game.LastPlayed > 0 && int64(game.LastPlayed) > lastScanTime {
			// Played since last scan — rescan for updated achievement data
			toScan = append(toScan, game)
		} else if entry.Total > 0 && entry.Achieved == entry.Total {
			// Perfected — check if schema total changed (new DLC achievements)
			if schemaTotal, hasSchema := services.HasAchievementsFromSchema(game.AppID); hasSchema && schemaTotal != entry.Total {
				toScan = append(toScan, game)
			}
		} else if entry.Total == 0 {
			// No achievements — skip old releases entirely
			if isOldRelease(entry) {
				continue
			}
			// Recent, early access, or unknown release date — check if achievements were added
			if schemaTotal, hasSchema := services.HasAchievementsFromSchema(game.AppID); hasSchema && schemaTotal > 0 {
				toScan = append(toScan, game)
			}
		}
	}

	if len(toScan) == 0 {
		return
	}

	// Scan installed games first so the user sees results for games they care about.
	sort.SliceStable(toScan, func(first, second int) bool {
		return toScan[first].Installed && !toScan[second].Installed
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	a.mu.Lock()
	a.scanCancel = cancel
	a.scanDone = done
	a.mu.Unlock()

	slog.Info("starting achievement scan", "games", len(toScan))

	go func() {
		defer close(done)
		a.runAchievementScan(ctx, toScan, cached)
	}()
}

func (a *App) RescanAllAchievements() {
	settings.ClearNonPerfectedCache()
	a.ScanAchievementCounts()
}

func (a *App) StopAchievementScan() {
	a.stopScan()
}

func (a *App) stopScan() bool {
	a.mu.Lock()
	cancel := a.scanCancel
	done := a.scanDone
	a.scanCancel = nil
	a.scanDone = nil
	a.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			slog.Warn("scan shutdown timed out")
		}
		return true
	}
	return false
}

func (a *App) startInstallWatcher() {
	// Seed the game service with the current installed state before the
	// watcher starts, so the first FetchGames() doesn't need to rescan.
	if initial, err := steam.ScanInstalledGames(); err == nil {
		initialMap := make(map[uint32]bool, len(initial))
		for _, g := range initial {
			initialMap[g.AppID] = true
		}
		a.mu.RLock()
		gs := a.gameService
		a.mu.RUnlock()
		if gs != nil {
			gs.SetInstalledApps(initialMap)
		}
	}

	watcher, err := steam.NewInstallWatcher(func(installed map[uint32]bool) {
		// Update the game service cache so subsequent FetchGames() calls
		// don't need to rescan the filesystem.
		a.mu.RLock()
		gs := a.gameService
		a.mu.RUnlock()
		if gs != nil {
			gs.SetInstalledApps(installed)
		}

		// Convert uint32 keys to strings for JSON serialization.
		payload := make(map[string]bool, len(installed))
		for appID, v := range installed {
			payload[strconv.FormatUint(uint64(appID), 10)] = v
		}
		wailsRuntime.EventsEmit(a.ctx, "games-install-changed", payload)
	})
	if err != nil {
		slog.Warn("failed to start install watcher", "error", err)
		return
	}

	a.mu.Lock()
	a.installWatcher = watcher
	a.mu.Unlock()
}

func (a *App) stopInstallWatcher() {
	a.mu.Lock()
	watcher := a.installWatcher
	a.installWatcher = nil
	a.mu.Unlock()

	if watcher != nil {
		watcher.Close()
	}
}

func (a *App) startAccountWatcher(steamID uint64) {
	a.stopAccountWatcher()

	watcher, err := steam.NewAccountWatcher(steamID, a.handleAccountChange)
	if err != nil {
		slog.Warn("failed to start account watcher", "error", err)
		return
	}

	a.mu.Lock()
	a.accountWatcher = watcher
	a.mu.Unlock()
}

func (a *App) stopAccountWatcher() {
	a.mu.Lock()
	watcher := a.accountWatcher
	a.accountWatcher = nil
	a.mu.Unlock()

	if watcher != nil {
		watcher.Close()
	}
}

func (a *App) handleAccountChange(newSteamID uint64) {
	slog.Info("account switch detected", "newSteamID", newSteamID)

	// Tear down current state
	a.stopScan()
	a.stopInstallWatcher()

	a.mu.Lock()
	oldClient := a.steamClient
	a.steamClient = nil
	a.gameService = nil
	a.achieveService = nil
	a.mu.Unlock()

	if oldClient != nil {
		oldClient.Close()
	}

	// Steam needs time to finish the account switch before we can reconnect.
	// Retry for up to 30 seconds.
	var client *steam.Client
	var gameService *services.GameService
	var achieveService *services.AchievementService
	for attempt := range 10 {
		select {
		case <-time.After(3 * time.Second):
		case <-a.ctx.Done():
			slog.Warn("account switch reconnect cancelled during shutdown")
			wailsRuntime.EventsEmit(a.ctx, "steam-disconnected", nil)
			return
		}
		var err error
		client, gameService, achieveService, err = a.initClient(0)
		if err == nil {
			break
		}
		slog.Debug("account switch reconnect attempt", "attempt", attempt+1, "error", err)
	}

	if client == nil {
		slog.Error("failed to reconnect after account switch")
		wailsRuntime.EventsEmit(a.ctx, "steam-disconnected", nil)
		return
	}

	a.mu.Lock()
	a.steamClient = client
	a.gameService = gameService
	a.achieveService = achieveService
	a.mu.Unlock()

	settings.SetCurrentUser(client.SteamID())
	a.startInstallWatcher()

	personaName := client.PersonaName()

	slog.Info("reconnected after account switch", "steamID", client.SteamID(), "persona", personaName)
	wailsRuntime.EventsEmit(a.ctx, "account-changed", map[string]any{
		"steamId":     strconv.FormatUint(client.SteamID(), 10),
		"personaName": personaName,
	})
}

func (a *App) restoreBaseConnection() {
	a.mu.Lock()
	oldClient := a.steamClient
	a.steamClient = nil
	a.achieveService = nil
	a.mu.Unlock()

	if oldClient != nil {
		oldClient.Close()
	}

	client, gameService, _, err := a.initClient(0)
	if err != nil {
		slog.Error("failed to restore base connection", "error", err)
		return
	}

	a.mu.Lock()
	a.steamClient = client
	a.gameService = gameService
	a.mu.Unlock()
}

func (a *App) runAchievementScan(ctx context.Context, gamesToScan []models.GameInfo, cached map[uint32]settings.AchievementCount) {
	totalGames := len(gamesToScan)

	// Step 1: Local resolution — use schema + local stats to resolve as many games as
	// possible without any network calls. Only games with no schema at all need the web API.
	a.mu.RLock()
	client := a.steamClient
	a.mu.RUnlock()

	var localCounts map[uint32]int
	if client != nil {
		localCounts = steam.ScanLocalAchievementCounts(client.SteamID())
	}

	var remaining []models.GameInfo
	schemaResolved := 0
	localResolved := 0
	for _, game := range gamesToScan {
		result := resolveGameLocally(game, cached, localCounts)
		if !result.resolved {
			remaining = append(remaining, game)
			continue
		}
		if result.method == "local" {
			localResolved++
		} else {
			schemaResolved++
		}
		event := map[string]any{
			"appId":    game.AppID,
			"achieved": result.achieved,
			"total":    result.total,
		}
		if result.releaseDate != "" {
			settings.SaveAchievementCountsRelease(game.AppID, result.achieved, result.total, result.releaseDate)
			event["earlyAccess"] = result.earlyAccess
			event["releaseDate"] = result.releaseDate
		} else {
			settings.SaveAchievementCounts(game.AppID, result.achieved, result.total)
		}
		wailsRuntime.EventsEmit(a.ctx, "scan-counts", event)
	}
	slog.Info("local resolution done",
		"schemaResolved", schemaResolved, "localResolved", localResolved,
		"remaining", len(remaining), "total", totalGames)

	// Step 2: Web API — only for games without a schema file.
	// No API key needed, works for public profiles.
	if client != nil && len(remaining) > 0 {
		webAPI := services.NewSteamWebAPI(client.SteamID(), ctx)
		a.runWebAPIScan(ctx, webAPI, remaining, schemaResolved+localResolved, totalGames)
		return
	}

	// No client or no games remaining
	a.finalizeScan()
}

type localResolution struct {
	resolved    bool
	achieved    int
	total       int
	method      string // "schema" or "local"
	earlyAccess bool
	releaseDate string
}

// resolveGameLocally attempts to resolve achievement counts from schema files
// and local stats without any network calls.
func resolveGameLocally(game models.GameInfo, cached map[uint32]settings.AchievementCount, localCounts map[uint32]int) localResolution {
	total, hasSchema := services.HasAchievementsFromSchema(game.AppID)
	if !hasSchema {
		return localResolution{}
	}
	if total == 0 {
		if existing, ok := cached[game.AppID]; ok && existing.ReleaseDate != "" {
			return localResolution{
				resolved: true, achieved: 0, total: 0, method: "schema",
				earlyAccess: existing.EarlyAccess, releaseDate: existing.ReleaseDate,
			}
		}
		return localResolution{}
	}
	if achieved, hasLocal := localCounts[game.AppID]; hasLocal {
		return localResolution{resolved: true, achieved: achieved, total: total, method: "local"}
	}
	return localResolution{resolved: true, achieved: -1, total: total, method: "schema"}
}

// isOldRelease returns true if the cached entry has a known release date older than 6 months.
// Returns false for "unreleased", empty (unknown), or recent dates.
func isOldRelease(entry settings.AchievementCount) bool {
	rd := entry.ReleaseDate
	if rd == "" || rd == "unreleased" {
		// Backward compat: old cache entries with EarlyAccess but no ReleaseDate
		if entry.EarlyAccess {
			return false
		}
		// Unknown release date — needs checking
		return false
	}
	t, err := time.Parse("2006-01-02", rd)
	if err != nil {
		// Non-standard date string stored in cache — treat as known release
		return true
	}
	const oldReleaseThreshold = 183 * 24 * time.Hour // ~6 months
	return time.Since(t) > oldReleaseThreshold
}

type scanResult struct {
	game     models.GameInfo
	achieved int
	total    int
	err      error
}

const scanWorkers = 3

// isTransientError returns true for errors that may resolve on retry
// (rate limits, server errors, network issues, XML parse failures from HTML responses).
func isTransientError(errMsg string) bool {
	return strings.Contains(errMsg, "parse xml") ||
		strings.Contains(errMsg, "community endpoint returned HTTP") ||
		strings.Contains(errMsg, "community request:")
}

// fetchWithRetry fetches achievement counts for a single game with exponential backoff.
// Retries transient errors (rate limits, network issues) up to 3 times.
// Returns immediately for permanent errors (private profile, no stats for game).
func fetchWithRetry(ctx context.Context, webAPI *services.SteamWebAPI, appID uint32, rateLimiter *time.Ticker) (int, int, error) {
	var achieved, total int
	var err error
	for attempt := range 3 {
		select {
		case <-ctx.Done():
			return 0, 0, ctx.Err()
		case <-rateLimiter.C:
		}
		achieved, total, err = webAPI.GetPlayerAchievements(appID)
		if err == nil {
			return achieved, total, nil
		}
		errorMessage := err.Error()
		// Permanent errors — no point retrying
		if !isTransientError(errorMessage) {
			return 0, 0, err
		}
		backoff := time.Duration(500<<attempt) * time.Millisecond
		slog.Debug("scan retry", "appID", appID, "attempt", attempt+1, "backoff", backoff)
		select {
		case <-ctx.Done():
			return 0, 0, ctx.Err()
		case <-time.After(backoff):
		}
	}
	return achieved, total, err
}

func (a *App) runWebAPIScan(ctx context.Context, webAPI *services.SteamWebAPI, games []models.GameInfo, offset, totalGames int) {
	jobs := make(chan models.GameInfo, scanWorkers)
	results := make(chan scanResult, scanWorkers)
	var privateFlag atomic.Bool

	// Rate limiter: 1 request per 500ms across all workers (~2 req/sec)
	rateLimiter := time.NewTicker(500 * time.Millisecond)
	defer rateLimiter.Stop()

	// Launch workers
	var wg sync.WaitGroup
	for range scanWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for game := range jobs {
				if privateFlag.Load() {
					results <- scanResult{game: game, err: errors.New("skipped: private")}
					continue
				}
				achieved, total, err := fetchWithRetry(ctx, webAPI, game.AppID, rateLimiter)
				results <- scanResult{game: game, achieved: achieved, total: total, err: err}
			}
		}()
	}

	// Feed jobs in a separate goroutine
	go func() {
		for _, game := range games {
			select {
			case <-ctx.Done():
				close(jobs)
				return
			case jobs <- game:
			}
		}
		close(jobs)
	}()

	// Close results when all workers finish
	go func() {
		wg.Wait()
		close(results)
	}()

	scanned := 0
	for result := range results {
		select {
		case <-ctx.Done():
			slog.Info("web api scan cancelled", "scanned", scanned, "total", len(games))
			wailsRuntime.EventsEmit(a.ctx, "scan-complete", nil)
			return
		default:
		}

		scanned++
		wailsRuntime.EventsEmit(a.ctx, "scan-progress", map[string]any{
			"current": offset + scanned,
			"total":   totalGames,
			"appId":   result.game.AppID,
			"name":    result.game.Name,
		})

		a.handleWebScanResult(result, webAPI, &privateFlag)
	}

	slog.Info("web api scan complete", "scanned", scanned, "total", totalGames)
	a.finalizeScan()
}

func (a *App) handleWebScanResult(result scanResult, webAPI *services.SteamWebAPI, privateFlag *atomic.Bool) {
	if result.err != nil {
		errorMessage := result.err.Error()
		slog.Debug("web api scan error", "appID", result.game.AppID, "error", errorMessage)

		if strings.Contains(errorMessage, "private") || strings.Contains(errorMessage, "friendsonly") {
			if privateFlag.CompareAndSwap(false, true) {
				slog.Info("profile is private, remaining games will use schema-only")
				wailsRuntime.EventsEmit(a.ctx, "profile-visibility", map[string]any{
					"public": false,
				})
			}
		}

		if privateFlag.Load() {
			total, hasSchema := services.HasAchievementsFromSchema(result.game.AppID)
			if hasSchema && total > 0 {
				settings.SaveAchievementCounts(result.game.AppID, -1, total)
				wailsRuntime.EventsEmit(a.ctx, "scan-counts", map[string]any{
					"appId":    result.game.AppID,
					"achieved": -1,
					"total":    total,
				})
			}
		} else if isTransientError(errorMessage) {
			slog.Debug("skipping cache for transient error", "appID", result.game.AppID, "error", errorMessage)
		} else {
			a.emitZeroAchievements(result.game, webAPI)
		}
		return
	}

	if result.total == 0 {
		a.emitZeroAchievements(result.game, webAPI)
		return
	}

	settings.SaveAchievementCounts(result.game.AppID, result.achieved, result.total)
	wailsRuntime.EventsEmit(a.ctx, "scan-counts", map[string]any{
		"appId":    result.game.AppID,
		"achieved": result.achieved,
		"total":    result.total,
	})
}

func (a *App) emitZeroAchievements(game models.GameInfo, webAPI *services.SteamWebAPI) {
	info := webAPI.GetReleaseInfo(game.AppID)
	settings.SaveAchievementCountsRelease(game.AppID, 0, 0, info.ReleaseDate)
	wailsRuntime.EventsEmit(a.ctx, "scan-counts", map[string]any{
		"appId":       game.AppID,
		"achieved":    0,
		"total":       0,
		"earlyAccess": info.EarlyAccess,
		"releaseDate": info.ReleaseDate,
	})
	if info.EarlyAccess {
		slog.Debug("early access game with no achievements", "appID", game.AppID, "name", game.Name)
	}
}

func (a *App) finalizeScan() {
	settings.FlushAchievementCache()
	a.saveScanTimestamp()
	wailsRuntime.EventsEmit(a.ctx, "scan-complete", nil)
}

func (a *App) saveScanTimestamp() {
	currentSettings := settings.Get()
	currentSettings.LastScanTime = time.Now().Unix()
	if err := settings.Save(currentSettings); err != nil {
		slog.Warn("failed to save scan timestamp", "error", err)
	}
}

func cacheAchievementCounts(appID uint32, achievements []models.Achievement) {
	achieved := 0
	for _, achievement := range achievements {
		if achievement.IsAchieved {
			achieved++
		}
	}
	settings.SaveAchievementCounts(appID, achieved, len(achievements))
}

// UpdateInfo contains the result of a version check against GitHub releases.
type UpdateInfo struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
	DownloadURL     string `json:"downloadUrl"`
}

// GetAppVersion returns the current application version.
func (a *App) GetAppVersion() string {
	return Version
}

// CheckForUpdates checks GitHub for the latest release and compares with the current version.
func (a *App) CheckForUpdates() (UpdateInfo, error) {
	result := UpdateInfo{CurrentVersion: Version}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(a.ctx, http.MethodGet, "https://api.github.com/repos/steamforge-app/steamforge/releases/latest", nil)
	if err != nil {
		return result, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return result, fmt.Errorf("decode response: %w", err)
	}

	result.LatestVersion = release.TagName
	result.DownloadURL = release.HTMLURL
	result.UpdateAvailable = Version != "dev" && normalizeVersion(release.TagName) != normalizeVersion(Version)

	return result, nil
}

// normalizeVersion strips a leading "v" for comparison.
func normalizeVersion(version string) string {
	return strings.TrimPrefix(strings.TrimSpace(version), "v")
}
