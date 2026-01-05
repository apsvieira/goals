<script lang="ts">
  import type { Goal } from '../api';

  export let goal: Goal;
  export let onSave: (updates: { name?: string; color?: string }) => void;
  export let onDelete: () => void;
  export let onClose: () => void;

  let name = goal.name;
  let color = goal.color;

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
    const updates: { name?: string; color?: string } = {};
    if (name !== goal.name) updates.name = name;
    if (color !== goal.color) updates.color = color;
    if (Object.keys(updates).length > 0) {
      onSave(updates);
    } else {
      onClose();
    }
  }

  function handleBackdropClick(e: MouseEvent) {
    if (e.target === e.currentTarget) {
      onClose();
    }
  }
</script>

<div class="modal-backdrop" on:click={handleBackdropClick} on:keydown={(e) => e.key === 'Escape' && onClose()} role="dialog" aria-modal="true" tabindex="-1">
  <div class="modal">
    <h2>Edit Goal</h2>

    <form on:submit|preventDefault={handleSave}>
      <label>
        Name
        <input type="text" bind:value={name} placeholder="Goal name" />
      </label>

      <div class="color-section">
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

      <div class="actions">
        <button type="button" class="btn-delete" on:click={onDelete}>
          Delete Goal
        </button>
        <div class="right-actions">
          <button type="button" class="btn-cancel" on:click={onClose}>
            Cancel
          </button>
          <button type="submit" class="btn-save" disabled={!name.trim()}>
            Save
          </button>
        </div>
      </div>
    </form>
  </div>
</div>

<style>
  .modal-backdrop {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.7);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
  }

  .modal {
    background: var(--bg-secondary);
    border-radius: 8px;
    padding: 24px;
    width: 90%;
    max-width: 400px;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.4);
    border: 1px solid var(--border);
  }

  h2 {
    margin: 0 0 20px 0;
    font-size: 20px;
    color: var(--text-primary);
  }

  label {
    display: block;
    margin-bottom: 16px;
    font-size: 14px;
    color: var(--text-secondary);
  }

  label input {
    display: block;
    width: 100%;
    margin-top: 4px;
    padding: 8px 12px;
    font-size: 16px;
    border: 1px solid var(--border);
    border-radius: 4px;
    box-sizing: border-box;
    background: var(--bg-tertiary);
    color: var(--text-primary);
  }

  .color-section {
    margin-bottom: 16px;
  }

  .label {
    display: block;
    font-size: 14px;
    color: var(--text-secondary);
    margin-bottom: 8px;
  }

  .color-picker {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }

  .color-swatch {
    width: 32px;
    height: 32px;
    border-radius: 50%;
    border: 2px solid transparent;
    cursor: pointer;
    transition: transform 0.1s ease;
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
    gap: 8px;
    padding: 12px;
    background: var(--bg-tertiary);
    border-radius: 4px;
    margin-bottom: 20px;
    color: var(--text-primary);
  }

  .preview-dot {
    width: 16px;
    height: 16px;
    border-radius: 50%;
  }

  .actions {
    display: flex;
    justify-content: space-between;
    gap: 12px;
  }

  .right-actions {
    display: flex;
    gap: 8px;
  }

  button {
    padding: 8px 16px;
    font-size: 14px;
    border-radius: 4px;
    cursor: pointer;
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
    background: var(--bg-primary);
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
</style>
