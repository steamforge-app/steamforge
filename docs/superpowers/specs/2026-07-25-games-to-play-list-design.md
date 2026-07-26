# "Games to play" backlog list

## Problem

There's no way to mark a game as "want to play/finish this" and come back to that list later. Steam has its own Collections feature, but that data lives in an undocumented, semi-obfuscated JSON blob inside `localconfig.vdf`'s `WebStorage` section that has changed shape across Steam client versions without notice — parsing it reliably would mean reverse-engineering a format Valve doesn't document and could silently break on a future update. A self-contained list, fully owned by SteamForge, avoids that risk entirely.

## Goals

- Mark/unmark any game as "to play" from the game grid, without opening it.
- Filter the library down to just that list, using the existing filter dropdown.
- Persist across restarts, per Steam account (matching how settings/achievement caches are already scoped).
- Visible at a glance in the grid (no hovering required) which games are on the list.

## Non-goals

- No import of Steam's own Collections/categories.
- No multiple named categories — one flat list only.
- No automated test suite — manual verification via `make dev`, per `CLAUDE.md`.

## Design

### Backend: `internal/settings/toplay.go` (new file)

Mirrors the existing `achievement_cache.go` / `game_cache.go` pattern (per-user JSON file, lazy-loaded in-memory map guarded by a mutex), but deliberately kept as its own file and its own JSON (`to_play.json` in the per-user config dir) rather than a field on `CachedGame`. Reason: `GameService.mergeWithCache` (`internal/services/game_service.go`) rebuilds every `CachedGame` entry from scratch on each library refresh (`merged[k] = settings.CachedGame{Name: g.Name, LogoURL: g.LogoURL}`) — any extra field stored there would be silently wiped on the next scan unless that merge is also changed. A standalone store sidesteps this entirely.

```go
package settings

func LoadToPlayList() map[uint32]bool   // returns a copy, same convention as LoadAchievementCache
func SetToPlay(appID uint32, want bool) // adds/removes an entry, writes to_play.json immediately
```

No debounce on write (unlike `achievement_cache.go`'s 500ms flush timer) — toggles are rare, deliberate user clicks, not a scan writing hundreds of entries in a burst.

### Backend: `app.go`

Two new Wails-bound methods. Neither needs a Steam client — it's a pure local list, so no `a.mu`/`steamClient` gate:

```go
func (a *App) GetToPlayList() []uint32
func (a *App) SetToPlay(appID uint32, want bool) error
```

### Frontend: `frontend/src/lib/stores/toplay.ts` (new file)

```ts
export const toPlayList = writable<Set<number>>(new Set());
export async function loadToPlayList(): Promise<void>   // calls GetToPlayList, populates the store
export async function toggleToPlay(appId: number): Promise<void>  // optimistic set/revert, calls SetToPlay
```

`loadToPlayList()` is called from the same place `achievementCounts` is first populated — `GamePicker.svelte`'s `onMount` and `handleSuccessfulConnection` (`GamePicker.svelte:26-38`, `:56-70`).

### Frontend: filter integration

- `stores/app.ts`: `GameFilter` gains a new value: `'all' | 'incomplete' | 'perfected' | 'none' | 'toPlay'`.
- `stores/games.ts`: `filteredGames`'s filter block gets one more branch: `if ($gameFilter === 'toPlay') return $toPlayList.has(game.appId);` (added to the derived store's dependency list alongside `games`/`searchQuery`/`settings`/`achievementCounts`/`gameFilter`).
- `GamePicker.svelte`: `filterOptions` gains `{ value: 'toPlay', label: 'Games to Play' }`, and `filterCounts` gains a matching count, following the existing pattern for `all`/`incomplete`/`perfected`/`none`.

### Frontend: card/list toggle UI

`GameCard.svelte` already has a hover-reveal corner-button overlay (store-page icon top-left, play/install icon top-right, both `bg-black/50` circular buttons, `opacity-0 group-hover:opacity-100`). A new star-shaped button goes in the bottom-left corner (the one free slot):

- On the list: filled amber star, **always visible** (not just on hover) — mirrors how the "fully completed" amber border is always-on regardless of hover state, so the backlog is scannable across the whole grid at a glance.
- Not on the list: outline star, hover-only, same treatment as the existing store/play buttons.
- Click calls `toggleToPlay(game.appId)`, `stopPropagation()`'d so it doesn't also trigger the card's navigate-to-game click handler (same pattern as `handlePlay` in `GameCard.svelte:43-46`).

`GameListRow.svelte` (the list-view equivalent) gets the same button, styled to fit its layout — not yet inspected in detail; the implementation plan should confirm its existing action-icon conventions before adding this.

## Error handling

- `toggleToPlay` optimistically flips the local `Set`, then calls `SetToPlay`. On failure, revert the flip and show an error toast (`addToast(..., 'error')`) — same optimistic-then-revert pattern already used for achievement toggling in `GameManager.svelte`.
- `loadToPlayList` failure: log and leave the set empty (nothing marked) rather than blocking the game grid from loading — the list is a convenience layer, not load-bearing for core functionality.

## Testing (manual, per project convention)

- Toggle a game on from the grid → star turns filled/amber immediately, no hover needed to see it afterward.
- Switch to the "Games to Play" filter → only starred games show, count in the dropdown matches.
- Restart the app → starred games are still starred, filter still works.
- Toggle off → star reverts to outline-on-hover-only, game drops out of the filtered view.
- Toggle from list view → stays in sync with grid view (same underlying store).
