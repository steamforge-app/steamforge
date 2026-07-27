import type { AchievementCount, HLTBCacheEntry } from '../stores/games';

export interface GameCardState {
  counts: AchievementCount | undefined;
  hasAchievementData: boolean;
  isFullyCompleted: boolean;
  playtimeHours: number | undefined;
  hltbMain: number | undefined;
  hltbCompletionist: number | undefined;
}

export function deriveGameCardState(
  appId: number,
  achievementCounts: Record<string, AchievementCount>,
  playtimes: Record<string, number>,
  hltbCache: Record<string, HLTBCacheEntry>
): GameCardState {
  const counts = achievementCounts[String(appId)];
  const hasAchievementData = !!counts && counts.total > 0 && counts.achieved >= 0;
  return {
    counts,
    hasAchievementData,
    isFullyCompleted: hasAchievementData && counts!.achieved === counts!.total,
    playtimeHours: playtimes[String(appId)],
    hltbMain: hltbCache[String(appId)]?.main,
    hltbCompletionist: hltbCache[String(appId)]?.completionist,
  };
}
