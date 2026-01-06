<script lang="ts">
  import type { User } from '../stores';

  export let user: User | null;
  export let isGuest: boolean;
  export let onClose: () => void;
  export let onLogout: () => void;
  export let onProfileClick: () => void;
  export let onSignIn: () => void = () => {};

  $: displayName = user?.name || (isGuest ? 'Anonymous' : 'User');
  $: avatarUrl = user?.avatar_url || null;
</script>

<div class="dropdown" role="menu" aria-label="User menu">
  <div class="user-info">
    <div class="avatar">
      {#if avatarUrl}
        <img src={avatarUrl} alt="{displayName}'s avatar" />
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

  {#if !isGuest}
    <div class="divider"></div>
    <button class="menu-item" on:click={onProfileClick} role="menuitem">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"></path>
        <circle cx="12" cy="7" r="4"></circle>
      </svg>
      <span>Profile</span>
    </button>
  {/if}

  <div class="divider"></div>

  {#if isGuest}
    <button class="menu-item sign-in" on:click={onSignIn} role="menuitem">
      <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
        <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
        <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
        <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/>
        <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
      </svg>
      <span>Sign in with Google</span>
    </button>
  {:else}
    <button class="menu-item logout" on:click={onLogout} role="menuitem">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"></path>
        <polyline points="16 17 21 12 16 7"></polyline>
        <line x1="21" y1="12" x2="9" y2="12"></line>
      </svg>
      <span>Log Out</span>
    </button>
  {/if}
</div>

<style>
  .dropdown {
    position: absolute;
    top: 100%;
    right: 0;
    margin-top: 8px;
    min-width: 200px;
    background: var(--bg-primary);
    border: 1px solid var(--border);
    border-radius: 8px;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
    z-index: 201;
    animation: fadeIn 0.15s ease-out;
  }

  @keyframes fadeIn {
    from {
      opacity: 0;
      transform: translateY(-8px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  .user-info {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 16px;
  }

  .avatar {
    width: 40px;
    height: 40px;
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
    font-size: 14px;
    font-weight: 500;
    color: var(--text-primary);
    word-break: break-word;
  }

  .divider {
    height: 1px;
    background: var(--border);
    margin: 4px 0;
  }

  .menu-item {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 10px 16px;
    background: transparent;
    border: none;
    color: var(--text-primary);
    font-size: 14px;
    cursor: pointer;
    text-align: left;
  }

  .menu-item:hover {
    background: var(--bg-secondary);
  }

  .menu-item svg {
    color: var(--text-secondary);
    flex-shrink: 0;
  }

  .menu-item.sign-in svg {
    color: inherit;
  }

  .menu-item.logout {
    color: var(--error);
  }

  .menu-item.logout svg {
    color: var(--error);
  }
</style>
