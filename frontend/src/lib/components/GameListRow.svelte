<script lang="ts">
  import type { GameInfo } from '../stores/games';
  import { achievementCounts, playtimes, hltbCache } from '../stores/games';
  import { navigateToManager } from '../stores/app';
  import { formatHoursMinutes } from '../utils/format';
  import { buildGameImageUrls } from '../utils/steam-images';
  import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
  import { settings } from '../stores/settings';
  import { showGameContextMenu } from '../utils/context-menu';
  import { toPlayList, toggleToPlay } from '../stores/toplay';

  interface Props {
    game: GameInfo;
  }

  let { game }: Props = $props();
  let imageIndex = $state(0);
  let allImagesFailed = $state(false);

  let imageUrls = $derived(buildGameImageUrls(game.appId, game.logoUrl));
  let currentImageSrc = $derived(imageUrls[imageIndex]);

  function handleImageError() {
    if (imageIndex < imageUrls.length - 1) {
      imageIndex++;
    } else {
      allImagesFailed = true;
    }
  }

  let counts = $derived($achievementCounts[String(game.appId)]);
  let hasAchievementData = $derived(counts && counts.total > 0 && counts.achieved >= 0);
  let isFullyCompleted = $derived(hasAchievementData && counts!.achieved === counts!.total);
  let isEarlyAccess = $derived(counts?.earlyAccess && counts.total === 0);
  let isOnToPlayList = $derived($toPlayList.has(game.appId));
  let playtimeHours = $derived($playtimes[String(game.appId)]);
  let hltbMain = $derived($hltbCache[String(game.appId)]?.main);
  let hltbCompletionist = $derived($hltbCache[String(game.appId)]?.completionist);
  let playtimeLabel = $derived(playtimeHours ? formatHoursMinutes(playtimeHours) : '');
  let hltbMainLabel = $derived(hltbMain ? formatHoursMinutes(hltbMain) : '');
  let hltbCompletionistLabel = $derived(hltbCompletionist ? formatHoursMinutes(hltbCompletionist) : '');
  let combinedLabel = $derived(
    [playtimeLabel, hltbMainLabel, hltbCompletionistLabel].filter(Boolean).join(' / ')
  );
  let combinedTitle = $derived(
    [
      playtimeLabel && `Played ${playtimeLabel}`,
      hltbMainLabel && `Main story ${hltbMainLabel}`,
      hltbCompletionistLabel && `100% ${hltbCompletionistLabel}`,
    ]
      .filter(Boolean)
      .join(' · ')
  );

  function handleToggleToPlay(e: Event) {
    e.stopPropagation();
    toggleToPlay(game.appId);
  }

  function handleClick() {
    navigateToManager(game.appId, game.name, game.installed);
  }

  function handlePlay(e: Event) {
    e.stopPropagation();
    BrowserOpenURL(`steam://rungameid/${game.appId}`);
  }

  function handleUninstall(e: Event) {
    e.stopPropagation();
    BrowserOpenURL(`steam://uninstall/${game.appId}`);
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  onclick={handleClick}
  onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') handleClick(); }}
  oncontextmenu={(e) => showGameContextMenu(e, game.appId, game.installed)}
  role="button"
  tabindex="0"
  class="w-full flex items-center gap-3 px-4 py-2 hover:bg-steam-hover transition-colors cursor-pointer border-b border-steam-border/30 text-left"
>
  <div class="w-16 h-[30px] flex-shrink-0 rounded overflow-hidden bg-steam-input">
    {#if !allImagesFailed}
      <img
        src={currentImageSrc}
        alt={game.name}
        class="w-full h-full object-cover"
        onerror={handleImageError}
      />
    {/if}
  </div>
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
      <button
        onclick={handlePlay}
        class="w-7 h-7 flex-shrink-0 flex items-center justify-center rounded bg-steam-success/20 text-steam-success hover:bg-steam-success/40 transition-colors cursor-pointer"
        title="Play game"
      >
        <svg class="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 24 24">
          <path d="M8 5v14l11-7z" />
        </svg>
      </button>
      <button
        onclick={handleUninstall}
        class="w-7 h-7 flex-shrink-0 flex items-center justify-center rounded bg-red-500/10 text-red-400/70 hover:bg-red-500/30 hover:text-red-400 transition-colors cursor-pointer"
        title="Uninstall game"
      >
        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
        </svg>
      </button>
    {:else}
      <button
        onclick={(e) => { e.stopPropagation(); BrowserOpenURL(`steam://install/${game.appId}`); }}
        class="w-7 h-7 flex-shrink-0 flex items-center justify-center rounded bg-steam-primary/20 text-steam-primary hover:bg-steam-primary/40 transition-colors cursor-pointer"
        title="Install game"
      >
        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
        </svg>
      </button>
    {/if}
  {/if}
  <div class="flex-1 min-w-0">
    <span class="text-sm text-steam-text truncate block">{game.name || `App ${game.appId}`}</span>
  </div>
  <span class="text-xs text-steam-text-dim w-36 text-right flex-shrink-0 truncate" title={combinedTitle}>{combinedLabel}</span>
  <span class="text-xs w-16 text-right flex-shrink-0 {isFullyCompleted ? 'text-amber-400 font-medium' : isEarlyAccess ? 'text-blue-400 font-medium' : 'text-steam-text-dim'}">{hasAchievementData ? `${counts!.achieved}/${counts!.total}` : isEarlyAccess ? 'EA' : counts && counts.total > 0 ? `${counts.total}` : ''}</span>
  <span class="text-xs text-steam-text-dim w-20 text-right flex-shrink-0">{game.appId}</span>
</div>
