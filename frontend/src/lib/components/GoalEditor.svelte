<script lang="ts">
  export let mode: 'add' | 'edit' = 'add';
  export let goal: { id: string; name: string; color: string } | null = null;
  export let onSave: (data: { name: string; color: string }) => void;
  export let onCancel: () => void;
  export let onDelete: (() => void) | null = null;

  let name = goal?.name ?? '';
  let color = goal?.color ?? '#4CAF50';
  let showDeleteConfirm = false;

  const colors = [
    '#4CAF50', // Green
    '#2196F3', // Blue
    '#FF9800', // Orange
    '#E91E63', // Pink
    '#9C27B0', // Purple
    '#00BCD4', // Cyan
    '#FF5722', // Deep Orange
    '#607D8B', // Blue Grey
    '#795548', // Brown
    '#F44336', // Red
    '#3F51B5', // Indigo
    '#009688', // Teal
  ];

  function handleSave() {
    if (name.trim()) {
      onSave({ name: name.trim(), color });
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

      <div class="field">
        <span class="label">Color</span>
        <div class="color-picker">
          {#each colors as c}
            <button
              type="button"
              class="color-swatch"
              class:selected={color === c}
              style="background-color: {c}"
              on:click={() => color = c}
              aria-label="Select color {c}"
            ></button>
          {/each}
        </div>
      </div>

      <div class="preview">
        <span class="preview-dot" style="background-color: {color}"></span>
        <span>{name || 'Goal Name'}</span>
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
      <div class="confirm-dialog" on:click|stopPropagation>
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

  label, .label {
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

  .color-picker {
    display: flex;
    gap: 12px;
    flex-wrap: wrap;
  }

  .color-swatch {
    width: 40px;
    height: 40px;
    border-radius: 50%;
    border: 3px solid transparent;
    cursor: pointer;
    transition: transform 0.1s ease;
    padding: 0;
  }

  .color-swatch:hover {
    transform: scale(1.1);
  }

  .color-swatch.selected {
    border-color: var(--text-primary);
  }

  .preview {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 16px;
    background: var(--bg-tertiary);
    border-radius: 8px;
    color: var(--text-primary);
    font-size: 16px;
  }

  .preview-dot {
    width: 20px;
    height: 20px;
    border-radius: 50%;
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

    .color-picker {
      gap: 10px;
    }

    .color-swatch {
      width: 36px;
      height: 36px;
    }

    button {
      padding: 10px 16px;
      font-size: 14px;
    }
  }
</style>
