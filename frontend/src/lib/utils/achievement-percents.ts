import type { Achievement } from '../stores/achievements';

/**
 * Manages background fetching of global achievement completion percentages.
 * Handles retry logic for failed fetches with exponential backoff.
 */
export interface PercentManager {
  fetchPercents(appId: number): void;
  stopFetch(): void;
}

export function createPercentManager(
  achievementsStore: any,
  cachePercentsFunc: (appId: number, achievements: Achievement[]) => void,
  buildPercentLookupFunc: (percents: any) => Map<string, number>,
  FetchGlobalPercents: (appId: number) => Promise<any>,
  selectedAppId: () => number
): PercentManager {
  let percentRetryTimer: ReturnType<typeof setTimeout> | null = null;

  function stopFetch() {
    if (percentRetryTimer) {
      clearTimeout(percentRetryTimer);
      percentRetryTimer = null;
    }
  }

  // Fetches percents immediately — no artificial delay. The backend already
  // returns a cached value instantly (fresh or stale) when one exists, and
  // refreshes stale values in the background (see the "percents-updated"
  // listener in App.svelte). Retries here only cover genuine fetch failures.
  function fetchPercents(appId: number) {
    stopFetch();
    let attempt = 0;
    const maxAttempts = 2; // ~2s, 4s

    async function attemptFetch() {
      try {
        const percents = await FetchGlobalPercents(appId);
        if (appId !== selectedAppId()) return; // navigated away — drop the result
        if (percents && Object.keys(percents).length > 0) {
          const lookup = buildPercentLookupFunc(percents);
          achievementsStore.update((list: Achievement[]) => {
            const updated = list.map(achievement => {
              if (achievement.percent !== 0) return achievement;
              const percent = lookup.get(achievement.id.toLowerCase());
              return percent ? { ...achievement, percent } : achievement;
            });
            cachePercentsFunc(appId, updated);
            return updated;
          });
        }
      } catch {
        attempt++;
        if (attempt < maxAttempts) {
          percentRetryTimer = setTimeout(attemptFetch, 2000 * attempt);
        }
      }
    }

    attemptFetch();
  }

  return {
    fetchPercents,
    stopFetch
  };
}
