<script lang="ts">
  interface Props {
    onlockall: () => void;
    onrefresh: () => void;
    searchQuery: string;
    onsearch: (query: string) => void;
    filterMode: string;
    onfilter: (mode: string) => void;
    sortMode: string;
    sortDirection: 'asc' | 'desc';
    onsort: (mode: string) => void;
    selectedCount: number;
    selectedLockedCount: number;
    selectedUnlockedCount: number;
    onlockselected: () => void;
    onunlockselected: () => void;
    onselectall: () => void;
    onclearselection: () => void;
    allowLock: boolean;
    visibleCount: number;
    totalCount: number;
  }

  let {
    onlockall, onrefresh,
    searchQuery, onsearch, filterMode, onfilter, sortMode, sortDirection, onsort,
    selectedCount, selectedLockedCount, selectedUnlockedCount,
    onlockselected, onunlockselected, onselectall, onclearselection,
    allowLock, visibleCount, totalCount,
  }: Props = $props();

  let filterOpen = $state(false);
  let sortOpen = $state(false);
  let searchInputEl = $state<HTMLInputElement | null>(null);

  const filterOptions = [
    { value: 'all', label: 'All', icon: 'M3 4a1 1 0 011-1h16a1 1 0 010 2H4a1 1 0 01-1-1zm2 4a1 1 0 011-1h10a1 1 0 010 2H6a1 1 0 01-1-1zm3 4a1 1 0 011-1h4a1 1 0 010 2H9a1 1 0 01-1-1z' },
    { value: 'locked', label: 'Locked', icon: 'M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z' },
    { value: 'unlocked', label: 'Unlocked', icon: 'M8 11V7a4 4 0 118 0m-4 8v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2z' },
    { value: 'hidden', label: 'Hidden', icon: 'M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.878 9.878L3 3m6.878 6.878L21 21' },
  ];

  const sortOptions = [
    { value: 'default', label: 'Default' },
    { value: 'name', label: 'Name' },
    { value: 'percent', label: 'Rarity' },
    { value: 'unlockTime', label: 'Unlock Date' },
  ];

  let activeFilterLabel = $derived(filterOptions.find(option => option.value === filterMode)?.label ?? 'All');
  let activeSortLabel = $derived(sortOptions.find(option => option.value === sortMode)?.label ?? 'Default');
  let isFiltered = $derived(searchQuery !== '' || filterMode !== 'all');

  function handleFilter(value: string) {
    onfilter(value);
    filterOpen = false;
  }

  function handleSort(value: string) {
    onsort(value);
    sortOpen = false;
  }

  function clearSearch() {
    onsearch('');
    searchInputEl?.focus();
  }
</script>

