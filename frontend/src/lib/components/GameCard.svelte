<script lang="ts">
  import type { GameInfo } from '../stores/games';
  import { achievementCounts } from '../stores/games';
  import { navigateToManager } from '../stores/app';
  import { settings } from '../stores/settings';
  import { formatLastPlayed } from '../utils/format';
  import { buildGameImageUrls } from '../utils/steam-images';
  import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
  import { showGameContextMenu } from '../utils/context-menu';
  import { toPlayList, toggleToPlay } from '../stores/toplay';

  interface Props {
    game: GameInfo;
  }

  let { game }: Props = $props();
  let imageLoaded = $state(false);
  let imageIndex = $state(0);
  let allImagesFailed = $state(false);

  let imageUrls = $derived(buildGameImageUrls(game.appId, game.logoUrl));
  let currentImageSrc = $derived(imageUrls[imageIndex]);

  function handleImageError() {
    if (imageIndex < imageUrls.length - 1) {
      imageLoaded = false;
      imageIndex++;
    } else {
      allImagesFailed = true;
    }
  }

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

  function handleClick() {
    navigateToManager(game.appId, game.name, game.installed);
  }

  function handlePlay(e: Event) {
    e.stopPropagation();
    BrowserOpenURL(`steam://rungameid/${game.appId}`);
  }

  // 3D tilt effect with lerp smoothing
  let wrapperEl = $state<HTMLDivElement | null>(null);
  let cardEl = $state<HTMLDivElement | null>(null);
  let targetX = 0;
  let targetY = 0;
  let currentX = 0;
  let currentY = 0;
  let isHovered = $state(false);
  let rafId: number | null = null;

  function lerp(a: number, b: number, t: number) {
    return a + (b - a) * t;
  }

  function animate() {
    const speed = isHovered ? 0.15 : 0.1;
    currentX = lerp(currentX, targetX, speed);
    currentY = lerp(currentY, targetY, speed);

    // Stop animating when close enough to target
    if (Math.abs(currentX - targetX) < 0.01 && Math.abs(currentY - targetY) < 0.01) {
      currentX = targetX;
      currentY = targetY;
      updateTransform();
      rafId = null;
      return;
    }

    updateTransform();
    rafId = requestAnimationFrame(animate);
  }

  function updateTransform() {
    if (!cardEl) return;
    const scale = isHovered ? 1.05 : 1;
    const ty = isHovered ? -4 : 0;
    cardEl.style.transform = `rotateX(${currentX}deg) rotateY(${currentY}deg) scale(${scale}) translateY(${ty}px)`;
  }

  function startAnimation() {
    if (rafId === null) {
      rafId = requestAnimationFrame(animate);
    }
  }

  function handleMouseMove(e: MouseEvent) {
    if (!wrapperEl) return;
    const rect = wrapperEl.getBoundingClientRect();
    const x = (e.clientX - rect.left) / rect.width;
    const y = (e.clientY - rect.top) / rect.height;
    targetX = (y - 0.5) * -20;
    targetY = (x - 0.5) * 20;
    isHovered = true;
    startAnimation();
  }

  function handleMouseLeave() {
    targetX = 0;
    targetY = 0;
    isHovered = false;
    startAnimation();
  }

</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<!-- Outer wrapper: owns perspective + mouse events, does NOT tilt -->
<div
  bind:this={wrapperEl}
  onclick={handleClick}
  onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') handleClick(); }}
  oncontextmenu={(e) => showGameContextMenu(e, game.appId)}
  onmousemove={handleMouseMove}
  onmouseleave={handleMouseLeave}
  role="button"
  tabindex="0"
  title={$settings.showLabels ? '' : `${game.name} (AppID: ${game.appId})`}
  class="group relative cursor-pointer text-left w-full {isHovered ? 'z-10' : ''}"
  style="perspective: 600px;"
