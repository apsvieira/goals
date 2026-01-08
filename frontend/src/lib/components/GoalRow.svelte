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
  export let isFocused = false;
  export let currentDay: number = 0;
  export let periodCompletions: number = 0; // Completions in current period (week/month)
  export let month: string = ''; // YYYY-MM format for 7-day limit

  $: hasTarget = goal.target_count && goal.target_period;
</script>

<div
  class="goal-row"
  class:drag-over={isDragOver}
  class:keyboard-focused={isFocused}
  on:dragover={onDragOver}
  on:drop={onDrop}
  on:dragleave={() => isDragOver = false}
  role="listitem"
  aria-label="Goal: {goal.name}{isFocused ? ' (selected for keyboard navigation)' : ''}"
>
  <div class="goal-info">
    <button
      class="goal-name"
      on:click={onEdit}
      draggable="true"
      on:dragstart={onDragStart}
      aria-label="Edit {goal.name}. Drag to reorder."
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
    {month}
  />
</div>

<style>
  .goal-row {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.375rem 0;
    margin: 0 0.3125rem;
  }

  .goal-row.drag-over {
    background-color: var(--bg-tertiary);
  }

  .goal-row.keyboard-focused {
    background-color: var(--bg-secondary);
    border-left: 3px solid var(--accent);
    padding-left: 0.125rem;
  }

  .goal-info {
    flex-shrink: 0;
    min-width: 7.5rem;
    width: 8.75rem;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    justify-content: center;
    align-self: stretch;
  }

  .goal-name {
    padding: 0.25rem 0.5rem;
    background: none;
    border: 1px solid transparent;
    border-radius: 0.25rem;
    cursor: grab;
    text-align: left;
    font-size: 0.875rem;
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
