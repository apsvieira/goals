<script lang="ts">
  import { _ } from 'svelte-i18n';

  export let month: string; // YYYY-MM format
  export let onPrev: () => void;
  export let onNext: () => void;
  export let disableNext: boolean = false;

  const monthKeys = [
    'month.jan', 'month.feb', 'month.mar', 'month.apr',
    'month.may', 'month.jun', 'month.jul', 'month.aug',
    'month.sep', 'month.oct', 'month.nov', 'month.dec'
  ];

  $: {
    const [year, monthNum] = month.split('-').map(Number);
    displayMonth = $_(monthKeys[monthNum - 1]);
  }

  let displayMonth: string;
</script>

<div class="month-nav">
  <button class="nav-btn" on:click={onPrev} aria-label={$_('aria.previousMonth')}>
    <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
      <path d="M15.41 7.41L14 6l-6 6 6 6 1.41-1.41L10.83 12z"/>
    </svg>
  </button>
  <span class="month-display">{displayMonth}</span>
  <button class="nav-btn" on:click={onNext} aria-label={$_('aria.nextMonth')} disabled={disableNext}>
    <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
      <path d="M10 6L8.59 7.41 13.17 12l-4.58 4.59L10 18l6-6z"/>
    </svg>
  </button>
</div>

<style>
  .month-nav {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .nav-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0.25rem;
    background: transparent;
    border: none;
    border-radius: 0.25rem;
    cursor: pointer;
    color: var(--text-secondary);
  }

  .nav-btn:hover:not(:disabled) {
    background: var(--bg-tertiary);
    color: var(--text-primary);
  }

  .nav-btn:disabled {
    opacity: 0.25;
    cursor: default;
    pointer-events: none;
  }

  .month-display {
    font-size: 1rem;
    font-weight: 500;
    color: var(--text-primary);
  }
</style>
