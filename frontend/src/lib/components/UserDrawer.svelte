<script lang="ts">
  import type { User } from '../stores';

  export let open: boolean;
  export let user: User | null;
  export let isGuest: boolean;
  export let onClose: () => void;
  export let onLogout: () => void;
  export let onProfileClick: () => void;
  export let onSignIn: () => void = () => {};

  function handleOverlayClick() {
    onClose();
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      onClose();
    }
  }

  $: displayName = user?.name || (isGuest ? 'Anonymous' : 'User');
  $: avatarUrl = user?.avatar_url || null;
</script>

<svelte:window on:keydown={handleKeydown} />

{#if open}
  <div class="overlay" on:click={handleOverlayClick} role="presentation"></div>
  <div class="drawer" role="dialog" aria-modal="true" aria-label="User menu">
    <div class="drawer-header">
      <button class="close-btn" on:click={onClose} aria-label="Close menu">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <line x1="18" y1="6" x2="6" y2="18"></line>
          <line x1="6" y1="6" x2="18" y2="18"></line>
        </svg>
      </button>
    </div>

    <div class="user-info">
      <div class="avatar">
        {#if avatarUrl}
          <img src={avatarUrl} alt="{displayName}'s avatar" />
        {:else}
          <div class="avatar-placeholder">
            <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"></path>
              <circle cx="12" cy="7" r="4"></circle>
            </svg>
          </div>
        {/if}
      </div>
      <span class="user-name">{displayName}</span>
    </div>

    {#if !isGuest}
      <div class="divider"></div>

      <nav class="menu">
        <button class="menu-item" on:click={onProfileClick}>
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"></path>
            <circle cx="12" cy="7" r="4"></circle>
          </svg>
          <span>Profile</span>
        </button>
      </nav>
    {/if}

    <div class="divider"></div>

    <div class="drawer-footer">
      {#if isGuest}
        <button class="btn-sign-in" on:click={onSignIn}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor">
            <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
            <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
            <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/>
            <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
          </svg>
          <span>Sign in with Google</span>
        </button>
      {:else}
        <button class="btn-logout" on:click={onLogout}>
          Log Out
        </button>
      {/if}
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.5);
    z-index: 200;
  }

  .drawer {
    position: fixed;
    top: 0;
    right: 0;
    bottom: 0;
    width: 280px;
    background: var(--bg-primary);
    border-left: 1px solid var(--border);
    z-index: 201;
    display: flex;
    flex-direction: column;
    animation: slideIn 0.2s ease-out;
  }

  @keyframes slideIn {
    from {
      transform: translateX(100%);
    }
    to {
      transform: translateX(0);
    }
  }

  .drawer-header {
    display: flex;
    justify-content: flex-end;
    padding: 16px;
  }

  .close-btn {
    background: transparent;
    border: none;
    color: var(--text-secondary);
    cursor: pointer;
    padding: 8px;
    border-radius: 6px;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .close-btn:hover {
    background: var(--bg-secondary);
    color: var(--text-primary);
  }

  .user-info {
    display: flex;
    align-items: center;
    gap: 16px;
    padding: 16px 24px;
  }

  .avatar {
    width: 56px;
    height: 56px;
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
    font-size: 18px;
    font-weight: 500;
    color: var(--text-primary);
    word-break: break-word;
  }

  .divider {
    height: 1px;
    background: var(--border);
    margin: 8px 24px;
  }

  .menu {
    display: flex;
    flex-direction: column;
    padding: 8px 16px;
  }

  .menu-item {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 16px;
    background: transparent;
    border: none;
    color: var(--text-primary);
    font-size: 16px;
    cursor: pointer;
    border-radius: 8px;
    text-align: left;
    width: 100%;
  }

  .menu-item:hover {
    background: var(--bg-secondary);
  }

  .menu-item svg {
    color: var(--text-secondary);
    flex-shrink: 0;
  }

  .drawer-footer {
    margin-top: auto;
    padding: 24px;
  }

  .btn-logout {
    width: 100%;
    padding: 12px 20px;
    font-size: 16px;
    font-weight: 500;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    color: var(--text-primary);
    border-radius: 8px;
    cursor: pointer;
  }

  .btn-logout:hover {
    background: var(--bg-tertiary, var(--bg-secondary));
  }

  .btn-sign-in {
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 12px;
    padding: 12px 20px;
    font-size: 16px;
    font-weight: 500;
    background: white;
    border: 1px solid var(--border);
    color: #333;
    border-radius: 8px;
    cursor: pointer;
  }

  .btn-sign-in:hover {
    background: #f5f5f5;
  }

  @media (max-width: 320px) {
    .drawer {
      width: 100%;
    }
  }
</style>
