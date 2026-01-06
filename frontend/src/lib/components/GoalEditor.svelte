<script lang="ts">
  import { onMount, onDestroy } from 'svelte';

  export let mode: 'add' | 'edit' = 'add';
  export let goal: { id: string; name: string; color: string; target_count?: number; target_period?: 'week' | 'month' } | null = null;
  export let previewColor: string = '#5B8C5A'; // Default to first palette color
  export let onSave: (data: { name: string; target_count?: number; target_period?: 'week' | 'month' }) => void;
  export let onCancel: () => void;
  export let onDelete: (() => void) | null = null;

  let name = goal?.name ?? '';
  let showDeleteConfirm = false;

  // Handle Escape key to close editor
  function handleKeyDown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      if (showDeleteConfirm) {
        cancelDelete();
      } else {
        onCancel();
      }
      e.preventDefault();
    }
  }

  onMount(() => {
    window.addEventListener('keydown', handleKeyDown);
  });

  onDestroy(() => {
    window.removeEventListener('keydown', handleKeyDown);
  });

  // Target state
  type TargetType = 'daily' | 'weekly' | 'monthly';
  let targetType: TargetType = goal?.target_period === 'week' ? 'weekly' : goal?.target_period === 'month' ? 'monthly' : 'daily';
  let targetCount = goal?.target_count ?? 4;

  // Use goal's assigned color for preview in edit mode, otherwise use previewColor
  $: displayColor = goal?.color ?? previewColor;
  $: maxTarget = targetType === 'weekly' ? 7 : targetType === 'monthly' ? 31 : 0;

  function handleSave() {
    if (name.trim()) {
      const data: { name: string; target_count?: number; target_period?: 'week' | 'month' } = {
        name: name.trim(),
      };
      if (targetType === 'weekly') {
        data.target_count = targetCount;
        data.target_period = 'week';
      } else if (targetType === 'monthly') {
        data.target_count = targetCount;
        data.target_period = 'month';
      }
      onSave(data);
    }
  }

  function handleDelete() {
    if (onDelete) {
      onDelete();
    }
  }

  function handleDeleteClick() {
    showDeleteConfirm = true;
  }

  function cancelDelete() {
    showDeleteConfirm = false;
  }
</script>

