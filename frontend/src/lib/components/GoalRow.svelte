<script lang="ts">
  import HexGrid from './HexGrid.svelte';
  import type { Goal } from '../api';

  export let goal: Goal;
  export let daysInMonth: number;
  export let completedDays: Set<number>;
  export let onToggle: (day: number) => void;
  export let onEdit: () => void;
  export let onDragStart: (e: DragEvent) => void;
  export let onDragOver: (e: DragEvent) => void;
  export let onDrop: (e: DragEvent) => void;
  export let isDragOver = false;
</script>

<div
  class="goal-row"
  class:drag-over={isDragOver}
  on:dragover={onDragOver}
  on:drop={onDrop}
  on:dragleave={() => isDragOver = false}
  role="listitem"
>
  <div class="goal-header">
    <button
      class="goal-label"
      on:click={onEdit}
      draggable="true"
      on:dragstart={onDragStart}
    >
      <span class="color-dot" style="background-color: {goal.color}"></span>
      <span class="goal-name">{goal.name}</span>
    </button>
  </div>
  <div class="goal-grid">
    <HexGrid
      {daysInMonth}
      color={goal.color}
      {completedDays}
      {onToggle}
    />
  </div>
</div>

<style>
  .goal-row {
    display: flex;
    flex-direction: column;
    gap: 12px;
    margin-bottom: 16px;
    padding: 12px;
    border-radius: 8px;
    background: var(--bg-secondary);
    transition: background-color 0.15s ease;
  }

  .goal-row.drag-over {
    background-color: var(--bg-tertiary);
    border: 2px dashed var(--accent);
  }

  .goal-header {
    display: flex;
    align-items: center;
  }

  .goal-label {
    display: flex;
    align-items: center;
    gap: 8px;
    flex: 1;
    padding: 8px 12px;
    background: none;
    border: 1px solid transparent;
    border-radius: 4px;
    cursor: grab;
    text-align: left;
    font-size: 14px;
    color: var(--text-primary);
    user-select: none;
  }

  .goal-label:hover {
    border-color: var(--border);
    background: var(--bg-tertiary);
  }

  .goal-label:active {
    cursor: grabbing;
  }

  .color-dot {
    width: 12px;
    height: 12px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .goal-name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .goal-grid {
    padding-left: 4px;
  }

  @media (min-width: 600px) {
    .goal-row {
      flex-direction: row;
      align-items: flex-start;
      gap: 16px;
    }

    .goal-header {
      min-width: 140px;
    }

    .goal-grid {
      padding-left: 0;
    }
  }
</style>
