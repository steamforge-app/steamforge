<script lang="ts">
  import { onDestroy } from 'svelte';
  import { selectedAppId, selectedGameName, selectedGameInstalled, addToast, isLoading, loadingMessage, navigateToPicker, profilePublic, gameComplete } from '../stores/app';
  import { achievements, achievementsLoading, cachePercents, applyCachedPercents } from '../stores/achievements';
  import { achievementCounts, navigableGames, type GameInfo } from '../stores/games';
  import { settings, updateSetting } from '../stores/settings';
  import AchievementList from '../components/AchievementList.svelte';
  import AchievementToolbar from '../components/AchievementToolbar.svelte';
  import ConfirmDialog from '../components/ConfirmDialog.svelte';
  import SettingsPanel from '../components/SettingsPanel.svelte';
  import {
    LoadAchievements, LoadAchievementsFromSchema, SetAchievement, ClearAchievement,
    ClearAllAchievements, StoreStats, DisconnectGame, FetchGlobalPercents, CheckGameEarlyAccess,
    GetHLTBTimes
  } from '../../../wailsjs/go/main/App';
  import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
  import { buildHeroImageUrls } from '../utils/steam-images';

  let confirmDialog = $state<{ title: string; message: string; confirmText?: string; action: () => void } | null>(null);
  let dirty = $state(false);
  let gameConnected = $state(false);
  let connecting = $state(false);
  let settingsOpen = $state(false);
  let settingsBtn: HTMLButtonElement | undefined = $state();
  let achievementSearchQuery = $state('');
  let achievementFilter = $state<'all' | 'locked' | 'unlocked' | 'hidden'>('all');
  let selectedIds = $state(new Set<string>());
  let lastSelectedId = $state<string | null>(null);
  let visibleAchievementIds = $state<string[]>([]);

  let originalState = $state<Map<string, boolean>>(new Map());

  let heroImageIndex = $state(0);
  let heroAllFailed = $state(false);
  let heroImageLoaded = $state(false);

  let hltbTimes = $state<{ main: number; mainExtra: number; completionist: number } | null>(null);
  let hltbLoading = $state(false);

  let heroImageUrls = $derived(buildHeroImageUrls($selectedAppId));
  let currentHeroSrc = $derived(heroImageUrls[heroImageIndex]);

  const HERO_MAX = 300;
  const HERO_MIN = 120;
  let heroHeight = $state(HERO_MAX);
  let scrollElement: HTMLDivElement | undefined = $state();

  let targetHeroHeight = HERO_MAX;
  let animationFrame: number | null = null;

  function handleContentScroll() {
    if (!scrollElement) return;
    targetHeroHeight = Math.max(HERO_MIN, HERO_MAX - scrollElement.scrollTop);
    if (!animationFrame) smoothHeroHeight();
  }

  function smoothHeroHeight() {
    const diff = targetHeroHeight - heroHeight;
    if (Math.abs(diff) < 0.5) {
      heroHeight = targetHeroHeight;
      animationFrame = null;
      return;
    }
    heroHeight += diff * 0.25;
    animationFrame = requestAnimationFrame(smoothHeroHeight);
  }

  let achievedCount = $derived($achievements.filter(achievement => achievement.isAchieved).length);
  let totalCount = $derived($achievements.length);
  let hasProtectedAchievements = $derived($achievements.some(a => a.permission > 0));
  let progressPercent = $derived(totalCount > 0 ? Math.round((achievedCount / totalCount) * 100) : 0);
  let pendingChanges = $derived.by(() => {
    if (originalState.size === 0) return 0;
    let count = 0;
    for (const achievement of $achievements) {
      if (originalState.get(achievement.id) !== achievement.isAchieved) count++;
    }
    return count;
  });
  let selectedCounts = $derived.by(() => {
    let locked = 0;
    let unlocked = 0;
    for (const achievement of $achievements) {
      if (!selectedIds.has(achievement.id)) continue;
      if (achievement.isAchieved) unlocked++;
      else locked++;
    }
    return { locked, unlocked };
  });
  let selectedLockedCount = $derived(selectedCounts.locked);
  let selectedUnlockedCount = $derived(selectedCounts.unlocked);

  let currentGameIndex = $derived($navigableGames.findIndex(g => g.appId === $selectedAppId));
  let previousGame = $derived(currentGameIndex > 0 ? $navigableGames[currentGameIndex - 1] : $navigableGames[$navigableGames.length - 1]);
  let nextGame = $derived(currentGameIndex < $navigableGames.length - 1 ? $navigableGames[currentGameIndex + 1] : $navigableGames[0]);
  let hasMultipleGames = $derived($navigableGames.length > 1);
  let innerWidth = $state(window.innerWidth);
  let isFullscreen = $derived(innerWidth < 1100);

  $effect(() => {
    $selectedAppId;
    heroImageIndex = 0;
    heroAllFailed = false;
    heroImageLoaded = false;
    heroHeight = HERO_MAX;
    targetHeroHeight = HERO_MAX;
    hltbTimes = null;
    hltbLoading = false;
    if (animationFrame) {
      cancelAnimationFrame(animationFrame);
      animationFrame = null;
    }
  });

  $effect(() => {
    const appId = $selectedAppId;
    const gameName = $selectedGameName;
    if (appId <= 0 || !gameName) return;
    hltbLoading = true;
    GetHLTBTimes(appId, gameName)
      .then(result => {
        if ($selectedAppId === appId) hltbTimes = result;
      })
      .catch((e: unknown) => { console.error('HLTB fetch failed:', e); })
      .finally(() => {
        if ($selectedAppId === appId) hltbLoading = false;
      });
  });

  $effect(() => {
    if (originalState.size === 0) {
      dirty = false;
      return;
    }
    const changed = $achievements.some(achievement => originalState.get(achievement.id) !== achievement.isAchieved);
    dirty = changed;
  });

  $effect(() => {
    gameComplete.set(achievedCount === totalCount && totalCount > 0);
  });

  let percentPollTimer: ReturnType<typeof setTimeout> | null = null;

  function stopPercentPolling() {
    if (percentPollTimer) {
      clearTimeout(percentPollTimer);
      percentPollTimer = null;
    }
  }

  function startPercentPolling() {
    stopPercentPolling();
    let attempt = 0;
    const maxAttempts = 6; // ~2s, 4s, 8s, 16s, 32s, 64s

    async function poll() {
      if ($achievements.length === 0) return;

      const hasMissingPercents = $achievements.some(a => a.percent === 0);
      if (!hasMissingPercents) return;

      attempt++;
      try {
        const percents = await FetchGlobalPercents($selectedAppId);
        if (percents && Object.keys(percents).length > 0) {
          achievements.update(list => {
            const updated = list.map(achievement => {
              if (achievement.percent === 0 && percents[achievement.id]) {
                return { ...achievement, percent: percents[achievement.id] };
              }
              return achievement;
            });
            cachePercents($selectedAppId, updated);
            return updated;
          });
          return; // Success — stop polling
        }
      } catch {
        // Will retry
      }

      if (attempt < maxAttempts) {
        const delay = Math.min(2000 * Math.pow(2, attempt - 1), 64000);
        percentPollTimer = setTimeout(poll, delay);
      }
    }

    // Start first poll after 2 seconds
    percentPollTimer = setTimeout(poll, 2000);
  }

  let loadDebounceTimer: ReturnType<typeof setTimeout> | null = null;

  $effect(() => {
    const appId = $selectedAppId;
    if (appId > 0) {
      if (loadDebounceTimer) clearTimeout(loadDebounceTimer);
      achievementsLoading.set(true);
      loadDebounceTimer = setTimeout(() => {
        loadDebounceTimer = null;
        loadAchievements();
        startPercentPolling();
      }, 350);
    }
  });

  onDestroy(() => {
    stopPercentPolling();
    if (animationFrame) cancelAnimationFrame(animationFrame);
    if (loadDebounceTimer) clearTimeout(loadDebounceTimer);
  });

  const earlyAccessChecked = new Set<number>();

  async function checkEarlyAccess() {
    const appId = $selectedAppId;
    if (earlyAccessChecked.has(appId)) return;
    const existing = $achievementCounts[String(appId)];
    if (existing?.earlyAccess) return;
    earlyAccessChecked.add(appId);
    try {
      const isEarlyAccess = await CheckGameEarlyAccess(appId);
      if (isEarlyAccess && appId === $selectedAppId) {
        achievementCounts.update(counts => {
          const current = counts[String(appId)] || { achieved: 0, total: 0 };
          return { ...counts, [String(appId)]: { ...current, earlyAccess: true } };
        });
      }
    } catch {
      earlyAccessChecked.delete(appId); // Retry on next open if failed
    }
  }

  function syncAchievementCounts() {
    const achieved = $achievements.filter(a => a.isAchieved).length;
    const total = $achievements.length;
    const isProtected = $achievements.some(a => a.permission > 0);
    achievementCounts.update(counts => {
      const existing = counts[String($selectedAppId)];
      return {
        ...counts,
        [String($selectedAppId)]: {
          achieved,
          total,
          earlyAccess: existing?.earlyAccess,
          protected: isProtected || existing?.protected,
        }
      };
    });
  }

  function snapshotState() {
    const snapshot = new Map<string, boolean>();
    for (const achievement of $achievements) {
      snapshot.set(achievement.id, achievement.isAchieved);
    }
    originalState = snapshot;
  }

  async function loadAchievements() {
    achievementsLoading.set(true);
    achievementSearchQuery = '';
    selectedIds = new Set();
    if (gameConnected) {
      try { await DisconnectGame(); } catch {}
    }
    gameConnected = false;

    // Skip loading entirely for games already known to have 0 achievements
    const cachedCount = $achievementCounts[String($selectedAppId)];
    if (cachedCount && cachedCount.total === 0) {
      achievements.set([]);
      snapshotState();
      achievementsLoading.set(false);
      checkEarlyAccess();
      return;
    }

    try {
      if ($profilePublic !== 'private') {
        try {
          const result = await LoadAchievementsFromSchema($selectedAppId);
          if (result && result.length > 0) {
            const withPercents = applyCachedPercents($selectedAppId, result);
            cachePercents($selectedAppId, withPercents);
            achievements.set(withPercents);
            snapshotState();
            syncAchievementCounts();
            return;
          }
        } catch {
          // Community unavailable — fall through to Steam client
        }
      }
      // Community unavailable — connect to Steam client unless protected
      if ($settings.protectLastPlayed) {
        achievements.set([]);
        snapshotState();
      } else {
        try {
          const result = await LoadAchievements($selectedAppId);
          const loaded = result || [];
          const withPercents = applyCachedPercents($selectedAppId, loaded);
          cachePercents($selectedAppId, withPercents);
          achievements.set(withPercents);
          snapshotState();
          syncAchievementCounts();
          gameConnected = true;
        } catch (e: any) {
          // Game may not support achievements — disconnect to avoid staying "in-game"
          try { await DisconnectGame(); } catch {}
          achievements.set([]);
          snapshotState();
          addToast(`Failed to load achievements: ${e.message || e}`, 'error');
        }
      }
    } finally {
      achievementsLoading.set(false);
      prefetchAdjacentGames();
      checkEarlyAccess();
    }
  }

  async function ensureConnected(): Promise<boolean> {
    if (gameConnected) return true;
    connecting = true;
    try {
      // Capture pending changes before reconnecting
      const pendingChanges = new Map<string, boolean>();
      for (const achievement of $achievements) {
        const original = originalState.get(achievement.id);
        if (original !== undefined && original !== achievement.isAchieved) {
          pendingChanges.set(achievement.id, achievement.isAchieved);
        }
      }

      const result = await LoadAchievements($selectedAppId);
      const loaded = result || [];

      // Capture the Steam-side state directly from the loaded data (not via
      // $achievements) so the diff in handleStore has the true baseline.
      const steamSnapshot = new Map<string, boolean>();
      for (const achievement of loaded) {
        steamSnapshot.set(achievement.id, achievement.isAchieved);
      }

      achievements.set(loaded);
      gameConnected = true;

      // Re-apply pending changes so handleStore can detect them
      if (pendingChanges.size > 0) {
        achievements.update(list => list.map(item => {
          const pending = pendingChanges.get(item.id);
          if (pending !== undefined) {
            return { ...item, isAchieved: pending, unlockTime: pending ? item.unlockTime : 0 };
          }
          return item;
        }));
      }

      // Set originalState to the Steam-saved state (not the pending state),
      // so handleStore's diff correctly detects all changes to apply.
      originalState = steamSnapshot;

      return true;
    } catch (e: any) {
      addToast(`Failed to connect to game: ${e.message || e}`, 'error');
      return false;
    } finally {
      connecting = false;
    }
  }

  async function autoStoreIfEnabled() {
    if (!$settings.autoStore) return;
    try {
      const ok = await StoreStats();
      if (ok) {
        snapshotState();
        syncAchievementCounts();
      } else {
        addToast('Auto-save failed — changes may not have been saved', 'error');
      }
    } catch (e: any) {
      addToast(`Auto-save failed: ${e.message || e}`, 'error');
    }
  }

  async function handleToggleAchievement(id: string, achieved: boolean) {
    if ($settings.autoStore) {
      if (!await ensureConnected()) return;
      try {
        const ok = achieved ? await SetAchievement(id) : await ClearAchievement(id);
        if (!ok) {
          addToast(`Failed to ${achieved ? 'unlock' : 'lock'} achievement`, 'error');
          return;
        }
        achievements.update(list => list.map(item =>
          item.id === id ? { ...item, isAchieved: achieved, unlockTime: achieved ? Math.floor(Date.now() / 1000) : 0 } : item
        ));
        await autoStoreIfEnabled();
        const name = $achievements.find(a => a.id === id)?.name || id;
        const actionLabel = achieved ? 'Unlocked' : 'Locked';
        addToast(`${actionLabel} ${name}`, 'success', {
          label: 'Undo',
          callback: async () => {
            try {
              if (achieved) {
                await ClearAchievement(id);
              } else {
                await SetAchievement(id);
              }
              achievements.update(list => list.map(item =>
                item.id === id ? { ...item, isAchieved: !achieved, unlockTime: !achieved ? Math.floor(Date.now() / 1000) : 0 } : item
              ));
              await autoStoreIfEnabled();
            } catch (e: any) {
              addToast(`Undo failed: ${e.message || e}`, 'error');
            }
          }
        });
      } catch (e: any) {
        addToast(`Failed to update achievement: ${e.message || e}`, 'error');
      }
    } else {
      // Defer SDK calls — only update the UI
      achievements.update(list => list.map(item =>
        item.id === id ? { ...item, isAchieved: achieved, unlockTime: achieved ? item.unlockTime : 0 } : item
      ));
    }
  }

  function handleSelect(id: string, shiftKey: boolean = false) {
    selectedIds = new Set(selectedIds);
    if (shiftKey && lastSelectedId && lastSelectedId !== id) {
      const lastIdx = visibleAchievementIds.indexOf(lastSelectedId);
      const curIdx = visibleAchievementIds.indexOf(id);
      if (lastIdx !== -1 && curIdx !== -1) {
        const start = Math.min(lastIdx, curIdx);
        const end = Math.max(lastIdx, curIdx);
        for (let i = start; i <= end; i++) {
          selectedIds.add(visibleAchievementIds[i]);
        }
        lastSelectedId = id;
        return;
      }
    }
    if (selectedIds.has(id)) {
      selectedIds.delete(id);
    } else {
      selectedIds.add(id);
    }
    lastSelectedId = id;
  }

  function handleSelectAll() {
    selectedIds = new Set(visibleAchievementIds);
  }

  function handleClearSelection() {
    selectedIds = new Set();
  }

  async function handleUnlockSelected() {
    const editable = [...selectedIds].filter(id => {
      const match = $achievements.find(a => a.id === id);
      return match && !match.permission;
    });
    const skipped = selectedIds.size - editable.length;

    if ($settings.autoStore) {
      if (!await ensureConnected()) return;
      let failed = 0;
      for (const id of editable) {
        const match = $achievements.find(achievement => achievement.id === id);
        if (match && !match.isAchieved) {
          const ok = await SetAchievement(id);
          if (!ok) failed++;
        }
      }
      if (failed > 0) {
        addToast(`${failed} achievement(s) failed to unlock`, 'error');
        return;
      }
    }
    const editableSet = new Set(editable);
    achievements.update(list => list.map(item =>
      editableSet.has(item.id) ? { ...item, isAchieved: true } : item
    ));
    const label = skipped > 0 ? `Unlocked ${editable.length} achievements (${skipped} protected, skipped)` : `Unlocked ${editable.length} achievements`;
    addToast(label, 'success');
    selectedIds = new Set();
    if ($settings.autoStore) await autoStoreIfEnabled();
  }

  async function handleLockSelected() {
    const editable = [...selectedIds].filter(id => {
      const match = $achievements.find(a => a.id === id);
      return match && !match.permission;
    });
    const skipped = selectedIds.size - editable.length;

    if ($settings.autoStore) {
      if (!await ensureConnected()) return;
      let failed = 0;
      for (const id of editable) {
        const match = $achievements.find(achievement => achievement.id === id);
        if (match && match.isAchieved) {
          const ok = await ClearAchievement(id);
          if (!ok) failed++;
        }
      }
      if (failed > 0) {
        addToast(`${failed} achievement(s) failed to lock`, 'error');
        return;
      }
    }
    const editableSet = new Set(editable);
    achievements.update(list => list.map(item =>
      editableSet.has(item.id) ? { ...item, isAchieved: false, unlockTime: 0 } : item
    ));
    const label = skipped > 0 ? `Locked ${editable.length} achievements (${skipped} protected, skipped)` : `Locked ${editable.length} achievements`;
    addToast(label, 'success');
    selectedIds = new Set();
    if ($settings.autoStore) await autoStoreIfEnabled();
  }

  function handleLockAll() {
    const protectedCount = $achievements.filter(a => a.permission > 0).length;
    const protectedNote = protectedCount > 0 ? ` ${protectedCount} protected achievement(s) will be skipped.` : '';
    const autoSaveMessage = $settings.autoStore ? '' : ' Changes will not be saved until you click "Save".';
    confirmDialog = {
      title: 'Lock All Achievements',
      message: `This will lock all editable achievements.${protectedNote}${autoSaveMessage}`,
      action: async () => {
        confirmDialog = null;
        if ($settings.autoStore) {
          if (!await ensureConnected()) return;
          try {
            const count = await ClearAllAchievements();
            achievements.update(list => list.map(item => item.permission > 0 ? item : { ...item, isAchieved: false, unlockTime: 0 }));
            addToast(`Locked ${count} achievements`, 'success');
            await autoStoreIfEnabled();
          } catch (e: any) {
            addToast(`Failed: ${e.message || e}`, 'error');
          }
        } else {
          const count = $achievements.filter(a => a.isAchieved && !a.permission).length;
          achievements.update(list => list.map(item => item.permission > 0 ? item : { ...item, isAchieved: false, unlockTime: 0 }));
          addToast(`Locked ${count} achievements`, 'success');
        }
      }
    };
  }

  function handleDiscardChanges() {
    achievements.update(list => list.map(item => {
      const original = originalState.get(item.id);
      if (original === undefined || original === item.isAchieved) return item;
      return { ...item, isAchieved: original, unlockTime: original ? item.unlockTime : 0 };
    }));
    addToast('Changes discarded', 'success');
  }

  async function handleStore() {
    // Capture changes BEFORE ensureConnected(), which may replace
    // both $achievements and originalState during reconnection.
    const changesToApply: Array<{id: string; achieved: boolean}> = [];
    for (const achievement of $achievements) {
      const original = originalState.get(achievement.id);
      if (original !== undefined && original !== achievement.isAchieved) {
        changesToApply.push({ id: achievement.id, achieved: achievement.isAchieved });
      }
    }
    if (changesToApply.length === 0) {
      addToast('No changes detected to save', 'error');
      return;
    }

    if (!await ensureConnected()) return;
    isLoading.set(true);
    loadingMessage.set('Storing changes...');
    try {
      let applied = 0;
      let failed = 0;
      for (const change of changesToApply) {
        if (change.achieved) {
          const ok = await SetAchievement(change.id);
          if (ok) applied++; else failed++;
        } else {
          const ok = await ClearAchievement(change.id);
          if (ok) applied++; else failed++;
        }
      }
      if (failed > 0) {
        addToast(`${failed} achievement(s) failed to update in Steam SDK`, 'error');
        return;
      }
      const ok = await StoreStats();
      if (ok) {
        addToast('Changes saved', 'success');
        snapshotState();
        syncAchievementCounts();
        DisconnectGame().catch(() => {});
        gameConnected = false;
      } else {
        addToast('Store returned false - changes may not have been saved', 'error');
      }
    } catch (e: any) {
      addToast(`Failed to store: ${e.message || e}`, 'error');
    } finally {
      isLoading.set(false);
      loadingMessage.set('');
    }
  }

  async function goBack() {
    gameComplete.set(false);
    navigateToPicker();
    // Disconnect in background — don't block the UI
    if (gameConnected) {
      DisconnectGame().catch(() => {});
    }
  }

  async function switchToGame(game: GameInfo) {
    if (dirty) {
      confirmDialog = {
        title: 'Unsaved Changes',
        message: 'You have unsaved changes. Switching games will discard them.',
        confirmText: 'Discard & Switch',
        action: () => {
          confirmDialog = null;
          performGameSwitch(game);
        }
      };
      return;
    }
    performGameSwitch(game);
  }

  function performGameSwitch(game: GameInfo) {
    stopPercentPolling();
    if (gameConnected) {
      DisconnectGame().catch(() => {});
    }
    gameConnected = false;
    gameComplete.set(false);
    if (scrollElement) scrollElement.scrollTop = 0;
    selectedAppId.set(game.appId);
    selectedGameName.set(game.name);
    selectedGameInstalled.set(game.installed);
  }

  function handleKeyNavigation(e: KeyboardEvent) {
    if (!hasMultipleGames) return;
    if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) return;
    if (e.key === 'ArrowLeft') {
      e.preventDefault();
      switchToPrevious();
    } else if (e.key === 'ArrowRight') {
      e.preventDefault();
      switchToNext();
    }
  }

  function prefetchAdjacentGames() {
    if (!hasMultipleGames) return;
    const gamesToPrefetch = [previousGame, nextGame].filter(Boolean);
    for (const game of gamesToPrefetch) {
      LoadAchievementsFromSchema(game.appId)
        .then(result => {
          if (result && result.length > 0) {
            cachePercents(game.appId, result);
          }
        })
        .catch(() => {});
      // Prefetch hero images so they display instantly on switch
      for (const url of buildHeroImageUrls(game.appId)) {
        new Image().src = url;
      }
    }
  }

  export function requestClose() {
    handleBack();
  }

  export function switchToPrevious() {
    if (!hasMultipleGames) return;
    switchToGame(previousGame);
  }

  export function switchToNext() {
    if (!hasMultipleGames) return;
    switchToGame(nextGame);
  }

  function handleBack() {
    if (dirty) {
      confirmDialog = {
        title: 'Unsaved Changes',
        message: 'You have unsaved achievement changes. Going back will discard them.',
        confirmText: 'Discard & Go Back',
        action: () => {
          confirmDialog = null;
          goBack();
        }
      };
    } else {
      goBack();
    }
  }

  function openStorePage() {
    BrowserOpenURL(`steam://store/${$selectedAppId}`);
  }

  function playGame() {
    BrowserOpenURL(`steam://rungameid/${$selectedAppId}`);
  }

  function installGame() {
    BrowserOpenURL(`steam://install/${$selectedAppId}`);
  }
