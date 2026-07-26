# Games To Play Backlog List Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let the user mark/unmark any game as "to play" from the game grid, filter the library down to just that list, and see at a glance (no hovering) which games are on it — without importing Steam's own undocumented Collections storage format.

**Architecture:** A small standalone JSON store (`to_play.json`, per-user config dir) holds a flat set of appIDs, following the same load/save pattern as the existing achievement/game caches but deliberately kept separate from `game_service.go`'s cache-merge cycle (which fully rebuilds `CachedGame` entries on every refresh and would silently wipe any extra field bolted onto it). Two new Wails-bound `App` methods expose it; a new Svelte store loads it once and updates it optimistically on toggle. The existing filter dropdown and game-grid/list-row action-icon conventions get one more entry each.

**Tech Stack:** Go 1.23 (backend, `internal/settings`, `app.go`), Svelte 5 + TypeScript (frontend), Wails v2 for the Go↔JS bridge.

## Global Constraints

- All Go build/vet/dev commands require `-tags webkit2_41` on Linux (per project `CLAUDE.md`).
- No automated test suite exists in this project — verification is manual via `make dev` (per project `CLAUDE.md`).
- `frontend/wailsjs/**` is auto-generated — never hand-edit; regenerate with `wails generate module -tags webkit2_41` after adding/changing `App` methods, then diff the result to confirm only the intended bindings changed.
- Design source of truth: `docs/superpowers/specs/2026-07-25-games-to-play-list-design.md`.

---

### Task 1: Backend storage for the to-play list

**Files:**
- Create: `internal/settings/toplay.go`
- Modify: `internal/settings/settings.go:85-95` (`SetCurrentUser`)

**Interfaces:**
- Produces: `settings.LoadToPlayList() map[uint32]bool` (returns a copy), `settings.SetToPlay(appID uint32, want bool)` (adds/removes and writes to disk immediately). Both consumed by Task 2.

- [ ] **Step 1: Create `internal/settings/toplay.go`**

```go
package settings

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

var (
	toPlayMu     sync.RWMutex
	toPlayList   map[uint32]bool
	toPlayLoaded bool
)

func toPlayFilePath() string {
	dir := userDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "to_play.json")
}

func resetToPlayList() {
	toPlayMu.Lock()
	defer toPlayMu.Unlock()
	toPlayList = nil
	toPlayLoaded = false
}

// LoadToPlayList returns a copy of the current "games to play" set.
func LoadToPlayList() map[uint32]bool {
	toPlayMu.Lock()
	defer toPlayMu.Unlock()

	if !toPlayLoaded {
		toPlayList = make(map[uint32]bool)
		toPlayLoaded = true

		p := toPlayFilePath()
		if p != "" {
			data, err := os.ReadFile(p)
			if err == nil {
				if err := json.Unmarshal(data, &toPlayList); err != nil {
					slog.Warn("failed to parse to-play list", "error", err)
					toPlayList = make(map[uint32]bool)
				}
			}
		}
	}

	result := make(map[uint32]bool, len(toPlayList))
	for k, v := range toPlayList {
		result[k] = v
	}
	return result
}

// SetToPlay adds or removes a game from the "games to play" list and writes
// the change to disk immediately — toggles are rare, deliberate user clicks,
// not a bulk scan, so no debounce is needed here (unlike achievement_cache.go).
func SetToPlay(appID uint32, want bool) {
	toPlayMu.Lock()
	if toPlayList == nil {
		toPlayList = make(map[uint32]bool)
		toPlayLoaded = true
	}
	if want {
		toPlayList[appID] = true
	} else {
		delete(toPlayList, appID)
	}
	snapshot := make(map[uint32]bool, len(toPlayList))
	for k, v := range toPlayList {
		snapshot[k] = v
	}
	toPlayMu.Unlock()

	p := toPlayFilePath()
	if p == "" {
		return
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		slog.Warn("failed to marshal to-play list", "error", err)
		return
	}

	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Warn("failed to create config dir", "error", err)
		return
	}
	if err := atomicWrite(p, data); err != nil {
		slog.Warn("failed to write to-play list", "error", err)
	}
}
```

- [ ] **Step 2: Reset the list when the active Steam user changes**

In `internal/settings/settings.go`, replace:

