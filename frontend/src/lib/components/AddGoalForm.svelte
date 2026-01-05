<script lang="ts">
  export let onAdd: (name: string, color: string) => void;
  export let onCancel: () => void;

  let name = '';
  let color = '#4CAF50';

  const colors = [
    '#4CAF50', // Green
    '#2196F3', // Blue
    '#FF9800', // Orange
    '#E91E63', // Pink
    '#9C27B0', // Purple
    '#00BCD4', // Cyan
    '#FF5722', // Deep Orange
    '#607D8B', // Blue Grey
  ];

  function handleSubmit() {
    if (name.trim()) {
      onAdd(name.trim(), color);
      name = '';
    }
  }
</script>

<form class="add-goal-form" on:submit|preventDefault={handleSubmit}>
  <input
    type="text"
    bind:value={name}
    placeholder="Goal name"
    class="name-input"
  />
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
  <div class="form-actions">
    <button type="button" class="btn-cancel" on:click={onCancel}>Cancel</button>
    <button type="submit" class="btn-add" disabled={!name.trim()}>Add Goal</button>
  </div>
</form>

<style>
  .add-goal-form {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 16px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 8px;
    max-width: 300px;
  }

  .name-input {
    padding: 8px 12px;
    font-size: 14px;
    border: 1px solid var(--border);
    border-radius: 4px;
    background: var(--bg-tertiary);
    color: var(--text-primary);
  }

  .name-input::placeholder {
    color: var(--text-muted);
  }

  .color-picker {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }

  .color-swatch {
    width: 28px;
    height: 28px;
    border-radius: 50%;
    border: 2px solid transparent;
    cursor: pointer;
  }

  .color-swatch.selected {
    border-color: var(--text-primary);
  }

  .form-actions {
    display: flex;
    gap: 8px;
    justify-content: flex-end;
  }

  .btn-cancel, .btn-add {
    padding: 6px 12px;
    font-size: 14px;
    border-radius: 4px;
    cursor: pointer;
  }

  .btn-cancel {
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
    color: var(--text-primary);
  }

  .btn-cancel:hover {
    background: var(--bg-primary);
  }

  .btn-add {
    background: var(--accent);
    color: white;
    border: none;
  }

  .btn-add:hover {
    background: var(--accent-hover);
  }

  .btn-add:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
</style>
