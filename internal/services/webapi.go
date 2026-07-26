package services

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"steamforge/internal/models"
	"steamforge/internal/settings"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const privacyPublic = "public"

// percentCache caches global achievement percentages per appID.
var percentCache struct {
	sync.RWMutex
	entries    map[uint32]percentCacheEntry
	refreshing map[uint32]bool
}

type percentCacheEntry struct {
	data      map[string]float32
	fetchedAt time.Time
}

const percentCacheTTL = 15 * time.Minute

func init() {
	percentCache.entries = make(map[uint32]percentCacheEntry)
	percentCache.refreshing = make(map[uint32]bool)
}

// SteamWebAPI fetches achievement data from the Steam community profile.
// No API key needed — works for any public profile.
type SteamWebAPI struct {
	steamID uint64
	client  *http.Client
	ctx     context.Context
}

// XML response from steamcommunity.com/profiles/{id}/stats/{appid}/?xml=1
type communityStatsResponse struct {
	XMLName      xml.Name               `xml:"playerstats"`
	PrivacyState string                 `xml:"privacyState"`
	Error        string                 `xml:"error"`
	Achievements communityAchievements  `xml:"achievements"`
}

type communityAchievements struct {
	Items []communityAchievement `xml:"achievement"`
}

type communityAchievement struct {
	Closed          int    `xml:"closed,attr"`
	APIName         string `xml:"apiname"`
	Name            string `xml:"name"`
	Description     string `xml:"description"`
	IconClosed      string `xml:"iconClosed"`
	IconOpen        string `xml:"iconOpen"`
	UnlockTimestamp int64  `xml:"unlockTimestamp"`
}

func NewSteamWebAPI(steamID uint64, ctx context.Context) *SteamWebAPI {
	return &SteamWebAPI{
		steamID: steamID,
		client:  &http.Client{Timeout: 15 * time.Second},
		ctx:     ctx,
	}
}

