<script lang="ts">
  import { _ } from 'svelte-i18n';

  export let onEnable: () => void;
  export let onDismiss: () => void;

  function handleBackdropClick(event: MouseEvent) {
    if (event.target === event.currentTarget) {
      onDismiss();
    }
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      onDismiss();
    }
  }
</script>

<svelte:window on:keydown={handleKeydown} />

<div class="backdrop" on:click={handleBackdropClick} role="presentation">
  <div class="dialog" role="alertdialog" aria-labelledby="prompt-title" aria-describedby="prompt-body">
    <h2 id="prompt-title" class="title">{$_('notifications.prompt.title')}</h2>
    <p id="prompt-body" class="body">{$_('notifications.prompt.body')}</p>
    <div class="actions">
      <button class="btn btn-secondary" on:click={onDismiss}>
        {$_('notifications.prompt.notNow')}
      </button>
      <button class="btn btn-primary" on:click={onEnable}>
        {$_('notifications.prompt.enable')}
      </button>
    </div>
  </div>
</div>

<style>
  @keyframes fadeIn {
    from { opacity: 0; }
    to   { opacity: 1; }
  }

  @keyframes slideUp {
    from { opacity: 0; transform: translateY(1rem); }
    to   { opacity: 1; transform: translateY(0); }
  }

  .backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.4);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 300;
    padding: 1rem;
    animation: fadeIn 0.4s ease-out;
  }

  .dialog {
    background: var(--bg-primary);
    border: 1px solid var(--border);
    border-radius: 0.75rem;
    padding: 1.5rem;
    max-width: 20rem;
    width: 100%;
    box-shadow: 0 0.5rem 1.5rem rgba(0, 0, 0, 0.2);
    animation: slideUp 0.5s ease-out;
  }

  .title {
    font-size: 1.125rem;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0 0 0.5rem 0;
  }

  .body {
    font-size: 0.875rem;
    color: var(--text-secondary);
    line-height: 1.5;
    margin: 0 0 1.25rem 0;
  }

  .actions {
    display: flex;
    gap: 0.75rem;
    justify-content: flex-end;
  }

  .btn {
    padding: 0.5rem 1rem;
    border-radius: 0.375rem;
    font-size: 0.875rem;
    font-weight: 500;
    cursor: pointer;
    border: none;
    transition: background-color 0.15s;
  }

  .btn-secondary {
    background: transparent;
    color: var(--text-secondary);
    border: 1px solid var(--border);
  }

  .btn-secondary:hover {
    background: var(--bg-secondary);
  }

  .btn-primary {
    background: var(--accent);
    color: white;
  }

  .btn-primary:hover {
    background: var(--accent-hover);
  }
</style>
