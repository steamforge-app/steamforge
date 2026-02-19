import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';

export function showGameContextMenu(e: MouseEvent, appId: number) {
  e.preventDefault();
  e.stopPropagation();
  const contextMenu = (window as any).__contextMenu;
  if (!contextMenu) return;
  contextMenu.show(e.clientX, e.clientY, [
    { label: `Copy AppID (${appId})`, action: () => navigator.clipboard.writeText(String(appId)) },
    { label: 'Open Steam Store', action: () => BrowserOpenURL(`steam://store/${appId}`) },
    { label: 'View on SteamDB', action: () => BrowserOpenURL(`https://steamdb.info/app/${appId}/`) },
  ]);
}
