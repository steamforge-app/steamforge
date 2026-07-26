# Cached playtime + main-story time on library cards

## Problem

Playtime and HowLongToBeat's main-story estimate are currently only visible after opening a game (`GameManager.svelte`). Both are already fetched and cached per-game the first time you visit — showing the cached values back on the library grid/list would surface information you've already paid the cost to fetch, without opening each game again. The AppID label on the grid card takes up space for information nobody's asked to see.

## Goals

- Show cached playtime and cached HLTB main-story time on both the grid card and list row, for any game where that data has already been fetched at least once.
- Never trigger a new HLTB network search from the library view — only serve what's already in `hltb.json`.
- Remove the `AppID: {appId}` label from the grid card.
- Keep the list view's existing AppID column as-is.

## Non-goals

- No changes to how/when HLTB or playtime get fetched in the first place (`GameManager.svelte`'s existing per-game effects are untouched).
- No mainExtra/completionist display on cards — only the main-story estimate, per the ask.
- No automated test suite — manual verification via `make dev`, per `CLAUDE.md`.

## Design

### Backend: `app.go`

Two new Wails-bound methods, both pure reads with no network calls:

```go
// GetAllPlaytimes returns hours played for every app with recorded playtime,
// via a single parse of localconfig.vdf (steam.ScanPlaytimeHours).
func (a *App) GetAllPlaytimes() map[uint32]float64

// GetAllCachedHLTB returns every HLTB entry already cached to disk.
// Never performs a live HLTB search — that only happens via the existing
// GetHLTBTimes, called when a game is actually opened.
func (a *App) GetAllCachedHLTB() map[uint32]settings.HLTBEntry
```

`GetAllPlaytimes` requires a connected Steam client (needs a SteamID), same guard as `GetToPlayList`'s neighbors. `GetAllCachedHLTB` needs no client — `settings.LoadHLTBCache()` is already client-independent.

### Frontend: `frontend/src/lib/stores/games.ts`

Two new stores, populated the same way `achievementCounts` already is:

```ts
export const playtimes = writable<Record<string, number>>({});
export const hltbCache = writable<Record<string, { main: number; mainExtra: number; completionist: number }>>({});
```

Loaded via `GetAllPlaytimes()`/`GetAllCachedHLTB()` (converting numeric appID keys to strings, matching `achievementCounts`' existing convention) at the same three points `GamePicker.svelte` already loads `achievementCounts`/`toPlayList`: `handleSuccessfulConnection`, `onMount` (already connected), `handleAccountChanged`.

### Grid card: `GameCard.svelte`

The title-footer line currently reads:
```
AppID: {game.appId}   {formatLastPlayed(game.lastPlayed)}
```
Becomes:
```
{formatLastPlayed(game.lastPlayed)}   Played {hours}h   Main {hours}h
```
- `Played {hours}h`: sky-blue (`text-sky-400`), shown only when `playtimes[appId]` exists and is > 0. Same color as the per-game view's playtime display.
- `Main {hours}h`: dim white (`text-white/35` label + default text color for the value, matching HLTB's existing per-game styling), shown only when `hltbCache[appId]?.main` exists and is > 0.
- Both use `Math.round()`, matching the per-game view's formatting.
- Neither renders anything (no placeholder, no loading state) when the value isn't cached.

### List row: `GameListRow.svelte`

One new column added between the existing last-played and achievement-count columns, matching the row's existing fixed-width (`w-16`) convention:
```
{playtimeHours}h / {hltbMain}h
```
with a `title` tooltip: `"Played {x}h · Main story {y}h"`. If only one of the two values is cached, show just that one (e.g. `10h` alone, no dangling `/`). If neither is cached, the column renders empty (no width collapse — keeps row alignment stable across rows where some games have data and others don't).

## Error handling

Both new backend reads degrade the same way `achievementCounts` already does on the frontend: a fetch failure leaves the store at its previous/empty value, logged server-side, no user-facing error — this is enrichment data, not load-bearing.

## Testing (manual, per project convention)

- Open two or three games first (populating their HLTB/playtime caches via existing per-game behavior), then return to the library.
- Grid view: confirm `Played Xh` and `Main Xh` show on those cards' footers in the right colors, AppID is gone from all cards, and games never opened show neither value.
- List view: confirm the new column shows combined values for visited games, AppID column is untouched, and alignment stays consistent across rows with partial/no data.
- Restart the app: confirm cached values still show without reopening those games (backed by the existing disk caches).
