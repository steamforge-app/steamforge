package main

import (
	"log/slog"
	"time"

	"steamforge/internal/services"
	"steamforge/internal/settings"
)

// hltbNegativeCacheTTL controls how long a confirmed "no HLTB match" result
// is trusted before it's worth re-checking live (HLTB's database grows over time).
// This applies regardless of install state — a miss is about HLTB's own
// database, not the local game's version.
const hltbNegativeCacheTTL = 30 * 24 * time.Hour

// hltbNotInstalledTTL is the fallback freshness window for a found entry when
// the game isn't installed, so there's no local update-time signal to compare
// against. Installed games use GameUpdatedAt instead (see GetHLTBTimes).
const hltbNotInstalledTTL = 24 * time.Hour

// getHLTBService returns a process-lifetime HLTBService singleton so its
// auth token (5-minute TTL) and search mutex are actually shared across calls,
// instead of being discarded and refetched on every lookup.
func (a *App) getHLTBService() *services.HLTBService {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.hltbService == nil {
		a.hltbService = services.NewHLTBService(a.ctx)
	}
	return a.hltbService
}

// GetHLTBTimes returns completion time estimates for a game from HowLongToBeat.
// Returns cached data immediately if it's still fresh; otherwise fetches live
// and caches the result. Freshness for a found entry is judged by the game's
// local appmanifest update time when installed (re-check only if the game's
// actually been updated since caching — DLC/patches can change completion
// times), or a fixed TTL when not installed (no local signal available).
func (a *App) GetHLTBTimes(appID uint32, gameName string) (*services.HLTBTimes, error) {
	a.mu.RLock()
	currentUpdatedAt, isInstalled := a.installedUpdateTimes[appID]
	a.mu.RUnlock()

	if entry, ok := settings.GetHLTBEntry(appID); ok {
		found := entry.Main > 0 || entry.MainExtra > 0 || entry.Completionist > 0
		var fresh bool
		switch {
		case found && isInstalled:
			fresh = entry.GameUpdatedAt != 0 && currentUpdatedAt <= uint32(entry.GameUpdatedAt)
		case found:
			fresh = time.Since(time.Unix(entry.CheckedAt, 0)) < hltbNotInstalledTTL
		default:
			fresh = time.Since(time.Unix(entry.CheckedAt, 0)) < hltbNegativeCacheTTL
		}
		if fresh {
			if found {
				return &services.HLTBTimes{
					Main:          entry.Main,
					MainExtra:     entry.MainExtra,
					Completionist: entry.Completionist,
				}, nil
			}
			return nil, nil
		}
	}

	times, err := a.getHLTBService().Search(gameName)
	if err != nil {
		slog.Warn("hltb search failed", "appID", appID, "game", gameName, "error", err)
		return nil, err
	}
	if times == nil {
		settings.SaveHLTBEntry(appID, settings.HLTBEntry{CheckedAt: time.Now().Unix(), GameUpdatedAt: int64(currentUpdatedAt)})
		return nil, nil
	}

	settings.SaveHLTBEntry(appID, settings.HLTBEntry{
		Main:          times.Main,
		MainExtra:     times.MainExtra,
		Completionist: times.Completionist,
		GameUpdatedAt: int64(currentUpdatedAt),
		CheckedAt:     time.Now().Unix(),
	})
	return times, nil
}

// GetAllCachedHLTB returns every HLTB entry already cached to disk.
// Never performs a live HLTB search — that only happens via GetHLTBTimes,
// called when a game is actually opened.
func (a *App) GetAllCachedHLTB() map[uint32]settings.HLTBEntry {
	return settings.LoadHLTBCache()
}
