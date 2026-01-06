<script lang="ts">
  import DayGrid from './DayGrid.svelte';
  import ProgressBar from './ProgressBar.svelte';
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
  export let periodCompletions: number = 0; // Completions in current period (week/month)

  $: hasTarget = goal.target_count && goal.target_period;
</script>

<div
  class="goal-row"
  class:drag-over={isDragOver}
  on:dragover={onDragOver}
  on:drop={onDrop}
  on:dragleave={() => isDragOver = false}
  role="listitem"
>
  <div class="goal-info">
    <button
      class="goal-name"
      on:click={onEdit}
      draggable="true"
      on:dragstart={onDragStart}
    >
      {goal.name}
    </button>
    {#if hasTarget}
      <ProgressBar
        current={periodCompletions}
        target={goal.target_count!}
        period={goal.target_period!}
        color={goal.color}
      />
    {/if}
  </div>
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
    align-items: flex-start;
    gap: 12px;
    padding: 6px 0;
  }

  .goal-row.drag-over {
    background-color: var(--bg-tertiary);
  }

  .goal-info {
    flex-shrink: 0;
    min-width: 120px;
    width: 140px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .goal-name {
    padding: 4px 8px;
    background: none;
    border: 1px solid transparent;
    border-radius: 4px;
    cursor: grab;
    text-align: left;
    font-size: 14px;
    color: var(--text-primary);
    user-select: none;
    word-wrap: break-word;
  }

  .goal-name:hover {
    border-color: var(--border);
    background: var(--bg-secondary);
  }

  .goal-name:active {
    cursor: grabbing;
  }
</style>