```go
func SetCurrentUser(steamID uint64) {
	currentUserMu.Lock()
	currentUserID = steamID
	currentUserMu.Unlock()

	resetAchievementCache()
	resetGameCache()
	reloadSettings()

	slog.Info("current user set", "steamID", steamID)
}
```

with:

```go
func SetCurrentUser(steamID uint64) {
	currentUserMu.Lock()
	currentUserID = steamID
	currentUserMu.Unlock()

	resetAchievementCache()
	resetGameCache()
	resetToPlayList()
	reloadSettings()

	slog.Info("current user set", "steamID", steamID)
}
```

Without this, switching Steam accounts (`handleAccountChange` in `app.go`) would keep showing the previous account's to-play list until the app restarts.

- [ ] **Step 3: Build and vet**

Run: `go build -tags webkit2_41 ./... && go vet -tags webkit2_41 ./...`
Expected: both succeed with no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/settings/toplay.go internal/settings/settings.go
git commit -m "Add per-user storage for the games-to-play list"
```

---

### Task 2: Backend API methods

**Files:**
- Modify: `app.go:400-402` (insert after `GetAchievementCounts`)

**Interfaces:**
- Consumes: `settings.LoadToPlayList() map[uint32]bool`, `settings.SetToPlay(appID uint32, want bool)` from Task 1.
- Produces: Wails-bound `GetToPlayList() []uint32` and `SetToPlay(appID uint32, want bool) error`, consumed by Task 3's frontend store.

- [ ] **Step 1: Add the two methods**

In `app.go`, after:

```go
func (a *App) GetAchievementCounts() map[uint32]settings.AchievementCount {
	return settings.LoadAchievementCache()
}
```

insert:

```go

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
```

- [ ] **Step 2: Build and vet**

Run: `go build -tags webkit2_41 ./... && go vet -tags webkit2_41 ./...`
Expected: both succeed with no errors.

- [ ] **Step 3: Regenerate Wails bindings**

Run: `wails generate module -tags webkit2_41`

Then run: `git diff --stat frontend/wailsjs`
Expected: only `frontend/wailsjs/go/main/App.d.ts` and `frontend/wailsjs/go/main/App.js` show real content changes (new `GetToPlayList`/`SetToPlay` exports). If other files show non-zero insertions/deletions (not just mode/whitespace no-ops), stop and investigate before continuing — that would mean the regeneration touched something unrelated.

- [ ] **Step 4: Commit**

```bash
git add app.go frontend/wailsjs
git commit -m "Add GetToPlayList/SetToPlay Wails bindings"
```

---

### Task 3: Frontend store

**Files:**
- Create: `frontend/src/lib/stores/toplay.ts`
- Modify: `frontend/src/lib/pages/GamePicker.svelte:13` (import), `:26-38` (`handleSuccessfulConnection`), `:56-70` (`onMount`), `:73-85` (`handleAccountChanged`)

**Interfaces:**
- Consumes: `GetToPlayList(): Promise<number[]>`, `SetToPlay(appId: number, want: boolean): Promise<void>` from Task 2 (`wailsjs/go/main/App`).
- Produces: `toPlayList: Writable<Set<number>>`, `loadToPlayList(): Promise<void>`, `toggleToPlay(appId: number): Promise<void>` — all exported from `frontend/src/lib/stores/toplay.ts`, consumed by Task 4 (filter) and Tasks 5/6 (card/list toggle buttons).

- [ ] **Step 1: Create `frontend/src/lib/stores/toplay.ts`**

```ts
import { writable, get } from 'svelte/store';
import { GetToPlayList, SetToPlay } from '../../../wailsjs/go/main/App';
import { addToast } from './app';

export const toPlayList = writable<Set<number>>(new Set());

export async function loadToPlayList(): Promise<void> {
  try {
    const appIds = await GetToPlayList();
    toPlayList.set(new Set(appIds || []));
  } catch {
    // Convenience layer only — leave the set empty rather than blocking the grid.
  }
}

