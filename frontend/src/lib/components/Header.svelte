<script lang="ts">
  import MonthNav from './MonthNav.svelte';
  import UserDropdown from './UserDropdown.svelte';
  import type { User } from '../stores';
  import { _ } from 'svelte-i18n';

  export let month: string;
  export let onPrev: () => void;
  export let onNext: () => void;
  export let disableNextMonth: boolean = false;
  export let showAddForm: boolean;
  export let onToggleAddForm: () => void;
  export let user: User | null = null;
  export let syncing: boolean = false;
  export let onLogout: () => void = () => {};
  export let onProfileClick: () => void = () => {};
  export let onNotificationsClick: () => void = () => {};
  export let onSignIn: () => void = () => {};

  let dropdownOpen = false;

  $: displayName = user?.name
    ? user.name.split(' ')[0]
    : '';

  function toggleDropdown() {
    dropdownOpen = !dropdownOpen;
  }

  function closeDropdown() {
    dropdownOpen = false;
  }

  function handleClickOutside(event: MouseEvent) {
    const target = event.target as HTMLElement;
    if (!target.closest('.user-menu')) {
      closeDropdown();
    }
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      closeDropdown();
    }
  }

  function handleProfileClick() {
    closeDropdown();
    onProfileClick();
  }

  function handleNotificationsClick() {
    closeDropdown();
    onNotificationsClick();
  }

  function handleLogout() {
    closeDropdown();
    onLogout();
  }

  function handleSignIn() {
    closeDropdown();
    onSignIn();
  }
</script>

<svelte:window on:click={handleClickOutside} on:keydown={handleKeydown} />

<header>
  <div class="header-content">
    <button
      class="add-btn"
      on:click={onToggleAddForm}
      aria-label={showAddForm ? $_('aria.closeForm') : $_('aria.addGoal')}
    >
      <svg class="plus-icon" viewBox="0 0 24 24" width="18" height="18" fill="currentColor">
        <path d="M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6v2z"/>
      </svg>
      <span class="btn-text">{showAddForm ? $_('header.cancel') : $_('header.newGoal')}</span>
    </button>
    <MonthNav {month} {onPrev} {onNext} disableNext={disableNextMonth} />
    {#if syncing}
      <span class="sync-cloud" title={$_('tooltip.syncing')}>
        <svg viewBox="0 0 24 20" width="18" height="15" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round">
          <path d="M6 14a4 4 0 0 1-.68-7.95A5.5 5.5 0 0 1 16.5 5h.5a4 4 0 0 1 1 7.87"/>
          <line class="drop drop-1" x1="10" y1="14" x2="10" y2="18"/>
          <line class="drop drop-2" x1="14" y1="13" x2="14" y2="17"/>
          <line class="drop drop-3" x1="8" y1="16" x2="8" y2="19"/>
        </svg>
      </span>
    {/if}
    <div class="user-menu">
      <button class="user-indicator" on:click|stopPropagation={toggleDropdown} aria-expanded={dropdownOpen}>
        {#if user?.avatar_url}
          <img src={user.avatar_url} alt={$_('alt.userAvatar')} class="avatar" />
        {:else}
          <div class="avatar avatar-placeholder">
            <svg viewBox="0 0 24 24" fill="currentColor" width="18" height="18">
              <path d="M12 12c2.21 0 4-1.79 4-4s-1.79-4-4-4-4 1.79-4 4 1.79 4 4 4zm0 2c-2.67 0-8 1.34-8 4v2h16v-2c0-2.66-5.33-4-8-4z"/>
            </svg>
          </div>
        {/if}
        <span class="user-name">{displayName}</span>
        <svg class="chevron" class:open={dropdownOpen} viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
          <path d="M7.41 8.59L12 13.17l4.59-4.58L18 10l-6 6-6-6z"/>
        </svg>
      </button>
      {#if dropdownOpen}
        <UserDropdown
          {user}
          onClose={closeDropdown}
          onLogout={handleLogout}
          onProfileClick={handleProfileClick}
          onNotificationsClick={handleNotificationsClick}
          onSignIn={handleSignIn}
        />
      {/if}
    </div>
  </div>
</header>

<style>
  header {
    position: sticky;
    top: 0;
    z-index: 100;
    background: var(--bg-primary);
    border-bottom: 1px solid var(--border);
    padding: 1rem 1.5rem;
  }

  .header-content {
    display: flex;
    justify-content: space-between;
    align-items: center;
    max-width: 87.5rem;
    margin: 0 auto;
  }

  .user-menu {
    position: relative;
  }

  .user-indicator {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.25rem 0.5rem 0.25rem 0.25rem;
    background: transparent;
    border: 1px solid var(--border);
    border-radius: 1.25rem;
    cursor: pointer;
    min-width: 5.625rem;
  }

  .user-indicator:hover {
    background: var(--bg-secondary);
  }

  .avatar {
    width: 2rem;
    height: 2rem;
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
    font-size: 0.875rem;
    color: var(--text-primary);
  }

  .chevron {
    fill: var(--text-secondary);
    transition: transform 0.2s ease;
  }

  .chevron.open {
    transform: rotate(180deg);
  }

  .add-btn {
    display: flex;
    align-items: center;
    gap: 0.375rem;
    padding: 0.5rem 1rem;
    font-size: 0.875rem;
    font-weight: 500;
    background: var(--accent);
    color: white;
    border: none;
    border-radius: 0.375rem;
    cursor: pointer;
    min-width: 5.625rem;
  }

  .add-btn:hover {
    background: var(--accent-hover);
  }

  .plus-icon {
    display: none;
  }

  .sync-cloud {
    position: absolute;
    right: 7rem;
    opacity: 0.45;
    animation: fade-in 0.3s ease;
  }

  .sync-cloud .drop {
    stroke: var(--accent);
    stroke-width: 1.5;
    animation: rain 0.8s ease-in infinite;
  }

  .sync-cloud .drop-2 { animation-delay: 0.25s; }
  .sync-cloud .drop-3 { animation-delay: 0.5s; }

  @keyframes rain {
    0%   { opacity: 0; transform: translateY(-2px); }
    30%  { opacity: 1; }
    100% { opacity: 0; transform: translateY(3px); }
  }

  @keyframes fade-in {
    from { opacity: 0; }
    to   { opacity: 0.45; }
  }

  @media (max-width: 480px) {
    .user-indicator {
      padding: 0.25rem;
      min-width: auto;
      border-radius: 50%;
    }

    .user-name,
    .chevron,
    .sync-cloud {
      display: none;
    }

    .add-btn {
      padding: 0.5rem;
      min-width: auto;
      border-radius: 0.375rem;
    }

    .btn-text {
      display: none;
    }

    .plus-icon {
      display: block;
    }
  }
</style>
