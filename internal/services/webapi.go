package services

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"steamforge/internal/models"
)

// percentCache caches global achievement percentages per appID.
var percentCache struct {
	sync.RWMutex
	entries map[uint32]percentCacheEntry
}

type percentCacheEntry struct {
	data      map[string]float32
	fetchedAt time.Time
}

const percentCacheTTL = 15 * time.Minute

func init() {
	percentCache.entries = make(map[uint32]percentCacheEntry)
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
	if result.PrivacyState != "" && result.PrivacyState != "public" {
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
	if result.PrivacyState != "" && result.PrivacyState != "public" {
		return nil, fmt.Errorf("profile is %s", result.PrivacyState)
	}

	percents := w.GetGlobalPercents(appID)
	slog.Info("global percents fetched", "appID", appID, "count", len(percents))

	matched := 0
	achievements := make([]models.Achievement, 0, len(result.Achievements.Items))
	for i, a := range result.Achievements.Items {
		if a.APIName == "" {
			continue
		}
		ach := models.Achievement{
			ID:          a.APIName,
			Name:        a.Name,
			Description: a.Description,
			IconURL:     a.IconClosed,
			IconGrayURL: a.IconOpen,
			IsAchieved:  a.Closed == 1,
			UnlockTime:  uint32(a.UnlockTimestamp),
		}
		if p, ok := percents[a.APIName]; ok {
			ach.Percent = p
			matched++
			if i < 3 {
				slog.Info("percent match sample", "apiName", a.APIName, "percent", p)
			}
		} else if i < 3 {
			// Log first few misses to help debug key mismatches
			slog.Warn("percent miss sample", "apiName", a.APIName)
		}
		achievements = append(achievements, ach)
	}
	slog.Info("achievements built", "appID", appID, "total", len(achievements), "withPercent", matched)
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
// Uses the public Steam Web API — no API key needed.
// Results are cached for 15 minutes. Retries once on failure.
func (w *SteamWebAPI) GetGlobalPercents(appID uint32) map[string]float32 {
	// Check cache first
	percentCache.RLock()
	if entry, ok := percentCache.entries[appID]; ok && time.Since(entry.fetchedAt) < percentCacheTTL {
		percentCache.RUnlock()
		slog.Info("global percents cache hit", "appID", appID, "count", len(entry.data), "age", time.Since(entry.fetchedAt).Round(time.Second))
		return entry.data
	}
	percentCache.RUnlock()
	slog.Info("global percents cache miss", "appID", appID)

	// Try up to 2 times with a delay between attempts
	for attempt := 1; attempt <= 2; attempt++ {
		if attempt > 1 {
			slog.Info("retrying global percents", "appID", appID, "attempt", attempt)
			time.Sleep(2 * time.Second)
		}

		percents := w.fetchGlobalPercents(appID)
		if percents != nil {
			// Store in cache
			percentCache.Lock()
			percentCache.entries[appID] = percentCacheEntry{data: percents, fetchedAt: time.Now()}
			percentCache.Unlock()
			return percents
		}
	}

	slog.Warn("global percents failed after retries", "appID", appID)
	return nil
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

// CheckEarlyAccess checks if a game is in Early Access via the Steam Store API.
// Detects both the "Early Access" genre tag (ID 70) and unreleased games (coming_soon).
func (w *SteamWebAPI) CheckEarlyAccess(appID uint32) bool {
	rawURL := fmt.Sprintf("https://store.steampowered.com/api/appdetails?appids=%d&filters=genres,release_date", appID)
	req, err := http.NewRequestWithContext(w.ctx, "GET", rawURL, nil)
	if err != nil {
		return false
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return false
	}

	var result map[string]struct {
		Success bool `json:"success"`
		Data    struct {
			Genres []struct {
				ID string `json:"id"`
			} `json:"genres"`
			ReleaseDate struct {
				ComingSoon bool `json:"coming_soon"`
			} `json:"release_date"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return false
	}

	appData, ok := result[fmt.Sprintf("%d", appID)]
	if !ok || !appData.Success {
		return false
	}
	for _, genre := range appData.Data.Genres {
		if genre.ID == "70" { // Early Access genre ID
			return true
		}
	}
	return appData.Data.ReleaseDate.ComingSoon
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
		return "public", nil // No privacy element means public
	}
	return result.PrivacyState, nil
}
