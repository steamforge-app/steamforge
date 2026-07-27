<script lang="ts">
  import { onDestroy } from 'svelte';
  import { buildHeroImageUrls } from '../utils/steam-images';
  import { achievementsLoading } from '../stores/achievements';
  import { achievementCounts } from '../stores/games';
  import SettingsPanel from './SettingsPanel.svelte';
  import type { GameInfo } from '../stores/games';

  interface Props {
    appId: number;
    gameName: string;
    gameInstalled: boolean;
    playtimeHours: number | null;
    hltbLoading: boolean;
    hltbTimes: { main: number; mainExtra: number; completionist: number } | null;
    achievedCount: number;
    totalCount: number;
    progressPercent: number;
    hasMultipleGames: boolean;
    isFullscreen: boolean;
    scrollTop?: number;
    previousGame?: GameInfo;
    nextGame?: GameInfo;
    onback: () => void;
    onplay: () => void;
    oninstall: () => void;
    onstore: () => void;
    onprevious: () => void;
    onnext: () => void;
  }

  let {
    appId, gameName, gameInstalled, playtimeHours, hltbLoading, hltbTimes,
    achievedCount, totalCount, progressPercent, hasMultipleGames, isFullscreen,
    scrollTop = 0, previousGame, nextGame, onback, onplay, oninstall, onstore, onprevious, onnext
  }: Props = $props();

  let heroImageIndex = $state(0);
  let heroAllFailed = $state(false);
  let heroImageLoaded = $state(false);
  let settingsOpen = $state(false);
  let settingsBtn: HTMLButtonElement | undefined = $state();
  let scrollElement: HTMLDivElement | undefined = $state();

  const HERO_MAX = 300;
  const HERO_MIN = 120;
  let heroHeight = $state(HERO_MAX);
  let targetHeroHeight = HERO_MAX;
  let animationFrame: number | null = null;

  let heroImageUrls = $derived(buildHeroImageUrls(appId));
  let currentHeroSrc = $derived(heroImageUrls[heroImageIndex]);

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

  $effect(() => {
    appId;
    heroImageIndex = 0;
    heroAllFailed = false;
    heroImageLoaded = false;
    heroHeight = HERO_MAX;
    targetHeroHeight = HERO_MAX;
    if (animationFrame) {
      cancelAnimationFrame(animationFrame);
      animationFrame = null;
    }
  });

  $effect(() => {
    scrollTop;
    targetHeroHeight = Math.max(HERO_MIN, HERO_MAX - scrollTop);
    if (!animationFrame) smoothHeroHeight();
  });

  onDestroy(() => {
    if (animationFrame) cancelAnimationFrame(animationFrame);
  });
</script>

<div
  class="relative overflow-hidden bg-steam-surface shrink-0"
  style="height: {heroHeight}px; will-change: height"
>
  {#if !heroAllFailed}
    <img
      src={currentHeroSrc}
      alt={gameName}
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
        onclick={onprevious}
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
      onclick={onback}
      class="p-2 rounded-full bg-black/40 text-white/80 hover:text-white hover:bg-black/60 transition-colors cursor-pointer"
      title="Close"
    >
      <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
      </svg>
    </button>
  </div>
  <div class="absolute top-4 right-4 flex items-center gap-1.5">
    {#if gameInstalled}
      <button
        onclick={onplay}
        class="px-2.5 py-1 rounded bg-steam-success/80 text-white hover:bg-steam-success text-xs font-medium transition-colors cursor-pointer"
        title="Launch game via Steam"
      >
        Play
      </button>
    {:else}
      <button
        onclick={oninstall}
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
      onclick={onstore}
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
        onclick={onnext}
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
        {gameName}
      </h1>
      {#if $achievementCounts[String(appId)]?.earlyAccess}
        <span class="px-2 py-0.5 text-xs font-medium rounded bg-blue-500/80 text-white" style="text-shadow: 0 1px 2px rgba(0,0,0,0.5)">
          Early Access
        </span>
      {/if}
    </div>
    {#if (playtimeHours !== null && playtimeHours > 0) || hltbLoading || (hltbTimes && (hltbTimes.main > 0 || hltbTimes.mainExtra > 0 || hltbTimes.completionist > 0))}
      <div class="flex items-center gap-3 text-xs" style="text-shadow: 0 1px 4px rgba(0,0,0,0.7)">
        {#if playtimeHours !== null && playtimeHours > 0}
          <span class="text-sky-400" title="Your playtime"><span class="text-sky-400/60">Played</span> {Math.round(playtimeHours)}h</span>
        {/if}
        {#if hltbLoading}
          <span class="flex items-center gap-1.5 text-white/40">
            <span class="w-3 h-3 border border-white/40 border-t-transparent rounded-full animate-spin"></span>
            Loading times...
          </span>
        {:else if hltbTimes && (hltbTimes.main > 0 || hltbTimes.mainExtra > 0 || hltbTimes.completionist > 0)}
          <span class="flex items-center gap-3 text-white/60">
            {#if hltbTimes.main > 0}
              <span title="Main story"><span class="text-white/35">Main</span> {hltbTimes.main}h</span>
            {/if}
            {#if hltbTimes.mainExtra > 0}
              <span title="Main + Extras"><span class="text-white/35">+Extras</span> {hltbTimes.mainExtra}h</span>
            {/if}
            {#if hltbTimes.completionist > 0}
              <span title="Completionist"><span class="text-white/35">100%</span> {hltbTimes.completionist}h</span>
            {/if}
          </span>
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

<style>
  :global(.gold-bar) {
    background: linear-gradient(90deg, #fbbf24 0%, #f59e0b 50%, #d97706 100%);
  }
</style>
