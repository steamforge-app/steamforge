const STEAM_CDN_BASE = 'https://shared.cloudflare.steamstatic.com/store_item_assets/steam/apps';

/**
 * Builds a prioritised list of Steam image URLs for a game.
 * Falls back through multiple CDN paths so at least one usually resolves.
 */
export function buildGameImageUrls(appId: number, logoUrl?: string): string[] {
  return [...new Set([
    logoUrl,
    `${STEAM_CDN_BASE}/${appId}/header.jpg`,
    `${STEAM_CDN_BASE}/${appId}/capsule_616x353.jpg`,
    `${STEAM_CDN_BASE}/${appId}/library_600x900.jpg`,
    `${STEAM_CDN_BASE}/${appId}/library_hero.jpg`,
    `${STEAM_CDN_BASE}/${appId}/page_bg_generated_v6b.jpg`,
    `${STEAM_CDN_BASE}/${appId}/capsule_sm_120.jpg`,
  ].filter(Boolean))] as string[];
}

/**
 * Builds a prioritised list of hero/banner image URLs for a game.
 */
export function buildHeroImageUrls(appId: number): string[] {
  return [
    `${STEAM_CDN_BASE}/${appId}/library_hero.jpg`,
    `${STEAM_CDN_BASE}/${appId}/header.jpg`,
    `${STEAM_CDN_BASE}/${appId}/capsule_616x353.jpg`,
    `${STEAM_CDN_BASE}/${appId}/page_bg_generated_v6b.jpg`,
  ];
}
