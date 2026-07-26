import { writable } from 'svelte/store';

export type Page = 'picker' | 'manager';

export const currentPage = writable<Page>('picker');
export const selectedAppId = writable<number>(0);
export const selectedGameName = writable<string>('');
export const selectedGameInstalled = writable<boolean>(false);
export const isConnected = writable<boolean>(false);
export const steamId = writable<string>('');
export const personaName = writable<string>('');
export const isLoading = writable<boolean>(false);
export const loadingMessage = writable<string>('');
export const scanning = writable<boolean>(false);
export const profilePublic = writable<'unknown' | 'public' | 'private'>('unknown');
export const scanProgress = writable<{ current: number; total: number; name: string }>({ current: 0, total: 0, name: '' });
export const gameComplete = writable<boolean>(false);

export type GameFilter = 'all' | 'incomplete' | 'perfected' | 'none' | 'toPlay' | 'abandoned';
export const gameFilter = writable<GameFilter>('all');

export interface ToastAction {
  label: string;
  callback: () => void;
}

export interface ToastMessage {
  id: number;
  text: string;
  type: 'success' | 'error' | 'info';
  action?: ToastAction;
  persistent?: boolean;
}

let toastId = 0;
export const toasts = writable<ToastMessage[]>([]);

export function addToast(text: string, type: 'success' | 'error' | 'info' = 'info', action?: ToastAction, persistent = false) {
  const id = ++toastId;
  toasts.update(current => [...current, { id, text, type, action, persistent }]);
  if (!persistent) {
    const timeout = action ? 6000 : 4000;
    setTimeout(() => {
      toasts.update(current => current.filter(toast => toast.id !== id));
    }, timeout);
  }
}

export function dismissToast(id: number) {
  toasts.update(current => current.filter(toast => toast.id !== id));
}

export function navigateToManager(appId: number, gameName: string, installed: boolean = false) {
  selectedAppId.set(appId);
  selectedGameName.set(gameName);
  selectedGameInstalled.set(installed);
  currentPage.set('manager');
}

export function navigateToPicker() {
  currentPage.set('picker');
}
