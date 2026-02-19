<script lang="ts">
  import { writable } from 'svelte/store';

  interface MenuItem {
    label: string;
    action: () => void;
  }

  interface MenuState {
    visible: boolean;
    x: number;
    y: number;
    items: MenuItem[];
  }

  const menu = writable<MenuState>({ visible: false, x: 0, y: 0, items: [] });

  // Expose globally so GameCard/GameListRow can trigger it
  if (typeof window !== 'undefined') {
    (window as any).__contextMenu = {
      show(x: number, y: number, items: MenuItem[]) {
        menu.set({ visible: true, x, y, items });
      }
    };
  }

  function hide() {
    menu.update(menuState => ({ ...menuState, visible: false }));
  }
</script>

<svelte:window onclick={hide} oncontextmenu={() => hide()} />

{#if $menu.visible}
  <div
    class="fixed z-50 bg-steam-surface border border-steam-border rounded-lg shadow-xl py-1 min-w-[180px]"
    style="left: {$menu.x}px; top: {$menu.y}px;"
  >
    {#each $menu.items as item}
      <button
        onclick={() => { item.action(); hide(); }}
        class="w-full text-left px-4 py-2 text-sm text-steam-text hover:bg-steam-hover transition-colors cursor-pointer"
      >
        {item.label}
      </button>
    {/each}
  </div>
{/if}