</script>

<svelte:window onkeydown={handleKeyNavigation} bind:innerWidth />

<div class="flex flex-col flex-1 overflow-hidden">
  <div
    class="relative overflow-hidden bg-steam-surface shrink-0"
    style="height: {heroHeight}px; will-change: height"
  >
    {#if !heroAllFailed}
      <img
        src={currentHeroSrc}
        alt={$selectedGameName}
        class="w-full h-full object-cover transition-opacity duration-200 {heroImageLoaded ? 'opacity-100' : 'opacity-0'}"
        onload={() => heroImageLoaded = true}
        onerror={() => {
          if (heroImageIndex < heroImageUrls.length - 1) {
            heroImageIndex++;
          } else {
            heroAllFailed = true;
          }
        }}
      />
    {/if}
    <div
      class="absolute inset-0"
      style="background: linear-gradient(to top, rgba(27,40,56,1) 0%, rgba(27,40,56,0.7) 40%, transparent 100%)"
    ></div>
    <div class="absolute top-4 left-4 flex items-center gap-1.5">
      {#if hasMultipleGames && isFullscreen}
        <button
          onclick={() => switchToPrevious()}
          class="p-2 rounded-full bg-black/40 text-white/60 hover:text-white hover:bg-black/60
                 transition-colors cursor-pointer"
          title={previousGame?.name}
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
          </svg>
        </button>
      {/if}
      <button
        onclick={handleBack}
        class="p-2 rounded-full bg-black/40 text-white/80 hover:text-white hover:bg-black/60 transition-colors cursor-pointer"
        title="Close"
      >
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>
    <div class="absolute top-4 right-4 flex items-center gap-1.5">
      {#if $selectedGameInstalled}
        <button
          onclick={playGame}
          class="px-2.5 py-1 rounded bg-steam-success/80 text-white hover:bg-steam-success text-xs font-medium transition-colors cursor-pointer"
          title="Launch game via Steam"
        >
          Play
        </button>
      {:else}
        <button
          onclick={installGame}
          class="px-2.5 py-1 rounded bg-steam-primary/80 text-white hover:bg-steam-primary text-xs font-medium transition-colors cursor-pointer flex items-center gap-1"
          title="Install game via Steam"
        >
          <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
          </svg>
          Install
        </button>
      {/if}
      <button
        onclick={openStorePage}
        class="px-2.5 py-1 rounded bg-black/40 text-white/70 hover:text-white text-xs transition-colors cursor-pointer"
        title="Open Steam store page"
      >
        Store Page
      </button>
      <div class="relative">
        <button
          bind:this={settingsBtn}
          onclick={() => settingsOpen = !settingsOpen}
          title="Settings"
          class="p-1.5 rounded-full transition-colors cursor-pointer
            {settingsOpen ? 'bg-white/30 text-white' : 'bg-black/40 text-white/70 hover:text-white hover:bg-black/60'}"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
        </button>
        {#if settingsOpen}
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div class="fixed inset-0 z-40" role="none" onclick={() => settingsOpen = false}></div>
          <div
            class="fixed z-50 bg-steam-surface border border-steam-border rounded-lg shadow-lg p-4 w-64"
            style="top: {(settingsBtn?.getBoundingClientRect().bottom ?? 0) + 4}px; right: {window.innerWidth - (settingsBtn?.getBoundingClientRect().right ?? 0)}px"
          >
            <SettingsPanel onclose={() => settingsOpen = false} />
          </div>
        {/if}
      </div>
      {#if hasMultipleGames && isFullscreen}
        <button
          onclick={() => switchToNext()}
          class="p-2 rounded-full bg-black/40 text-white/60 hover:text-white hover:bg-black/60
                 transition-colors cursor-pointer"
          title={nextGame?.name}
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
          </svg>
        </button>
      {/if}
    </div>
    <div class="absolute bottom-4 left-6 flex flex-col gap-1">
      <div class="flex items-center gap-2">
        <h1
          class="text-2xl font-bold text-white"
          style="text-shadow: 0 2px 8px rgba(0,0,0,0.7)"
        >
          {$selectedGameName}
        </h1>
        {#if $achievementCounts[String($selectedAppId)]?.earlyAccess}
          <span class="px-2 py-0.5 text-xs font-medium rounded bg-blue-500/80 text-white" style="text-shadow: 0 1px 2px rgba(0,0,0,0.5)">
            Early Access
          </span>
        {/if}
      </div>
      {#if hltbLoading}
        <div class="flex items-center gap-1.5">
          <div class="w-3 h-3 border border-white/40 border-t-transparent rounded-full animate-spin"></div>
          <span class="text-xs text-white/40" style="text-shadow: 0 1px 4px rgba(0,0,0,0.7)">Loading times...</span>
        </div>
      {:else if hltbTimes && (hltbTimes.main > 0 || hltbTimes.mainExtra > 0 || hltbTimes.completionist > 0)}
        <div class="flex items-center gap-3 text-xs text-white/60" style="text-shadow: 0 1px 4px rgba(0,0,0,0.7)">
          {#if hltbTimes.main > 0}
            <span title="Main story"><span class="text-white/35">Main</span> {hltbTimes.main}h</span>
          {/if}
          {#if hltbTimes.mainExtra > 0}
            <span title="Main + Extras"><span class="text-white/35">+Extras</span> {hltbTimes.mainExtra}h</span>
          {/if}
          {#if hltbTimes.completionist > 0}
            <span title="Completionist"><span class="text-white/35">100%</span> {hltbTimes.completionist}h</span>
          {/if}
        </div>
      {/if}
    </div>
    {#if totalCount > 0 && !$achievementsLoading}
      <div class="absolute bottom-4 right-6 flex items-center gap-3">
        <span class="text-sm {achievedCount === totalCount ? 'text-amber-400 font-medium' : 'text-steam-text-dim'}" style="text-shadow: 0 1px 4px rgba(0,0,0,0.7)">
          {achievedCount}/{totalCount} ({progressPercent}%)
        </span>
        <div class="w-32 h-2 bg-black/40 rounded-full overflow-hidden">
          <div
            class="h-full rounded-full transition-all duration-300 {achievedCount === totalCount ? 'gold-bar' : 'bg-steam-primary'}"
            style="width: {progressPercent}%"
          ></div>
        </div>
      </div>
    {/if}
  </div>

  {#if $achievementsLoading}
    <div class="flex-1 flex items-center justify-center">
      <div class="flex flex-col items-center gap-3">
        <div class="w-8 h-8 border-2 border-steam-primary border-t-transparent rounded-full animate-spin"></div>
        <p class="text-steam-text-dim text-sm">Loading achievements...</p>
      </div>
    </div>
  {:else if totalCount === 0}
    {@const isKnownEarlyAccess = $achievementCounts[String($selectedAppId)]?.earlyAccess}
    <div class="flex-1 flex items-center justify-center">
      <div class="flex flex-col items-center gap-4 text-center px-8">
        {#if isKnownEarlyAccess}
          <svg class="w-16 h-16 text-blue-400/40" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <div>
            <p class="text-steam-text text-sm font-medium">Early Access Game</p>
            <p class="text-steam-text-dim text-xs mt-1">This game is in early access and doesn't have achievements yet.</p>
            <p class="text-steam-text-dim text-xs">Achievements may be added in a future update.</p>
          </div>
        {:else}
          <svg class="w-16 h-16 text-steam-text-dim/40" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z" />
          </svg>
          <div>
            <p class="text-steam-text text-sm font-medium">No Achievements</p>
            <p class="text-steam-text-dim text-xs mt-1">This game doesn't have any achievements yet.</p>
            <p class="text-steam-text-dim text-xs">It may be in early access or not support achievements.</p>
          </div>
        {/if}
        <button
          onclick={handleBack}
          class="mt-2 px-4 py-2 text-xs rounded-lg bg-steam-surface-light text-steam-text-dim hover:text-steam-text transition-colors cursor-pointer"
        >
          Back to Games
        </button>
      </div>
    </div>
  {:else}
    <AchievementToolbar
      onlockall={handleLockAll}
      onrefresh={loadAchievements}
      searchQuery={achievementSearchQuery}
      onsearch={(query) => achievementSearchQuery = query}
      filterMode={achievementFilter}
      onfilter={(mode) => achievementFilter = mode as typeof achievementFilter}
      sortMode={$settings.achievementSort}
      sortDirection={$settings.achievementSortDir}
      onsort={(mode) => {
        if ($settings.achievementSort === mode) {
          updateSetting('achievementSortDir', $settings.achievementSortDir === 'asc' ? 'desc' : 'asc');
        } else {
          updateSetting('achievementSort', mode as typeof $settings.achievementSort);
          const defaultDesc = mode === 'unlockTime' || mode === 'percent';
          updateSetting('achievementSortDir', defaultDesc ? 'desc' : 'asc');
        }
      }}
      selectedCount={selectedIds.size}
      {selectedLockedCount}
      {selectedUnlockedCount}
      onlockselected={handleLockSelected}
      onunlockselected={handleUnlockSelected}
      onselectall={handleSelectAll}
      onclearselection={handleClearSelection}
      allowLock={$settings.allowLock}
      visibleCount={visibleAchievementIds.length}
      {totalCount}
    />
    {#if hasProtectedAchievements}
      <div class="px-4 py-2 bg-amber-400/5 border-b border-amber-400/20 flex items-center gap-2 shrink-0">
        <svg class="w-4 h-4 text-amber-400/70 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
        </svg>
        <span class="text-xs text-amber-400/70">This game uses server-side achievement protection. Achievements cannot be edited.</span>
      </div>
    {/if}
    <div bind:this={scrollElement} onscroll={handleContentScroll} class="flex-1 min-h-0 overflow-y-auto">
      <AchievementList
        ontoggle={handleToggleAchievement}
        searchQuery={achievementSearchQuery}
        filterMode={achievementFilter}
        sortMode={$settings.achievementSort}
        sortDirection={$settings.achievementSortDir}
        {selectedIds}
        onselect={handleSelect}
        allowLock={$settings.allowLock}
        showUnlockDates={$settings.showUnlockDates}
        {originalState}
        onvisiblechange={(ids) => visibleAchievementIds = ids}
      />
    </div>
    {#if !$settings.autoStore && dirty}
      <div class="px-4 py-3 bg-steam-surface/95 backdrop-blur-sm border-t border-steam-border flex items-center justify-between shrink-0">
        <span class="text-xs text-steam-text-dim">
          {pendingChanges} unsaved {pendingChanges === 1 ? 'change' : 'changes'}
        </span>
        <div class="flex items-center gap-2">
          <button
            onclick={handleDiscardChanges}
            class="px-4 py-2 text-sm rounded-lg font-medium transition-colors cursor-pointer bg-steam-surface-light text-steam-text-dim hover:text-steam-text"
          >
            Discard
          </button>
          <button
            onclick={handleStore}
            class="px-5 py-2 text-sm rounded-lg font-medium transition-colors cursor-pointer bg-steam-primary text-steam-bg hover:bg-steam-primary-dark"
          >
            Save
          </button>
        </div>
      </div>
    {/if}
  {/if}
</div>

{#if confirmDialog}
  <ConfirmDialog
    title={confirmDialog.title}
    message={confirmDialog.message}
    confirmText={confirmDialog.confirmText}
    onconfirm={confirmDialog.action}
    oncancel={() => confirmDialog = null}
  />
{/if}

