import { writable, derived } from 'svelte/store';
import { settings } from './settings';
import { gameFilter } from './app';
import { toPlayList } from './toplay';

export interface GameInfo {
  appId: number;
  name: string;
  logoUrl: string;
  installed: boolean;
  lastPlayed: number;
  isSoftware: boolean;
}

export interface AchievementCount {
  achieved: number;
  total: number;
  earlyAccess?: boolean;
  releaseDate?: string;
  protected?: boolean;
}

export const games = writable<GameInfo[]>([]);
export const searchQuery = writable<string>('');
export const gamesLoading = writable<boolean>(false);
export const achievementCounts = writable<Record<string, AchievementCount>>({});

export interface HLTBCacheEntry {
  main: number;
  mainExtra: number;
  completionist: number;
}

export const playtimes = writable<Record<string, number>>({});
export const hltbCache = writable<Record<string, HLTBCacheEntry>>({});

export const filteredGames = derived(
  [games, searchQuery, settings, achievementCounts, gameFilter, toPlayList],
  ([$games, $searchQuery, $settings, $achievementCounts, $gameFilter, $toPlayList]) => {
    let result = $games;

    // Hide software/tools unless the setting is enabled
    if (!$settings.showSoftware) {
      result = result.filter(game => !game.isSoftware);
    }

    if ($searchQuery) {
      const query = $searchQuery.toLowerCase();
      result = result.filter(game => game.name.toLowerCase().includes(query));
    }

    const { sortBy, sortOrder } = $settings;

    if ($gameFilter === 'toPlay') {
      result = result.filter(game => $toPlayList.has(game.appId));
    } else if ($gameFilter && $gameFilter !== 'all') {
      result = result.filter(game => {
        const counts = $achievementCounts[String(game.appId)];
        if ($gameFilter === 'none') return !counts || counts.total === 0;
        if (!counts || counts.total === 0) return false;
        if ($gameFilter === 'perfected') return counts.achieved === counts.total;
        if ($gameFilter === 'incomplete') return counts.achieved < counts.total;
        return true;
      });
    }

    result = [...result].sort((gameA, gameB) => {
      let comparison = 0;
      if (sortBy === 'appId') {
        comparison = gameA.appId - gameB.appId;
      } else if (sortBy === 'lastPlayed') {
        const lastPlayedA = gameA.lastPlayed || 0;
        const lastPlayedB = gameB.lastPlayed || 0;
        if (lastPlayedA === 0 && lastPlayedB === 0) comparison = 0;
        else if (lastPlayedA === 0) comparison = -1;
        else if (lastPlayedB === 0) comparison = 1;
        else comparison = lastPlayedA - lastPlayedB;
      } else if (sortBy === 'achievements') {
        const countA = $achievementCounts[String(gameA.appId)];
        const countB = $achievementCounts[String(gameB.appId)];
        const percentA = countA && countA.total > 0 ? countA.achieved / countA.total : -1;
        const percentB = countB && countB.total > 0 ? countB.achieved / countB.total : -1;
        if (percentA === percentB) comparison = 0;
        else if (percentA === -1) comparison = -1;
        else if (percentB === -1) comparison = 1;
        else comparison = percentA - percentB;
      } else {
        comparison = gameA.name.toLowerCase().localeCompare(gameB.name.toLowerCase());
      }
      return sortOrder === 'desc' ? -comparison : comparison;
    });

    return result;
  }
);

// Display order: installed games first, then non-installed, matching GameGrid's layout.
// Used for prev/next navigation in GameManager.
export const navigableGames = derived(filteredGames, ($filteredGames) => {
  const installed = $filteredGames.filter(game => game.installed);
  const other = $filteredGames.filter(game => !game.installed);
  return [...installed, ...other];
});
