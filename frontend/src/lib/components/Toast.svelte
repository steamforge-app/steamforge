<script lang="ts">
  import { toasts, dismissToast } from '../stores/app';
</script>

{#if $toasts.length > 0}
  <div class="fixed bottom-8 right-4 z-50 flex flex-col-reverse gap-1.5">
    {#each $toasts as toast (toast.id)}
      <div
        class="px-3 py-1.5 rounded shadow-md text-xs max-w-xs animate-slide-in flex items-center gap-2
          {toast.type === 'success' ? 'bg-steam-success/90 text-white/90' :
           toast.type === 'error' ? 'bg-steam-danger/90 text-white/90' :
           'bg-steam-surface-light/90 text-steam-text-dim'}"
      >
        <span>{toast.text}</span>
        {#if toast.action}
          <button
            onclick={() => { toast.action?.callback(); dismissToast(toast.id); }}
            class="ml-auto px-1.5 py-0.5 text-[10px] rounded bg-white/20 hover:bg-white/30 transition-colors cursor-pointer whitespace-nowrap"
          >
            {toast.action.label}
          </button>
        {/if}
        {#if toast.persistent}
          <button
            onclick={() => dismissToast(toast.id)}
            class="p-0.5 rounded hover:bg-white/20 transition-colors cursor-pointer {toast.action ? '' : 'ml-auto'}"
            title="Dismiss"
          >
            <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        {/if}
      </div>
    {/each}
  </div>
{/if}

<style>
  @keyframes slide-in {
    from {
      transform: translateX(100%);
      opacity: 0;
    }
    to {
      transform: translateX(0);
      opacity: 1;
    }
  }
  .animate-slide-in {
    animation: slide-in 0.2s ease-out;
  }
</style>
