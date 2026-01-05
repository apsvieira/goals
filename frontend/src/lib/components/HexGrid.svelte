<script lang="ts">
  import Hex from './Hex.svelte';

  export let daysInMonth: number;
  export let color: string;
  export let completedDays: Set<number>;
  export let onToggle: (day: number) => void;

  $: days = Array.from({ length: daysInMonth }, (_, i) => i + 1);
</script>

<div class="hex-grid">
  {#each days as day}
    <Hex
      {day}
      {color}
      filled={completedDays.has(day)}
      onClick={() => onToggle(day)}
    />
  {/each}
</div>

<style>
  .hex-grid {
    display: grid;
    grid-template-columns: repeat(7, auto);
    gap: 4px;
    justify-content: start;
  }
</style>
