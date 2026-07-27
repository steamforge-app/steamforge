<script lang="ts">
  import { settings } from '../stores/settings';
  import { GetAppVersion } from '../../../wailsjs/go/main/App';
  import GeneralSettingsTab from './settings/GeneralSettingsTab.svelte';
  import DisplaySettingsTab from './settings/DisplaySettingsTab.svelte';
  import AdvancedSettingsTab from './settings/AdvancedSettingsTab.svelte';

  interface Props {
    onclose: () => void;
  }

  let { onclose }: Props = $props();

  let activeTab = $state<'general' | 'display' | 'advanced'>('general');

  let appVersion = $state('');
  let updateStatus = $state<'idle' | 'checking' | 'up-to-date' | 'update-available'>('idle');
  let latestVersion = $state('');
  let downloadUrl = $state('');

  GetAppVersion().then(version => appVersion = version).catch(() => {});

  const tabs = [
    { id: 'general' as const, label: 'General' },
    { id: 'display' as const, label: 'Display' },
    { id: 'advanced' as const, label: 'Advanced' },
  ];
</script>

<!-- Tabs -->
<div class="flex gap-1 mb-3 border-b border-steam-border">
  {#each tabs as tab}
    <button
      onclick={() => activeTab = tab.id}
      class="px-3 py-1.5 text-xs font-medium transition-colors cursor-pointer -mb-px
        {activeTab === tab.id
          ? 'text-steam-primary border-b-2 border-steam-primary'
          : 'text-steam-text-dim hover:text-steam-text'}"
    >
      {tab.label}
    </button>
  {/each}
</div>

<!-- Tab Content -->
{#if activeTab === 'general'}
  <GeneralSettingsTab
    autoStore={$settings.autoStore}
    allowLock={$settings.allowLock}
    protectLastPlayed={$settings.protectLastPlayed}
  />
{:else if activeTab === 'display'}
  <DisplaySettingsTab
    showLabels={$settings.showLabels}
    showSoftware={$settings.showSoftware}
    showCardButtons={$settings.showCardButtons}
    showUnlockDates={$settings.showUnlockDates}
  />
{:else}
  <AdvancedSettingsTab
    {appVersion}
    updateStatus={updateStatus}
    latestVersion={latestVersion}
    downloadUrl={downloadUrl}
    {onclose}
  />
{/if}
