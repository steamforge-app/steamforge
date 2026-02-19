<script lang="ts">
  import { achievements } from '../stores/achievements';
  import AchievementRow from './AchievementRow.svelte';

  interface Props {
    ontoggle: (id: string, achieved: boolean) => void;
    searchQuery: string;
    filterMode: string;
    sortMode: string;
    sortDirection: 'asc' | 'desc';
    selectedIds: Set<string>;
    onselect: (id: string, shiftKey: boolean) => void;
    allowLock: boolean;
    showUnlockDates: boolean;
    originalState: Map<string, boolean>;
    onvisiblechange?: (ids: string[]) => void;
  }

  let { ontoggle, searchQuery, filterMode, sortMode, sortDirection, selectedIds, onselect, allowLock, showUnlockDates, originalState, onvisiblechange }: Props = $props();

  let filtered = $derived.by(() => {
    let result = $achievements;

    if (searchQuery) {
      const query = searchQuery.toLowerCase();
      result = result.filter(achievement =>
        (achievement.name || '').toLowerCase().includes(query) ||
        (achievement.description || '').toLowerCase().includes(query) ||
        achievement.id.toLowerCase().includes(query)
      );
    }

    if (filterMode === 'locked') {
      result = result.filter(achievement => !achievement.isAchieved);
    } else if (filterMode === 'unlocked') {
      result = result.filter(achievement => achievement.isAchieved);
    } else if (filterMode === 'hidden') {
      result = result.filter(achievement => achievement.isHidden);
    }

    const dir = sortDirection === 'desc' ? -1 : 1;
    if (sortMode === 'name') {
      result = [...result].sort((first, second) => dir * (first.name || first.id).localeCompare(second.name || second.id));
    } else if (sortMode === 'percent') {
      result = [...result].sort((first, second) => dir * (first.percent - second.percent));
    } else if (sortMode === 'unlockTime') {
      result = [...result].sort((first, second) => dir * ((first.unlockTime || 0) - (second.unlockTime || 0)));
    }

    return result;
  });

  $effect(() => {
    onvisiblechange?.(filtered.map(a => a.id));
  });
</script>

{#if $achievements.length === 0}
  <div class="flex-1 flex items-center justify-center text-steam-text-dim">
    <p>No achievements found for this game</p>
  </div>
{:else if filtered.length === 0}
  <div class="flex-1 flex items-center justify-center text-steam-text-dim">
    <p>No achievements match the current filter</p>
  </div>
{:else}
  <div>
    {#each filtered as achievement (achievement.id)}
      <AchievementRow
        {achievement}
        {ontoggle}
        selected={selectedIds.has(achievement.id)}
        {onselect}
        {allowLock}
        {showUnlockDates}
        originalAchieved={originalState.get(achievement.id)}
      />
    {/each}
  </div>
{/if}
