import { writable, get } from 'svelte/store';
import { GetToPlayList, SetToPlay } from '../../../wailsjs/go/main/App';
import { addToast } from './app';

export const toPlayList = writable<Set<number>>(new Set());

export async function loadToPlayList(): Promise<void> {
  try {
    const appIds = await GetToPlayList();
    toPlayList.set(new Set(appIds || []));
  } catch {
    // Convenience layer only — leave the set empty rather than blocking the grid.
  }
}

export async function toggleToPlay(appId: number): Promise<void> {
  const wasOnList = get(toPlayList).has(appId);
  const want = !wasOnList;

  toPlayList.update(current => {
    const next = new Set(current);
    if (want) next.add(appId); else next.delete(appId);
    return next;
  });

  try {
    await SetToPlay(appId, want);
  } catch (e: any) {
    // Revert the optimistic update on failure
    toPlayList.update(current => {
      const next = new Set(current);
      if (wasOnList) next.add(appId); else next.delete(appId);
      return next;
    });
    addToast(`Failed to update games-to-play list: ${e.message || e}`, 'error');
  }
}
