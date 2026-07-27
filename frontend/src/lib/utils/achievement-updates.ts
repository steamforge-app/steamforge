import type { Achievement } from '../stores/achievements';

export function updateAchievementIsAchieved(
  list: Achievement[],
  id: string,
  achieved: boolean
): Achievement[] {
  return list.map(item =>
    item.id === id
      ? { ...item, isAchieved: achieved, unlockTime: achieved ? Math.floor(Date.now() / 1000) : 0 }
      : item
  );
}

export function batchUpdateAchievements(
  list: Achievement[],
  ids: Set<string>,
  achieved: boolean
): Achievement[] {
  return list.map(item =>
    ids.has(item.id)
      ? { ...item, isAchieved: achieved, unlockTime: achieved ? Math.floor(Date.now() / 1000) : 0 }
      : item
  );
}
