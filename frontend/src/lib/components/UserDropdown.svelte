<script lang="ts">
  import type { User } from '../stores';
  import { _, locale } from 'svelte-i18n';
  import { Capacitor } from '@capacitor/core';
  import { supportedLocales, saveLocale } from '../i18n';

  export let user: User | null;
  export let onClose: () => void;
  export let onLogout: () => void;
  export let onProfileClick: () => void;
  export let onNotificationsClick: () => void = () => {};
  export let onSignIn: () => void = () => {};

  $: displayName = user?.name || $_('fallback.user');
  $: avatarUrl = user?.avatar_url || null;
</script>

<div class="dropdown" role="menu" aria-label={$_('aria.userMenu')}>
  <div class="user-info">
    <div class="avatar">
      {#if avatarUrl}
        <img src={avatarUrl} alt={$_('alt.userAvatarOf', { values: { name: displayName }})} />
      {:else}
        <div class="avatar-placeholder">
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"></path>
            <circle cx="12" cy="7" r="4"></circle>
          </svg>
        </div>
      {/if}
    </div>
    <span class="user-name">{displayName}</span>
  </div>

  <div class="divider"></div>
  <button class="menu-item" on:click={onProfileClick} role="menuitem">
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"></path>
      <circle cx="12" cy="7" r="4"></circle>
    </svg>
    <span>{$_('menu.profile')}</span>
  </button>

  {#if Capacitor.isNativePlatform()}
    <button class="menu-item" on:click={onNotificationsClick} role="menuitem">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"></path>
        <path d="M13.73 21a2 2 0 0 1-3.46 0"></path>
      </svg>
      <span>{$_('menu.notifications')}</span>
    </button>
  {/if}

  <div class="divider"></div>
  <div class="menu-section">
    <span class="menu-label">{$_('menu.language')}</span>
    <div class="language-options">
      {#each supportedLocales as loc}
        <button
          class="language-btn"
          class:active={$locale === loc.code}
          on:click={() => saveLocale(loc.code)}
          role="menuitemradio"
          aria-checked={$locale === loc.code}
        >
          {loc.label}
        </button>
      {/each}
    </div>
  </div>

  <div class="divider"></div>

  <button class="menu-item logout" on:click={onLogout} role="menuitem">
    <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor">
      <path d="M17 7l-1.41 1.41L18.17 11H8v2h10.17l-2.58 2.58L17 17l5-5zM4 5h8V3H4c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h8v-2H4V5z"/>
    </svg>
    <span>{$_('menu.logOut')}</span>
  </button>
</div>

<style>
  .dropdown {
    position: absolute;
    top: 100%;
    right: 0;
    margin-top: 0.5rem;
    min-width: 12.5rem;
    background: var(--bg-primary);
    border: 1px solid var(--border);
    border-radius: 0.5rem;
    box-shadow: 0 0.25rem 0.75rem rgba(0, 0, 0, 0.15);
    z-index: 201;
    animation: fadeIn 0.15s ease-out;
  }

  @keyframes fadeIn {
    from {
      opacity: 0;
      transform: translateY(-0.5rem);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  .user-info {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.75rem 1rem;
  }

  .avatar {
    width: 2.5rem;
    height: 2.5rem;
    border-radius: 50%;
    overflow: hidden;
    flex-shrink: 0;
  }

  .avatar img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .avatar-placeholder {
    width: 100%;
    height: 100%;
    background: var(--bg-secondary);
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-secondary);
  }

  .user-name {
    font-size: 0.875rem;
    font-weight: 500;
    color: var(--text-primary);
    word-break: break-word;
  }

  .divider {
    height: 1px;
    background: var(--border);
    margin: 0.25rem 0;
  }

  .menu-item {
    display: flex;
    align-items: center;
    gap: 0.625rem;
    width: 100%;
    padding: 0.625rem 1rem;
    background: transparent;
    border: none;
    color: var(--text-primary);
    font-size: 0.875rem;
    cursor: pointer;
    text-align: left;
  }

  .menu-item:hover {
    background: var(--bg-secondary);
  }

  .menu-item svg {
    fill: var(--text-secondary);
    flex-shrink: 0;
  }

  .menu-item.sign-in svg {
    color: inherit;
  }

  .menu-item.logout {
    color: var(--error);
  }

  .menu-item.logout svg {
    fill: var(--error);
  }

  .menu-section {
    padding: 0.5rem 1rem;
  }

  .menu-label {
    font-size: 0.75rem;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .language-options {
    display: flex;
    flex-direction: column;
    gap: 0.125rem;
    margin-top: 0.375rem;
  }

  .language-btn {
    display: flex;
    align-items: center;
    width: 100%;
    padding: 0.375rem 0.5rem;
    background: transparent;
    border: none;
    border-radius: 0.25rem;
    color: var(--text-primary);
    font-size: 0.8125rem;
    cursor: pointer;
    text-align: left;
  }

  .language-btn:hover {
    background: var(--bg-secondary);
  }

  .language-btn.active {
    background: var(--bg-tertiary);
    font-weight: 600;
  }
</style>
