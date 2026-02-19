<script lang="ts">
  import { searchQuery } from '../stores/games';
  import { filteredGames } from '../stores/games';
  import { games } from '../stores/games';

  let inputElement = $state<HTMLInputElement | null>(null);
  let inputValue = $state($searchQuery);

  let isFiltered = $derived(inputValue !== '');
  let matchCount = $derived($filteredGames.length);
  let totalGameCount = $derived($games.length);

  // Keep local state in sync if store changes externally (e.g. Escape key clears it)
  $effect(() => {
    inputValue = $searchQuery;
  });

  function handleInput(e: Event) {
    const target = e.target as HTMLInputElement;
    inputValue = target.value;
    searchQuery.set(inputValue);
  }

  function clearSearch() {
    inputValue = '';
    searchQuery.set('');
    inputElement?.focus();
  }
</script>

<div class="relative">
  <input
    bind:this={inputElement}
    type="text"
    data-search-input
    placeholder="Search games... (press / to focus)"
    value={inputValue}
    oninput={handleInput}
    class="w-full bg-steam-input border border-steam-border rounded-lg px-4 py-2 pl-10 pr-16 text-steam-text placeholder-steam-text-dim text-sm focus:outline-none focus:border-steam-primary/70 focus:ring-1 focus:ring-steam-primary/30 transition-all"
  />
  <svg class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-steam-text-dim pointer-events-none" fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
  </svg>
  <div class="absolute right-2.5 top-1/2 -translate-y-1/2 flex items-center gap-1.5">
    {#if inputValue}
      <button
        onclick={clearSearch}
        class="p-0.5 rounded text-steam-text-dim hover:text-steam-text transition-colors cursor-pointer"
        title="Clear search"
      >
        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    {/if}
    {#if isFiltered}
      <span class="text-[11px] text-steam-text-dim tabular-nums">{matchCount}/{totalGameCount}</span>
    {/if}
  </div>
</div>
