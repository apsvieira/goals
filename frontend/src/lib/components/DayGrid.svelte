<script lang="ts">
  import DaySquare from './DaySquare.svelte';
  import type { CalendarCell } from '../calendar';

  export let cells: CalendarCell[];
  export let color: string;
  export let completedDates: Map<string, string>;
  export let onToggle: (dateString: string) => void;

  // Calculate the cutoff date (7 days ago from today)
  $: sevenDaysAgo = (() => {
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    const cutoff = new Date(today);
    cutoff.setDate(cutoff.getDate() - 6); // 6 days ago + today = 7 days window
    return cutoff;
  })();

  $: today = (() => {
    const t = new Date();
    t.setHours(0, 0, 0, 0);
    return t;
  })();

  // Future dates and dates older than the 7-day lockout window are disabled.
  function isCellDisabled(cell: CalendarCell): boolean {
    const cellDate = new Date(cell.dateString + 'T00:00:00');
    cellDate.setHours(0, 0, 0, 0);
    if (cellDate > today) {
      return true;
    }
    if (cellDate < sevenDaysAgo) {
      return true;
    }
    return false;
  }
</script>

<div class="day-grid">
  {#each cells as cell (cell.dateString)}
    <DaySquare
      day={cell.dayNumber}
      dateString={cell.dateString}
      outsideMonth={!cell.isCurrentMonth}
      {color}
      filled={completedDates.has(cell.dateString)}
      onClick={() => onToggle(cell.dateString)}
      disabled={isCellDisabled(cell)}
    />
  {/each}
</div>

<style>
  .day-grid {
    display: grid;
    grid-template-columns: repeat(7, 1fr);
    gap: 1px;
    flex: 1;
    min-width: 0;
  }
</style>
