# Cached Playtime + HLTB on Library Cards Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show already-cached playtime and HLTB main-story time on the game library's grid cards and list rows, without triggering any new network fetches, and remove the now-redundant AppID label from the grid card.

**Architecture:** Two new read-only Wails-bound backend methods expose the existing on-disk caches (`localconfig.vdf` via `steam.ScanPlaytimeHours`, and `hltb.json` via `settings.LoadHLTBCache`) in bulk. The frontend loads both into two new plain `writable` stores in `games.ts`, populated at the same three lifecycle points that already populate `achievementCounts`. `GameCard.svelte` and `GameListRow.svelte` read those stores per-card, matching the existing `achievementCounts` lookup pattern (`$store[String(game.appId)]`).

**Tech Stack:** Go 1.23 (Wails v2 backend), Svelte 5 + TypeScript (frontend), no automated test suite — verification via `go build`/`go vet -tags webkit2_41 ./...`, `cd frontend && npm run check`, and manual testing via `make dev`.

## Global Constraints

- `-tags webkit2_41` required for all Go build/vet commands on Linux (project `CLAUDE.md`).
- No automated test suite exists in this project — verify each task via build/vet/svelte-check, per `CLAUDE.md`.
- Never trigger a live HLTB search from the library view — `GetAllCachedHLTB` must only read `hltb.json`, never call `services.NewHLTBService(...).Search()`.
- `GetAllPlaytimes` requires a connected Steam client (needs a SteamID) and must return `errNotConnected` when `a.steamClient` is nil, matching the existing guard pattern in `GetPlaytime` (`app.go:521-534`).
- `GetAllCachedHLTB` needs no Steam client — `settings.LoadHLTBCache()` is already client-independent.
- Remove `AppID: {appId}` from the grid card footer only. The list row's AppID column (`GameListRow.svelte:109`) stays untouched.

---

### Task 1: Backend bulk-read methods

**Files:**
- Modify: `app.go` (add after `GetHLTBTimes`, i.e. after line 448)

**Interfaces:**
- Consumes: `a.steamClient *steam.Client` (existing field), `errNotConnected` (existing package var, `app.go:33`), `steam.ScanPlaytimeHours(steamID uint64) map[uint32]float64` (existing, `internal/steam/appmanifest.go:228`), `settings.LoadHLTBCache() map[uint32]settings.HLTBEntry` (existing, `internal/settings/hltb_cache.go:33`).
- Produces: `func (a *App) GetAllPlaytimes() (map[uint32]float64, error)`, `func (a *App) GetAllCachedHLTB() map[uint32]settings.HLTBEntry` — both consumed by Task 2's frontend bindings.

- [ ] **Step 1: Add `GetAllPlaytimes` to `app.go`**

Insert immediately after `GetHLTBTimes` (after line 448, before `func (a *App) GetSettings()`):

```go
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
```

- [ ] **Step 2: Verify it builds**

Run: `go build -tags webkit2_41 ./...`
Expected: no errors. (`steam` and `settings` packages are already imported in `app.go` — no new imports needed.)

- [ ] **Step 3: Regenerate Wails bindings**

Run: `wails generate module -tags webkit2_41`
Expected: `frontend/wailsjs/go/main/App.js` and `App.d.ts` now declare `GetAllPlaytimes` and `GetAllCachedHLTB`. Confirm with:
`grep -n "GetAllPlaytimes\|GetAllCachedHLTB" frontend/wailsjs/go/main/App.d.ts`
Expected: both names present.

- [ ] **Step 4: Clean up incidental file-mode churn**

`wails generate module` may leave mode-only (644→755) changes on `frontend/wailsjs/runtime/{package.json,runtime.d.ts,runtime.js}` with zero content diff. If `git status` shows those three files as modified with no content change (`git diff --stat` shows 0 insertions/deletions), discard them:
`git checkout -- frontend/wailsjs/runtime/package.json frontend/wailsjs/runtime/runtime.d.ts frontend/wailsjs/runtime/runtime.js`

- [ ] **Step 5: Run go vet**

