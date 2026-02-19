<script lang="ts">
  import { settings, updateSetting, resetSettings } from '../stores/settings';
  import { scanning, scanProgress, profilePublic, steamId, addToast } from '../stores/app';
  import { achievementCounts } from '../stores/games';
  import {
    CheckProfileVisibility, RescanAllAchievements, StopAchievementScan, OpenDataDir, GetLogContent,
    GetAppVersion, CheckForUpdates
  } from '../../../wailsjs/go/main/App';
  import { BrowserOpenURL, ClipboardSetText } from '../../../wailsjs/runtime/runtime';

  interface Props {
    onclose: () => void;
  }

  let { onclose }: Props = $props();

  let activeTab = $state<'general' | 'display' | 'advanced'>('general');

  let profileStatus = $state<'unknown' | 'checking' | 'public' | 'private'>(
    $profilePublic === 'public' ? 'public' : $profilePublic === 'private' ? 'private' : 'unknown'
  );

  let logViewerOpen = $state(false);
  let logContent = $state('');
  let logCopied = $state(false);

  let appVersion = $state('');
  let updateStatus = $state<'idle' | 'checking' | 'up-to-date' | 'update-available'>('idle');
  let latestVersion = $state('');
  let downloadUrl = $state('');

  GetAppVersion().then(version => appVersion = version).catch(() => {});

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

  function handleRescan() {
    onclose();
    scanning.set(true);
    scanProgress.set({ current: 0, total: 0, name: '' });
    achievementCounts.set({});
    RescanAllAchievements().catch(() => { scanning.set(false); });
  }

  function handleStopScan() {
    StopAchievementScan().catch(() => {});
    scanning.set(false);
  }

  async function openLogViewer() {
    try {
      logContent = await GetLogContent();
    } catch (e: any) {
      logContent = 'Failed to load logs: ' + (e.message || e);
    }
    logViewerOpen = true;
  }

  async function copyLogs() {
    try {
      await ClipboardSetText(logContent);
      logCopied = true;
      setTimeout(() => logCopied = false, 2000);
    } catch {
      try {
        await navigator.clipboard.writeText(logContent);
        logCopied = true;
        setTimeout(() => logCopied = false, 2000);
      } catch {
        addToast('Failed to copy logs', 'error');
      }
    }
  }

  async function checkForUpdates() {
    updateStatus = 'checking';
    try {
      const info = await CheckForUpdates();
      if (info.updateAvailable) {
        updateStatus = 'update-available';
        latestVersion = info.latestVersion;
        downloadUrl = info.downloadUrl;
      } else {
        updateStatus = 'up-to-date';
      }
    } catch {
      updateStatus = 'idle';
      addToast('Failed to check for updates', 'error');
    }
  }

  $effect(() => {
    if (profileStatus === 'unknown') checkProfile();
  });

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

