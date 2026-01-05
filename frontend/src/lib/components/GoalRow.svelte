<script lang="ts">
  import DayGrid from './DayGrid.svelte';
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
  export let currentDay: number = 0;
</script>

<div
  class="goal-row"
  class:drag-over={isDragOver}
  on:dragover={onDragOver}
  on:drop={onDrop}
  on:dragleave={() => isDragOver = false}
  role="listitem"
>
  <button
    class="goal-name"
    on:click={onEdit}
    draggable="true"
    on:dragstart={onDragStart}
  >
    {goal.name}
  </button>
  <DayGrid
    {daysInMonth}
    color={goal.color}
    {completedDays}
    {onToggle}
    {currentDay}
  />
</div>

<style>
  .goal-row {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 6px 0;
  }

  .goal-row.drag-over {
    background-color: var(--bg-tertiary);
  }

  .goal-name {
    flex-shrink: 0;
    min-width: 120px;
    max-width: 200px;
    padding: 4px 8px;
    background: none;
    border: 1px solid transparent;
    border-radius: 4px;
    cursor: grab;
    text-align: left;
    font-size: 14px;
    color: var(--text-primary);
    user-select: none;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .goal-name:hover {
    border-color: var(--border);
    background: var(--bg-secondary);
  }

  .goal-name:active {
    cursor: grabbing;
  }
</style>