<div class="flex flex-col gap-2 px-4 py-2.5 bg-steam-surface/80 border-b border-steam-border">
  <!-- Row 1: Search bar with integrated controls -->
  <div class="flex items-center gap-1.5">
    <div class="relative flex-1">
      <input
        bind:this={searchInputEl}
        type="text"
        placeholder="Search achievements..."
        value={searchQuery}
        oninput={(e) => onsearch((e.target as HTMLInputElement).value)}
        onkeydown={(e) => {
          if (e.key === 'Escape' && searchQuery) {
            e.stopPropagation();
            onsearch('');
          }
        }}
        class="w-full bg-steam-input border border-steam-border rounded-lg px-3 py-1.5 pl-8 pr-16 text-steam-text placeholder-steam-text-dim text-xs focus:outline-none focus:border-steam-primary/70 focus:ring-1 focus:ring-steam-primary/30 transition-all"
      />
      <svg class="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-steam-text-dim pointer-events-none" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
      </svg>
      <div class="absolute right-2 top-1/2 -translate-y-1/2 flex items-center gap-1">
        {#if searchQuery}
          <button
            onclick={clearSearch}
            class="p-0.5 rounded text-steam-text-dim hover:text-steam-text transition-colors cursor-pointer"
            title="Clear search"
          >
            <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        {/if}
        {#if isFiltered}
          <span class="text-[10px] text-steam-text-dim tabular-nums">{visibleCount}/{totalCount}</span>
        {/if}
      </div>
    </div>

    <!-- Filter dropdown -->
    <div class="relative">
      <button
        onclick={() => { filterOpen = !filterOpen; sortOpen = false; }}
        class="flex items-center gap-1 px-2 py-1.5 text-xs rounded-lg border transition-colors cursor-pointer whitespace-nowrap
          {filterMode !== 'all'
            ? 'bg-steam-primary/10 border-steam-primary/30 text-steam-primary hover:bg-steam-primary/15'
            : 'bg-steam-input border-steam-border text-steam-text-dim hover:text-steam-text hover:border-steam-primary/50'}"
      >
        <svg class="w-3 h-3 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 4a1 1 0 011-1h16a1 1 0 010 2H4a1 1 0 01-1-1zm2 4a1 1 0 011-1h10a1 1 0 010 2H6a1 1 0 01-1-1zm3 4a1 1 0 011-1h4a1 1 0 010 2H9a1 1 0 01-1-1z" />
        </svg>
        {activeFilterLabel}
        <svg class="w-2.5 h-2.5 transition-transform {filterOpen ? 'rotate-180' : ''}" fill="currentColor" viewBox="0 0 20 20">
          <path fill-rule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clip-rule="evenodd" />
        </svg>
      </button>
      {#if filterOpen}
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div class="fixed inset-0 z-20" role="none" onclick={() => filterOpen = false}></div>
        <div class="absolute right-0 top-full mt-1 z-30 bg-steam-surface border border-steam-border rounded-lg shadow-lg py-1 min-w-[130px]">
          {#each filterOptions as option}
            <button
              onclick={() => handleFilter(option.value)}
              class="w-full text-left px-3 py-1.5 text-xs cursor-pointer transition-colors flex items-center gap-2
                {filterMode === option.value ? 'text-steam-primary' : 'text-steam-text-dim hover:text-steam-text hover:bg-steam-hover'}"
            >
              <svg class="w-3 h-3 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={option.icon} />
              </svg>
              {option.label}
            </button>
          {/each}
        </div>
      {/if}
    </div>

    <!-- Sort dropdown -->
    <div class="relative">
      <button
        onclick={() => { sortOpen = !sortOpen; filterOpen = false; }}
        class="flex items-center gap-1 px-2 py-1.5 text-xs rounded-lg border transition-colors cursor-pointer whitespace-nowrap
          {sortMode !== 'default'
            ? 'bg-steam-primary/10 border-steam-primary/30 text-steam-primary hover:bg-steam-primary/15'
            : 'bg-steam-input border-steam-border text-steam-text-dim hover:text-steam-text hover:border-steam-primary/50'}"
      >
        <svg class="w-3 h-3 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 4h13M3 8h9m-9 4h6m4 0l4-4m0 0l4 4m-4-4v12" />
        </svg>
        {activeSortLabel}
        <span class="text-[10px] opacity-70">{sortDirection === 'asc' ? '↑' : '↓'}</span>
      </button>
      {#if sortOpen}
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div class="fixed inset-0 z-20" role="none" onclick={() => sortOpen = false}></div>
        <div class="absolute right-0 top-full mt-1 z-30 bg-steam-surface border border-steam-border rounded-lg shadow-lg py-1 min-w-[130px]">
          {#each sortOptions as option}
            <button
              onclick={() => handleSort(option.value)}
              class="w-full text-left px-3 py-1.5 text-xs cursor-pointer transition-colors flex items-center justify-between
                {sortMode === option.value ? 'text-steam-primary' : 'text-steam-text-dim hover:text-steam-text hover:bg-steam-hover'}"
            >
              {option.label}
              {#if sortMode === option.value}
                <span class="text-[10px]">{sortDirection === 'asc' ? '↑' : '↓'}</span>
              {/if}
            </button>
          {/each}
        </div>
      {/if}
    </div>

    <!-- Divider -->
    <div class="w-px h-5 bg-steam-border/50"></div>

    <!-- Action buttons -->
    <button
      onclick={onselectall}
      title="Select all visible"
      class="p-1.5 rounded-lg text-steam-text-dim hover:text-steam-text hover:bg-steam-surface-light transition-colors cursor-pointer"
    >
      <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
      </svg>
    </button>

    {#if allowLock}
      <button
        onclick={onlockall}
        title="Lock all achievements"
        class="p-1.5 rounded-lg text-steam-danger/70 hover:text-steam-danger hover:bg-steam-danger/10 transition-colors cursor-pointer"
      >
        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
        </svg>
      </button>
    {/if}

    <button
      onclick={onrefresh}
      title="Refresh achievements"
      class="p-1.5 rounded-lg text-steam-text-dim hover:text-steam-text hover:bg-steam-surface-light transition-colors cursor-pointer"
    >
      <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
      </svg>
    </button>
  </div>

  <!-- Row 2: Selection action bar (conditional) -->
  {#if selectedCount > 0}
    <div class="flex items-center gap-2 pt-2 border-t border-steam-border/50">
      <span class="text-xs text-steam-text-dim">{selectedCount} selected</span>
      <div class="w-px h-4 bg-steam-border"></div>
      {#if selectedLockedCount > 0}
        <button
          onclick={onunlockselected}
          class="px-2.5 py-1 text-xs rounded bg-steam-success/20 text-steam-success hover:bg-steam-success/30 transition-colors cursor-pointer"
        >
          Unlock {selectedLockedCount}
        </button>
      {/if}
      {#if allowLock && selectedUnlockedCount > 0}
        <button
          onclick={onlockselected}
          class="px-2.5 py-1 text-xs rounded bg-steam-danger/20 text-steam-danger hover:bg-steam-danger/30 transition-colors cursor-pointer"
        >
          Lock {selectedUnlockedCount}
        </button>
      {/if}
      <button
        onclick={onclearselection}
        class="px-2.5 py-1 text-xs rounded bg-steam-surface-light text-steam-text-dim hover:text-steam-text transition-colors cursor-pointer"
      >
        Clear
      </button>
    </div>
  {/if}
</div>
