<script lang="ts">
  import { onMount } from 'svelte';
  import { fade } from 'svelte/transition';
  import { currentPage, isConnected, steamId, personaName, scanning, scanProgress, profilePublic, gameComplete, selectedAppId, addToast } from './lib/stores/app';
  import { games, achievementCounts, navigableGames } from './lib/stores/games';
  import { settings } from './lib/stores/settings';
  import { achievementsLoading } from './lib/stores/achievements';
  import Toast from './lib/components/Toast.svelte';
  import GamePicker from './lib/pages/GamePicker.svelte';
  import GameManager from './lib/pages/GameManager.svelte';
  import { CheckForUpdates, GetAppVersion } from '../wailsjs/go/main/App';
  import { EventsOn, BrowserOpenURL } from '../wailsjs/runtime/runtime';

  let appVersion = $state('');
  let updateInfo = $state<{ updateAvailable: boolean; latestVersion: string; currentVersion: string; downloadUrl: string } | null>(null);

  let innerWidth = $state(window.innerWidth);
  let isModal = $derived(innerWidth >= 1100);
  let gameManager = $state<GameManager>();

  let currentGameIndex = $derived($navigableGames.findIndex(g => g.appId === $selectedAppId));
  let hasMultipleGames = $derived($navigableGames.length > 1 && currentGameIndex >= 0);
  let previousGame = $derived(currentGameIndex > 0 ? $navigableGames[currentGameIndex - 1] : $navigableGames[$navigableGames.length - 1]);
  let nextGame = $derived(currentGameIndex < $navigableGames.length - 1 ? $navigableGames[currentGameIndex + 1] : $navigableGames[0]);

  function handleOverlayKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape' && $currentPage === 'manager') {
      gameManager?.requestClose();
    }
  }

  function handleBackdropClick(e: MouseEvent) {
    if (isModal && e.target === e.currentTarget) {
      gameManager?.requestClose();
    }
  }

  // Scan event listeners live at the app level so they persist across page navigation.
  onMount(() => {
    GetAppVersion().then(v => appVersion = v).catch(() => {});
    CheckForUpdates().then(info => updateInfo = info).catch(() => {});
    EventsOn('scan-progress', (data: any) => {
      scanProgress.set({ current: data.current, total: data.total, name: data.name });
    });
    EventsOn('scan-counts', (data: any) => {
      achievementCounts.update(counts => ({
        ...counts,
        [String(data.appId)]: { achieved: data.achieved, total: data.total, earlyAccess: data.earlyAccess || false, releaseDate: data.releaseDate || '' }
      }));
    });
    EventsOn('scan-complete', () => {
      scanning.set(false);
    });
    EventsOn('profile-visibility', (data: any) => {
      profilePublic.set(data.public ? 'public' : 'private');
    });
    EventsOn('games-install-changed', (data: Record<string, boolean>) => {
      games.update(list => list.map(game => ({
        ...game,
        installed: data[String(game.appId)] ?? false
      })));
    });
    EventsOn('account-changed', (data: any) => {
      // Reset all state for the new account
      steamId.set(data.steamId);
      personaName.set(data.personaName || '');
      isConnected.set(true);
      games.set([]);
      achievementCounts.set({});
      scanning.set(false);
      scanProgress.set({ current: 0, total: 0, name: '' });
      profilePublic.set('unknown');
      if ($currentPage === 'manager') {
        currentPage.set('picker');
      }
      addToast(`Switched to ${data.personaName || 'new account'}`, 'info');
      // Trigger re-fetch by emitting a custom event the GamePicker listens for
      window.dispatchEvent(new CustomEvent('steamforge-account-changed'));
    });
    EventsOn('steam-disconnected', () => {
      isConnected.set(false);
      personaName.set('');
      games.set([]);
      achievementCounts.set({});
      scanning.set(false);
      if ($currentPage === 'manager') {
        currentPage.set('picker');
      }
      // GamePicker will detect isConnected=false and enter retry
      window.dispatchEvent(new CustomEvent('steamforge-account-changed'));
    });
  });

  let totalGameCount = $derived(
    $games.filter(game => $settings.showSoftware || !game.isSoftware).length
  );

  let stats = $derived.by(() => {
    const entries = Object.values($achievementCounts);
    if (entries.length === 0) return null;

    let perfectGames = 0;
    let totalAchieved = 0;
    let totalAvailable = 0;

    for (const entry of entries) {
      if (entry.total <= 0) continue;
      if (entry.achieved >= 0) {
        totalAchieved += entry.achieved;
      }
      totalAvailable += entry.total;
      if (entry.achieved === entry.total) {
        perfectGames++;
      }
    }

    return { perfectGames, totalAchieved, totalAvailable };
  });