>
  <!-- Inner card: tilts via JS transform -->
  <div
    bind:this={cardEl}
    class="rounded-lg overflow-hidden bg-steam-surface border {isFullyCompleted ? 'border-amber-500/50 group-hover:border-amber-400 shadow-lg shadow-amber-500/10 group-hover:shadow-xl group-hover:shadow-amber-500/20' : 'border-steam-border group-hover:border-steam-primary/50 group-hover:shadow-lg group-hover:shadow-steam-primary/10'}"
    style="will-change: transform; transition: border-color 200ms, box-shadow 200ms;"
  >
    <div class="aspect-[460/215] bg-steam-input relative overflow-hidden">
      {#if !allImagesFailed}
        <img
          src={currentImageSrc}
          alt={game.name}
          decoding="async"
          class="w-full h-full object-cover"
          class:opacity-0={!imageLoaded}
          class:opacity-100={imageLoaded}
          onload={() => imageLoaded = true}
          onerror={handleImageError}
        />
      {/if}
      {#if !imageLoaded || allImagesFailed}
        <div class="absolute inset-0 flex items-center justify-center text-steam-text-dim text-xs p-2 text-center">
          {game.name || `App ${game.appId}`}
        </div>
      {/if}
    </div>
    {#if counts && counts.total > 0}
      <div class="relative h-4 bg-black/40">
        <div class="absolute left-0 top-0 bottom-0 {isFullyCompleted ? 'gold-bar' : 'bg-steam-primary/80'}" style="width: {completionPercent}%"></div>
      </div>
    {:else}
      <div class="h-4 bg-black/40"></div>
    {/if}
    {#if $settings.showLabels}
      <div class="p-2.5">
        <h3 class="text-sm font-medium text-steam-text truncate group-hover:text-steam-primary transition-colors">
          {game.name || `App ${game.appId}`}
        </h3>
        <div class="flex items-center gap-2 mt-0.5">
          <span class="text-xs text-steam-text-dim">AppID: {game.appId}</span>
          {#if game.lastPlayed}
            <span class="text-xs text-steam-text-dim" title={new Date(game.lastPlayed * 1000).toLocaleDateString()}>
              {formatLastPlayed(game.lastPlayed)}
            </span>
          {/if}
        </div>
      </div>
    {/if}
  </div>

  <!-- Static overlay: sits on top, never tilts -->
  <div class="absolute inset-0 pointer-events-none">
    <div class="aspect-[460/215] relative">
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
          <button
            onclick={handlePlay}
            class="pointer-events-auto absolute top-1.5 right-1.5 p-1 rounded-full bg-steam-success/80 cursor-pointer transition-opacity
              opacity-0 group-hover:opacity-100 text-white hover:bg-steam-success"
            title="Play game"
          >
            <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
              <path d="M8 5v14l11-7z" />
            </svg>
          </button>
        {:else}
          <button
            onclick={(e) => { e.stopPropagation(); BrowserOpenURL(`steam://install/${game.appId}`); }}
            class="pointer-events-auto absolute top-1.5 right-1.5 p-1 rounded-full bg-steam-primary/80 cursor-pointer transition-opacity
              opacity-0 group-hover:opacity-100 text-white hover:bg-steam-primary"
            title="Install game"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
            </svg>
          </button>
        {/if}
      {/if}
    </div>
    <div class="h-0 flex items-center justify-center relative z-10">
      {#if hasAchievementData}
        <span class="text-xs font-bold text-white px-2 py-0.5 rounded-full {isFullyCompleted ? 'bg-amber-500/90' : 'bg-black/60'}" style="text-shadow: 0 1px 2px rgba(0,0,0,0.8); backdrop-filter: blur(4px)">
          {#if isProtected}<svg class="w-3 h-3 inline-block -mt-px mr-0.5 text-amber-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" /></svg>{/if}{counts.achieved}/{counts.total}
        </span>
      {:else if counts && counts.total === 0}
        <span class="text-[11px] px-2 py-0.5 rounded-full {isEarlyAccess ? 'text-blue-300 bg-blue-500/30' : 'text-steam-text-dim bg-black/50'}" style="text-shadow: 0 1px 2px rgba(0,0,0,0.8)">{isEarlyAccess ? 'Early Access' : 'No achievements'}</span>
      {:else}
        <span class="text-[11px] text-steam-text-dim px-2 py-0.5 rounded-full bg-black/50" style="text-shadow: 0 1px 2px rgba(0,0,0,0.8)">&middot;&middot;&middot;</span>
      {/if}
    </div>
  </div>
</div>
