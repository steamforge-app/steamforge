# Lazy, viewport-triggered HLTB fetching for the library view

## Problem

HowLongToBeat completion times only populate on a game's card once you've opened that specific game at least once (`GameManager.svelte` triggers the live fetch). Games you've never opened show no `Main`/`100%` time on their cards, and there's no way to backfill that data without visiting every game individually.

A naive fix — fetch HLTB for the whole library on connect — would fire one live network request per uncached game (HLTB has no bulk-lookup endpoint), potentially hundreds at once for a large library. That's the wrong shape: it hammers a third-party API for games the user may never even look at.

## Goals

- Backfill HLTB data for games the user hasn't opened, without a bulk fetch-everything job.
- Only ever fetch for games actually visible on screen, at the pace the user scrolls.
- Never fire more than one live HLTB request at a time, regardless of how many cards become visible at once (fast scroll, window resize, etc.).
- Stop re-querying HLTB for a game that's confirmed to have no entry, but allow it to be re-checked eventually (HLTB's database grows over time).

## Non-goals

- No changes to playtime fetching — it's already a single bulk parse of `localconfig.vdf` via `GetAllPlaytimes`, no per-game network cost, not part of this problem.
- No manual "refresh HLTB" button or UI for negative-cached entries — a 30-day TTL on "not found" results handles staleness automatically.
- No progress indicator/toast for this backfill — it's meant to be invisible background behavior, not a batch job the user watches complete.
- No change to `GameManager.svelte`'s existing per-game HLTB fetch effect (it already writes into the shared `hltbCache` store) — only the new lazy grid/list triggering is added.

## Design

### Why viewport-triggered works here

`GameGrid.svelte` already wraps both the grid and list views in `VirtualGrid.svelte`, which only mounts the `GameCard`/`GameListRow` instances currently in view (plus a small `overscan` buffer — currently 3 rows). Instances for games scrolled out of range are destroyed; scrolling back in creates fresh instances. This means "run some logic once when this card mounts" is already equivalent to "run it once each time this game becomes visible" — no new visibility-tracking code is needed on the frontend.

### Backend: `internal/services/hltb.go`

Add a mutex to `HLTBService` so only one live HLTB exchange (token refresh + search) runs at a time:

```go
type HLTBService struct {
	client   *http.Client
	ctx      context.Context
	token    *hltbToken
	searchMu sync.Mutex
}

func (h *HLTBService) Search(name string) (*HLTBTimes, error) {
	h.searchMu.Lock()
	defer h.searchMu.Unlock()
	// ...existing body unchanged...
}
```

This only serializes calls made through the *same* `HLTBService` instance — see the `app.go` change below for why that now matters.

### Backend: `app.go` — reuse a single `HLTBService` instance

Today, `GetHLTBTimes` does `svc := services.NewHLTBService(a.ctx)` on every call, so the 5-minute auth token cache (`hltbTokenTTL`) is thrown away and refetched on every single lookup — each call becomes two HTTP round-trips (token init + search) instead of one. That's wasteful even at today's low call volume, and directly counterproductive once card-mount events can trigger many lookups per scroll session.

Fix: hold one `*services.HLTBService` on `App`, created lazily on first use:

```go
func (a *App) getHLTBService() *services.HLTBService {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.hltbService == nil {
		a.hltbService = services.NewHLTBService(a.ctx)
	}
	return a.hltbService
}
```

HLTB lookups don't depend on Steam connection state, so `a.hltbService` is never torn down on disconnect/reconnect (unlike `a.steamClient`) — it lives for the process lifetime once created. `GetHLTBTimes` calls `a.getHLTBService()` instead of constructing a fresh service. This gives two effects for free: the auth token is actually reused as designed, and the new `searchMu` serializes every live HLTB call app-wide, since they now all go through the same instance.

### Backend: negative-result caching with a TTL

`internal/settings/hltb_cache.go`'s `HLTBEntry` gets one new field:

```go
type HLTBEntry struct {
	Main          float32 `json:"main"`
	MainExtra     float32 `json:"mainExtra"`
	Completionist float32 `json:"completionist"`
	CheckedAt     int64   `json:"checkedAt"` // unix seconds; only consulted when Main/MainExtra/Completionist are all zero
}
```