</script>

<svelte:window bind:innerWidth onkeydown={handleOverlayKeydown} />

<div class="flex flex-col h-screen bg-steam-bg">
  <main class="flex-1 flex flex-col overflow-hidden">
    <GamePicker />
  </main>

  {#if $currentPage === 'manager'}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
      transition:fade={{ duration: 200 }}
      class="fixed inset-0 z-50 {isModal ? 'flex items-center justify-center bg-black/50 backdrop-blur-sm' : ''}"
      role="none"
      onclick={handleBackdropClick}
    >
      {#if hasMultipleGames && isModal}
        <button
          onclick={(e) => { e.stopPropagation(); gameManager?.switchToPrevious(); }}
          class="absolute left-3 top-1/2 -translate-y-1/2 z-[51] p-2.5 rounded-full
                 bg-black/40 text-white/60 hover:text-white hover:bg-black/60
                 transition-all cursor-pointer"
          title={previousGame?.name}
        >
          <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
          </svg>
        </button>
        <button
          onclick={(e) => { e.stopPropagation(); gameManager?.switchToNext(); }}
          class="absolute right-3 top-1/2 -translate-y-1/2 z-[51] p-2.5 rounded-full
                 bg-black/40 text-white/60 hover:text-white hover:bg-black/60
                 transition-all cursor-pointer"
          title={nextGame?.name}
        >
          <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
          </svg>
        </button>
      {/if}
      <div
        class="absolute inset-0 {isModal ? 'w-[92%] h-[90%] max-w-7xl m-auto rounded-xl shadow-2xl' : ''} flex flex-col bg-steam-bg overflow-hidden {$gameComplete ? 'ring-2 ring-amber-500 shadow-lg shadow-amber-500/20' : ''}"
      >
        <GameManager bind:this={gameManager} />
      </div>
    </div>
  {/if}
  <footer class="flex items-center px-4 py-1 bg-steam-surface border-t border-steam-border text-xs">
    <div class="flex-1 flex items-center gap-3">
      {#if $scanning && $scanProgress.total > 0}
        {@const scanPercent = Math.round(($scanProgress.current / $scanProgress.total) * 100)}
        <span class="flex items-center gap-1.5 text-steam-primary" title="Scanning: {$scanProgress.name}">
          <span class="w-3 h-3 border border-steam-primary border-t-transparent rounded-full animate-spin"></span>
          Scanning {$scanProgress.current}/{$scanProgress.total}
          <span class="inline-block w-24 h-1.5 bg-black/30 rounded-full overflow-hidden">
            <span class="block h-full bg-steam-primary rounded-full transition-all" style="width: {scanPercent}%"></span>
          </span>
        </span>
      {:else if stats && totalGameCount > 0}
        <span class="text-steam-text-dim">
          <span class="text-steam-primary">{stats.perfectGames}</span>/{totalGameCount} perfected
        </span>
        <span class="text-steam-border">|</span>
        <span class="text-steam-text-dim">
          <span class="text-steam-text">{stats.totalAchieved.toLocaleString()}</span>/{stats.totalAvailable.toLocaleString()} achievements{#if stats.totalAvailable > 0}
            ({Math.round((stats.totalAchieved / stats.totalAvailable) * 100)}%){/if}
        </span>
      {/if}
    </div>
    <div class="flex items-center gap-3">
      {#if $isConnected}
        <span class="flex items-center gap-1.5">
          <span class="w-1.5 h-1.5 rounded-full bg-steam-success"></span>
          <span class="text-steam-text-dim">{$personaName || 'Connected'}</span>
        </span>
      {:else}
        <span class="flex items-center gap-1.5">
          <span class="w-1.5 h-1.5 rounded-full bg-steam-danger"></span>
          <span class="text-steam-text-dim">Disconnected</span>
        </span>
      {/if}
      {#if updateInfo?.updateAvailable}
        <button
          onclick={() => BrowserOpenURL(updateInfo!.downloadUrl)}
          class="flex items-center gap-1 text-steam-primary hover:text-steam-primary-light transition-colors cursor-pointer"
          title="Download {updateInfo.latestVersion}"
        >
          <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
          </svg>
          {updateInfo.latestVersion} available
        </button>
      {:else if appVersion || updateInfo}
        <span class="text-steam-text-dim">{appVersion || updateInfo?.currentVersion}</span>
      {/if}
    </div>
  </footer>
  <Toast />
</div>
