package main

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"steamforge/internal/models"
	"steamforge/internal/services"
	"steamforge/internal/settings"
	"steamforge/internal/steam"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

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
