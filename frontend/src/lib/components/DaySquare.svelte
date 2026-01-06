<script lang="ts">
  export let filled = false;
  export let color = '#4CAF50';
  export let day: number;
  export let onClick: () => void = () => {};
  export let disabled = false;

  function handleClick() {
    if (!disabled) {
      onClick();
    }
  }
</script>

<button
  class="day-square"
  class:filled
  class:disabled
  style="--day-color: {color}"
  on:click={handleClick}
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
    border-radius: 0.1875rem;
    background: var(--bg-tertiary);
    opacity: 0.75;
    cursor: pointer;
    padding: 0;
    transition: opacity 0.1s ease;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.875rem;
    font-weight: 500;
    color: var(--day-color);
  }

  .day-square.filled {
    background: var(--day-color);
    color: var(--bg-tertiary);
    opacity: 1;
  }

  .day-square:hover:not(.disabled) {
    opacity: 1;
  }

  .day-square:focus {
    outline: 2px solid var(--day-color);
    outline-offset: 1px;
  }

  .day-square.disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
</style>