<div class="goal-editor">
  <form on:submit|preventDefault={handleSave}>
    <div class="form-content">
      <div class="field">
        <label for="goal-name">Name</label>
        <input
          id="goal-name"
          type="text"
          bind:value={name}
          placeholder="Goal name"
        />
      </div>

      <fieldset class="field target-field">
        <legend>Target frequency</legend>
        <div class="target-options">
          <label class="target-option">
            <input type="radio" bind:group={targetType} value="daily" />
            <span>Daily (no target)</span>
          </label>
          <label class="target-option">
            <input type="radio" bind:group={targetType} value="weekly" />
            <span>Weekly target</span>
          </label>
          <label class="target-option">
            <input type="radio" bind:group={targetType} value="monthly" />
            <span>Monthly target</span>
          </label>
        </div>
        {#if targetType !== 'daily'}
          <div class="target-count">
            <label for="target-count">Times per {targetType === 'weekly' ? 'week' : 'month'}</label>
            <input
              id="target-count"
              type="number"
              min="1"
              max={maxTarget}
              bind:value={targetCount}
            />
          </div>
        {/if}
      </fieldset>

      <div class="preview">
        <span class="preview-name">{name || 'Goal Name'}</span>
        <div class="preview-squares">
          {#each Array(7) as _}
            <span class="preview-square" style="border-color: {displayColor}; background-color: {displayColor}"></span>
          {/each}
        </div>
      </div>
    </div>

    <div class="actions">
      {#if mode === 'edit' && onDelete}
        <button type="button" class="btn-delete" on:click={handleDeleteClick}>
          Delete
        </button>
      {/if}
      <div class="right-actions">
        <button type="button" class="btn-cancel" on:click={onCancel}>
          Cancel
        </button>
        <button type="submit" class="btn-save" disabled={!name.trim()}>
          {mode === 'add' ? 'Add Goal' : 'Save'}
        </button>
      </div>
    </div>
  </form>

  {#if showDeleteConfirm}
    <div class="confirm-overlay" on:click={cancelDelete} on:keydown={(e) => e.key === 'Escape' && cancelDelete()} role="dialog" aria-modal="true" tabindex="-1">
      <div class="confirm-dialog" role="document" on:click|stopPropagation on:keydown|stopPropagation>
        <p>Are you sure you want to delete this goal?</p>
        <div class="confirm-actions">
          <button type="button" class="btn-cancel" on:click={cancelDelete}>
            Cancel
          </button>
          <button type="button" class="btn-confirm-delete" on:click={handleDelete}>
            Delete
          </button>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .goal-editor {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: var(--bg-primary);
    display: flex;
    flex-direction: column;
    z-index: 100;
  }

  form {
    flex: 1;
    display: flex;
    flex-direction: column;
    max-width: 480px;
    width: 100%;
    margin: 0 auto;
    padding: 24px;
    box-sizing: border-box;
  }

  .form-content {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 24px;
    padding-top: 24px;
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  label {
    font-size: 14px;
    color: var(--text-secondary);
  }

  input {
    padding: 12px 16px;
    font-size: 16px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--bg-tertiary);
    color: var(--text-primary);
  }

  input::placeholder {
    color: var(--text-muted);
  }

  input:focus {
    outline: none;
    border-color: var(--accent);
  }

  .target-field {
    border: none;
    padding: 0;
    margin: 0;
  }

  .target-field legend {
    font-size: 14px;
    color: var(--text-secondary);
    padding: 0;
    margin-bottom: 8px;
  }

  .target-options {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .target-option {
    display: flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;
    font-size: 14px;
    color: var(--text-primary);
  }

  .target-option input[type="radio"] {
    width: auto;
    padding: 0;
    margin: 0;
    cursor: pointer;
  }

  .target-count {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-top: 12px;
    padding: 12px;
    background: var(--bg-tertiary);
    border-radius: 8px;
  }

  .target-count label {
    font-size: 14px;
    color: var(--text-secondary);
  }

  .target-count input[type="number"] {
    width: 80px;
    padding: 8px 12px;
    text-align: center;
  }

  .preview {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 16px;
    background: var(--bg-tertiary);
    border-radius: 8px;
    color: var(--text-primary);
    font-size: 14px;
  }

  .preview-name {
    width: 100px;
    flex-shrink: 0;
    word-wrap: break-word;
  }

  .preview-squares {
    display: flex;
    gap: 1px;
    flex: 1;
  }

  .preview-square {
    width: 16px;
    height: 16px;
    border-radius: 3px;
    border: 1.5px solid;
    flex-shrink: 0;
  }

  .actions {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 12px;
    padding-top: 24px;
    border-top: 1px solid var(--border);
  }

  .right-actions {
    display: flex;
    gap: 12px;
    margin-left: auto;
  }

  button {
    padding: 12px 20px;
    font-size: 16px;
    border-radius: 8px;
    cursor: pointer;
    font-weight: 500;
  }

  .btn-delete {
    background: transparent;
    color: var(--error);
    border: 1px solid var(--error);
  }

  .btn-delete:hover {
    background: var(--error-bg);
  }

  .btn-cancel {
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
    color: var(--text-primary);
  }

  .btn-cancel:hover {
    background: var(--bg-secondary);
  }

  .btn-save {
    background: var(--accent);
    color: white;
    border: none;
  }

  .btn-save:hover {
    background: var(--accent-hover);
  }

  .btn-save:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  /* Delete confirmation overlay */
  .confirm-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.7);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 200;
  }

  .confirm-dialog {
    background: var(--bg-secondary);
    border-radius: 12px;
    padding: 24px;
    max-width: 320px;
    width: 90%;
    border: 1px solid var(--border);
    box-shadow: 0 4px 24px rgba(0, 0, 0, 0.4);
  }

  .confirm-dialog p {
    margin: 0 0 20px 0;
    font-size: 16px;
    color: var(--text-primary);
    text-align: center;
  }

  .confirm-actions {
    display: flex;
    gap: 12px;
    justify-content: center;
  }

  .btn-confirm-delete {
    background: var(--error);
    color: white;
    border: none;
  }

  .btn-confirm-delete:hover {
    opacity: 0.9;
  }

  @media (max-width: 480px) {
    form {
      padding: 16px;
    }

    .form-content {
      gap: 20px;
      padding-top: 16px;
    }

    button {
      padding: 10px 16px;
      font-size: 14px;
    }
  }
</style>
