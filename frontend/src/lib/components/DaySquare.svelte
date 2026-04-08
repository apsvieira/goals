<script lang="ts">
  import { _ } from 'svelte-i18n';

  export let filled = false;
  export let color = '#4CAF50';
  export let day: number;
  export let dateString: string = ''; // "YYYY-MM-DD"
  export let outsideMonth: boolean = false;
  export let onClick: () => void = () => {};
  export let disabled = false;

  const WEEKDAY_LONG_KEYS = [
    'weekday.sun_long',
    'weekday.mon_long',
    'weekday.tue_long',
    'weekday.wed_long',
    'weekday.thu_long',
    'weekday.fri_long',
    'weekday.sat_long',
  ];
  const MONTH_KEYS = [
    'month.jan',
    'month.feb',
    'month.mar',
    'month.apr',
    'month.may',
    'month.jun',
    'month.jul',
    'month.aug',
    'month.sep',
    'month.oct',
    'month.nov',
    'month.dec',
  ];

  // Localized "Weekday, Mon D" label (e.g. "Sunday, Mar 29").
  $: ariaDate = (() => {
    if (!dateString) {
      return $_('aria.day', { values: { day } });
    }
    const d = new Date(dateString + 'T00:00:00');
    const weekday = $_(WEEKDAY_LONG_KEYS[d.getDay()]);
    const monthLabel = $_(MONTH_KEYS[d.getMonth()]);
    return `${weekday}, ${monthLabel} ${d.getDate()}`;
  })();

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
  class:outside-month={outsideMonth}
  style="--day-color: {color}"
  on:click={handleClick}
  aria-label={ariaDate}
  title={ariaDate}
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

  .day-square.outside-month {
    opacity: 0.35;
    border-style: dashed;
  }

  .day-square.outside-month.filled {
    opacity: 0.6;
  }

  .day-square.outside-month.disabled {
    opacity: 0.25;
  }
</style>
