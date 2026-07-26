import { GetHLTBTimes } from '../../../wailsjs/go/main/App';
import { hltbCache } from '../stores/games';
import { get } from 'svelte/store';

const inFlight = new Set<number>();

export function fetchHLTBIfMissing(appId: number, gameName: string): void {
  if (appId <= 0 || !gameName) return;
  if (get(hltbCache)[String(appId)] || inFlight.has(appId)) return;
  inFlight.add(appId);
  GetHLTBTimes(appId, gameName)
    .then(result => {
      if (result) hltbCache.update(cache => ({ ...cache, [String(appId)]: result }));
    })
    .catch(() => { /* transient failure — retried next time this card is visible */ })
    .finally(() => inFlight.delete(appId));
}
