<script lang="ts">
  export let filled = false;
  export let color = '#4CAF50';
  export let day: number;
  export let onClick: () => void = () => {};
</script>

<button
  class="day-square"
  class:filled
  style="--day-color: {color}"
  on:click={onClick}
  aria-label="Day {day}"
  title="Day {day}"
>
  {day}
</button>

<style>
  .day-square {
    width: 100%;
    aspect-ratio: 1;
    border: 1.5px solid var(--day-color);
    border-radius: 3px;
    background: var(--bg-tertiary);
    opacity: 0.6;
    cursor: pointer;
    padding: 0;
    transition: transform 0.1s ease, opacity 0.1s ease;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 14px;
    font-weight: 500;
    color: var(--day-color);
  }

  .day-square.filled {
    background: var(--day-color);
    color: var(--bg-tertiary);
    opacity: 1;
  }

  .day-square:hover {
    transform: scale(1.1);
    opacity: 1;
  }

  .day-square:focus {
    outline: 2px solid var(--day-color);
    outline-offset: 1px;
  }
</style>
