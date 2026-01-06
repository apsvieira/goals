<script lang="ts">
  import DaySquare from './DaySquare.svelte';

  export let daysInMonth: number;
  export let color: string;
  export let completedDays: Set<number>;
  export let onToggle: (day: number) => void;
  export let currentDay: number = 0;
  export let month: string = ''; // YYYY-MM format

  $: days = Array.from({ length: daysInMonth }, (_, i) => i + 1);

  // Calculate the cutoff date (7 days ago from today)
  $: sevenDaysAgo = (() => {
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    const cutoff = new Date(today);
    cutoff.setDate(cutoff.getDate() - 6); // 6 days ago + today = 7 days window
    return cutoff;
  })();

  // Check if a day is disabled (either future or older than 7 days)
  function isDayDisabled(day: number): boolean {
    // Future days are disabled (existing behavior)
    if (day > currentDay && currentDay > 0) {
      return true;
    }
    // Days older than 7 days are disabled
    if (month) {
      const [year, monthNum] = month.split('-').map(Number);
      const dayDate = new Date(year, monthNum - 1, day);
      dayDate.setHours(0, 0, 0, 0);
      return dayDate < sevenDaysAgo;
    }
    return false;
  }
</script>

<div class="day-grid">
  {#each days as day}
    <DaySquare
      {day}
      {color}
      filled={completedDays.has(day)}
      onClick={() => onToggle(day)}
      disabled={isDayDisabled(day)}
    />
  {/each}
</div>

<style>
  .day-grid {
    display: grid;
    grid-template-columns: repeat(31, 1fr);
    gap: 1px;
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
