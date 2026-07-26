# Code Quality Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring SteamForge's codebase in line with the project's hard coding standards (SOLID, DRY, KISS, YAGNI, Object Calisthenics) — eliminate the two files over ~800 lines, remove verified duplication in the backend service/steam/settings layers, and reduce SRP violations in the largest frontend components.

**Architecture:** This is a pure refactor — no user-facing behavior changes. Each task is scoped to one subsystem and independently buildable/verifiable. Backend tasks split `App`'s methods across multiple files in the same `package main` (Go's standard multi-file-package idiom — no new types or interfaces, just file reorganization) or extract shared helpers for genuinely duplicated logic. Frontend tasks extract child components/utilities from oversized `.svelte` files using existing Svelte 5 runes patterns already used elsewhere in the codebase.

**Tech Stack:** Go 1.23 (backend), Svelte 5 + TypeScript (frontend). No automated test suite exists — verification is `go build`/`go vet -tags webkit2_41 ./...` (backend) and `cd frontend && npm run check` (frontend) after every task, per `CLAUDE.md`. Each task also ends with a full `make build-linux` to catch anything the individual checks miss (e.g. wails binding generation issues).

## Global Constraints

- `-tags webkit2_41` required for all Go build/vet commands on Linux (project `CLAUDE.md`).
- This is a refactor, not a feature change — behavior must be identical before and after each task. No new functionality, no bug fixes bundled in (if a task's audit uncovered an actual bug, e.g. a missing cache reset, it's called out explicitly as an intentional fix in that task's description, not left implicit).
- No automated test suite exists — verify via build/vet/svelte-check plus a full `make build-linux` per task, per `CLAUDE.md`.
- Preserve all existing exported Go function/method signatures used by Wails bindings (`app.go`'s public `App` methods) exactly — the frontend's generated bindings must not need regeneration for behavior to keep working, though regenerating them (`wails generate module -tags webkit2_41`) after backend changes is expected and fine since only file locations change, not signatures.
- Commit after each task (or each numbered step group within a task where noted), following the existing repo convention of small, focused commits.

---

### Task 1: Split `app.go` into domain files

**Files:**
- Modify: `app.go` (shrinks from 1274 lines to ~100 — struct definition, `NewApp()`, `startup()`, `shutdown()`, `Version`, `errNotConnected` only)
- Create: `app_steam.go`, `app_games.go`, `app_achievements.go`, `app_toplay.go`, `app_hltb.go`, `app_stats.go`, `app_settings.go`, `app_webapi.go`, `app_image.go`, `app_scan.go`, `app_watchers.go`, `app_version.go`

**Interfaces:**
- Consumes: nothing new — this is a pure move of existing methods/functions/consts between files in the same `package main`.
- Produces: nothing new consumed by other tasks — Wails bindings (`frontend/wailsjs/go/main/App.js`/`.d.ts`) must be regenerated after this task since the Go source moved, but their contents (method signatures) don't change.

- [ ] **Step 1: Move Steam connection lifecycle to `app_steam.go`**

Move these from `app.go` to a new `app_steam.go` (same `package main`, same imports as needed): `initClient`, `ConnectSteam`, `GetPersonaName`, `IsConnected`, `reconnectForGame`, `DisconnectGame`, `restoreBaseConnection`. Keep the `App` struct, `NewApp()`, `startup()`, `shutdown()` in `app.go`.

- [ ] **Step 2: Move game library operations to `app_games.go`**

Move `FetchGames`, `SearchGames` to `app_games.go`.

- [ ] **Step 3: Move achievement operations to `app_achievements.go`**

Move `LoadAchievementsFromSchema`, `LoadAchievements`, `GetAchievements`, `SetAchievement`, `ClearAchievement`, `SetAllAchievements`, `ClearAllAchievements`, `StoreStats`, `GetAchievementCounts` to `app_achievements.go`.

- [ ] **Step 4: Move "games to play" list to `app_toplay.go`**

Move `GetToPlayList`, `SetToPlay` to `app_toplay.go`.

- [ ] **Step 5: Move HLTB logic to `app_hltb.go`**

Move `getHLTBService`, `GetHLTBTimes`, `GetAllCachedHLTB`, and the `hltbNegativeCacheTTL`/`hltbNotInstalledTTL` constants to `app_hltb.go`.

- [ ] **Step 6: Move playtime reads to `app_stats.go`**

Move `GetAllPlaytimes`, `GetPlaytime` to `app_stats.go`.

- [ ] **Step 7: Move settings/system passthroughs to `app_settings.go`**

Move `GetSettings`, `SaveSettings`, `GetDataDir`, `OpenDataDir`, `GetLogContent` to `app_settings.go`.

- [ ] **Step 8: Move web API integration to `app_webapi.go`**

Move `CheckProfileVisibility`, `FetchGlobalPercents`, `CheckGameEarlyAccess` to `app_webapi.go`.

- [ ] **Step 9: Move image service to `app_image.go`**

Move `GetImageBase64` to `app_image.go`.

- [ ] **Step 10: Move achievement scanning pipeline to `app_scan.go`**

Move `ScanAchievementCounts`, `RescanAllAchievements`, `StopAchievementScan`, `stopScan`, `runAchievementScan`, `resolveGameLocally`, `isOldRelease`, the `localResolution`/`scanResult` structs, `isTransientError`, `fetchWithRetry`, `runWebAPIScan`, `handleWebScanResult`, `emitZeroAchievements`, `finalizeScan`, `saveScanTimestamp`, `cacheAchievementCounts`, and the `scanWorkers` constant to `app_scan.go`. This is the largest of the new files (~580 lines) — that's expected, it's one cohesive pipeline (local resolution → web API fallback → finalization).

- [ ] **Step 11: Move file watchers to `app_watchers.go`**

Move `startInstallWatcher`, `stopInstallWatcher`, `startAccountWatcher`, `stopAccountWatcher`, `handleAccountChange` to `app_watchers.go`.

- [ ] **Step 12: Move version/update checking to `app_version.go`**

Move `GetAppVersion`, `CheckForUpdates`, `normalizeVersion`, and the `UpdateInfo` struct to `app_version.go`.

- [ ] **Step 13: Fix imports and verify build**

Go's compiler will tell you exactly which imports each new file needs (unused-import errors) and which are missing (undefined-symbol errors) — run the build and fix iteratively:

Run: `go build -tags webkit2_41 ./...`
Expected: eventually PASS once each file's imports match what it actually uses. This is mechanical — a method that uses `slog` needs `"log/slog"` imported in whichever file it now lives in, etc.

- [ ] **Step 14: Confirm `app.go` itself is now small**

Run: `wc -l app.go`
Expected: roughly 90-110 lines (struct definition, `NewApp()`, `startup()`, `shutdown()`, `Version` var, `errNotConnected`).

- [ ] **Step 15: Run go vet**

Run: `go vet -tags webkit2_41 ./...`
Expected: only the pre-existing `internal/steam` unsafe.Pointer warnings (unrelated to this change, present on main already). No new warnings.

- [ ] **Step 16: Regenerate Wails bindings and do a full build**

Run: `wails generate module -tags webkit2_41` then `make build-linux`
Expected: both succeed. Check `git diff frontend/wailsjs/go/main/App.d.ts` — there should be NO content changes (only method locations in Go source changed, not signatures). If there's diff noise in `frontend/wailsjs/runtime/*` (mode-only 644→755 changes), discard those: `git checkout -- frontend/wailsjs/runtime/package.json frontend/wailsjs/runtime/runtime.d.ts frontend/wailsjs/runtime/runtime.js`.

- [ ] **Step 17: Commit**

```bash
git add app.go app_steam.go app_games.go app_achievements.go app_toplay.go app_hltb.go app_stats.go app_settings.go app_webapi.go app_image.go app_scan.go app_watchers.go app_version.go
git commit -m "Split app.go into per-domain files"
```

---

### Task 2: Deduplicate `achievement_service.go`

**Files:**
- Modify: `internal/services/achievement_service.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: nothing new consumed by other tasks. `LoadAchievementsFromSchema`, `LoadAchievements`, `SetAchievement`, `ClearAchievement`, `SetAllAchievements`, `ClearAllAchievements` keep their exact existing signatures (Task 1 already moved their callers to `app_achievements.go`; this task doesn't touch app.go/app_achievements.go at all).

- [ ] **Step 1: Read the current `loadFromSchema`/`loadFromAPI` pair**

Read `internal/services/achievement_service.go` in full first — the exact line numbers below are from the pre-Task-1 audit and may have shifted slightly; use them as a starting point and confirm against the current file content.

The two functions (originally ~lines 156-296) share: 4 identical `CString` creations with matching deferred cleanup, the same icon base URL construction, the same per-achievement loop building `models.Achievement` structs, and the same `resolveIconsFromSDK()` + percent-fetching calls. They differ only in where the raw achievement data comes from (a schema slice vs. an API index lookup) and in `loadFromAPI`'s schema-fallback logic.

- [ ] **Step 2: Extract the shared per-achievement builder**

Add a helper that takes a source-agnostic achievement description and produces the populated `models.Achievement`:

```go
// achievementSource is the minimal per-achievement data needed regardless of
// whether it came from the local schema or the Steam API.
type achievementSource struct {
	ID          string
	DisplayName string
	Description string
	Hidden      bool
}

// buildAchievement populates a models.Achievement from a source-agnostic
// description, resolving icons and unlock state via the SDK. Shared by
// loadFromSchema and loadFromAPI so icon/unlock-state logic lives in one place.
func (s *AchievementService) buildAchievement(src achievementSource, iconBaseURL string, csKeyName, csKeyDesc, csKeyHidden, csKeyIcon *steam.CString) models.Achievement {
	// Move the existing per-achievement body here verbatim from whichever of
	// loadFromSchema/loadFromAPI has the more complete version (check both —
	// they should be near-identical; if they differ, the schema version is
	// authoritative since it's the richer data source).
}
```

Adjust the exact parameter list to match what the existing loop body actually references (read the current code — don't guess at field names not yet confirmed in the file).

- [ ] **Step 3: Rewrite `loadFromSchema` and `loadFromAPI` to call the shared builder**

Both functions keep their existing signatures and existing "get the raw data" logic (schema slice iteration vs. API index lookup + schema fallback), but their loop bodies become a call to `s.buildAchievement(...)` instead of duplicating the full per-achievement logic.

- [ ] **Step 4: Extract the shared CString-key creation**

Replace the duplicated 4x `CString` creation + defer-cleanup block (present in both functions) with one helper:

```go
// achievementKeyStrings holds the CStrings needed to read achievement fields
// via the SDK, so both loadFromSchema and loadFromAPI create them once each
// instead of duplicating the pattern.
type achievementKeyStrings struct {
	name, desc, hidden, icon *steam.CString
}

func newAchievementKeyStrings() achievementKeyStrings {
	return achievementKeyStrings{
		name:   steam.NewCString("name"),
		desc:   steam.NewCString("desc"),
		hidden: steam.NewCString("hidden"),
		icon:   steam.NewCString("icon"),
	}
}

func (k achievementKeyStrings) free() {
	k.name.Free()
	k.desc.Free()
	k.hidden.Free()
	k.icon.Free()
}
```

(Adjust field/key names to match what's actually in the current file — the four keys above are illustrative; use the real ones.) Use it in both functions as: `keys := newAchievementKeyStrings(); defer keys.free()`.

- [ ] **Step 5: Extract the shared `SetAchievement`/`ClearAchievement` logic**

These two functions (originally ~lines 360-411) differ only in calling `userStats.SetAchievement(name)` vs `userStats.ClearAchievement(name)` and in whether `IsAchieved` becomes `true` (with a fresh `UnlockTime`) or `false` (with `UnlockTime` reset). Extract:

```go
// setAchievementState is shared by SetAchievement and ClearAchievement —
// both do the same validation, SDK call, and in-memory update, differing
// only in the target state.
func (s *AchievementService) setAchievementState(id string, achieved bool, sdkCall func(name string) bool) (bool, error) {
	// Move the existing shared validation/locking/logging body here,
	// calling sdkCall(name) where the two functions currently call
	// userStats.SetAchievement/ClearAchievement directly, and setting
	// IsAchieved/UnlockTime based on the achieved parameter.
}

func (s *AchievementService) SetAchievement(id string) (bool, error) {
	return s.setAchievementState(id, true, s.userStats.SetAchievement)
}

func (s *AchievementService) ClearAchievement(id string) (bool, error) {
	return s.setAchievementState(id, false, s.userStats.ClearAchievement)
}
```

Adjust to match the actual current signatures/fields — read the file first, don't assume `s.userStats` is the exact field name without confirming.

- [ ] **Step 6: Extract the shared `SetAllAchievements`/`ClearAllAchievements` logic**

Same pattern as Step 5, one level up — these bulk functions (originally ~lines 426-471) share the lock → loop-with-permission-check → operation → field-update → count → unlock structure. Extract a shared `updateAllAchievements(achieved bool, sdkCall func(string) bool) (int, error)` helper that both call with the appropriate direction, mirroring Step 5's pattern.

- [ ] **Step 7: Build and verify**

Run: `go build -tags webkit2_41 ./... && go vet -tags webkit2_41 ./...`
Expected: PASS, only the pre-existing `internal/steam` warnings.

- [ ] **Step 8: Manual smoke check**

Run: `make build-linux`
Expected: succeeds. (No automated test suite exists for this project — this refactor's correctness relies on the extraction being behavior-preserving, which is why each step above says "move the existing body" rather than rewriting logic from scratch.)

- [ ] **Step 9: Commit**

```bash
git add internal/services/achievement_service.go
git commit -m "Deduplicate achievement load/set/clear logic in achievement_service.go"
```

---

### Task 3: Deduplicate `internal/steam` binary/VDF parsing

**Files:**
- Create: `internal/steam/binary_reader.go`
- Modify: `internal/steam/userstats_cache.go`, `internal/steam/packageinfo.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: `binaryReader` type (shared low-level byte reader) consumed only within `internal/steam` by the two modified files.

- [ ] **Step 1: Read both existing readers**

Read `internal/steam/userstats_cache.go`'s `statsReader` (originally ~lines 180-227) and `internal/steam/packageinfo.go`'s `pkgReader` (originally ~lines 202-251) in full. Confirm their `readByte()`, `readU32()`, `readCString()`, `skip()` methods are functionally identical (same signatures, same logic) before merging — if they've diverged in any way beyond the type name, note the difference and keep it as a parameter rather than silently dropping it.

- [ ] **Step 2: Create the shared `binaryReader` type**

```go
package steam

import (
	"bytes"
	"errors"
	"strings"
)

// binaryReader is a minimal little-endian byte-slice reader shared by the
// VDF-derived binary formats this package parses (userstats cache, package
// info cache). Both formats use the same primitive read operations even
// though their higher-level structure differs.
type binaryReader struct {
	data []byte
	pos  int
}

func newBinaryReader(data []byte) *binaryReader {
	return &binaryReader{data: data}
}

func (r *binaryReader) readByte() (byte, error) {
	if r.pos >= len(r.data) {
		return 0, errors.New("binary reader: unexpected end of data")
	}
	b := r.data[r.pos]
	r.pos++
	return b, nil
}

func (r *binaryReader) readU32() (uint32, error) {
	if r.pos+4 > len(r.data) {
		return 0, errors.New("binary reader: unexpected end of data")
	}
	v := uint32(r.data[r.pos]) | uint32(r.data[r.pos+1])<<8 | uint32(r.data[r.pos+2])<<16 | uint32(r.data[r.pos+3])<<24
	r.pos += 4
	return v, nil
}

func (r *binaryReader) readCString() (string, error) {
	var sb strings.Builder
	for {
		b, err := r.readByte()
		if err != nil {
			return "", err
		}
		if b == 0 {
			return sb.String(), nil
		}
		sb.WriteByte(b)
	}
}

func (r *binaryReader) skip(n int) error {
	if r.pos+n > len(r.data) {
		return errors.New("binary reader: skip past end of data")
	}
	r.pos += n
	return nil
}
```

Reconcile this against the ACTUAL current implementations in both files (exact byte order, error messages, and buffer-vs-slice approach may differ from this sketch) — use their real logic, just merged into one type. Use `strings.Builder` for `readCString` (fixes the byte-by-byte-append inefficiency both original versions had) — confirm this doesn't change any observable behavior (it doesn't; it only changes the internal accumulation strategy).

- [ ] **Step 3: Replace `statsReader` with `binaryReader` in `userstats_cache.go`**

Update all call sites in `userstats_cache.go` that construct/use `statsReader` to use `binaryReader`/`newBinaryReader` instead. Remove the now-dead `statsReader` type and its methods.

- [ ] **Step 4: Replace `pkgReader` with `binaryReader` in `packageinfo.go`**

Same as Step 3 for `pkgReader` in `packageinfo.go`.

- [ ] **Step 5: Build and verify**

Run: `go build -tags webkit2_41 ./... && go vet -tags webkit2_41 ./...`
Expected: PASS, only pre-existing warnings.

- [ ] **Step 6: Commit**

```bash
git add internal/steam/binary_reader.go internal/steam/userstats_cache.go internal/steam/packageinfo.go
git commit -m "Extract shared binaryReader, removing duplicated statsReader/pkgReader"
```

**Note on VDF walker duplication:** the audit also flagged `walkVDFForAchievements()` (userstats_cache.go) and `vdfCollectAppIDs()` (packageinfo.go) as duplicated recursive walkers, and proposed a generic `WalkVDF()` with a visitor callback. That merge is higher-risk (the two walkers collect different things — achievement unlock records vs. app ID lists — and forcing them through one generic callback shape risks obscuring rather than clarifying the logic). Skip that specific merge in this pass; the `binaryReader` extraction above already removes the most mechanical, lowest-risk duplication. Revisit the VDF walker merge separately if it comes up again after this cleanup.

---

### Task 4: Generic cache abstraction + fix missing resets/flush in `internal/settings`

**Files:**
- Create: `internal/settings/json_cache.go`
- Modify: `internal/settings/achievement_cache.go`, `internal/settings/hltb_cache.go`, `internal/settings/percent_cache.go`, `internal/settings/game_cache.go`, `internal/settings/toplay.go`, `internal/settings/settings.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: `JSONCache[K, V]` generic type, used internally by the 5 cache files. Every existing exported function (`LoadHLTBCache`, `GetHLTBEntry`, `SaveHLTBEntry`, `LoadAchievementCache`, etc.) keeps its exact existing signature — `app.go`/`app_*.go` and the frontend are unaffected by this task.

- [ ] **Step 1: Read all 5 existing cache files in full**

Read `achievement_cache.go`, `hltb_cache.go`, `percent_cache.go`, `game_cache.go`, `toplay.go` before writing the generic abstraction — confirm the exact current lazy-load / lock / debounced-flush pattern each uses, since `game_cache.go` and `toplay.go` write synchronously (no debounce) while the other three debounce. The generic type needs to support both modes.

- [ ] **Step 2: Create `JSONCache[K comparable, V any]`**

```go
package settings

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// JSONCache is a lazily-loaded, in-memory map backed by a single JSON file,
// shared by every per-user cache in this package (achievements, HLTB times,
// global percents, game list, to-play list). Callers get a snapshot copy on
// Load/Get and never touch the internal map directly, so all locking lives
// here instead of being reimplemented per cache.
type JSONCache[K comparable, V any] struct {
	mu          sync.RWMutex
	data        map[K]V
	loaded      bool
	filePath    func() string
	flushDelay  time.Duration // 0 means write synchronously on every Set
	flushTimer  *time.Timer
	logLabel    string
}

func NewJSONCache[K comparable, V any](filePath func() string, flushDelay time.Duration, logLabel string) *JSONCache[K, V] {
	return &JSONCache[K, V]{filePath: filePath, flushDelay: flushDelay, logLabel: logLabel}
}

// Load returns a snapshot copy of the entire cache, loading from disk first if needed.
func (c *JSONCache[K, V]) Load() map[K]V {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureLoadedLocked()

	result := make(map[K]V, len(c.data))
	for k, v := range c.data {
		result[k] = v
	}
	return result
}

// Get returns a single entry, loading from disk first if needed.
func (c *JSONCache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureLoadedLocked()
	v, ok := c.data[key]
	return v, ok
}

// Set stores an entry and persists it — immediately if flushDelay is 0,
// otherwise via a debounced timer.
func (c *JSONCache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	c.ensureLoadedLocked()
	c.data[key] = value
	if c.flushDelay == 0 {
		data := c.snapshotLocked()
		c.mu.Unlock()
		c.writeToDisk(data)
		return
	}
	if c.flushTimer != nil {
		c.flushTimer.Stop()
	}
	c.flushTimer = time.AfterFunc(c.flushDelay, c.Flush)
	c.mu.Unlock()
}

// Flush forces an immediate write, canceling any pending debounce timer.
// Safe to call even when flushDelay is 0 (no-op timer state, still writes).
func (c *JSONCache[K, V]) Flush() {
	c.mu.Lock()
	if c.flushTimer != nil {
		c.flushTimer.Stop()
		c.flushTimer = nil
	}
	data := c.snapshotLocked()
	c.mu.Unlock()
	c.writeToDisk(data)
}

// Reset clears the in-memory cache (e.g. on Steam account switch) without
// touching the on-disk file for the previous user.
func (c *JSONCache[K, V]) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.flushTimer != nil {
		c.flushTimer.Stop()
		c.flushTimer = nil
	}
	c.data = nil
	c.loaded = false
}

