<script lang="ts">
  import type { Achievement } from '../stores/achievements';
  import { getRarityTier } from '../utils/rarity';

  interface Props {
    achievement: Achievement;
    ontoggle: (id: string, achieved: boolean) => void;
    selected: boolean;
    onselect: (id: string, shiftKey: boolean) => void;
    allowLock: boolean;
    showUnlockDates: boolean;
    originalAchieved?: boolean;
  }

  let { achievement, ontoggle, selected, onselect, allowLock, showUnlockDates, originalAchieved }: Props = $props();
  let iconLoadFailed = $state(false);

  // Block locking when allowLock is false, UNLESS this is a pending unsaved change (originally locked, now unlocked)
  let isToggleDisabled = $derived(
    achievement.isAchieved && !allowLock && originalAchieved !== false
  );

  function handleToggle(e: Event) {
    e.stopPropagation();
    if (isToggleDisabled) return;
    ontoggle(achievement.id, !achievement.isAchieved);
  }

  function handleRowClick(e: MouseEvent) {
    onselect(achievement.id, e.shiftKey);
  }

  function formatUnlockDate(timestamp: number): string {
    if (!timestamp) return '';
    return new Date(timestamp * 1000).toLocaleDateString(undefined, {
      year: 'numeric', month: 'short', day: 'numeric',
      hour: '2-digit', minute: '2-digit'
    });
  }

  let iconSrc = $derived(
    achievement.isAchieved ? achievement.iconUrl : (achievement.iconGrayUrl || achievement.iconUrl)
  );

  let rarity = $derived(achievement.percent > 0 ? getRarityTier(achievement.percent) : null);

  let rowBackgroundGradient = $derived(
    achievement.isAchieved
      ? 'linear-gradient(to right, rgba(91,163,43,0.12), transparent 60%)'
      : achievement.isHidden
        ? 'linear-gradient(to right, rgba(0,0,0,0.15), transparent 60%)'
        : 'linear-gradient(to right, rgba(200,60,60,0.10), transparent 60%)'
  );
</script>

<div
  role="row"
  tabindex="0"
  onclick={handleRowClick}
  onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onselect(achievement.id, e.shiftKey); } }}
  class="flex items-center gap-2 pr-4 py-3 pl-3 transition-all duration-150 border-b border-steam-border/50 cursor-pointer
    {selected ? 'bg-steam-primary/5' : 'hover:bg-steam-hover'}"
  style="background: {selected ? '' : rowBackgroundGradient}"
>
  <div class="w-4 h-4 flex-shrink-0 rounded border transition-colors duration-150 flex items-center justify-center
    {selected ? 'bg-steam-primary border-steam-primary' : 'border-steam-border'}"
  >
    {#if selected}
      <svg class="w-2.5 h-2.5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7" />
      </svg>
    {/if}
  </div>
  <div class="relative flex-shrink-0 w-12 h-12">
    {#if rarity && achievement.percent < 5}
      <div class="rarity-rays" style="--ray-color: {rarity.borderColor}"></div>
    {/if}
    <div
      class="w-12 h-12 rounded bg-steam-input overflow-hidden relative z-[1]"
      style="{rarity?.borderColor ? `border: 2px solid ${rarity.borderColor};` : ''}{rarity?.glow ? ` box-shadow: ${rarity.glow};` : ''}"
    >
      {#if iconSrc && !iconLoadFailed}
        <img
          src={iconSrc}
          alt={achievement.name}
          class="w-full h-full object-cover"
          onerror={() => iconLoadFailed = true}
        />
      {:else}
        <div class="w-full h-full flex items-center justify-center text-steam-text-dim text-xs">?</div>
      {/if}
    </div>
  </div>

  <div class="flex-1 min-w-0">
    <div class="flex items-center gap-2">
      <span class="text-sm font-medium text-steam-text truncate">
        {achievement.name || achievement.id}
      </span>
      {#if achievement.isHidden}
        <span class="text-xs text-steam-text-dim bg-steam-input px-1.5 py-0.5 rounded">hidden</span>
      {/if}
    </div>
    <p class="text-xs text-steam-text-dim truncate mt-0.5">
      {achievement.description || 'No description'}
    </p>
    {#if showUnlockDates && achievement.isAchieved && achievement.unlockTime}
      <p class="text-xs text-steam-success mt-0.5">Unlocked: {formatUnlockDate(achievement.unlockTime)}</p>
    {/if}
  </div>

  <div class="flex items-center gap-3 flex-shrink-0">
    {#if rarity}
      <span class="text-xs w-14 text-right" style="color: {rarity.color}" title={rarity.label}>{achievement.percent.toFixed(1)}%</span>
    {/if}
    <button
      onclick={handleToggle}
      disabled={isToggleDisabled}
      aria-label={achievement.isAchieved ? 'Lock achievement' : 'Unlock achievement'}
      class="p-1.5 rounded transition-colors duration-150
        {isToggleDisabled ? 'opacity-40 cursor-not-allowed' : 'cursor-pointer hover:bg-white/10'}
        {achievement.isAchieved ? 'text-steam-success' : 'text-steam-text-dim'}"
    >
      <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        {#if achievement.isAchieved}
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 11V7a4 4 0 118 0m-4 8v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2z" />
        {:else}
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
        {/if}
      </svg>
    </button>
  </div>
</div>