Run: `go vet -tags webkit2_41 ./...`
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add app.go frontend/wailsjs/go/main/App.js frontend/wailsjs/go/main/App.d.ts
git commit -m "Add bulk playtime and cached-HLTB read endpoints"
```

---

### Task 2: Frontend stores for playtime and HLTB cache

**Files:**
- Modify: `frontend/src/lib/stores/games.ts`

**Interfaces:**
- Consumes: `GetAllPlaytimes(): Promise<Record<string, number>>`, `GetAllCachedHLTB(): Promise<Record<string, HLTBEntry>>` from `frontend/wailsjs/go/main/App` (Task 1). Wails marshals Go `map[uint32]T` to a JS object with string keys — same convention already used by `GetAchievementCounts` → `achievementCounts`.
- Produces: `export const playtimes: Writable<Record<string, number>>`, `export const hltbCache: Writable<Record<string, { main: number; mainExtra: number; completionist: number }>>` — consumed by Task 3 (loaders), Task 4 (`GameCard.svelte`), Task 5 (`GameListRow.svelte`).

- [ ] **Step 1: Add the two new stores**

In `frontend/src/lib/stores/games.ts`, after the existing `achievementCounts` declaration (currently the line `export const achievementCounts = writable<Record<string, AchievementCount>>({});`), add:

```ts
export interface HLTBCacheEntry {
  main: number;
  mainExtra: number;
  completionist: number;
}

export const playtimes = writable<Record<string, number>>({});
export const hltbCache = writable<Record<string, HLTBCacheEntry>>({});
```

- [ ] **Step 2: Verify svelte-check passes**

Run: `cd frontend && npm run check`
Expected: no new errors (the stores aren't consumed yet, so this just confirms the file still parses/type-checks).

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/stores/games.ts
git commit -m "Add playtimes and hltbCache stores"
```

---

### Task 3: Wire loaders into GamePicker's lifecycle points

**Files:**
- Modify: `frontend/src/lib/pages/GamePicker.svelte`

**Interfaces:**
- Consumes: `playtimes`, `hltbCache` (Task 2, from `../stores/games`), `GetAllPlaytimes`, `GetAllCachedHLTB` (Task 1, from `../../../wailsjs/go/main/App`).
- Produces: nothing new consumed by later tasks — this task only makes the stores non-empty at runtime so Task 4/5's card lookups have data.

- [ ] **Step 1: Add imports**

In `frontend/src/lib/pages/GamePicker.svelte`, change line 3 from:

```ts
import { games, gamesLoading, searchQuery, achievementCounts } from '../stores/games';
```

to:

```ts
import { games, gamesLoading, searchQuery, achievementCounts, playtimes, hltbCache } from '../stores/games';
```

Change line 14 from:

```ts
import { ConnectSteam, FetchGames, GetAchievementCounts, ScanAchievementCounts, GetPersonaName } from '../../../wailsjs/go/main/App';
```

to:

```ts
import { ConnectSteam, FetchGames, GetAchievementCounts, ScanAchievementCounts, GetPersonaName, GetAllPlaytimes, GetAllCachedHLTB } from '../../../wailsjs/go/main/App';
```

- [ ] **Step 2: Add a shared loader function**

Add this function near `startRetry` (after the `stopRetry` function, before `handleSuccessfulConnection`):

```ts
async function loadCardEnrichment() {
  try {
    const playtimeMap = await GetAllPlaytimes();
    playtimes.set(playtimeMap || {});
  } catch { /* cache not critical */ }
  try {
    const hltbMap = await GetAllCachedHLTB();
    hltbCache.set(hltbMap || {});
  } catch { /* cache not critical */ }
}
```

This mirrors the existing `try { ... } catch { /* cache not critical */ }` convention already used for `GetAchievementCounts` at the three call sites below.

- [ ] **Step 3: Call it in `handleSuccessfulConnection`**

In `handleSuccessfulConnection` (current lines 27-40), change:

```ts
    loadToPlayList();
    startScan();
  }
```

to:

```ts
    loadToPlayList();
    loadCardEnrichment();
    startScan();
  }
```

- [ ] **Step 4: Call it in `onMount`'s already-connected branch**

In `onMount` (current lines 58-74), change the `else` branch:

```ts
    } else {
      try {
        const counts = await GetAchievementCounts();
        achievementCounts.set(counts || {});
      } catch { /* cache not critical */ }
      loadToPlayList();
      if (!$scanning) {
        startScan();
      }
    }
```

to:

