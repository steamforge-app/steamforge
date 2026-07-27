<script lang="ts">
  import { updateSetting } from '../../stores/settings';
  import { profilePublic, steamId } from '../../stores/app';
  import { CheckProfileVisibility } from '../../../../wailsjs/go/main/App';
  import { BrowserOpenURL } from '../../../../wailsjs/runtime/runtime';

  interface Props {
    autoStore: boolean;
    allowLock: boolean;
    protectLastPlayed: boolean;
  }

  let { autoStore, allowLock, protectLastPlayed }: Props = $props();

  let profileStatus = $state<'unknown' | 'checking' | 'public' | 'private'>(
    $profilePublic === 'public' ? 'public' : $profilePublic === 'private' ? 'private' : 'unknown'
  );

  async function checkProfile() {
    profileStatus = 'checking';
    try {
      const result = await CheckProfileVisibility();
      profileStatus = result === 'public' ? 'public' : 'private';
      profilePublic.set(result === 'public' ? 'public' : 'private');
    } catch {
      profileStatus = 'unknown';
    }
  }

  $effect(() => {
    if (profileStatus === 'unknown') checkProfile();
  });
</script>

<div class="flex flex-col gap-3">
  <div class="flex items-center justify-between">
    <div class="flex flex-col">
      <span class="text-xs text-steam-text">Auto Save</span>
      <span class="text-[10px] text-steam-text-dim leading-tight">Save changes immediately</span>
    </div>
    <button
      onclick={() => updateSetting('autoStore', !autoStore)}
      title={autoStore ? 'Disable auto save' : 'Enable auto save'}
      class="toggle-switch {autoStore ? 'active green' : ''}"
    >
      <span class="toggle-knob"></span>
    </button>
  </div>
  <div class="flex items-center justify-between">
    <div class="flex flex-col">
      <span class="text-xs text-steam-text">Allow Lock</span>
      <span class="text-[10px] text-steam-text-dim leading-tight">Enable locking achievements</span>
    </div>
    <button
      onclick={() => updateSetting('allowLock', !allowLock)}
      title={allowLock ? 'Disable locking' : 'Enable locking'}
      class="toggle-switch {allowLock ? 'active amber' : ''}"
    >
      <span class="toggle-knob"></span>
    </button>
  </div>
  <div class="flex items-center justify-between">
    <div class="flex flex-col">
      <span class="text-xs text-steam-text">Protect Last Played</span>
      <span class="text-[10px] text-steam-text-dim leading-tight">Defer connection until saving</span>
    </div>
    <button
      onclick={() => updateSetting('protectLastPlayed', !protectLastPlayed)}
      title={protectLastPlayed ? 'Disable last played protection' : 'Enable last played protection'}
      class="toggle-switch {protectLastPlayed ? 'active green' : ''}"
    >
      <span class="toggle-knob"></span>
    </button>
  </div>
</div>

<div class="mt-3 pt-3 border-t border-steam-border">
  <span class="text-[10px] text-steam-text-dim uppercase tracking-wider font-medium">Steam Profile</span>
  <div class="mt-2 flex items-center justify-between px-3 py-2 rounded-lg bg-steam-input border border-steam-border">
    <div class="flex items-center gap-2">
      {#if profileStatus === 'checking'}
        <span class="w-2 h-2 rounded-full bg-steam-text-dim animate-pulse"></span>
        <span class="text-xs text-steam-text-dim">Checking...</span>
      {:else if profileStatus === 'public'}
        <span class="w-2 h-2 rounded-full bg-steam-success"></span>
        <span class="text-xs text-steam-text">Public</span>
      {:else if profileStatus === 'private'}
        <span class="w-2 h-2 rounded-full bg-red-400"></span>
        <span class="text-xs text-red-400">Private</span>
      {:else}
        <span class="w-2 h-2 rounded-full bg-steam-text-dim"></span>
        <span class="text-xs text-steam-text-dim">Unknown</span>
      {/if}
    </div>
    {#if profileStatus !== 'checking'}
      <button onclick={checkProfile} class="text-[10px] text-steam-primary hover:text-steam-text cursor-pointer transition-colors">
        Recheck
      </button>
    {/if}
  </div>
  {#if profileStatus === 'private'}
    <p class="text-[10px] text-steam-text-dim mt-1.5 px-1">
      Set your profile to public in
      <button class="text-steam-primary underline cursor-pointer" onclick={() => BrowserOpenURL('https://steamcommunity.com/my/edit/settings')}>Steam privacy settings</button>
      for best results.
    </p>
  {/if}
  {#if $steamId}
    <p class="text-[10px] text-steam-text-dim mt-1.5 px-1">Steam ID: {$steamId}</p>
  {/if}
</div>