func (c *JSONCache[K, V]) ensureLoadedLocked() {
	if c.loaded {
		return
	}
	c.data = make(map[K]V)
	c.loaded = true
	p := c.filePath()
	if p == "" {
		return
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return
	}
	if err := json.Unmarshal(raw, &c.data); err != nil {
		slog.Warn("failed to parse cache", "cache", c.logLabel, "error", err)
		c.data = make(map[K]V)
	}
}

func (c *JSONCache[K, V]) snapshotLocked() map[K]V {
	result := make(map[K]V, len(c.data))
	for k, v := range c.data {
		result[k] = v
	}
	return result
}

func (c *JSONCache[K, V]) writeToDisk(data map[K]V) {
	p := c.filePath()
	if p == "" {
		return
	}
	marshaled, err := json.Marshal(data)
	if err != nil {
		slog.Warn("failed to marshal cache", "cache", c.logLabel, "error", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		slog.Warn("failed to create cache dir", "cache", c.logLabel, "error", err)
		return
	}
	if err := atomicWrite(p, marshaled); err != nil {
		slog.Warn("failed to write cache", "cache", c.logLabel, "error", err)
	}
}
```

Reconcile the debounce delay value against the existing `cacheFlushDelay` constant (currently `const cacheFlushDelay = 500 * time.Millisecond` in `achievement_cache.go:20`) — move this constant's declaration into `json_cache.go` as its new canonical home (it's a `JSONCache`-level concern now, not achievement-specific), and delete it from `achievement_cache.go` once Step 6 rewrites that file. Since Go compiles all files in a package together, exactly which file declares a package-level const doesn't affect intermediate build steps within this task — only the final state matters, and the final state should have it in `json_cache.go`.

- [ ] **Step 3: Verify it builds standalone**

Run: `go build -tags webkit2_41 ./internal/settings/...`
Expected: PASS (this file has no callers yet, just needs to compile on its own).

- [ ] **Step 4: Migrate `hltb_cache.go` to use `JSONCache`**

Replace the package-level `hltbMu`/`hltbCache`/`hltbLoaded`/`hltbFlushTimer` vars and the `LoadHLTBCache`/`GetHLTBEntry`/`SaveHLTBEntry`/`flushHLTBCache` functions with:

```go
var hltbCacheStore = NewJSONCache[uint32, HLTBEntry](hltbCacheFilePath, cacheFlushDelay, "hltb")

func LoadHLTBCache() map[uint32]HLTBEntry {
	return hltbCacheStore.Load()
}

func GetHLTBEntry(appID uint32) (HLTBEntry, bool) {
	return hltbCacheStore.Get(appID)
}

func SaveHLTBEntry(appID uint32, entry HLTBEntry) {
	hltbCacheStore.Set(appID, entry)
}

// FlushHLTBCache forces an immediate write, for use at shutdown so a
// recently-fetched HLTB entry isn't lost if the app closes before the
// debounce timer fires.
func FlushHLTBCache() {
	hltbCacheStore.Flush()
}

func resetHLTBCache() {
	hltbCacheStore.Reset()
}
```

Keep `hltbCacheFilePath()` as-is (it's the `filePath func() string` passed to `NewJSONCache`). Keep the `HLTBEntry` struct definition unchanged.

- [ ] **Step 5: Migrate `percent_cache.go` to use `JSONCache`**

Same pattern as Step 4, adapted to `percent_cache.go`'s actual current function names (`LoadPercentEntry`/`SavePercentEntry` or whatever they currently are — read the file first, this task's earlier audit noted an `ensurePercentCache()` helper that becomes unnecessary once `JSONCache.ensureLoadedLocked` handles it). Add a `resetPercentCache()` function following the same shape as `resetHLTBCache()` above — this didn't exist before and is a genuine (small) fix, not just a refactor: see Step 8.

- [ ] **Step 6: Migrate `achievement_cache.go` to use `JSONCache`**

Same pattern, adapted to `achievement_cache.go`'s current API (`LoadAchievementCache`, `resetAchievementCache`, `ClearAchievementCache`, `ClearNonPerfectedCache`, `SaveAchievementEntry`, `FlushAchievementCache`). `ClearNonPerfectedCache` and the `SaveAchievementCounts*` wrapper variants are achievement-domain-specific logic on top of the generic cache — keep those as thin wrappers around the `JSONCache` instance rather than trying to generalize them further.

- [ ] **Step 7: Migrate `game_cache.go` and `toplay.go` to use `JSONCache`**

Same pattern, using `flushDelay: 0` for both (they currently write synchronously — preserve that behavior, don't introduce debouncing where there wasn't any before, since the design doc for the to-play feature explicitly settled on "always save without error" semantics).

- [ ] **Step 8: Fix the missing HLTB/percent cache resets on account switch**

In `settings.go`'s `SetCurrentUser`, add the two missing reset calls next to the existing ones:

```go
func SetCurrentUser(steamID uint64) {
	currentUserMu.Lock()
	currentUserID = steamID
	currentUserMu.Unlock()

	resetAchievementCache()
	resetGameCache()
	resetHLTBCache()
	resetPercentCache()
	resetToPlayList()
	reloadSettings()

	slog.Info("current user set", "steamID", steamID)
}
```

This is the one behavior change in this task: previously, switching Steam accounts mid-session left the in-memory HLTB and percent caches pointed at the previous account's data until the app restarted. Since HLTB times and global unlock percentages are per-game (not per-account) data, this was never a privacy issue, but it was a correctness inconsistency with the other three caches — worth fixing while touching this exact code path anyway.

- [ ] **Step 9: Call `FlushHLTBCache()` at shutdown**

Find where the app's shutdown path already flushes other debounced caches (search for existing `Flush*Cache()` calls in `app.go`'s `shutdown()` — after Task 1 this is now in the trimmed `app.go`) and add `settings.FlushHLTBCache()` alongside them, so a just-fetched HLTB entry isn't lost if the app closes before its debounce timer fires.

- [ ] **Step 10: Build and verify**

Run: `go build -tags webkit2_41 ./... && go vet -tags webkit2_41 ./...`
Expected: PASS, only pre-existing warnings.

- [ ] **Step 11: Manual smoke check**

Run: `make build-linux`
Expected: succeeds.

- [ ] **Step 12: Commit**

```bash
git add internal/settings/json_cache.go internal/settings/achievement_cache.go internal/settings/hltb_cache.go internal/settings/percent_cache.go internal/settings/game_cache.go internal/settings/toplay.go internal/settings/settings.go app.go
git commit -m "Extract generic JSONCache abstraction, fix missing HLTB/percent resets on account switch"
```

---

### Task 5: Split `GameManager.svelte`

**Files:**
- Modify: `frontend/src/lib/pages/GameManager.svelte`
- Create: `frontend/src/lib/components/HeroSection.svelte`, `frontend/src/lib/utils/achievement-updates.ts`

**Interfaces:**
- Consumes: nothing new from other tasks.
- Produces: `updateAchievementIsAchieved(list, id, achieved)` and `batchUpdateAchievements(list, ids, achieved)` from `achievement-updates.ts`, usable by any future component touching the achievements list (none currently need it beyond `GameManager.svelte`).

- [ ] **Step 1: Read the current file in full**

Read `frontend/src/lib/pages/GameManager.svelte` completely before making changes — line numbers below are from the pre-Task-1 audit and may have shifted; confirm against current content.

- [ ] **Step 2: Extract the achievement-update helper**

Create `frontend/src/lib/utils/achievement-updates.ts`:

```ts
import type { Achievement } from '../stores/achievements';

export function updateAchievementIsAchieved(
  list: Achievement[],
  id: string,
  achieved: boolean
): Achievement[] {
  return list.map(item =>
    item.id === id
      ? { ...item, isAchieved: achieved, unlockTime: achieved ? Math.floor(Date.now() / 1000) : 0 }
      : item
  );
}

export function batchUpdateAchievements(
  list: Achievement[],
  ids: Set<string>,
  achieved: boolean
): Achievement[] {
  return list.map(item =>
    ids.has(item.id)
      ? { ...item, isAchieved: achieved, unlockTime: achieved ? Math.floor(Date.now() / 1000) : 0 }
      : item
  );
}
```

Confirm the `Achievement` type's actual import path and field names (`isAchieved`, `unlockTime`) against `frontend/src/lib/stores/achievements.ts` before finalizing — adjust if they differ from this sketch.

- [ ] **Step 3: Replace the repeated inline update pattern in `GameManager.svelte`**

Find every place in the file that does `achievements.update(list => list.map(item => item.id === id ? {...item, isAchieved: ..., unlockTime: ...} : item))` (the single-achievement toggle/undo paths) and replace with `achievements.update(list => updateAchievementIsAchieved(list, id, achieved))`. Find every place doing the equivalent for a batch/selection (`Set<string>` of ids) and replace with `achievements.update(list => batchUpdateAchievements(list, idSet, achieved))`. Import both functions from `../utils/achievement-updates`.

- [ ] **Step 4: Verify behavior is unchanged so far**

Run: `cd frontend && npm run check`
Expected: 0 errors. This step only deduplicated existing logic — no behavior change.

- [ ] **Step 5: Commit the dedup separately from the component extraction**

```bash
git add frontend/src/lib/utils/achievement-updates.ts frontend/src/lib/pages/GameManager.svelte
git commit -m "Extract shared achievement-update helper from GameManager.svelte"
```

- [ ] **Step 6: Identify the hero section's exact boundaries**

Re-read the (now-updated) `GameManager.svelte` and find: the hero image carousel state/logic (image index, image-load-failure fallback, parallax scroll-driven height animation via requestAnimationFrame), the hero markup block (image, back/prev/next buttons, play/install/store buttons, the playtime+HLTB stats row, the settings-panel toggle button), and the `<SettingsPanel>` render call currently inline within that markup block.

- [ ] **Step 7: Create `HeroSection.svelte`**

Create `frontend/src/lib/components/HeroSection.svelte` with a `Props` interface covering what the hero section actually needs — build this from the CURRENT file's real state/markup, not from scratch. At minimum it needs: `appId`, `gameName`, `gameInstalled`, `playtimeHours`, `hltbLoading`, `hltbTimes`, `achievedCount`, `totalCount`, `hasMultipleGames`, `isFullscreen`, callback props for back/play/install/store/settings-toggle/prev/next, and a `settingsOpen` boolean (with the `<SettingsPanel>` moved inside this component, gated on that prop, so `GameManager.svelte` doesn't need to know about `SettingsPanel` directly). Move the hero-specific state (image index/failure tracking, scroll-driven height animation via `requestAnimationFrame`, the `lerp`/`smoothHeroHeight` logic) into this new component entirely — it becomes fully self-contained.

- [ ] **Step 8: Wire `HeroSection` into `GameManager.svelte`**

Replace the moved markup block with `<HeroSection {appId} {gameName} ... onBack={handleBack} onPlay={playGame} ... />`, passing the props identified in Step 7. Remove the now-dead state/functions from `GameManager.svelte` that moved into `HeroSection`.

- [ ] **Step 9: Verify**

Run: `cd frontend && npm run check`
Expected: 0 errors.

- [ ] **Step 10: Confirm file size reduction**

Run: `wc -l frontend/src/lib/pages/GameManager.svelte`
Expected: comfortably under 800 lines (roughly 700-750, per the audit's estimate — this task doesn't need to hit an exact number, just get well clear of the 800-line threshold).

- [ ] **Step 11: Manual smoke check**

Run: `make build-linux`
Expected: succeeds.

- [ ] **Step 12: Commit**

```bash
git add frontend/src/lib/components/HeroSection.svelte frontend/src/lib/pages/GameManager.svelte
git commit -m "Extract HeroSection.svelte from GameManager.svelte"
```

---

### Task 6: Split `SettingsPanel.svelte`

**Files:**
- Modify: `frontend/src/lib/components/SettingsPanel.svelte`
- Create: `frontend/src/lib/components/settings/GeneralSettingsTab.svelte`, `frontend/src/lib/components/settings/DisplaySettingsTab.svelte`, `frontend/src/lib/components/settings/AdvancedSettingsTab.svelte`

**Interfaces:**
- Consumes: nothing new.
- Produces: nothing consumed by other tasks — these are leaf UI components only `SettingsPanel.svelte` renders.

- [ ] **Step 1: Read the current file in full**

Read `frontend/src/lib/components/SettingsPanel.svelte` completely. Confirm its tab structure (general / display / advanced, or however it's currently organized) and exactly which local state variables and handler functions belong to each tab's concern versus which are genuinely panel-wide (e.g. `activeTab` selection itself, the open/close state passed in as a prop).

- [ ] **Step 2: Create `settings/GeneralSettingsTab.svelte`**

Extract the general-settings toggles (auto-save, lock, protect-last-played, and whatever else is grouped here in the current file) into this component. Props: whatever settings values + update callbacks this section actually reads/writes — read the current markup to determine the exact list rather than guessing.

- [ ] **Step 3: Create `settings/DisplaySettingsTab.svelte`**

Extract the display-settings toggles (labels, show-software, card-buttons, unlock-dates, etc.) the same way.

- [ ] **Step 4: Create `settings/AdvancedSettingsTab.svelte`**

Extract the advanced section: rescan/stop-scan controls, diagnostics (open data folder, view logs, reset settings), and the log-viewer modal UI (including its `logCopied` timeout-reset state — this can stay local to this new component since nothing outside it needs that state).

- [ ] **Step 5: Wire the three tabs into `SettingsPanel.svelte`**

`SettingsPanel.svelte` keeps: the overall open/close state, tab-selection state, and profile-visibility-check logic (since that's used for a status indicator visible across the panel, not specific to one tab — confirm this against the actual current code; if it turns out to be general-tab-specific instead, move it there in Step 2 rather than leaving it in the parent). It renders whichever of the three tab components matches the active tab, passing down props.

- [ ] **Step 6: Verify**

Run: `cd frontend && npm run check`
Expected: 0 errors.

- [ ] **Step 7: Confirm file size reduction**

Run: `wc -l frontend/src/lib/components/SettingsPanel.svelte`
Expected: well under 800 (it was 386 before this split — this task is about SRP clarity, not hitting a line-count target, since it was never actually over the 800-line threshold).

- [ ] **Step 8: Manual smoke check**

Run: `make build-linux`
Expected: succeeds.

- [ ] **Step 9: Commit**

```bash
git add frontend/src/lib/components/settings/ frontend/src/lib/components/SettingsPanel.svelte
git commit -m "Split SettingsPanel.svelte into per-tab components"
```

---

### Task 7: Deduplicate `GameCard.svelte`/`GameListRow.svelte` and `GameGrid.svelte`

**Files:**
- Modify: `frontend/src/lib/components/GameCard.svelte`, `frontend/src/lib/components/GameListRow.svelte`, `frontend/src/lib/components/GameGrid.svelte`
- Create: `frontend/src/lib/utils/game-card-state.ts`

**Interfaces:**
- Consumes: `achievementCounts`, `playtimes`, `hltbCache` stores from `../stores/games` (already used by both components).
- Produces: `deriveGameCardState(game)` helper, used by both `GameCard.svelte` and `GameListRow.svelte`.

- [ ] **Step 1: Read both components' current derived-state blocks**

Read `GameCard.svelte` and `GameListRow.svelte` in full. Confirm they currently duplicate: `counts` (from `achievementCounts`), `hasAchievementData`, `isFullyCompleted`, `playtimeHours` (from `playtimes`), `hltbMain`/`hltbCompletionist` (from `hltbCache`) — all keyed by `String(game.appId)`.

- [ ] **Step 2: Extract the shared derivation into a plain function**

Svelte 5's `$derived` runes only work inside component script blocks, so this can't be a reusable rune-returning function called from two places — instead, extract a plain function that both components call inside their own `$derived.by(...)`:

Create `frontend/src/lib/utils/game-card-state.ts`:

```ts
import type { AchievementCount } from '../stores/games';

export interface GameCardState {
  counts: AchievementCount | undefined;
  hasAchievementData: boolean;
  isFullyCompleted: boolean;
  playtimeHours: number | undefined;
  hltbMain: number | undefined;
  hltbCompletionist: number | undefined;
}

export function deriveGameCardState(
  appId: number,
  achievementCounts: Record<string, AchievementCount>,
  playtimes: Record<string, number>,
  hltbCache: Record<string, { main: number; mainExtra: number; completionist: number }>
): GameCardState {
  const counts = achievementCounts[String(appId)];
  const hasAchievementData = !!counts && counts.total > 0 && counts.achieved >= 0;
  return {
    counts,
    hasAchievementData,
    isFullyCompleted: hasAchievementData && counts!.achieved === counts!.total,
    playtimeHours: playtimes[String(appId)],
    hltbMain: hltbCache[String(appId)]?.main,
    hltbCompletionist: hltbCache[String(appId)]?.completionist,
  };
}
```

- [ ] **Step 3: Use it in `GameCard.svelte`**

Replace the individual `$derived` lines for `counts`/`hasAchievementData`/`isFullyCompleted`/`playtimeHours`/`hltbMain`/`hltbCompletionist` with:

```ts
let cardState = $derived(deriveGameCardState(game.appId, $achievementCounts, $playtimes, $hltbCache));
```

Then update every reference in the template/script from e.g. `counts` to `cardState.counts`, `isFullyCompleted` to `cardState.isFullyCompleted`, etc. — check every usage site in the file (progress bar width calc, badge rendering, footer Played/Main/100% spans) since this changes the accessor path.

- [ ] **Step 4: Use it in `GameListRow.svelte`**

Same as Step 3, updating this file's usages (the combined-label/title derivations that build on `playtimeHours`/`hltbMain`/`hltbCompletionist` now read from `cardState.playtimeHours` etc.).

- [ ] **Step 5: Verify both components**

Run: `cd frontend && npm run check`
Expected: 0 errors.

- [ ] **Step 6: Deduplicate `GameGrid.svelte`'s installed/other sections**

Read `GameGrid.svelte` in full. Confirm the installed-games block and other-games block are structurally identical (same `VirtualGrid` wrapper, same list/grid conditional, same child rendering) differing only in which array (`installedGames` vs `otherGames`) and toggle state/label. Extract a Svelte snippet:

```svelte
{#snippet gameSection(games, label, isOpen, onToggle)}
  <div>
    <button onclick={onToggle} class="sticky top-0 z-20 w-full flex items-center gap-2 px-4 py-2.5 bg-steam-bg/95 backdrop-blur-sm text-steam-text text-sm font-medium cursor-pointer hover:bg-steam-hover transition-colors border-b border-steam-border/50">
      <svg class="w-4 h-4 transition-transform duration-200 {isOpen ? 'rotate-90' : ''}" fill="currentColor" viewBox="0 0 20 20">
        <path d="M6.293 4.293a1 1 0 011.414 0L14 10.586l-6.293 6.293a1 1 0 01-1.414-1.414L11.172 10.586 6.293 5.707a1 1 0 010-1.414z" />
      </svg>
      {label}
      <span class="text-xs text-steam-text bg-steam-surface-light/80 px-1.5 py-0.5 rounded-full">{games.length}</span>
    </button>
    {#if isOpen}
      {#if $settings.viewMode === 'list'}
        <div>
          {@render listHeader()}
          <VirtualGrid {scrollContainer} items={games} columnCount={1} rowHeight={LIST_ROW_HEIGHT} keyFn={(game) => game.appId}>
            {#snippet children(game)}
              <GameListRow {game} />
            {/snippet}
          </VirtualGrid>
        </div>
      {:else}
        <div class="p-4">
          <VirtualGrid {scrollContainer} items={games} {columnCount} rowHeight={gridRowHeight} class="grid" style="grid-template-columns: repeat({columnCount}, 1fr); gap: {GAP}px" keyFn={(game) => game.appId}>
            {#snippet children(game)}
              <GameCard {game} />
            {/snippet}
          </VirtualGrid>
        </div>
      {/if}
    {/if}
  </div>
{/snippet}
```

Reconcile this against the file's ACTUAL current markup (exact classes, SVG path, snippet references) before replacing — the sketch above is based on the file's state as of the last full read in this conversation and may not match exactly if anything changed since. Then replace both the installed-games block and the other-games block with:

```svelte
{#if installedGames.length > 0}
  {@render gameSection(installedGames, 'Installed Games', $settings.installedOpen, toggleInstalled)}
{/if}
{#if otherGames.length > 0}
  {@render gameSection(otherGames, 'Other Games', $settings.otherOpen, toggleOther)}
{/if}
```

- [ ] **Step 7: Verify**

Run: `cd frontend && npm run check`
Expected: 0 errors.

- [ ] **Step 8: Manual smoke check**

Run: `make build-linux`
Expected: succeeds.

- [ ] **Step 9: Commit**

```bash
git add frontend/src/lib/utils/game-card-state.ts frontend/src/lib/components/GameCard.svelte frontend/src/lib/components/GameListRow.svelte frontend/src/lib/components/GameGrid.svelte
git commit -m "Deduplicate game-card derived state and GameGrid's installed/other sections"
```

---

### Task 8: Final verification and file-size confirmation

**Files:** none (verification only)

- [ ] **Step 1: Confirm no Go file exceeds 800 lines**

Run: `find . -name '*.go' -not -path './build/*' | xargs wc -l | sort -rn | head -5`
Expected: the largest file (likely `app_scan.go`) is comfortably under 800 lines.

- [ ] **Step 2: Confirm no frontend file exceeds 800 lines**

Run: `find frontend/src -name '*.svelte' -o -name '*.ts' | grep -v wailsjs | xargs wc -l | sort -rn | head -5`
Expected: the largest file is comfortably under 800 lines.

- [ ] **Step 3: Full backend verification**

Run: `go build -tags webkit2_41 ./... && go vet -tags webkit2_41 ./...`
Expected: PASS, only the pre-existing `internal/steam` unsafe.Pointer warnings.

- [ ] **Step 4: Full frontend verification**

Run: `cd frontend && npm run check`
Expected: 0 errors, 0 warnings.

- [ ] **Step 5: Full production build**

Run: `make build-linux`
Expected: succeeds, produces a working binary.

- [ ] **Step 6: Confirm working tree is clean**

Run: `git status --short`
Expected: no unexpected modifications (only the incidental `wailsjs/runtime` mode churn from the build step, if any — discard via `git checkout -- frontend/wailsjs/runtime/package.json frontend/wailsjs/runtime/runtime.d.ts frontend/wailsjs/runtime/runtime.js` if present with zero content diff).

- [ ] **Step 7: Manual behavioral spot-check (human, since no automated tests exist)**

Run `make dev` and confirm: the app connects to Steam, the library loads and shows achievement counts/playtime/HLTB data on cards as before, opening a game shows the hero section and achievement list correctly, toggling an achievement and saving still works, the settings panel's three tabs all show their expected controls, and switching Steam accounts (if testable) doesn't show stale HLTB/percent data from the previous account.
