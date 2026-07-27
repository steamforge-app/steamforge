import type { Achievement } from '../stores/achievements';

/**
 * Manages the "dirty" state of achievements and syncing to persistent storage.
 * Tracks which achievements have been modified and provides utilities for
 * storing changes and reverting to the last saved state.
 */
export interface AchievementStorage {
  originalState: Map<string, boolean>;
  pendingChanges: number;
  isDirty: boolean;

  snapshotState(achievements: Achievement[]): void;
  syncAchievementCounts(appId: number, achievements: Achievement[], counts: any): void;
  calculateDirty(achievements: Achievement[]): boolean;
  calculatePendingChanges(achievements: Achievement[]): number;
  discardChanges(achievements: Achievement[], originalState: Map<string, boolean>): Achievement[];
  captureChangesToApply(achievements: Achievement[], originalState: Map<string, boolean>): Array<{ id: string; achieved: boolean }>;
}

export function createAchievementStorage(): AchievementStorage {
  let originalState = new Map<string, boolean>();

  function snapshotState(achievements: Achievement[]) {
    const snapshot = new Map<string, boolean>();
    for (const achievement of achievements) {
      snapshot.set(achievement.id, achievement.isAchieved);
    }
    originalState = snapshot;
  }

  function syncAchievementCounts(
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

  function calculateDirty(achievements: Achievement[]): boolean {
    if (originalState.size === 0) return false;
    return achievements.some(achievement => originalState.get(achievement.id) !== achievement.isAchieved);
  }

  function calculatePendingChanges(achievements: Achievement[]): number {
    if (originalState.size === 0) return 0;
    let count = 0;
    for (const achievement of achievements) {
      if (originalState.get(achievement.id) !== achievement.isAchieved) count++;
    }
    return count;
  }

  function discardChanges(
    achievements: Achievement[],
    _originalState: Map<string, boolean>
  ): Achievement[] {
    return achievements.map(item => {
      const original = originalState.get(item.id);
      if (original === undefined || original === item.isAchieved) return item;
      return { ...item, isAchieved: original, unlockTime: original ? item.unlockTime : 0 };
    });
  }

  function captureChangesToApply(
    achievements: Achievement[],
    _originalState: Map<string, boolean>
  ): Array<{ id: string; achieved: boolean }> {
    const changesToApply: Array<{ id: string; achieved: boolean }> = [];
    for (const achievement of achievements) {
      const original = originalState.get(achievement.id);
      if (original !== undefined && original !== achievement.isAchieved) {
        changesToApply.push({ id: achievement.id, achieved: achievement.isAchieved });
      }
    }
    return changesToApply;
  }

  return {
    get originalState() {
      return originalState;
    },
    set originalState(value: Map<string, boolean>) {
      originalState = value;
    },
    get pendingChanges(): number {
      return 0; // This is calculated dynamically
    },
    get isDirty(): boolean {
      return originalState.size > 0;
    },
    snapshotState,
    syncAchievementCounts,
    calculateDirty,
    calculatePendingChanges,
    discardChanges,
    captureChangesToApply
  };
}
