<script lang="ts">
  import { updateSetting } from '../../stores/settings';

  interface Props {
    showLabels: boolean;
    showSoftware: boolean;
    showCardButtons: boolean;
    showUnlockDates: boolean;
  }

  let { showLabels, showSoftware, showCardButtons, showUnlockDates }: Props = $props();
</script>

<div class="flex flex-col gap-2">
  <span class="text-[10px] text-steam-text-dim uppercase tracking-wider font-medium">Game Library</span>
  <div class="flex items-center justify-between">
    <div class="flex flex-col">
      <span class="text-xs text-steam-text">Show Labels</span>
      <span class="text-[10px] text-steam-text-dim leading-tight">Show game names below cards</span>
    </div>
    <button
      onclick={() => updateSetting('showLabels', !showLabels)}
      title={showLabels ? 'Hide labels' : 'Show labels'}
      class="toggle-switch {showLabels ? 'active green' : ''}"
    >
      <span class="toggle-knob"></span>
    </button>
  </div>
  <div class="flex items-center justify-between">
    <div class="flex flex-col">
      <span class="text-xs text-steam-text">Show Software</span>
      <span class="text-[10px] text-steam-text-dim leading-tight">Show tools & redistributables</span>
    </div>
    <button
      onclick={() => updateSetting('showSoftware', !showSoftware)}
      title={showSoftware ? 'Hide software' : 'Show software'}
      class="toggle-switch {showSoftware ? 'active green' : ''}"
    >
      <span class="toggle-knob"></span>
    </button>
  </div>
  <div class="flex items-center justify-between">
    <div class="flex flex-col">
      <span class="text-xs text-steam-text">Card Buttons</span>
      <span class="text-[10px] text-steam-text-dim leading-tight">Show play, install & store buttons</span>
    </div>
    <button
      onclick={() => updateSetting('showCardButtons', !showCardButtons)}
      title={showCardButtons ? 'Hide card buttons' : 'Show card buttons'}
      class="toggle-switch {showCardButtons ? 'active green' : ''}"
    >
      <span class="toggle-knob"></span>
    </button>
  </div>
</div>

<div class="mt-3 pt-3 border-t border-steam-border flex flex-col gap-2">
  <span class="text-[10px] text-steam-text-dim uppercase tracking-wider font-medium">Achievements</span>
  <div class="flex items-center justify-between">
    <div class="flex flex-col">
      <span class="text-xs text-steam-text">Unlock Dates</span>
      <span class="text-[10px] text-steam-text-dim leading-tight">Show achievement unlock dates</span>
    </div>
    <button
      onclick={() => updateSetting('showUnlockDates', !showUnlockDates)}
      title={showUnlockDates ? 'Hide unlock dates' : 'Show unlock dates'}
      class="toggle-switch {showUnlockDates ? 'active green' : ''}"
    >
      <span class="toggle-knob"></span>
    </button>
  </div>
</div>