<!-- General Tab -->
{#if activeTab === 'general'}
  <div class="flex flex-col gap-3">
    <div class="flex items-center justify-between">
      <div class="flex flex-col">
        <span class="text-xs text-steam-text">Auto Save</span>
        <span class="text-[10px] text-steam-text-dim leading-tight">Save changes immediately</span>
      </div>
      <button
        onclick={() => updateSetting('autoStore', !$settings.autoStore)}
        title={$settings.autoStore ? 'Disable auto save' : 'Enable auto save'}
        class="toggle-switch {$settings.autoStore ? 'active green' : ''}"
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
        onclick={() => updateSetting('allowLock', !$settings.allowLock)}
        title={$settings.allowLock ? 'Disable locking' : 'Enable locking'}
        class="toggle-switch {$settings.allowLock ? 'active amber' : ''}"
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

<!-- Display Tab -->
{:else if activeTab === 'display'}
  <div class="flex flex-col gap-2">
    <span class="text-[10px] text-steam-text-dim uppercase tracking-wider font-medium">Game Library</span>
    <div class="flex items-center justify-between">
      <div class="flex flex-col">
        <span class="text-xs text-steam-text">Show Labels</span>
        <span class="text-[10px] text-steam-text-dim leading-tight">Show game names below cards</span>
      </div>
      <button
        onclick={() => updateSetting('showLabels', !$settings.showLabels)}
        title={$settings.showLabels ? 'Hide labels' : 'Show labels'}
        class="toggle-switch {$settings.showLabels ? 'active green' : ''}"
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
        onclick={() => updateSetting('showSoftware', !$settings.showSoftware)}
        title={$settings.showSoftware ? 'Hide software' : 'Show software'}
        class="toggle-switch {$settings.showSoftware ? 'active green' : ''}"
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
        onclick={() => updateSetting('showCardButtons', !$settings.showCardButtons)}
        title={$settings.showCardButtons ? 'Hide card buttons' : 'Show card buttons'}
        class="toggle-switch {$settings.showCardButtons ? 'active green' : ''}"
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
        onclick={() => updateSetting('showUnlockDates', !$settings.showUnlockDates)}
        title={$settings.showUnlockDates ? 'Hide unlock dates' : 'Show unlock dates'}
        class="toggle-switch {$settings.showUnlockDates ? 'active green' : ''}"
      >
        <span class="toggle-knob"></span>
      </button>
    </div>
  </div>

<!-- Advanced Tab -->
{:else}
  <!-- Scan section -->
  <div class="flex flex-col gap-2">
    <span class="text-[10px] text-steam-text-dim uppercase tracking-wider font-medium">Achievement Data</span>
    {#if $scanning}
      <button
        onclick={handleStopScan}
        class="w-full flex items-center gap-2 px-3 py-2 text-xs rounded-lg bg-steam-danger/10 border border-steam-danger/30 text-steam-danger hover:bg-steam-danger/20 transition-colors cursor-pointer"
      >
        <svg class="w-3.5 h-3.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
        <div class="flex flex-col items-start">
          <span>Stop Scan</span>
          <span class="text-[10px] text-steam-danger/70">{$scanProgress.current}/{$scanProgress.total} scanned</span>
        </div>
      </button>
    {:else}
      <button
        onclick={handleRescan}
        class="w-full flex items-center gap-2 px-3 py-2 text-xs rounded-lg bg-steam-input hover:bg-steam-hover border border-steam-border text-steam-text-dim hover:text-steam-text hover:border-steam-primary/30 transition-all cursor-pointer"
      >
        <svg class="w-3.5 h-3.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
        </svg>
        <div class="flex flex-col items-start">
          <span>Rescan Achievements</span>
          <span class="text-[10px] opacity-60">Refresh counts for all games</span>
        </div>
      </button>
    {/if}
  </div>

  <!-- Diagnostics section -->
  <div class="mt-3 pt-3 border-t border-steam-border flex flex-col gap-2">
    <span class="text-[10px] text-steam-text-dim uppercase tracking-wider font-medium">Diagnostics</span>
    <div class="flex gap-1.5">
      <button
        onclick={() => { onclose(); OpenDataDir(); }}
        class="flex-1 flex items-center justify-center gap-1.5 px-2 py-1.5 text-xs rounded-lg bg-steam-input hover:bg-steam-hover border border-steam-border text-steam-text-dim hover:text-steam-text hover:border-steam-primary/30 transition-all cursor-pointer"
      >
        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
        </svg>
        Data Folder
      </button>
      <button
        onclick={openLogViewer}
        class="flex-1 flex items-center justify-center gap-1.5 px-2 py-1.5 text-xs rounded-lg bg-steam-input hover:bg-steam-hover border border-steam-border text-steam-text-dim hover:text-steam-text hover:border-steam-primary/30 transition-all cursor-pointer"
      >
        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
        </svg>
        View Logs
      </button>
    </div>
    <button
      onclick={() => { resetSettings(); onclose(); }}
      class="w-full flex items-center justify-center gap-1.5 px-3 py-1.5 text-[10px] rounded-lg text-steam-text-dim hover:text-red-400 transition-colors cursor-pointer"
    >
      Reset to Defaults
    </button>
  </div>

  <!-- Version section -->
  <div class="mt-3 pt-3 border-t border-steam-border flex items-center justify-between">
    <span class="text-[10px] text-steam-text-dim">
      {appVersion ? `v${appVersion.replace(/^v/, '')}` : ''}
    </span>
    {#if updateStatus === 'checking'}
      <span class="text-[10px] text-steam-text-dim flex items-center gap-1">
        <span class="w-2.5 h-2.5 border border-steam-text-dim border-t-transparent rounded-full animate-spin"></span>
        Checking...
      </span>
    {:else if updateStatus === 'up-to-date'}
      <span class="text-[10px] text-steam-success">Up to date</span>
    {:else if updateStatus === 'update-available'}
      <span class="text-[10px] flex items-center gap-1.5">
        <span class="text-steam-primary">{latestVersion} available</span>
        <button onclick={() => BrowserOpenURL(downloadUrl)} class="text-steam-primary underline cursor-pointer hover:text-steam-text">Download</button>
      </span>
    {:else}
      <button onclick={checkForUpdates} class="text-[10px] text-steam-text-dim hover:text-steam-primary cursor-pointer transition-colors">
        Check for Updates
      </button>
    {/if}
  </div>
{/if}

{#if logViewerOpen}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60" role="none" onclick={() => logViewerOpen = false} onkeydown={(e) => { if (e.key === 'Escape') logViewerOpen = false; }}>
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="bg-steam-surface border border-steam-border rounded-lg shadow-xl w-[700px] max-h-[80vh] flex flex-col" role="none" onclick={(e) => e.stopPropagation()} onkeydown={() => {}}>
      <div class="flex items-center justify-between px-4 py-3 border-b border-steam-border">
        <h3 class="text-sm font-medium text-steam-text">Session Logs</h3>
        <div class="flex items-center gap-2">
          <button
            onclick={copyLogs}
            class="px-3 py-1 text-xs rounded bg-steam-input border border-steam-border text-steam-text-dim hover:text-steam-text hover:border-steam-primary/50 transition-colors cursor-pointer"
          >
            {logCopied ? 'Copied!' : 'Copy'}
          </button>
          <button
            onclick={() => logViewerOpen = false}
            title="Close"
            class="p-1 rounded text-steam-text-dim hover:text-steam-text transition-colors cursor-pointer"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>
      <pre class="flex-1 overflow-auto p-4 text-[11px] leading-relaxed text-steam-text-dim font-mono whitespace-pre-wrap break-words select-text cursor-text">{logContent}</pre>
    </div>
  </div>
{/if}