export async function toggleToPlay(appId: number): Promise<void> {
  const wasOnList = get(toPlayList).has(appId);
  const want = !wasOnList;

  toPlayList.update(current => {
    const next = new Set(current);
    if (want) next.add(appId); else next.delete(appId);
    return next;
  });

  try {
    await SetToPlay(appId, want);
  } catch (e: any) {
    // Revert the optimistic update on failure
    toPlayList.update(current => {
      const next = new Set(current);
      if (wasOnList) next.add(appId); else next.delete(appId);
      return next;
    });
    addToast(`Failed to update games-to-play list: ${e.message || e}`, 'error');
  }
}
```

- [ ] **Step 2: Wire up loading in `GamePicker.svelte`**

Add the import. Replace:

```ts
  import { games, gamesLoading, searchQuery, achievementCounts } from '../stores/games';
```

with:

```ts
  import { games, gamesLoading, searchQuery, achievementCounts } from '../stores/games';
  import { loadToPlayList } from '../stores/toplay';
```

Then add a `loadToPlayList()` call alongside each existing `GetAchievementCounts()` call. Replace:

```ts
  async function handleSuccessfulConnection(id: number) {
    stopRetry();
    isConnected.set(true);
    steamId.set(id.toString());
    GetPersonaName().then(name => personaName.set(name)).catch(() => {});
    await loadSettings();
    await loadGames();
    try {
      const counts = await GetAchievementCounts();
      achievementCounts.set(counts || {});
    } catch { /* cache not critical */ }
    startScan();
  }
```

with:

```ts
  async function handleSuccessfulConnection(id: number) {
    stopRetry();
    isConnected.set(true);
    steamId.set(id.toString());
    GetPersonaName().then(name => personaName.set(name)).catch(() => {});
    await loadSettings();
    await loadGames();
    try {
      const counts = await GetAchievementCounts();
      achievementCounts.set(counts || {});
    } catch { /* cache not critical */ }
    loadToPlayList();
    startScan();
  }
```

Replace:

```ts
  onMount(async () => {
    window.addEventListener('steamforge-account-changed', handleAccountChanged);
    await loadSettings();

    if (!$isConnected) {
      await connect();
    } else {
      try {
        const counts = await GetAchievementCounts();
        achievementCounts.set(counts || {});
      } catch { /* cache not critical */ }
      if (!$scanning) {
        startScan();
      }
    }
  });
