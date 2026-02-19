<script lang="ts">
  import { tick } from 'svelte';
  import { filteredGames } from '../stores/games';
  import { scanning } from '../stores/app';
  import { settings, updateSetting, setSortColumn } from '../stores/settings';
  import type { Settings } from '../stores/settings';
  import GameCard from './GameCard.svelte';
  import GameListRow from './GameListRow.svelte';

  // Subtle reorder animation: gentle crossfade for cards that changed position.
  // Disabled during scanning to avoid visual noise from frequent reorders.
  function fadeReorder(_node: Element, { from, to }: { from: DOMRect; to: DOMRect }, { duration = 200 }: { duration?: number } = {}) {
    const dx = from.left - to.left;
    const dy = from.top - to.top;
    if (Math.abs(dx) < 1 && Math.abs(dy) < 1 || duration === 0) return { duration: 0 };
    return {
      duration,
      css: (t: number) => {
        // Gentle dip to 40% opacity at midpoint, not a full blackout
        const opacity = 0.4 + 0.6 * Math.abs(2 * t - 1);
        return `opacity: ${opacity}`;
      },
    };
  }

  let animationDuration = $derived($scanning ? 0 : 200);

  function sortIndicator(column: Settings['sortBy']): string {
    if ($settings.sortBy !== column) return '';
    return $settings.sortOrder === 'asc' ? ' ↑' : ' ↓';
  }

  const BATCH_SIZE = 60;

  let otherVisibleCount = $state(BATCH_SIZE);
  let scrollContainer: HTMLDivElement | undefined = $state();

  let installedGames = $derived($filteredGames.filter(game => game.installed));
  let otherGames = $derived($filteredGames.filter(game => !game.installed));

  $effect(() => {
    $filteredGames;
    otherVisibleCount = BATCH_SIZE;
  });

  let visibleOtherGames = $derived(otherGames.slice(0, otherVisibleCount));
  let hasMoreOther = $derived(otherVisibleCount < otherGames.length);

  // Load more batches until the content overflows the container (creates a scrollbar).
  // Fixes 4K/large monitors where the initial batch doesn't fill the viewport.
  // Also re-checks when container is resized (e.g. moving window to a larger monitor).
  let containerHeight = $state(0);

  $effect(() => {
    otherVisibleCount;
    containerHeight; // re-run when container resizes
    if (!hasMoreOther || !scrollContainer) return;
    tick().then(() => {
      if (scrollContainer && scrollContainer.scrollHeight <= scrollContainer.clientHeight) {
        otherVisibleCount = Math.min(otherVisibleCount + BATCH_SIZE, otherGames.length);
      }
    });
  });

  // Watch for container resize (window move to larger monitor, window resize, etc.)
  $effect(() => {
    if (!scrollContainer) return;
    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        containerHeight = entry.contentRect.height;
      }
    });
    observer.observe(scrollContainer);
    return () => observer.disconnect();
  });

  function handleScroll() {
    if (!scrollContainer || !hasMoreOther) return;
    const { scrollTop, scrollHeight, clientHeight } = scrollContainer;
    if (scrollHeight - scrollTop - clientHeight < 400) {
      otherVisibleCount = Math.min(otherVisibleCount + BATCH_SIZE, otherGames.length);
    }
  }

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
    onscroll={handleScroll}
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
              {#each installedGames as game (game.appId)}
                <div animate:fadeReorder={{ duration: animationDuration }}>
                  <GameListRow {game} />
                </div>
              {/each}
            </div>
          {:else}
            <div class="grid gap-4 p-4" style="grid-template-columns: repeat(auto-fill, minmax({$settings.cardMinWidth}px, 1fr))">
              {#each installedGames as game (game.appId)}
                <div animate:fadeReorder={{ duration: animationDuration }}>
                  <GameCard {game} />
                </div>
              {/each}
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
              {#each visibleOtherGames as game (game.appId)}
                <div animate:fadeReorder={{ duration: animationDuration }}>
                  <GameListRow {game} />
                </div>
              {/each}
            </div>
          {:else}
            <div class="grid gap-4 p-4" style="grid-template-columns: repeat(auto-fill, minmax({$settings.cardMinWidth}px, 1fr))">
              {#each visibleOtherGames as game (game.appId)}
                <div animate:fadeReorder={{ duration: animationDuration }}>
                  <GameCard {game} />
                </div>
              {/each}
            </div>
          {/if}
          {#if hasMoreOther}
            <div class="text-center py-4 text-steam-text-dim text-sm">
              Showing {otherVisibleCount} of {otherGames.length} games — scroll for more
            </div>
          {/if}
        {/if}
      </div>
    {/if}
  </div>
{/if}
