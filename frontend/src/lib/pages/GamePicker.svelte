<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { games, gamesLoading, searchQuery, achievementCounts } from '../stores/games';
  import { currentPage, isConnected, steamId, addToast, isLoading, loadingMessage, scanning, scanProgress, profilePublic, gameFilter } from '../stores/app';
  import type { GameFilter } from '../stores/app';
  import { settings, updateSetting, updateSettingDebounced, loadSettings, setSortColumn } from '../stores/settings';
  import type { Settings } from '../stores/settings';
  import { filteredGames } from '../stores/games';
  import GameSearch from '../components/GameSearch.svelte';
  import GameGrid from '../components/GameGrid.svelte';
  import ContextMenu from '../components/ContextMenu.svelte';
  import SettingsPanel from '../components/SettingsPanel.svelte';
  import { ConnectSteam, FetchGames, GetAchievementCounts, ScanAchievementCounts } from '../../../wailsjs/go/main/App';
  import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';

  let retryInterval = $state<ReturnType<typeof setInterval> | null>(null);
  let retryCount = $state(0);

  function stopRetry() {
    if (retryInterval) {
      clearInterval(retryInterval);
      retryInterval = null;
    }
  }

  function startRetry() {
    stopRetry();
    retryCount = 0;
    retryInterval = setInterval(async () => {
      retryCount++;
      try {
        const id = await ConnectSteam();
        stopRetry();
        isConnected.set(true);
        steamId.set(id.toString());
        isLoading.set(false);
        loadingMessage.set('');
        await loadSettings();
        await loadGames();
        try {
          const counts = await GetAchievementCounts();
          achievementCounts.set(counts || {});
        } catch { /* cache not critical */ }
        startScan();
      } catch {
        // Still not available, keep polling
      }
    }, 3000);
  }

  onMount(async () => {
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

  onDestroy(() => {
    stopRetry();
  });

  function startScan() {
    scanning.set(true);
    scanProgress.set({ current: 0, total: 0, name: '' });
    ScanAchievementCounts().catch(() => { scanning.set(false); });
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === '/' && !isInputFocused()) {
      e.preventDefault();
      document.querySelector<HTMLInputElement>('[data-search-input]')?.focus();
    }
    if (e.key === 'Escape' && $currentPage === 'picker' && $searchQuery) {
      searchQuery.set('');
      document.querySelector<HTMLInputElement>('[data-search-input]')?.focus();
    }
  }

  function isInputFocused(): boolean {
    const activeElement = document.activeElement;
    return activeElement instanceof HTMLInputElement || activeElement instanceof HTMLTextAreaElement;
  }

  async function connect() {
    isLoading.set(true);
    loadingMessage.set('Connecting to Steam...');
    try {
      const id = await ConnectSteam();
      stopRetry();
      isConnected.set(true);
      steamId.set(id.toString());
      // Reload settings now that per-user config is available
      await loadSettings();
      await loadGames();
      try {
        const counts = await GetAchievementCounts();
        achievementCounts.set(counts || {});
      } catch { /* cache not critical */ }
      startScan();
    } catch {
      startRetry();
    } finally {
      isLoading.set(false);
      loadingMessage.set('');
    }
  }

  async function loadGames() {
    gamesLoading.set(true);
    loadingMessage.set('Fetching games...');
    try {
      const result = await FetchGames();
      games.set(result || []);
      addToast(`Loaded ${result?.length || 0} games`, 'info');
    } catch (e: any) {
      addToast(`Failed to load games: ${e.message || e}`, 'error');
    } finally {
      gamesLoading.set(false);
      loadingMessage.set('');
    }
  }

  let settingsOpen = $state(false);

  let profilePrivate = $state(false);
  $effect(() => {
    if ($profilePublic === 'private') profilePrivate = true;
  });
  let sortDropdownOpen = $state(false);
  let filterDropdownOpen = $state(false);

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

  const filterOptions: { value: GameFilter; label: string }[] = [
    { value: 'all', label: 'All' },
    { value: 'incomplete', label: 'Incomplete' },
    { value: 'perfected', label: 'Perfected' },
    { value: 'none', label: 'No Achievements' },
  ];

  let activeFilterLabel = $derived(filterOptions.find(o => o.value === $gameFilter)?.label ?? 'All');

  function handleFilterOption(value: GameFilter) {
    gameFilter.set(value);
    filterDropdownOpen = false;
  }

  const sortOptions: { value: Settings['sortBy']; label: string }[] = [
    { value: 'name', label: 'Name' },
    { value: 'appId', label: 'App ID' },
    { value: 'lastPlayed', label: 'Last Played' },
    { value: 'achievements', label: 'Achievement %' },
  ];

  let activeSortLabel = $derived(sortOptions.find(option => option.value === $settings.sortBy)?.label ?? 'Name');

  function handleSortOption(value: Settings['sortBy']) {
    setSortColumn(value);
    sortDropdownOpen = false;
  }

</script>

<svelte:window onkeydown={handleKeydown} />

<div class="flex flex-col flex-1 overflow-hidden">
  <div class="px-4 pt-3 pb-2 flex flex-wrap items-center gap-x-3 gap-y-2 border-b border-steam-border/50">
    <div class="flex items-center">
      <button
        onclick={() => BrowserOpenURL('https://ko-fi.com/ratkill')}
        title="Support SteamForge"
        class="flex items-center gap-1.5 px-2.5 py-1.5 text-xs rounded-lg bg-pink-500/10 border border-pink-500/30 text-pink-400 hover:bg-pink-500/20 hover:border-pink-500/50 transition-colors cursor-pointer"
      >
        <svg class="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 24 24">
          <path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z" />
        </svg>
        Buy Me a Coffee
      </button>
    </div>
    <div class="w-full order-first lg:order-none lg:w-auto lg:flex-1 lg:min-w-0 lg:max-w-md">
      <GameSearch />
    </div>
    <div class="flex items-center gap-3 ml-auto flex-shrink-0">
      <div class="relative">
        <button
          onclick={() => filterDropdownOpen = !filterDropdownOpen}
          title="Filter games"
          class="flex items-center gap-1.5 px-2.5 py-2 text-xs rounded-lg bg-steam-input border border-steam-border text-steam-text-dim hover:text-steam-text hover:border-steam-primary/50 transition-colors cursor-pointer whitespace-nowrap
            {$gameFilter !== 'all' ? 'border-steam-primary/50 text-steam-text' : ''}"
        >
          {activeFilterLabel}
          <svg class="w-3 h-3 transition-transform {filterDropdownOpen ? 'rotate-180' : ''}" fill="currentColor" viewBox="0 0 20 20">
            <path fill-rule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clip-rule="evenodd" />
          </svg>
        </button>
        {#if filterDropdownOpen}
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div class="fixed inset-0 z-20" role="none" onclick={() => filterDropdownOpen = false}></div>
          <div class="absolute right-0 top-full mt-1 z-30 bg-steam-surface border border-steam-border rounded-lg shadow-lg py-1 min-w-[160px]">
            {#each filterOptions as option}
              <button
                onclick={() => handleFilterOption(option.value)}
                class="w-full text-left px-3 py-1.5 text-xs cursor-pointer transition-colors flex items-center justify-between
                  {$gameFilter === option.value ? 'text-steam-primary' : 'text-steam-text-dim hover:text-steam-text hover:bg-steam-hover'}"
              >
                {option.label}
                <span class="text-[10px] tabular-nums opacity-60">{filterCounts[option.value]}</span>
              </button>
            {/each}
          </div>
        {/if}
      </div>
      <div class="relative">
        <button
          onclick={() => sortDropdownOpen = !sortDropdownOpen}
          title="Sort games"
          class="flex items-center gap-1.5 px-2.5 py-2 text-xs rounded-lg bg-steam-input border border-steam-border text-steam-text-dim hover:text-steam-text hover:border-steam-primary/50 transition-colors cursor-pointer whitespace-nowrap"
        >
          {activeSortLabel}
          <span class="text-[10px]">{$settings.sortOrder === 'asc' ? '↑' : '↓'}</span>
          <svg class="w-3 h-3 transition-transform {sortDropdownOpen ? 'rotate-180' : ''}" fill="currentColor" viewBox="0 0 20 20">
            <path fill-rule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clip-rule="evenodd" />
          </svg>
        </button>
        {#if sortDropdownOpen}
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div class="fixed inset-0 z-20" role="none" onclick={() => sortDropdownOpen = false}></div>
          <div class="absolute right-0 top-full mt-1 z-30 bg-steam-surface border border-steam-border rounded-lg shadow-lg py-1 min-w-[140px]">
            {#each sortOptions as option}
              <button
                onclick={() => handleSortOption(option.value)}
                class="w-full text-left px-3 py-1.5 text-xs cursor-pointer transition-colors flex items-center justify-between
                  {$settings.sortBy === option.value ? 'text-steam-primary' : 'text-steam-text-dim hover:text-steam-text hover:bg-steam-hover'}"
              >
                {option.label}
                {#if $settings.sortBy === option.value}
                  <span class="text-[10px]">{$settings.sortOrder === 'asc' ? '↑' : '↓'}</span>
                {/if}
              </button>
            {/each}
          </div>
        {/if}
      </div>

      <div class="flex rounded-lg overflow-hidden border border-steam-border flex-shrink-0">
        <button
          onclick={() => updateSetting('viewMode', 'grid')}
          title="Grid view"
          class="p-2 cursor-pointer transition-colors {$settings.viewMode === 'grid' ? 'bg-steam-primary text-steam-bg' : 'bg-steam-input text-steam-text-dim hover:text-steam-text'}"
        >
          <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
            <path d="M5 3a2 2 0 00-2 2v2a2 2 0 002 2h2a2 2 0 002-2V5a2 2 0 00-2-2H5zM5 11a2 2 0 00-2 2v2a2 2 0 002 2h2a2 2 0 002-2v-2a2 2 0 00-2-2H5zM11 5a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V5zM11 13a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z" />
          </svg>
        </button>
        <button
          onclick={() => updateSetting('viewMode', 'list')}
          title="List view"
          class="p-2 cursor-pointer transition-colors {$settings.viewMode === 'list' ? 'bg-steam-primary text-steam-bg' : 'bg-steam-input text-steam-text-dim hover:text-steam-text'}"
        >
          <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
            <path fill-rule="evenodd" d="M3 4a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1zm0 4a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1zm0 4a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1zm0 4a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1z" clip-rule="evenodd" />
          </svg>
        </button>
      </div>

      {#if $settings.viewMode === 'grid'}
        <div class="flex items-center gap-1.5 flex-shrink-0">
          <svg class="w-3.5 h-3.5 text-steam-text-dim" fill="currentColor" viewBox="0 0 20 20">
            <path d="M5 3a2 2 0 00-2 2v2a2 2 0 002 2h2a2 2 0 002-2V5a2 2 0 00-2-2H5zM5 11a2 2 0 00-2 2v2a2 2 0 002 2h2a2 2 0 002-2v-2a2 2 0 00-2-2H5zM11 5a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V5zM11 13a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z" />
          </svg>
          <input
            type="range"
            min="150"
            max="400"
            step="10"
            value={$settings.cardMinWidth}
            oninput={(e) => updateSettingDebounced('cardMinWidth', Number(e.currentTarget.value))}
            title="Card size: {$settings.cardMinWidth}px"
            class="w-20 h-1 accent-steam-primary cursor-pointer"
          />
          <svg class="w-4.5 h-4.5 text-steam-text-dim" fill="currentColor" viewBox="0 0 20 20">
            <path d="M5 3a2 2 0 00-2 2v2a2 2 0 002 2h2a2 2 0 002-2V5a2 2 0 00-2-2H5zM5 11a2 2 0 00-2 2v2a2 2 0 002 2h2a2 2 0 002-2v-2a2 2 0 00-2-2H5zM11 5a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V5zM11 13a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z" />
          </svg>
        </div>
      {/if}

      <div class="relative flex-shrink-0">
        <button
          onclick={() => settingsOpen = !settingsOpen}
          title="Settings"
          class="p-2 rounded-lg border cursor-pointer transition-colors
            {settingsOpen ? 'bg-steam-primary text-steam-bg border-steam-primary' : 'bg-steam-input text-steam-text-dim border-steam-border hover:text-steam-text hover:border-steam-primary/50'}"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
        </button>
        {#if settingsOpen}
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div class="fixed inset-0 z-20" role="none" onclick={() => settingsOpen = false}></div>
          <div class="absolute right-0 top-full mt-1 z-30 bg-steam-surface border border-steam-border rounded-lg shadow-lg p-4 w-72">
            <SettingsPanel onclose={() => settingsOpen = false} />
          </div>
        {/if}
      </div>

      <button
        onclick={loadGames}
        disabled={$gamesLoading || !$isConnected}
        title="Refresh games"
        class="p-2 rounded-lg border cursor-pointer transition-colors disabled:opacity-50 disabled:cursor-not-allowed
          bg-steam-input text-steam-text-dim border-steam-border hover:text-steam-text hover:border-steam-primary/50"
      >
        <svg class="w-4 h-4 {$gamesLoading ? 'animate-spin' : ''}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
        </svg>
      </button>
    </div>
  </div>

  {#if $gamesLoading}
    <div class="flex-1 flex items-center justify-center">
      <div class="flex flex-col items-center gap-3">
        <div class="w-8 h-8 border-2 border-steam-primary border-t-transparent rounded-full animate-spin"></div>
        <p class="text-steam-text-dim text-sm">{$loadingMessage || 'Loading...'}</p>
      </div>
    </div>
  {:else if !$isConnected}
    <div class="flex-1 flex items-center justify-center">
      <div class="flex flex-col items-center gap-4 max-w-sm text-center">
        <svg class="w-16 h-16 text-steam-text-dim opacity-40" fill="currentColor" viewBox="0 0 24 24">
          <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z" opacity="0" />
          <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z" />
        </svg>
        <div>
          <p class="text-steam-text font-medium text-lg">Steam is not running</p>
          <p class="text-steam-text-dim text-sm mt-1">
            Please open Steam on this computer. SteamForge will connect automatically once Steam is detected.
          </p>
        </div>
        {#if retryInterval}
          <div class="flex items-center gap-2 text-xs text-steam-text-dim">
            <div class="w-3 h-3 border-2 border-steam-primary border-t-transparent rounded-full animate-spin"></div>
            Waiting for Steam...
          </div>
        {/if}
        <div class="flex items-center gap-3">
          <button
            onclick={() => BrowserOpenURL('steam://open/main')}
            class="px-6 py-3 rounded-lg bg-steam-primary text-steam-bg font-medium hover:bg-steam-primary-dark transition-colors cursor-pointer"
          >
            Open Steam
          </button>
          <button
            onclick={connect}
            class="px-6 py-3 rounded-lg bg-steam-surface-light text-steam-text-dim font-medium hover:text-steam-text transition-colors cursor-pointer"
          >
            Retry Now
          </button>
        </div>
      </div>
    </div>
  {:else}
    {#if profilePrivate}
      <div class="mx-4 mt-2 px-4 py-3 rounded-lg bg-amber-900/40 border border-amber-600/50 flex items-start gap-3">
        <svg class="w-5 h-5 text-amber-400 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z" />
        </svg>
        <div class="flex-1">
          <p class="text-sm font-medium text-amber-200">Your Steam profile is private</p>
          <p class="text-xs text-amber-200/70 mt-0.5">
            Achievement progress can't be tracked in the game list. Set your profile to public in
            <button class="text-amber-300 underline cursor-pointer" onclick={() => { import('../../../wailsjs/runtime/runtime').then(r => r.BrowserOpenURL('https://steamcommunity.com/my/edit/settings')); }}>Steam privacy settings</button>
            and rescan.
          </p>
        </div>
        <button onclick={() => profilePrivate = false} title="Dismiss" class="text-amber-400/60 hover:text-amber-300 cursor-pointer flex-shrink-0">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    {/if}
    <GameGrid />
  {/if}
</div>

<ContextMenu />
