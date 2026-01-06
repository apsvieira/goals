<script lang="ts">
  import DaySquare from './DaySquare.svelte';

  export let daysInMonth: number;
  export let color: string;
  export let completedDays: Set<number>;
  export let onToggle: (day: number) => void;
  export let currentDay: number = 0;

  $: days = Array.from({ length: daysInMonth }, (_, i) => i + 1);
</script>

<div class="day-grid">
  {#each days as day}
    <DaySquare
      {day}
      {color}
      filled={completedDays.has(day)}
      onClick={() => onToggle(day)}
      disabled={day > currentDay && currentDay > 0}
    />
  {/each}
</div>

<style>
  .day-grid {
    display: grid;
    grid-template-columns: repeat(31, 1fr);
    gap: 0;
    flex: 1;
    min-width: 0;
  }

  /* Small screens: 5 rows (7 per row) */
  @media (max-width: 500px) {
    .day-grid {
      grid-template-columns: repeat(7, 1fr);
    }
  }
</style>
