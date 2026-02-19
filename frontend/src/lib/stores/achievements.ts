import { writable } from 'svelte/store';

export interface Achievement {
  id: string;
  name: string;
  description: string;
  iconUrl: string;
  iconGrayUrl: string;
  isAchieved: boolean;
  unlockTime: number;
  isHidden: boolean;
  permission: number;
  percent: number;
}

export const achievements = writable<Achievement[]>([]);
export const achievementsLoading = writable<boolean>(false);

// Module-level percent cache — persists across component mounts within the same session.
// Keyed by appId, stores a map of achievementId → percent.
const percentCache = new Map<number, Map<string, number>>();

export function cachePercents(appId: number, achievementList: Achievement[]) {
  const percents = new Map<string, number>();
  for (const achievement of achievementList) {
    if (achievement.percent > 0) {
      percents.set(achievement.id, achievement.percent);
    }
  }
  if (percents.size > 0) {
    percentCache.set(appId, percents);
  }
}

export function applyCachedPercents(appId: number, achievementList: Achievement[]): Achievement[] {
  const cached = percentCache.get(appId);
  if (!cached) return achievementList;
  return achievementList.map(achievement => {
    if (achievement.percent === 0) {
      const cachedPercent = cached.get(achievement.id);
      if (cachedPercent) {
        return { ...achievement, percent: cachedPercent };
      }
    }
    return achievement;
  });
}