```

with:

```ts
  onMount(async () => {
    window.addEventListener('steamforge-account-changed', handleAccountChanged);
    await loadSettings();

    if (!$isConnected) {
      await connect();
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
  });
```

Replace:

```ts
  function handleAccountChanged() {
    stopRetry();
    if ($isConnected) {
      loadSettings().then(() => loadGames()).then(() => {
        try {
          GetAchievementCounts().then(counts => achievementCounts.set(counts || {}));
        } catch { /* cache not critical */ }
        startScan();
      });
    } else {
      startRetry();
    }
  }
```

with:

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

This mirrors every existing place `GetAchievementCounts` is (re)loaded — on first successful connect, on remount while already connected, and after an account switch — since the to-play list is per-account just like achievement counts.

- [ ] **Step 3: Type-check**

Run: `cd frontend && npm run check`
Expected: no new TypeScript errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/stores/toplay.ts frontend/src/lib/pages/GamePicker.svelte
git commit -m "Add games-to-play frontend store"
```

---

### Task 4: Filter dropdown integration

**Files:**
- Modify: `frontend/src/lib/stores/app.ts:19` (`GameFilter` type)
- Modify: `frontend/src/lib/stores/games.ts:1-3` (imports), `:27-53` (`filteredGames`)
- Modify: `frontend/src/lib/pages/GamePicker.svelte:153-178` (`filterCounts`), `:180-185` (`filterOptions`)

**Interfaces:**
- Consumes: `toPlayList: Writable<Set<number>>` from Task 3.
- Produces: `GameFilter` now includes `'toPlay'`, consumed anywhere `GameFilter` is used (only `GamePicker.svelte` and `games.ts` today).

- [ ] **Step 1: Extend the `GameFilter` type**

In `frontend/src/lib/stores/app.ts`, replace:

```ts
export type GameFilter = 'all' | 'incomplete' | 'perfected' | 'none';
```

with:

```ts
export type GameFilter = 'all' | 'incomplete' | 'perfected' | 'none' | 'toPlay';
```

- [ ] **Step 2: Filter by the to-play list in `filteredGames`**

In `frontend/src/lib/stores/games.ts`, replace the import block:

```ts
import { writable, derived } from 'svelte/store';
import { settings } from './settings';
import { gameFilter } from './app';
```

with:

```ts
import { writable, derived } from 'svelte/store';
import { settings } from './settings';
import { gameFilter } from './app';
import { toPlayList } from './toplay';
```

Replace:

```ts
export const filteredGames = derived(
  [games, searchQuery, settings, achievementCounts, gameFilter],
  ([$games, $searchQuery, $settings, $achievementCounts, $gameFilter]) => {
    let result = $games;

    // Hide software/tools unless the setting is enabled
    if (!$settings.showSoftware) {
      result = result.filter(game => !game.isSoftware);
    }

    if ($searchQuery) {
      const query = $searchQuery.toLowerCase();
      result = result.filter(game => game.name.toLowerCase().includes(query));
    }

    const { sortBy, sortOrder } = $settings;

    if ($gameFilter && $gameFilter !== 'all') {
      result = result.filter(game => {
        const counts = $achievementCounts[String(game.appId)];
        if ($gameFilter === 'none') return !counts || counts.total === 0;
        if (!counts || counts.total === 0) return false;
        if ($gameFilter === 'perfected') return counts.achieved === counts.total;
        if ($gameFilter === 'incomplete') return counts.achieved < counts.total;
        return true;
      });
    }
```

with:

```ts
export const filteredGames = derived(
  [games, searchQuery, settings, achievementCounts, gameFilter, toPlayList],
  ([$games, $searchQuery, $settings, $achievementCounts, $gameFilter, $toPlayList]) => {
    let result = $games;

    // Hide software/tools unless the setting is enabled
    if (!$settings.showSoftware) {
      result = result.filter(game => !game.isSoftware);
    }

    if ($searchQuery) {
      const query = $searchQuery.toLowerCase();
      result = result.filter(game => game.name.toLowerCase().includes(query));
    }

    const { sortBy, sortOrder } = $settings;

    if ($gameFilter === 'toPlay') {
      result = result.filter(game => $toPlayList.has(game.appId));
    } else if ($gameFilter && $gameFilter !== 'all') {
      result = result.filter(game => {
        const counts = $achievementCounts[String(game.appId)];
        if ($gameFilter === 'none') return !counts || counts.total === 0;
        if (!counts || counts.total === 0) return false;
        if ($gameFilter === 'perfected') return counts.achieved === counts.total;
        if ($gameFilter === 'incomplete') return counts.achieved < counts.total;
        return true;
      });
    }
```

(The rest of `filteredGames` — the sort block and its `return result;` — is unchanged.)

- [ ] **Step 3: Add the filter option and its count**

In `frontend/src/lib/pages/GamePicker.svelte`, add the import. Replace:

```ts
  import { games, gamesLoading, searchQuery, achievementCounts } from '../stores/games';
  import { loadToPlayList } from '../stores/toplay';
```

with:

```ts
  import { games, gamesLoading, searchQuery, achievementCounts } from '../stores/games';
  import { loadToPlayList, toPlayList } from '../stores/toplay';
```

Replace:

```ts
  // Compute filter counts from search-filtered (but not game-filter-filtered) games
  let filterCounts = $derived.by(() => {
    // Apply software and search filters to get the base list for counting
    let searchFiltered = $settings.showSoftware ? $games : $games.filter(game => !game.isSoftware);
    if ($searchQuery) {
      const query = $searchQuery.toLowerCase();
      searchFiltered = searchFiltered.filter(game => game.name.toLowerCase().includes(query));
    }

    let incomplete = 0, perfected = 0, noAchievements = 0;
    for (const game of searchFiltered) {
      const counts = $achievementCounts[String(game.appId)];
      if (!counts || counts.total === 0) {
        noAchievements++;
      } else if (counts.achieved === counts.total) {
        perfected++;
      } else {
        incomplete++;
      }
    }
    return {
      all: searchFiltered.length,
      incomplete,
      perfected,
      none: noAchievements,
    };
  });
```

with:

```ts
  // Compute filter counts from search-filtered (but not game-filter-filtered) games
  let filterCounts = $derived.by(() => {
    // Apply software and search filters to get the base list for counting
    let searchFiltered = $settings.showSoftware ? $games : $games.filter(game => !game.isSoftware);
    if ($searchQuery) {
      const query = $searchQuery.toLowerCase();
      searchFiltered = searchFiltered.filter(game => game.name.toLowerCase().includes(query));
    }

    let incomplete = 0, perfected = 0, noAchievements = 0, toPlay = 0;
    for (const game of searchFiltered) {
      const counts = $achievementCounts[String(game.appId)];
      if (!counts || counts.total === 0) {
        noAchievements++;
      } else if (counts.achieved === counts.total) {
        perfected++;
      } else {
        incomplete++;
      }
      if ($toPlayList.has(game.appId)) toPlay++;
    }
    return {
      all: searchFiltered.length,
      incomplete,
      perfected,
      none: noAchievements,
      toPlay,
    };
  });
```

Replace:

```ts
  const filterOptions: { value: GameFilter; label: string }[] = [
    { value: 'all', label: 'All' },
    { value: 'incomplete', label: 'Incomplete' },
    { value: 'perfected', label: 'Perfected' },
    { value: 'none', label: 'No Achievements' },
  ];
```

with:

```ts
  const filterOptions: { value: GameFilter; label: string }[] = [
    { value: 'all', label: 'All' },
    { value: 'incomplete', label: 'Incomplete' },
    { value: 'perfected', label: 'Perfected' },
    { value: 'none', label: 'No Achievements' },
    { value: 'toPlay', label: 'Games to Play' },
  ];
```

(No change needed to the dropdown markup itself — it already renders `filterOptions` and reads `filterCounts[option.value]` generically for each entry.)

- [ ] **Step 4: Type-check**

Run: `cd frontend && npm run check`
Expected: no new TypeScript errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/stores/app.ts frontend/src/lib/stores/games.ts frontend/src/lib/pages/GamePicker.svelte
git commit -m "Add Games to Play as a library filter option"
```

---

### Task 5: Toggle star button on the game card (grid view)

**Files:**
- Modify: `frontend/src/lib/components/GameCard.svelte:1-9` (imports), `:32-37` (derived state), `:177-227` (overlay markup)

**Interfaces:**
- Consumes: `toPlayList: Writable<Set<number>>`, `toggleToPlay(appId: number): Promise<void>` from Task 3.

- [ ] **Step 1: Import the store and toggle function**

Replace:

```ts
  import type { GameInfo } from '../stores/games';
  import { achievementCounts } from '../stores/games';
  import { navigateToManager } from '../stores/app';
  import { settings } from '../stores/settings';
  import { formatLastPlayed } from '../utils/format';
  import { buildGameImageUrls } from '../utils/steam-images';
  import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
  import { showGameContextMenu } from '../utils/context-menu';
```

with:

```ts
  import type { GameInfo } from '../stores/games';
  import { achievementCounts } from '../stores/games';
  import { navigateToManager } from '../stores/app';
  import { settings } from '../stores/settings';
  import { formatLastPlayed } from '../utils/format';
  import { buildGameImageUrls } from '../utils/steam-images';
  import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
  import { showGameContextMenu } from '../utils/context-menu';
  import { toPlayList, toggleToPlay } from '../stores/toplay';
```

- [ ] **Step 2: Add derived "on the list" state and a click handler**

Replace:

```ts
  let counts = $derived($achievementCounts[String(game.appId)]);
  let hasAchievementData = $derived(counts && counts.total > 0 && counts.achieved >= 0);
  let completionPercent = $derived(hasAchievementData ? Math.round((counts!.achieved / counts!.total) * 100) : -1);
  let isFullyCompleted = $derived(hasAchievementData && counts!.achieved === counts!.total);
  let isEarlyAccess = $derived(counts?.earlyAccess === true);
  let isProtected = $derived(counts?.protected === true);
```

with:

```ts
  let counts = $derived($achievementCounts[String(game.appId)]);
  let hasAchievementData = $derived(counts && counts.total > 0 && counts.achieved >= 0);
  let completionPercent = $derived(hasAchievementData ? Math.round((counts!.achieved / counts!.total) * 100) : -1);
  let isFullyCompleted = $derived(hasAchievementData && counts!.achieved === counts!.total);
  let isEarlyAccess = $derived(counts?.earlyAccess === true);
  let isProtected = $derived(counts?.protected === true);
  let isOnToPlayList = $derived($toPlayList.has(game.appId));

  function handleToggleToPlay(e: Event) {
    e.stopPropagation();
    toggleToPlay(game.appId);
  }
```

- [ ] **Step 3: Add the star button to the overlay**

The overlay currently has a store-page button (top-left) and a play/install button (top-right). Replace:

```svelte
      {#if $settings.showCardButtons}
        <button
          onclick={(e) => { e.stopPropagation(); BrowserOpenURL(`steam://store/${game.appId}`); }}
          class="pointer-events-auto absolute top-1.5 left-1.5 p-1 rounded-full bg-black/50 cursor-pointer transition-opacity
            opacity-0 group-hover:opacity-100 text-white/70 hover:text-steam-primary"
          title="Open Steam store page"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 11V7a4 4 0 00-8 0v4M5 9h14l1 12H4L5 9z" />
          </svg>
        </button>
        {#if game.installed}
```

with:

```svelte
      {#if $settings.showCardButtons}
        <button
          onclick={(e) => { e.stopPropagation(); BrowserOpenURL(`steam://store/${game.appId}`); }}
          class="pointer-events-auto absolute top-1.5 left-1.5 p-1 rounded-full bg-black/50 cursor-pointer transition-opacity
            opacity-0 group-hover:opacity-100 text-white/70 hover:text-steam-primary"
          title="Open Steam store page"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 11V7a4 4 0 00-8 0v4M5 9h14l1 12H4L5 9z" />
          </svg>
        </button>
        <button
          onclick={handleToggleToPlay}
          class="pointer-events-auto absolute bottom-1.5 left-1.5 p-1 rounded-full bg-black/50 cursor-pointer transition-opacity
            {isOnToPlayList ? 'opacity-100' : 'opacity-0 group-hover:opacity-100'} text-amber-400 hover:text-amber-300"
          title={isOnToPlayList ? 'Remove from Games to Play' : 'Add to Games to Play'}
        >
          <svg class="w-4 h-4" fill={isOnToPlayList ? 'currentColor' : 'none'} stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11.48 3.499a.562.562 0 011.04 0l2.125 5.111a.563.563 0 00.475.345l5.518.442c.499.04.701.663.321.988l-4.204 3.602a.563.563 0 00-.182.557l1.285 5.385a.562.562 0 01-.84.61l-4.725-2.885a.562.562 0 00-.586 0L6.982 20.54a.562.562 0 01-.84-.61l1.285-5.386a.562.562 0 00-.182-.557l-4.204-3.602a.562.562 0 01.321-.988l5.518-.442a.563.563 0 00.475-.345L11.48 3.5z" />
          </svg>
        </button>
        {#if game.installed}
```

Note this button sits inside the same `{#if $settings.showCardButtons}` block as the store/play/install buttons (`GameCard.svelte:180-214`), so it respects the existing "show card buttons" setting and is placed in the bottom-left corner — the one free slot, since top-left is the store link and top-right is play/install.

- [ ] **Step 4: Type-check**

Run: `cd frontend && npm run check`
Expected: no new TypeScript errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/components/GameCard.svelte
git commit -m "Add games-to-play star toggle to the game card"
```

---

### Task 6: Toggle star button on the list row (list view)

**Files:**
- Modify: `frontend/src/lib/components/GameListRow.svelte:1-9` (imports), `:30-33` (derived state), `:64-86` (action buttons)

**Interfaces:**
- Consumes: `toPlayList: Writable<Set<number>>`, `toggleToPlay(appId: number): Promise<void>` from Task 3.

- [ ] **Step 1: Import the store and toggle function**

Replace:

```ts
  import type { GameInfo } from '../stores/games';
  import { achievementCounts } from '../stores/games';
  import { navigateToManager } from '../stores/app';
  import { formatLastPlayed } from '../utils/format';
  import { buildGameImageUrls } from '../utils/steam-images';
  import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
  import { settings } from '../stores/settings';
  import { showGameContextMenu } from '../utils/context-menu';
```

with:

```ts
  import type { GameInfo } from '../stores/games';
  import { achievementCounts } from '../stores/games';
  import { navigateToManager } from '../stores/app';
  import { formatLastPlayed } from '../utils/format';
  import { buildGameImageUrls } from '../utils/steam-images';
  import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
  import { settings } from '../stores/settings';
  import { showGameContextMenu } from '../utils/context-menu';
  import { toPlayList, toggleToPlay } from '../stores/toplay';
```

- [ ] **Step 2: Add derived "on the list" state and a click handler**

Replace:

```ts
  let counts = $derived($achievementCounts[String(game.appId)]);
  let hasAchievementData = $derived(counts && counts.total > 0 && counts.achieved >= 0);
  let isFullyCompleted = $derived(hasAchievementData && counts!.achieved === counts!.total);
  let isEarlyAccess = $derived(counts?.earlyAccess && counts.total === 0);
```

with:

```ts
  let counts = $derived($achievementCounts[String(game.appId)]);
  let hasAchievementData = $derived(counts && counts.total > 0 && counts.achieved >= 0);
  let isFullyCompleted = $derived(hasAchievementData && counts!.achieved === counts!.total);
  let isEarlyAccess = $derived(counts?.earlyAccess && counts.total === 0);
  let isOnToPlayList = $derived($toPlayList.has(game.appId));

  function handleToggleToPlay(e: Event) {
    e.stopPropagation();
    toggleToPlay(game.appId);
  }
```

- [ ] **Step 3: Add the star button before the play/install button**

Unlike the grid card, list-row action buttons are always visible (not hover-reveal) — the play/install button already works this way here. Replace:

```svelte
  {#if $settings.showCardButtons}
    {#if game.installed}
```

with:

```svelte
  {#if $settings.showCardButtons}
    <button
      onclick={handleToggleToPlay}
      class="w-7 h-7 flex-shrink-0 flex items-center justify-center rounded transition-colors cursor-pointer
        {isOnToPlayList ? 'bg-amber-500/20 text-amber-400 hover:bg-amber-500/40' : 'bg-steam-input text-steam-text-dim hover:text-amber-400'}"
      title={isOnToPlayList ? 'Remove from Games to Play' : 'Add to Games to Play'}
    >
      <svg class="w-3.5 h-3.5" fill={isOnToPlayList ? 'currentColor' : 'none'} stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11.48 3.499a.562.562 0 011.04 0l2.125 5.111a.563.563 0 00.475.345l5.518.442c.499.04.701.663.321.988l-4.204 3.602a.563.563 0 00-.182.557l1.285 5.385a.562.562 0 01-.84.61l-4.725-2.885a.562.562 0 00-.586 0L6.982 20.54a.562.562 0 01-.84-.61l1.285-5.386a.562.562 0 00-.182-.557l-4.204-3.602a.562.562 0 01.321-.988l5.518-.442a.563.563 0 00.475-.345L11.48 3.5z" />
      </svg>
    </button>
    {#if game.installed}
```

- [ ] **Step 4: Type-check**

Run: `cd frontend && npm run check`
Expected: no new TypeScript errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/components/GameListRow.svelte
git commit -m "Add games-to-play star toggle to the list row"
```

---

### Task 7: End-to-end manual verification

**Files:** none (verification only)

- [ ] **Step 1: Build and launch**

Run: `make dev`

- [ ] **Step 2: Verify toggling from the grid**

Hover a game card in grid view. Confirm the amber star appears bottom-left alongside the store/play icons. Click it. Confirm the star becomes filled/amber and stays visible even after moving the mouse away (no hover needed once active).

- [ ] **Step 3: Verify the filter**

Open the filter dropdown. Confirm "Games to Play" appears as an option with a count matching the number of games you've starred. Select it. Confirm only starred games show in the grid.

- [ ] **Step 4: Verify persistence across restart**

Close the app (`Ctrl+C` the `make dev` process) and relaunch. Confirm previously-starred games are still starred, and the "Games to Play" filter still returns the same set.

- [ ] **Step 5: Verify toggling off**

With the "Games to Play" filter still active, click a starred game's star to remove it. Confirm it immediately drops out of the filtered view (the filter is reactive to the same store the star button writes to).

- [ ] **Step 6: Verify list view stays in sync**

Switch to list view (if the app has a view toggle — check `settings.viewMode`). Confirm the same games show as starred there, and toggling from list view is reflected back in grid view.

- [ ] **Step 7: Verify a failed write reverts the star**

This is hard to trigger without simulating a disk-write failure; if you can (e.g. temporarily make the per-user config dir read-only), confirm clicking the star shows an error toast and the star reverts to its prior state rather than getting stuck in a state that doesn't match disk.
