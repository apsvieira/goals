<script lang="ts">
  import MonthNav from './MonthNav.svelte';
  import type { User } from '../stores';

  export let month: string;
  export let onPrev: () => void;
  export let onNext: () => void;
  export let showAddForm: boolean;
  export let onToggleAddForm: () => void;
  export let user: User | null = null;
  export let isGuest: boolean = false;
  export let onUserClick: () => void = () => {};

  $: displayName = user?.name
    ? user.name.split(' ')[0]
    : isGuest
      ? 'Anonymous'
      : '';
</script>

<header>
  <div class="header-content">
    <button class="user-indicator" on:click={onUserClick}>
      {#if user?.avatar_url}
        <img src={user.avatar_url} alt="User avatar" class="avatar" />
      {:else}
        <div class="avatar avatar-placeholder">
          <svg viewBox="0 0 24 24" fill="currentColor" width="18" height="18">
            <path d="M12 12c2.21 0 4-1.79 4-4s-1.79-4-4-4-4 1.79-4 4 1.79 4 4 4zm0 2c-2.67 0-8 1.34-8 4v2h16v-2c0-2.66-5.33-4-8-4z"/>
          </svg>
        </div>
      {/if}
      <span class="user-name">{displayName}</span>
    </button>
    <MonthNav {month} {onPrev} {onNext} />
    <button
      class="add-btn"
      on:click={onToggleAddForm}
      aria-label={showAddForm ? 'Close form' : 'Add goal'}
    >
      {showAddForm ? 'Cancel' : 'New Goal'}
    </button>
  </div>
</header>

<style>
  header {
    position: sticky;
    top: 0;
    z-index: 100;
    background: var(--bg-primary);
    border-bottom: 1px solid var(--border);
    padding: 16px 24px;
  }

  .header-content {
    display: flex;
    justify-content: space-between;
    align-items: center;
    max-width: 1400px;
    margin: 0 auto;
  }

  .user-indicator {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 12px 4px 4px;
    background: transparent;
    border: 1px solid var(--border);
    border-radius: 20px;
    cursor: pointer;
    min-width: 90px;
  }

  .user-indicator:hover {
    background: var(--bg-secondary);
  }

  .avatar {
    width: 32px;
    height: 32px;
    border-radius: 50%;
    object-fit: cover;
  }

  .avatar-placeholder {
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-secondary);
    color: var(--text-secondary);
  }

  .user-name {
    font-size: 14px;
    color: var(--text-primary);
  }

  .add-btn {
    padding: 8px 16px;
    font-size: 14px;
    font-weight: 500;
    background: var(--accent);
    color: white;
    border: none;
    border-radius: 6px;
    cursor: pointer;
    min-width: 90px;
  }

  .add-btn:hover {
    background: var(--accent-hover);
  }
</style>