```ts
    } else {
      try {
        const counts = await GetAchievementCounts();
        achievementCounts.set(counts || {});
      } catch { /* cache not critical */ }
      loadToPlayList();
      loadCardEnrichment();
      if (!$scanning) {
        startScan();
      }
    }
```

- [ ] **Step 5: Call it in `handleAccountChanged`**

In `handleAccountChanged` (current lines 76-89), change:

```ts
  function handleAccountChanged() {
    stopRetry();
    if ($isConnected) {
      loadSettings().then(() => loadGames()).then(() => {
        try {
          GetAchievementCounts().then(counts => achievementCounts.set(counts || {}));
        } catch { /* cache not critical */ }
        loadToPlayList();
        startScan();
      });
    } else {
      startRetry();
    }
  }
```

to:

```ts
  function handleAccountChanged() {
    stopRetry();
    if ($isConnected) {
      loadSettings().then(() => loadGames()).then(() => {
        try {
          GetAchievementCounts().then(counts => achievementCounts.set(counts || {}));
        } catch { /* cache not critical */ }
        loadToPlayList();
        loadCardEnrichment();
        startScan();
      });
    } else {
      startRetry();
    }
  }
```

- [ ] **Step 6: Verify svelte-check passes**

Run: `cd frontend && npm run check`
Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/lib/pages/GamePicker.svelte
git commit -m "Load cached playtime and HLTB data alongside achievement counts"
```

---

### Task 4: Grid card — remove AppID, show Played/Main

**Files:**
- Modify: `frontend/src/lib/components/GameCard.svelte`

**Interfaces:**
- Consumes: `playtimes`, `hltbCache` (Task 2/3, from `../stores/games`).
- Produces: nothing consumed by later tasks (Task 5 is independent).

- [ ] **Step 1: Import the new stores**

Change line 3 from:

```ts
import { achievementCounts } from '../stores/games';
```

to:

```ts
import { achievementCounts, playtimes, hltbCache } from '../stores/games';
```

- [ ] **Step 2: Add derived values**

After the existing `let isOnToPlayList = $derived(...)` line (line 39), add:

```ts
let playtimeHours = $derived($playtimes[String(game.appId)]);
let hltbMain = $derived($hltbCache[String(game.appId)]?.main);
```

- [ ] **Step 3: Replace the footer line**

Change the footer block (lines 172-179):

```svelte
        <div class="flex items-center gap-2 mt-0.5">
          <span class="text-xs text-steam-text-dim">AppID: {game.appId}</span>
          {#if game.lastPlayed}
            <span class="text-xs text-steam-text-dim" title={new Date(game.lastPlayed * 1000).toLocaleDateString()}>
              {formatLastPlayed(game.lastPlayed)}
            </span>
          {/if}
        </div>
```

to:

```svelte
        <div class="flex items-center gap-2 mt-0.5">
          {#if game.lastPlayed}
            <span class="text-xs text-steam-text-dim" title={new Date(game.lastPlayed * 1000).toLocaleDateString()}>
              {formatLastPlayed(game.lastPlayed)}
            </span>
          {/if}
          {#if playtimeHours}
            <span class="text-xs text-sky-400">Played {Math.round(playtimeHours)}h</span>
          {/if}
          {#if hltbMain}
            <span class="text-xs"><span class="text-white/35">Main</span> {Math.round(hltbMain)}h</span>
          {/if}
        </div>
```

- [ ] **Step 4: Verify svelte-check passes**

Run: `cd frontend && npm run check`
Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/components/GameCard.svelte
git commit -m "Show cached playtime and HLTB main-story time on grid cards"
```

---

### Task 5: List row — combined playtime/HLTB column

**Files:**
- Modify: `frontend/src/lib/components/GameListRow.svelte`

**Interfaces:**
- Consumes: `playtimes`, `hltbCache` (Task 2/3, from `../stores/games`).
- Produces: nothing consumed by later tasks.

- [ ] **Step 1: Import the new stores**

Change line 3 from:

```ts
import { achievementCounts } from '../stores/games';
```

to:

```ts
import { achievementCounts, playtimes, hltbCache } from '../stores/games';
```

- [ ] **Step 2: Add derived values**

After the existing `let isOnToPlayList = $derived(...)` line (line 35), add:

```ts
let playtimeHours = $derived($playtimes[String(game.appId)]);
let hltbMain = $derived($hltbCache[String(game.appId)]?.main);
let playtimeLabel = $derived(playtimeHours ? `${Math.round(playtimeHours)}h` : '');
let hltbLabel = $derived(hltbMain ? `${Math.round(hltbMain)}h` : '');
let combinedLabel = $derived(
  playtimeLabel && hltbLabel ? `${playtimeLabel} / ${hltbLabel}` : playtimeLabel || hltbLabel
);
let combinedTitle = $derived(
  [playtimeLabel && `Played ${playtimeLabel}`, hltbLabel && `Main story ${hltbLabel}`]
    .filter(Boolean)
    .join(' · ')
);
```

- [ ] **Step 3: Insert the new column**

Between the existing last-played span and the achievement-count span (current lines 107-108):

```svelte
  <span class="text-xs text-steam-text-dim w-16 text-right flex-shrink-0" title={game.lastPlayed ? new Date(game.lastPlayed * 1000).toLocaleDateString() : ''}>{game.lastPlayed ? formatLastPlayed(game.lastPlayed) : ''}</span>
  <span class="text-xs w-16 text-right flex-shrink-0 {isFullyCompleted ? 'text-amber-400 font-medium' : isEarlyAccess ? 'text-blue-400 font-medium' : 'text-steam-text-dim'}">{hasAchievementData ? `${counts!.achieved}/${counts!.total}` : isEarlyAccess ? 'EA' : counts && counts.total > 0 ? `${counts.total}` : ''}</span>
```

Insert the new column between them, so the block reads:

```svelte
  <span class="text-xs text-steam-text-dim w-16 text-right flex-shrink-0" title={game.lastPlayed ? new Date(game.lastPlayed * 1000).toLocaleDateString() : ''}>{game.lastPlayed ? formatLastPlayed(game.lastPlayed) : ''}</span>
  <span class="text-xs text-steam-text-dim w-16 text-right flex-shrink-0" title={combinedTitle}>{combinedLabel}</span>
  <span class="text-xs w-16 text-right flex-shrink-0 {isFullyCompleted ? 'text-amber-400 font-medium' : isEarlyAccess ? 'text-blue-400 font-medium' : 'text-steam-text-dim'}">{hasAchievementData ? `${counts!.achieved}/${counts!.total}` : isEarlyAccess ? 'EA' : counts && counts.total > 0 ? `${counts.total}` : ''}</span>
```

The AppID column (`<span class="text-xs text-steam-text-dim w-20 text-right flex-shrink-0">{game.appId}</span>`, current line 109) is left untouched.

- [ ] **Step 4: Verify svelte-check passes**

Run: `cd frontend && npm run check`
Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/components/GameListRow.svelte
git commit -m "Show combined playtime/HLTB column on list rows"
```

---

### Task 6: End-to-end manual verification

**Files:** none (verification only)

- [ ] **Step 1: Build and launch**

Run: `make dev`

- [ ] **Step 2: Populate caches**

Open two or three different games from the library (click into `GameManager.svelte` for each), then navigate back to the library. This exercises the existing per-game `GetPlaytime`/`GetHLTBTimes` effects, populating `localconfig.vdf`-backed playtime and `hltb.json`.

- [ ] **Step 3: Verify grid view**

Confirm:
- The games just opened show `Played Xh` in sky-blue and `Main Xh` (dim "Main" label) in their card footers.
- No card anywhere shows an `AppID:` label.
- Games never opened this session (and with no prior cached data) show neither `Played` nor `Main`.

- [ ] **Step 4: Verify list view**

Switch to list view. Confirm:
- The new column shows `Xh / Yh` (or just one side) for visited games, empty for others.
- The AppID column is still present and unchanged.
- Row alignment stays consistent whether or not a row has playtime/HLTB data.

- [ ] **Step 5: Verify persistence across restart**

Close and restart the app (`make dev` again, or the built binary). Confirm the same cards still show `Played`/`Main` values without reopening those games — this confirms the data is read from disk (`localconfig.vdf`, `hltb.json`), not from in-memory state.

- [ ] **Step 6: Confirm working tree is clean**

Run: `git status --short`
Expected: no unexpected modifications (only the incidental `wailsjs/runtime` mode churn from `make dev`'s build step, if any — discard via `git checkout -- frontend/wailsjs/runtime/package.json frontend/wailsjs/runtime/runtime.d.ts frontend/wailsjs/runtime/runtime.js` if present with zero content diff).