Backward compatible: every entry saved by the old code represents an actual match (the old code never saved on a miss), so old entries always have a nonzero `Main` and are returned as before regardless of `CheckedAt`. `CheckedAt` is only consulted when a stored entry's times are all zero — i.e. an explicit "not found" record — to decide whether it's worth re-checking. (Accepted edge case: a real HLTB entry whose Main/MainExtra/Completionist are all genuinely zero would be indistinguishable from "not found" and re-checked every 30 days instead of cached permanently — harmless, and not a pattern HLTB's real data exhibits.)

`GetHLTBTimes` in `app.go` changes to:

```go
const hltbNegativeCacheTTL = 30 * 24 * time.Hour

func (a *App) GetHLTBTimes(appID uint32, gameName string) (*services.HLTBTimes, error) {
	if entry, ok := settings.GetHLTBEntry(appID); ok {
		if entry.Main > 0 || entry.MainExtra > 0 || entry.Completionist > 0 {
			return &services.HLTBTimes{Main: entry.Main, MainExtra: entry.MainExtra, Completionist: entry.Completionist}, nil
		}
		if time.Since(time.Unix(entry.CheckedAt, 0)) < hltbNegativeCacheTTL {
			return nil, nil
		}
		// stale negative entry — fall through and re-check live
	}

	svc := a.getHLTBService()
	times, err := svc.Search(gameName)
	if err != nil {
		slog.Warn("hltb search failed", "appID", appID, "game", gameName, "error", err)
		return nil, err // transient failure — not cached, retried next time this card is visible
	}

	if times == nil {
		settings.SaveHLTBEntry(appID, settings.HLTBEntry{CheckedAt: time.Now().Unix()})
		return nil, nil
	}

	settings.SaveHLTBEntry(appID, settings.HLTBEntry{
		Main:          times.Main,
		MainExtra:     times.MainExtra,
		Completionist: times.Completionist,
		CheckedAt:     time.Now().Unix(),
	})
	return times, nil
}
```

No change to the method's signature or the generated Wails binding — every existing frontend caller keeps working unchanged.

### Frontend: shared lazy-fetch helper

New file `frontend/src/lib/utils/hltb-lazy-fetch.ts`:

```ts
import { GetHLTBTimes } from '../../../wailsjs/go/main/App';
import { hltbCache } from '../stores/games';
import { get } from 'svelte/store';

const inFlight = new Set<number>();

export function fetchHLTBIfMissing(appId: number, gameName: string): void {
  if (appId <= 0 || !gameName) return;
  if (get(hltbCache)[String(appId)] || inFlight.has(appId)) return;
  inFlight.add(appId);
  GetHLTBTimes(appId, gameName)
    .then(result => {
      if (result) hltbCache.update(cache => ({ ...cache, [String(appId)]: result }));
    })
    .catch(() => { /* transient failure — retried next time this card is visible */ })
    .finally(() => inFlight.delete(appId));
}
```

The `inFlight` set is a lightweight guard against the same card re-mounting and firing a second request before the first resolves; it does not replace the backend's `searchMu` serialization, which is the real protection against parallel requests across *different* games.

### Frontend: trigger on card mount

`GameCard.svelte` and `GameListRow.svelte` each add:

```ts
import { fetchHLTBIfMissing } from '../utils/hltb-lazy-fetch';

$effect(() => {
  fetchHLTBIfMissing(game.appId, game.name);
});
```

Because `VirtualGrid` destroys/recreates these components as games leave/enter the visible range, this effect firing on mount is exactly "check this game's HLTB status once each time it scrolls into view." Games already in `hltbCache` (positive or negative) no-op immediately; only genuinely-unchecked games reach the backend, and the backend serializes those regardless of how many cards mounted at once.

## Error handling

- Transient failures (network error, HLTB down) are not cached and are silently retried the next time the card becomes visible — matching the existing `GameManager.svelte` HLTB effect's error handling (log and move on, no user-facing error).
- A confirmed "no match" is cached for 30 days to avoid repeat queries, then naturally re-checked.
- Nothing here changes existing UI error states — this is enrichment data, same as the rest of the cached-cards feature.

## Testing (manual, per project convention)

- Scroll through a chunk of never-opened games in both grid and list view; confirm `Main`/`100%` populate progressively as cards scroll into view, not all at once.
- Watch `steamforge.log` while scrolling quickly back and forth over the same set of games — confirm searches don't fire more than once per game, and never overlap in time (one at a time).
- Confirm a game with no HLTB match gets cached (check `hltb.json` for a zero-time entry with a `checkedAt` timestamp) and isn't re-queried on subsequent visits within 30 days.
- Restart the app — confirm both positive and negative cached entries persist without re-fetching.