func (w *SteamWebAPI) fetchStats(appID uint32) ([]byte, error) {
	rawURL := fmt.Sprintf(
		"https://steamcommunity.com/profiles/%d/stats/%d/?xml=1",
		w.steamID, appID,
	)
	req, err := http.NewRequestWithContext(w.ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("community request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("community endpoint returned HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
}

// GetPlayerAchievements fetches achievement counts for a game from the Steam community profile.
func (w *SteamWebAPI) GetPlayerAchievements(appID uint32) (int, int, error) {
	body, err := w.fetchStats(appID)
	if err != nil {
		return 0, 0, err
	}

	var result communityStatsResponse
	if err := xml.Unmarshal(body, &result); err != nil {
		return 0, 0, fmt.Errorf("parse xml: %w", err)
	}

	if result.Error != "" {
		return 0, 0, fmt.Errorf("steam: %s", result.Error)
	}
	if result.PrivacyState != "" && result.PrivacyState != privacyPublic {
		return 0, 0, fmt.Errorf("profile is %s", result.PrivacyState)
	}

	total := len(result.Achievements.Items)
	achieved := 0
	for _, a := range result.Achievements.Items {
		if a.Closed == 1 {
			achieved++
		}
	}
	return achieved, total, nil
}

// GetFullAchievements fetches complete achievement data from the community profile.
// Returns display names, descriptions, icons, and unlock status — no API key or client reconnect needed.
func (w *SteamWebAPI) GetFullAchievements(appID uint32) ([]models.Achievement, error) {
	body, err := w.fetchStats(appID)
	if err != nil {
		return nil, err
	}

	var result communityStatsResponse
	if err := xml.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse xml: %w", err)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("steam: %s", result.Error)
	}
	if result.PrivacyState != "" && result.PrivacyState != privacyPublic {
		return nil, fmt.Errorf("profile is %s", result.PrivacyState)
	}

	achievements := make([]models.Achievement, 0, len(result.Achievements.Items))
	for _, a := range result.Achievements.Items {
		if a.APIName == "" {
			continue
		}
		achievements = append(achievements, models.Achievement{
			ID:          a.APIName,
			Name:        a.Name,
			Description: a.Description,
			IconURL:     a.IconClosed,
			IconGrayURL: a.IconOpen,
			IsAchieved:  a.Closed == 1,
			UnlockTime:  uint32(a.UnlockTimestamp),
		})
	}
	return achievements, nil
}

// JSON response from ISteamUserStats/GetGlobalAchievementPercentagesForApp
type globalPercentResponse struct {
	AchievementPercentages struct {
		Achievements []struct {
			Name    string  `json:"name"`
			Percent float64 `json:"percent,string"`
		} `json:"achievements"`
	} `json:"achievementpercentages"`
}

// GetGlobalPercents fetches global unlock percentages for a game.
// A fresh (<15 min) in-memory value or any disk-cached value (regardless of
// age) is returned immediately. A disk-cached value older than 24h also
// triggers a background refresh that emits a "percents-updated" event once
// it completes. Only a true first-time miss blocks on a live network fetch.
func (w *SteamWebAPI) GetGlobalPercents(appID uint32) map[string]float32 {
	// Check in-memory cache first (15 min)
	percentCache.RLock()
	if entry, ok := percentCache.entries[appID]; ok && time.Since(entry.fetchedAt) < percentCacheTTL {
		percentCache.RUnlock()
		slog.Info("global percents memory cache hit", "appID", appID, "count", len(entry.data))
		return entry.data
	}
	percentCache.RUnlock()

	// Check disk cache before hitting the network — any age is usable immediately.
	if diskData, stale, found := settings.LoadPercentEntry(appID); found {
		slog.Info("global percents disk cache hit", "appID", appID, "count", len(diskData), "stale", stale)
		percentCache.Lock()
		percentCache.entries[appID] = percentCacheEntry{data: diskData, fetchedAt: time.Now()}
		percentCache.Unlock()

		if stale {
			w.refreshGlobalPercentsAsync(appID)
		}
		return diskData
	}

	slog.Info("global percents cache miss", "appID", appID)

	percents := w.fetchGlobalPercents(appID)
	if percents != nil {
		percentCache.Lock()
		percentCache.entries[appID] = percentCacheEntry{data: percents, fetchedAt: time.Now()}
		percentCache.Unlock()
		settings.SavePercentEntry(appID, percents)
		return percents
	}

	slog.Warn("global percents fetch failed", "appID", appID)
	return nil
}

// refreshGlobalPercentsAsync re-fetches percentages in the background for a
// stale cache entry and emits "percents-updated" when a fresh value lands.
// No-op if a refresh for this appID is already running.
func (w *SteamWebAPI) refreshGlobalPercentsAsync(appID uint32) {
	percentCache.Lock()
	if percentCache.refreshing[appID] {
		percentCache.Unlock()
		return
	}
	percentCache.refreshing[appID] = true
	percentCache.Unlock()

	go func() {
		defer func() {
			percentCache.Lock()
			delete(percentCache.refreshing, appID)
			percentCache.Unlock()
		}()

		percents := w.fetchGlobalPercents(appID)
		if percents == nil {
			slog.Warn("background percent refresh failed", "appID", appID)
			return
		}

		percentCache.Lock()
		percentCache.entries[appID] = percentCacheEntry{data: percents, fetchedAt: time.Now()}
		percentCache.Unlock()
		settings.SavePercentEntry(appID, percents)

		wailsRuntime.EventsEmit(w.ctx, "percents-updated", map[string]any{
			"appId":    appID,
			"percents": percents,
		})
	}()
}

// fetchGlobalPercents performs a single fetch of global achievement percentages.
func (w *SteamWebAPI) fetchGlobalPercents(appID uint32) map[string]float32 {
	rawURL := fmt.Sprintf(
		"https://api.steampowered.com/ISteamUserStats/GetGlobalAchievementPercentagesForApp/v2/?gameid=%d",
		appID,
	)
	req, err := http.NewRequestWithContext(w.ctx, "GET", rawURL, nil)
	if err != nil {
		slog.Warn("global percent request create failed", "error", err)
		return nil
	}
	resp, err := w.client.Do(req)
	if err != nil {
		slog.Warn("global percent fetch failed", "appID", appID, "error", err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.Warn("global percent endpoint returned non-200", "appID", appID, "status", resp.StatusCode)
		return nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		slog.Warn("global percent read body failed", "appID", appID, "error", err)
		return nil
	}

	var result globalPercentResponse
	if err := json.Unmarshal(body, &result); err != nil {
		slog.Warn("global percent parse failed", "appID", appID, "error", err, "bodyPreview", string(body[:min(len(body), 200)]))
		return nil
	}

	percents := make(map[string]float32, len(result.AchievementPercentages.Achievements))
	for _, a := range result.AchievementPercentages.Achievements {
		percents[a.Name] = float32(a.Percent)
	}
	slog.Info("global percents fetched", "appID", appID, "count", len(percents))
	return percents
}

// ReleaseInfo holds release metadata for a game from the Steam Store API.
type ReleaseInfo struct {
	EarlyAccess bool
	ReleaseDate string // "YYYY-MM-DD", "unreleased", or ""
}

// GetReleaseInfo fetches release date and early access status via the Steam Store API.
// Returns ReleaseDate as "YYYY-MM-DD" for released games, "unreleased" for early access
// or coming-soon titles, and "" if the API call fails.
func (w *SteamWebAPI) GetReleaseInfo(appID uint32) ReleaseInfo {
	rawURL := fmt.Sprintf("https://store.steampowered.com/api/appdetails?appids=%d&filters=genres,release_date", appID)
	req, err := http.NewRequestWithContext(w.ctx, "GET", rawURL, nil)
	if err != nil {
		return ReleaseInfo{}
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return ReleaseInfo{}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ReleaseInfo{}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return ReleaseInfo{}
	}

	var result map[string]struct {
		Success bool `json:"success"`
		Data    struct {
			Genres []struct {
				ID string `json:"id"`
			} `json:"genres"`
			ReleaseDate struct {
				ComingSoon bool   `json:"coming_soon"`
				Date       string `json:"date"`
			} `json:"release_date"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return ReleaseInfo{}
	}

	appData, ok := result[strconv.FormatUint(uint64(appID), 10)]
	if !ok || !appData.Success {
		return ReleaseInfo{}
	}

	isEarlyAccess := appData.Data.ReleaseDate.ComingSoon
	if !isEarlyAccess {
		for _, genre := range appData.Data.Genres {
			if genre.ID == "70" { // Early Access genre ID
				isEarlyAccess = true
				break
			}
		}
	}

	if isEarlyAccess {
		return ReleaseInfo{EarlyAccess: true, ReleaseDate: "unreleased"}
	}

	releaseDate := parseSteamDate(appData.Data.ReleaseDate.Date)
	return ReleaseInfo{EarlyAccess: false, ReleaseDate: releaseDate}
}

// parseSteamDate converts Steam's display date (e.g. "Jan 15, 2020" or "15 Jan, 2020")
// into "YYYY-MM-DD" format. Returns the original string if parsing fails.
func parseSteamDate(raw string) string {
	if raw == "" {
		return "unreleased"
	}
	formats := []string{
		"Jan 2, 2006",
		"2 Jan, 2006",
		"January 2, 2006",
		"2 January, 2006",
		"Jan 2006",
		"January 2006",
	}
	for _, layout := range formats {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.Format("2006-01-02")
		}
	}
	// Unparseable but non-empty — treat as released with unknown date.
	// Store raw string so it's visible in the cache for debugging.
	slog.Debug("unparseable steam release date", "raw", raw)
	return raw
}

// CheckProfileVisibility checks if the user's Steam community profile is public.
// Returns "public", "private", or "friendsonly".
func (w *SteamWebAPI) CheckProfileVisibility() (string, error) {
	// Use the community profile XML — a well-known game (TF2) to test visibility
	url := fmt.Sprintf(
		"https://steamcommunity.com/profiles/%d/?xml=1",
		w.steamID,
	)

	req, err := http.NewRequestWithContext(w.ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("community request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("profile endpoint returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	// The profile XML has a <privacyState> element
	type profileResponse struct {
		XMLName      xml.Name `xml:"profile"`
		PrivacyState string   `xml:"privacyState"`
	}

	var result profileResponse
	if err := xml.Unmarshal(body, &result); err != nil {
		slog.Debug("profile xml parse failed", "error", err, "bodyLen", len(body))
		return "", fmt.Errorf("parse profile xml: %w", err)
	}

	if result.PrivacyState == "" {
		return privacyPublic, nil // No privacy element means public
	}
	return result.PrivacyState, nil
}
