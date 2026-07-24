# Stale-while-revalidate achievement percent cache

## Problem

Achievement unlock percentages sometimes show blank (0%) for several seconds after opening a game, even when a recent value is already cached on disk. Two gaps cause this:

1. The frontend waits a hardcoded 2 seconds before its first attempt to fetch percentages (`GameManager.svelte`), regardless of whether a cached value is available immediately.
2. The disk cache (`percents.json`, `internal/settings/percent_cache.go`) treats any entry older than 24 hours as a total miss and discards it, forcing a live network fetch (and a blank UI) before anything can be shown — instead of serving the last-known value and refreshing quietly in the background.

## Goals

- Reopening a game whose percentages have been fetched before shows them immediately, even if the cached value is more than 24 hours old.
- Stale values are refreshed in the background without blocking the UI, and the display updates in place when the refresh completes.
- A game whose percentages have never been fetched still behaves as today (blocks briefly on a live fetch).
- No new persistence layer — reuse and extend the existing memory/disk cache in `webapi.go` / `percent_cache.go`.

## Non-goals

- No changes to the Steam SDK-based percent path (`achievement_service.go`) — this only affects the Web API path.
- No automated test suite is being introduced; this project verifies manually via `make dev` (per `CLAUDE.md`).

## Design

### Backend: `internal/settings/percent_cache.go`

`LoadPercentEntry(appID uint32)` changes signature from `(map[string]float32, bool)` to:

```go
func LoadPercentEntry(appID uint32) (data map[string]float32, stale bool, found bool)
```

- `found` is `true` whenever an entry exists on disk, regardless of age.
- `stale` is `true` when the entry is older than the existing 24h TTL (`percentCacheDiskTTL`).
- Entries are never deleted for being old — only overwritten by a newer successful `SavePercentEntry` call.

### Backend: `internal/services/webapi.go`

`GetGlobalPercents(appID uint32)`:

1. Memory cache check (15 min TTL) — unchanged, fast path.
2. Disk cache check via the updated `LoadPercentEntry`:
   - If `found`, return `data` immediately (refresh the in-memory cache entry with it).
   - If also `stale`, and no refresh is already in flight for this `appID`, spawn a goroutine that calls the existing `fetchGlobalPercents`, and on success:
     - updates the in-memory cache entry,
     - calls `settings.SavePercentEntry`,
     - emits a `percents-updated` Wails event via `wailsRuntime.EventsEmit(w.ctx, "percents-updated", map[string]any{"appId": appID, "percents": percents})` — `w.ctx` is the same app context `app.go` already uses for its other `EventsEmit` calls (e.g. `scan-progress`).
     - On failure, logs a warning and emits nothing; the disk entry's `fetchedAt` is left untouched so the next open retries naturally.
3. If not `found` at all, behavior is unchanged: block and call `fetchGlobalPercents` synchronously.

**In-flight de-dupe:** a small `map[uint32]bool` (guarded by the existing `percentCache` mutex) tracks appIDs with a refresh goroutine currently running. A stale hit for an appID already in flight is a no-op — it just returns the stale data without spawning a second refresh. The flag is cleared when the goroutine finishes (success or failure).

### Frontend: `frontend/src/lib/pages/GameManager.svelte`

- Remove `startPercentPolling`'s hardcoded 2-second initial delay and the "poll while any achievement has `percent === 0`" loop.
- Call `FetchGlobalPercents(appId)` immediately, in parallel with `LoadAchievementsFromSchema` / `LoadAchievements` (not gated behind achievement load completing). Merge the result into the achievement list once both resolve, same merge logic as today (`achievement.percent === 0 && percents[id]`).
- On failure, retry twice with a 2s/4s backoff (down from the current 6-attempt, up-to-64s backoff) — this now only covers genuine fetch failures, not "waiting for cache."
- Add one `EventsOn('percents-updated', ...)` listener (set up once per component lifecycle, torn down `onDestroy`), filtered to the currently selected `appId`. On a matching event, patch the returned percents into the `achievements` store the same way `cachePercents` already does.

### Frontend: `frontend/src/lib/stores/achievements.ts`

No changes — the existing session-level `percentCache` Map and `applyCachedPercents`/`cachePercents` helpers continue to work as an additional fast path across game switches within the same app session.

## Error handling

- Live-fetch failure on a true first-time miss: unchanged — `percent` stays 0, a couple of short frontend retries, no artificial delay before the first attempt.
- Background refresh failure on a stale hit: logged as a warning server-side, no event emitted, no user-facing error. The stale value keeps displaying; the next time the game is opened, the same stale-hit-and-refresh path retries.
- In-flight de-dupe prevents duplicate concurrent refreshes against Steam's endpoint when a user switches between games with stale caches in quick succession.

## Testing (manual, per project convention)

- Open a game never fetched before → unchanged behavior: brief block, then live percentages appear.
- Open a game with a fresh (<24h) `percents.json` entry → percentages appear instantly, no 2s wait.
- Manually age an entry in `percents.json` past 24h → old numbers display immediately on open; `steamforge.log` shows a background refetch; UI patches in updated numbers shortly after.
- Rapidly switch between two games with stale entries → only one refresh goroutine/log line per appID, no duplicate fetches.
