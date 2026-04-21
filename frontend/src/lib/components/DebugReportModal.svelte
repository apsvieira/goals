<script lang="ts">
  import { tick } from 'svelte';
  import { _ } from 'svelte-i18n';
  import { debugReportModalOpen, closeDebugReport } from '../stores';
  import {
    sendDebugReport,
    isClientRateLimited,
  } from '../diagnostics/debug-report';

  let textarea: HTMLTextAreaElement | null = null;
  let description = '';
  let sending = false;
  let errorMessage: string | null = null;
  let successMessage: string | null = null;
  let rateLimitedOnOpen = false;

  // Subscribe to the store so we can react to open/close transitions.
  let open = false;
  debugReportModalOpen.subscribe((v) => {
    const wasOpen = open;
    open = v;
    if (!wasOpen && open) {
      void handleOpen();
    } else if (wasOpen && !open) {
      resetState();
    }
  });

  async function handleOpen() {
    rateLimitedOnOpen = isClientRateLimited();
    errorMessage = rateLimitedOnOpen ? $_('debugReport.clientRateLimited') : null;
    successMessage = null;
    description = '';
    await tick();
    textarea?.focus();
  }

  function resetState() {
    description = '';
    sending = false;
    errorMessage = null;
    successMessage = null;
    rateLimitedOnOpen = false;
  }

  function handleBackdropClick(event: MouseEvent) {
    if (event.target === event.currentTarget && !sending) {
      closeDebugReport();
    }
  }

  function handleKeydown(event: KeyboardEvent) {
    if (!open) return;
    if (event.key === 'Escape' && !sending) {
      closeDebugReport();
    }
  }

  async function handleSend() {
    if (sending) return;
    if (isClientRateLimited()) {
      errorMessage = $_('debugReport.clientRateLimited');
      rateLimitedOnOpen = true;
      return;
    }

    sending = true;
    errorMessage = null;
    successMessage = null;

    try {
      const result = await sendDebugReport({
        description: description.trim(),
        trigger: 'shake',
      });

      if (result.outcome === 'sent') {
        successMessage = $_('debugReport.sentToast');
        setTimeout(() => closeDebugReport(), 1200);
        return;
      }
      if (result.outcome === 'queued') {
        successMessage = $_('debugReport.queuedToast');
        setTimeout(() => closeDebugReport(), 1500);
        return;
      }
      if (result.outcome === 'rate_limited') {
        errorMessage = result.message || $_('debugReport.rateLimited');
        return;
      }
      // client_rate_limited
      errorMessage = result.message || $_('debugReport.clientRateLimited');
      rateLimitedOnOpen = true;
    } catch {
      errorMessage = $_('debugReport.genericError');
    } finally {
      sending = false;
    }
  }

  $: sendDisabled = sending || rateLimitedOnOpen || successMessage !== null;
</script>

<svelte:window on:keydown={handleKeydown} />

{#if open}
  <div
    class="backdrop"
    on:click={handleBackdropClick}
    role="presentation"
  >
    <div
      class="dialog"
      role="dialog"
      aria-modal="true"
      aria-labelledby="debug-report-title"
      aria-describedby="debug-report-explainer"
    >
      <h2 id="debug-report-title" class="title">{$_('debugReport.title')}</h2>

      <label class="label" for="debug-report-desc">
        {$_('debugReport.promptLabel')}
      </label>
      <textarea
        id="debug-report-desc"
        bind:this={textarea}
        bind:value={description}
        class="textarea"
        rows="6"
        maxlength="2000"
        placeholder={$_('debugReport.placeholder')}
        disabled={sending}
      ></textarea>

      <p id="debug-report-explainer" class="explainer">
        {$_('debugReport.explainer')}
      </p>

      {#if errorMessage}
        <p class="error" role="alert">{errorMessage}</p>
      {/if}
      {#if successMessage}
        <p class="success" role="status">{successMessage}</p>
      {/if}

      <div class="actions">
        <button
          class="btn btn-secondary"
          type="button"
          on:click={closeDebugReport}
          disabled={sending}
        >
          {$_('debugReport.cancel')}
        </button>
        <button
          class="btn btn-primary"
          type="button"
          on:click={handleSend}
          disabled={sendDisabled}
        >
          {sending ? $_('debugReport.sending') : $_('debugReport.send')}
        </button>
      </div>
    </div>
  </div>
{/if}

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
    z-index: 400;
    padding: 1rem;
    animation: fadeIn 0.3s ease-out;
  }

  .dialog {
    background: var(--bg-primary);
    border: 1px solid var(--border);
    border-radius: 0.75rem;
    padding: 1.5rem;
    max-width: 24rem;
    width: 100%;
    box-shadow: 0 0.5rem 1.5rem rgba(0, 0, 0, 0.2);
    animation: slideUp 0.4s ease-out;
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .title {
    font-size: 1.125rem;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0;
  }

  .label {
    font-size: 0.875rem;
    font-weight: 500;
    color: var(--text-primary);
  }

  .textarea {
    width: 100%;
    font: inherit;
    font-size: 0.875rem;
    color: var(--text-primary);
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 0.375rem;
    padding: 0.5rem 0.75rem;
    resize: vertical;
    box-sizing: border-box;
    line-height: 1.4;
  }

  .textarea:focus {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }

  .explainer {
    font-size: 0.8125rem;
    color: var(--text-secondary);
    line-height: 1.45;
    margin: 0;
  }

  .error {
    font-size: 0.8125rem;
    color: var(--error);
    background: var(--error-bg);
    border-radius: 0.375rem;
    padding: 0.5rem 0.75rem;
    margin: 0;
  }

  .success {
    font-size: 0.8125rem;
    color: var(--success);
    margin: 0;
  }

  .actions {
    display: flex;
    gap: 0.75rem;
    justify-content: flex-end;
    margin-top: 0.25rem;
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

  .btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .btn-secondary {
    background: transparent;
    color: var(--text-secondary);
    border: 1px solid var(--border);
  }

  .btn-secondary:hover:not(:disabled) {
    background: var(--bg-secondary);
  }

  .btn-primary {
    background: var(--accent);
    color: white;
  }

  .btn-primary:hover:not(:disabled) {
    background: var(--accent-hover);
  }
</style>
