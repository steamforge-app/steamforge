import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';

export function showGameContextMenu(e: MouseEvent, appId: number, installed: boolean) {
  e.preventDefault();
  e.stopPropagation();
  const contextMenu = (window as any).__contextMenu;
  if (!contextMenu) return;
  const items = [
    { label: `Copy AppID (${appId})`, action: () => navigator.clipboard.writeText(String(appId)) },
    { label: 'Open Steam Store', action: () => BrowserOpenURL(`steam://store/${appId}`) },
    { label: 'View on SteamDB', action: () => BrowserOpenURL(`https://steamdb.info/app/${appId}/`) },
  ];
  if (installed) {
    items.push({ label: 'Uninstall', action: () => BrowserOpenURL(`steam://uninstall/${appId}`) });
  }
  contextMenu.show(e.clientX, e.clientY, items);
}
