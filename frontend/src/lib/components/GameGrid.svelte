<script lang="ts">
  import { filteredGames } from '../stores/games';
  import { settings, updateSetting, setSortColumn } from '../stores/settings';
  import type { Settings } from '../stores/settings';
  import GameCard from './GameCard.svelte';
  import GameListRow from './GameListRow.svelte';
  import VirtualGrid from './VirtualGrid.svelte';

  const GAP = 16;
  const GRID_PADDING = 32;
  const PROGRESS_BAR_HEIGHT = 16;
  const CARD_BORDER = 2;
  const LABELS_HEIGHT = 58;
  const LIST_ROW_HEIGHT = 47;
  const IMAGE_ASPECT = 215 / 460;

  function sortIndicator(column: Settings['sortBy']): string {
    if ($settings.sortBy !== column) return '';
    return $settings.sortOrder === 'asc' ? ' ↑' : ' ↓';
  }

  let scrollContainer: HTMLDivElement | undefined = $state();
  let gridContainerWidth = $state(0);

  let installedGames = $derived($filteredGames.filter(game => game.installed));
  let otherGames = $derived($filteredGames.filter(game => !game.installed));

  // Track scroll container width for column calculations
  $effect(() => {
    if (!scrollContainer) return;
    const observer = new ResizeObserver((entries) => {
      gridContainerWidth = entries[0].contentRect.width;
    });
    observer.observe(scrollContainer);
    return () => observer.disconnect();
  });

  let columnCount = $derived.by(() => {
    if ($settings.viewMode === 'list') return 1;
    const effectiveWidth = gridContainerWidth - GRID_PADDING;
    if (effectiveWidth <= 0) return 1;
    return Math.max(1, Math.floor((effectiveWidth + GAP) / ($settings.cardMinWidth + GAP)));
  });

  let actualColumnWidth = $derived.by(() => {
    const effectiveWidth = gridContainerWidth - GRID_PADDING;
    if (effectiveWidth <= 0 || columnCount <= 0) return $settings.cardMinWidth;
    return (effectiveWidth - (columnCount - 1) * GAP) / columnCount;
  });

  let gridRowHeight = $derived.by(() => {
    const imageHeight = (actualColumnWidth - CARD_BORDER) * IMAGE_ASPECT;
    const labelsHeight = $settings.showLabels ? LABELS_HEIGHT : 0;
    const cardHeight = imageHeight + PROGRESS_BAR_HEIGHT + labelsHeight + CARD_BORDER;
    return cardHeight + GAP;
  });

  function toggleInstalled() {
    updateSetting('installedOpen', !$settings.installedOpen);
  }

  function toggleOther() {
    updateSetting('otherOpen', !$settings.otherOpen);
  }
</script>

