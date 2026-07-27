import type { Achievement } from '../stores/achievements';

/**
 * Determines how unlockTime should be handled when updating an achievement.
 * - 'stamp': Set unlockTime to current timestamp when achieved=true, zero when false
 * - 'preserve': Keep existing unlockTime when achieved=true, zero when false (for UI-only updates)
 * - 'zero': Always zero the unlockTime (for clearing/locking)
 * - 'ignore': Leave unlockTime completely untouched
 */
type UnlockTimeMode = 'stamp' | 'preserve' | 'zero' | 'ignore';

export function updateAchievementIsAchieved(
  list: Achievement[],
  id: string,
  achieved: boolean,
  unlockTimeMode: UnlockTimeMode = 'stamp'
): Achievement[] {
  return list.map(item => {
    if (item.id !== id) return item;

    let unlockTime = item.unlockTime;
    if (unlockTimeMode === 'stamp') {
      unlockTime = achieved ? Math.floor(Date.now() / 1000) : 0;
    } else if (unlockTimeMode === 'preserve') {
      unlockTime = achieved ? item.unlockTime : 0;
    } else if (unlockTimeMode === 'zero') {
      unlockTime = 0;
    }
    // 'ignore' leaves unlockTime as-is

    return { ...item, isAchieved: achieved, unlockTime };
  });
}

export function batchUpdateAchievements(
  list: Achievement[],
  ids: Set<string>,
  achieved: boolean,
  unlockTimeMode: UnlockTimeMode = 'stamp'
): Achievement[] {
  return list.map(item => {
    if (!ids.has(item.id)) return item;

    let unlockTime = item.unlockTime;
    if (unlockTimeMode === 'stamp') {
      unlockTime = achieved ? Math.floor(Date.now() / 1000) : 0;
    } else if (unlockTimeMode === 'preserve') {
      unlockTime = achieved ? item.unlockTime : 0;
    } else if (unlockTimeMode === 'zero') {
      unlockTime = 0;
    }
    // 'ignore' leaves unlockTime as-is

    return { ...item, isAchieved: achieved, unlockTime };
  });
}

/**
 * Locks all editable (non-protected) achievements in the list.
 * Protected achievements (permission > 0) are left unchanged.
 */
export function lockAllEditableAchievements(list: Achievement[]): Achievement[] {
  return list.map(item =>
    item.permission > 0 ? item : { ...item, isAchieved: false, unlockTime: 0 }
  );
}
