import { writable, get } from 'svelte/store';

export interface Settings {
  viewMode: 'grid' | 'list';
  showLabels: boolean;
  sortBy: 'name' | 'appId' | 'lastPlayed' | 'achievements';
  sortOrder: 'asc' | 'desc';
  installedOpen: boolean;
  otherOpen: boolean;
  autoStore: boolean;
  allowLock: boolean;
  showUnlockDates: boolean;
  achievementSort: 'default' | 'name' | 'unlockTime' | 'percent';
  achievementSortDir: 'asc' | 'desc';
  showSoftware: boolean;
  showCardButtons: boolean;
  protectLastPlayed: boolean;
  cardMinWidth: number;
  windowWidth: number;
  windowHeight: number;
}

const defaults: Settings = {
  viewMode: 'grid',
  showLabels: false,
  sortBy: 'name',
  sortOrder: 'asc',
  installedOpen: true,
  otherOpen: true,
  autoStore: false,
  allowLock: false,
  showUnlockDates: true,
  achievementSort: 'unlockTime',
  achievementSortDir: 'asc',
  showSoftware: false,
  showCardButtons: true,
  protectLastPlayed: false,
  cardMinWidth: 200,
  windowWidth: 1280,
  windowHeight: 800,
};

export const settings = writable<Settings>(defaults);

export async function saveSettings() {
  const { SaveSettings } = await import('../../../wailsjs/go/main/App');
  const s = get(settings);
  try {
    await SaveSettings(s);
  } catch (e) {
    console.error('Failed to save settings:', e);
  }
}

export async function loadSettings() {
  const { GetSettings } = await import('../../../wailsjs/go/main/App');
  try {
    const s = await GetSettings();
    if (s) {
      settings.set({ ...defaults, ...s } as Settings);
    }
  } catch (e) {
    console.error('Failed to load settings:', e);
  }
}

export function updateSetting<K extends keyof Settings>(key: K, value: Settings[K]) {
  settings.update(s => ({ ...s, [key]: value }));
  saveSettings();
}

let debouncedSaveTimer: ReturnType<typeof setTimeout> | null = null;

export function updateSettingDebounced<K extends keyof Settings>(key: K, value: Settings[K]) {
  settings.update(s => ({ ...s, [key]: value }));
  if (debouncedSaveTimer) clearTimeout(debouncedSaveTimer);
  debouncedSaveTimer = setTimeout(() => {
    debouncedSaveTimer = null;
    saveSettings();
  }, 300);
}

export function resetSettings() {
  settings.set({ ...defaults });
  saveSettings();
}

export function setSortColumn(column: Settings['sortBy']) {
  settings.update(s => {
    if (s.sortBy === column) {
      return { ...s, sortOrder: s.sortOrder === 'asc' ? 'desc' as const : 'asc' as const };
    }
    const defaultDesc = column === 'lastPlayed' || column === 'achievements';
    return { ...s, sortBy: column, sortOrder: defaultDesc ? 'desc' as const : 'asc' as const };
  });
  saveSettings();
}
