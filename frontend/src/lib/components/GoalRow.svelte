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
  draggable="true"
  on:dragstart={onDragStart}
  on:dragover={onDragOver}
  on:drop={onDrop}
  on:dragleave={() => isDragOver = false}
  role="listitem"
>
  <div class="drag-handle" title="Drag to reorder">&#9776;</div>
  <button class="goal-label" on:click={onEdit}>
    <span class="color-dot" style="background-color: {goal.color}"></span>
    <span class="goal-name">{goal.name}</span>
  </button>
  <HexGrid
    {daysInMonth}
    color={goal.color}
    {completedDays}
    {onToggle}
  />
</div>

<style>
  .goal-row {
    display: flex;
    align-items: flex-start;
    gap: 8px;
    margin-bottom: 24px;
    padding: 4px;
    border-radius: 4px;
    transition: background-color 0.15s ease;
  }

  .goal-row.drag-over {
    background-color: #e3f2fd;
    border: 2px dashed #2196F3;
  }

  .drag-handle {
    cursor: grab;
    padding: 4px;
    color: #999;
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
    min-width: 120px;
    padding: 4px 8px;
    background: none;
    border: 1px solid transparent;
    border-radius: 4px;
    cursor: pointer;
    text-align: left;
    font-size: 14px;
  }

  .goal-label:hover {
    border-color: #ccc;
    background: #f5f5f5;
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
</style>
