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
    <div class="drag-handle" title="Drag to reorder">&#9776;</div>
    <button class="goal-label" on:click={onEdit}>
      <span class="color-dot" style="background-color: {goal.color}"></span>
      <span class="goal-name">{goal.name}</span>
    </button>
  </div>
  <div
    class="goal-grid"
    draggable="true"
    on:dragstart={onDragStart}
    role="group"
  >
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
    gap: 8px;
  }

  .drag-handle {
    cursor: grab;
    padding: 4px;
    color: var(--text-muted);
    font-size: 14px;
    user-select: none;
  }

  .drag-handle:active {
    cursor: grabbing;
  }

  .goal-label {
    display: flex;
    align-items: center;
    gap: 8px;
    flex: 1;
    padding: 4px 8px;
    background: none;
    border: 1px solid transparent;
    border-radius: 4px;
    cursor: pointer;
    text-align: left;
    font-size: 14px;
    color: var(--text-primary);
  }

  .goal-label:hover {
    border-color: var(--border);
    background: var(--bg-tertiary);
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
