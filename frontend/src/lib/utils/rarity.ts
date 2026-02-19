export interface RarityTier {
  color: string;
  label: string;
  borderColor: string;
  glow: string;
}

const ULTRA_RARE: RarityTier = {
  color: '#fbbf24',
  label: 'Ultra Rare',
  borderColor: '#fbbf24',
  glow: '0 0 8px 3px rgba(251,191,36,0.5), 0 0 16px 6px rgba(251,191,36,0.25)',
};

const RARE: RarityTier = {
  color: '#a78bfa',
  label: 'Rare',
  borderColor: '#a78bfa',
  glow: '0 0 8px 2px rgba(167,139,250,0.45), 0 0 14px 4px rgba(167,139,250,0.2)',
};

const UNCOMMON: RarityTier = {
  color: '#60a5fa',
  label: 'Uncommon',
  borderColor: '#60a5fa',
  glow: '0 0 6px 2px rgba(96,165,250,0.35), 0 0 12px 3px rgba(96,165,250,0.15)',
};

const COMMON: RarityTier = {
  color: '#8f98a0',
  label: 'Common',
  borderColor: '',
  glow: '',
};

const ULTRA_RARE_THRESHOLD = 5;
const RARE_THRESHOLD = 15;
const UNCOMMON_THRESHOLD = 30;

/**
 * Returns the rarity tier for a given global unlock percentage.
 */
export function getRarityTier(percent: number): RarityTier {
  if (percent < ULTRA_RARE_THRESHOLD) return ULTRA_RARE;
  if (percent < RARE_THRESHOLD) return RARE;
  if (percent < UNCOMMON_THRESHOLD) return UNCOMMON;
  return COMMON;
}
