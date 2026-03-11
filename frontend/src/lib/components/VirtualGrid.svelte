<script lang="ts" generics="T">
  import type { Snippet } from 'svelte';

  interface Props {
    scrollContainer: HTMLElement | undefined;
    items: T[];
    columnCount: number;
    rowHeight: number;
    overscan?: number;
    class?: string;
    style?: string;
    children: Snippet<[T]>;
    keyFn: (item: T) => string | number;
  }

  let {
    scrollContainer,
    items,
    columnCount,
    rowHeight,
    overscan = 3,
    class: className = '',
    style: styleProp = '',
    children,
    keyFn,
  }: Props = $props();

  let wrapperEl = $state<HTMLDivElement>();
  let startRow = $state(0);
  let endRow = $state(30);

  let totalRows = $derived(Math.ceil(items.length / columnCount));
  let totalHeight = $derived(totalRows * rowHeight);
  let spacerHeight = $derived(startRow * rowHeight);
  let visibleItems = $derived(
    items.slice(startRow * columnCount, Math.min(items.length, endRow * columnCount))
  );

  function recalculate() {
    if (!wrapperEl || !scrollContainer || rowHeight <= 0) return;

    const containerRect = scrollContainer.getBoundingClientRect();
    const wrapperRect = wrapperEl.getBoundingClientRect();
    const relativeTop = wrapperRect.top - containerRect.top;
    const offsetIntoWrapper = Math.max(0, -relativeTop);

    const rows = Math.ceil(items.length / columnCount);
    const newStartRow = Math.max(0, Math.floor(offsetIntoWrapper / rowHeight) - overscan);
    const newEndRow = Math.min(rows, Math.ceil((offsetIntoWrapper + containerRect.height) / rowHeight) + overscan);
    if (newStartRow !== startRow || newEndRow !== endRow) {
      startRow = newStartRow;
      endRow = newEndRow;
    }
  }

  let rafId = 0;

  function onScroll() {
    if (rafId) cancelAnimationFrame(rafId);
    rafId = requestAnimationFrame(recalculate);
  }

  // Scroll listener
  $effect(() => {
    if (!scrollContainer) return;
    scrollContainer.addEventListener('scroll', onScroll, { passive: true });
    return () => {
      scrollContainer!.removeEventListener('scroll', onScroll);
      if (rafId) { cancelAnimationFrame(rafId); rafId = 0; }
    };
  });

  // Viewport resize
  $effect(() => {
    if (!scrollContainer) return;
    const observer = new ResizeObserver(recalculate);
    observer.observe(scrollContainer);
    return () => observer.disconnect();
  });

  // Recalculate on prop/mount changes
  $effect(() => {
    items;
    columnCount;
    rowHeight;
    wrapperEl;
    scrollContainer;
    recalculate();
  });
</script>

{#if items.length > 0}
  <div bind:this={wrapperEl} style="height: {totalHeight}px; position: relative;">
    <div class={className} style="position: absolute; top: 0; left: 0; right: 0; will-change: transform; transform: translateY({spacerHeight}px);{styleProp ? ` ${styleProp}` : ''}">
      {#each visibleItems as item (keyFn(item))}
        {@render children(item)}
      {/each}
    </div>
  </div>
{/if}
