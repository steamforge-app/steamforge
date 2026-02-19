<script lang="ts">
  interface Props {
    title: string;
    message: string;
    confirmText?: string;
    cancelText?: string;
    onconfirm: () => void;
    oncancel: () => void;
  }

  let { title, message, confirmText = 'Confirm', cancelText = 'Cancel', onconfirm, oncancel }: Props = $props();
</script>

<div class="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
  <div class="bg-steam-surface rounded-lg shadow-xl p-6 max-w-md w-full mx-4 border border-steam-border">
    <h3 class="text-lg font-semibold text-steam-text mb-2">{title}</h3>
    <p class="text-steam-text-dim text-sm mb-6">{message}</p>
    <div class="flex justify-end gap-3">
      <button
        onclick={oncancel}
        class="px-4 py-2 text-sm rounded bg-steam-surface-light text-steam-text hover:bg-steam-primary-dark/30 transition-colors cursor-pointer"
      >
        {cancelText}
      </button>
      <button
        onclick={onconfirm}
        class="px-4 py-2 text-sm rounded bg-steam-primary text-steam-bg font-medium hover:bg-steam-primary-dark transition-colors cursor-pointer"
      >
        {confirmText}
      </button>
    </div>
  </div>
</div>
