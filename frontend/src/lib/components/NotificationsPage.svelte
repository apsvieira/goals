<script lang="ts">
  import { _ } from 'svelte-i18n';
  import NotificationSettings from './NotificationSettings.svelte';

  export let onBack: () => void;
</script>

<div class="legal-container">
  <div class="legal-card">
    <button class="back-btn" on:click={onBack}>
      <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M19 12H5M12 19l-7-7 7-7"/>
      </svg>
      {$_('notifications.pageTitle')}
    </button>

    <NotificationSettings />
  </div>
</div>

<style>
  .legal-container {
    min-height: 100vh;
    padding: 24px;
    background: var(--bg-primary);
  }

  .legal-card {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 48px;
    max-width: 800px;
    margin: 0 auto;
  }

  .back-btn {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 16px;
    font-size: 14px;
    font-weight: 500;
    background: transparent;
    color: var(--text-secondary);
    border: 1px solid var(--border);
    border-radius: 6px;
    cursor: pointer;
    margin-bottom: 24px;
    transition: background-color 0.2s, color 0.2s;
  }

  .back-btn:hover {
    background: var(--bg-tertiary);
    color: var(--text-primary);
  }

  @media (max-width: 768px) {
    .legal-card {
      padding: 24px;
    }
  }
</style>
