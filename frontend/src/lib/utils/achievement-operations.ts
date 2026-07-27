/**
 * Achievement state synchronization and initialization utilities.
 */
import type { Achievement } from '../stores/achievements';

/**
 * Updates the achievement counts for a game in the cache.
 */
export function syncAchievementCounts(
  appId: number,
  achievements: Achievement[],
  countsStore: any
) {
  const achieved = achievements.filter(a => a.isAchieved).length;
  const total = achievements.length;
  const isProtected = achievements.some(a => a.permission > 0);
  countsStore.update((counts: any) => {
    const existing = counts[String(appId)];
    return {
      ...counts,
      [String(appId)]: {
        achieved,
        total,
        earlyAccess: existing?.earlyAccess,
        protected: isProtected || existing?.protected,
      }
    };
  });
}

/**
 * Creates a snapshot of achievement states for dirty tracking.
 */
export function snapshotAchievements(achievements: Achievement[]): Map<string, boolean> {
  const snapshot = new Map<string, boolean>();
  for (const achievement of achievements) {
    snapshot.set(achievement.id, achievement.isAchieved);
  }
  return snapshot;
}

/**
 * Computes the number of pending changes compared to a snapshot.
 */
export function calculatePendingChanges(
  achievements: Achievement[],
  originalState: Map<string, boolean>
): number {
  if (originalState.size === 0) return 0;
  let count = 0;
  for (const achievement of achievements) {
    if (originalState.get(achievement.id) !== achievement.isAchieved) count++;
  }
  return count;
}

/**
 * Captures all changes made since the last snapshot for storage.
 */
export function captureChangesToApply(
  achievements: Achievement[],
  originalState: Map<string, boolean>
): Array<{ id: string; achieved: boolean }> {
  const changes: Array<{ id: string; achieved: boolean }> = [];
  for (const achievement of achievements) {
    const original = originalState.get(achievement.id);
    if (original !== undefined && original !== achievement.isAchieved) {
      changes.push({ id: achievement.id, achieved: achievement.isAchieved });
    }
  }
  return changes;
}

/**
 * Filters achievements to find those editable by the user.
 */
export function getEditableAchievements(
  achievements: Achievement[],
  selectedIds: Set<string>
): { editable: string[]; skipped: number } {
  const editable = [...selectedIds].filter(id => {
    const match = achievements.find(a => a.id === id);
    return match && !match.permission;
  });
  return {
    editable,
    skipped: selectedIds.size - editable.length
  };
}