{#snippet listHeader()}
  <div class="flex items-center gap-3 px-4 py-1.5 border-b border-steam-border/50 text-xs uppercase tracking-wider font-medium">
    <span class="w-16 flex-shrink-0"></span>
    <span class="w-7 flex-shrink-0"></span>
    <button onclick={() => setSortColumn('name')} class="flex-1 min-w-0 text-left cursor-pointer transition-colors {$settings.sortBy === 'name' ? 'text-steam-primary' : 'text-steam-text-dim hover:text-steam-text'}">
      Name{sortIndicator('name')}
    </button>
    <button onclick={() => setSortColumn('lastPlayed')} class="w-16 text-right flex-shrink-0 cursor-pointer transition-colors {$settings.sortBy === 'lastPlayed' ? 'text-steam-primary' : 'text-steam-text-dim hover:text-steam-text'}">
      Played{sortIndicator('lastPlayed')}
    </button>
    <button onclick={() => setSortColumn('achievements')} class="w-16 text-right flex-shrink-0 cursor-pointer transition-colors {$settings.sortBy === 'achievements' ? 'text-steam-primary' : 'text-steam-text-dim hover:text-steam-text'}">
      Achs{sortIndicator('achievements')}
    </button>
    <button onclick={() => setSortColumn('appId')} class="w-20 text-right flex-shrink-0 cursor-pointer transition-colors {$settings.sortBy === 'appId' ? 'text-steam-primary' : 'text-steam-text-dim hover:text-steam-text'}">
      AppID{sortIndicator('appId')}
    </button>
  </div>
{/snippet}

{#if $filteredGames.length === 0}
  <div class="flex-1 flex items-center justify-center text-steam-text-dim">
    <p>No games found</p>
  </div>
{:else}
  <div
    bind:this={scrollContainer}
    class="flex-1 overflow-y-auto min-h-0"
  >
    {#if installedGames.length > 0}
      <div>
        <button
          onclick={toggleInstalled}
          class="sticky top-0 z-20 w-full flex items-center gap-2 px-4 py-2.5 bg-steam-bg/95 backdrop-blur-sm
            text-steam-text text-sm font-medium cursor-pointer hover:bg-steam-hover transition-colors border-b border-steam-border/50"
        >
          <svg
            class="w-4 h-4 transition-transform duration-200 {$settings.installedOpen ? 'rotate-90' : ''}"
            fill="currentColor" viewBox="0 0 20 20"
          >
            <path d="M6.293 4.293a1 1 0 011.414 0L14 10.586l-6.293 6.293a1 1 0 01-1.414-1.414L11.172 10.586 6.293 5.707a1 1 0 010-1.414z" />
          </svg>
          Installed Games
          <span class="text-xs text-steam-text bg-steam-surface-light/80 px-1.5 py-0.5 rounded-full">
            {installedGames.length}
          </span>
        </button>
        {#if $settings.installedOpen}
          {#if $settings.viewMode === 'list'}
            <div>
              {@render listHeader()}
              <VirtualGrid
                {scrollContainer}
                items={installedGames}
                columnCount={1}
                rowHeight={LIST_ROW_HEIGHT}
                keyFn={(game) => game.appId}
              >
                {#snippet children(game)}
                  <GameListRow {game} />
                {/snippet}
              </VirtualGrid>
            </div>
          {:else}
            <div class="p-4">
              <VirtualGrid
                {scrollContainer}
                items={installedGames}
                {columnCount}
                rowHeight={gridRowHeight}
                class="grid"
                style="grid-template-columns: repeat({columnCount}, 1fr); gap: {GAP}px"
                keyFn={(game) => game.appId}
              >
                {#snippet children(game)}
                  <GameCard {game} />
                {/snippet}
              </VirtualGrid>
            </div>
          {/if}
        {/if}
      </div>
    {/if}

    {#if otherGames.length > 0}
      <div>
        <button
          onclick={toggleOther}
          class="sticky top-0 z-20 w-full flex items-center gap-2 px-4 py-2.5 bg-steam-bg/95 backdrop-blur-sm
            text-steam-text text-sm font-medium cursor-pointer hover:bg-steam-hover transition-colors border-b border-steam-border/50"
        >
          <svg
            class="w-4 h-4 transition-transform duration-200 {$settings.otherOpen ? 'rotate-90' : ''}"
            fill="currentColor" viewBox="0 0 20 20"
          >
            <path d="M6.293 4.293a1 1 0 011.414 0L14 10.586l-6.293 6.293a1 1 0 01-1.414-1.414L11.172 10.586 6.293 5.707a1 1 0 010-1.414z" />
          </svg>
          Other Games
          <span class="text-xs text-steam-text bg-steam-surface-light/80 px-1.5 py-0.5 rounded-full">
            {otherGames.length}
          </span>
        </button>
        {#if $settings.otherOpen}
          {#if $settings.viewMode === 'list'}
            <div>
              {@render listHeader()}
              <VirtualGrid
                {scrollContainer}
                items={otherGames}
                columnCount={1}
                rowHeight={LIST_ROW_HEIGHT}
                keyFn={(game) => game.appId}
              >
                {#snippet children(game)}
                  <GameListRow {game} />
                {/snippet}
              </VirtualGrid>
            </div>
          {:else}
            <div class="p-4">
              <VirtualGrid
                {scrollContainer}
                items={otherGames}
                {columnCount}
                rowHeight={gridRowHeight}
                class="grid"
                style="grid-template-columns: repeat({columnCount}, 1fr); gap: {GAP}px"
                keyFn={(game) => game.appId}
              >
                {#snippet children(game)}
                  <GameCard {game} />
                {/snippet}
              </VirtualGrid>
            </div>
          {/if}
        {/if}
      </div>
    {/if}
  </div>
{/if}
