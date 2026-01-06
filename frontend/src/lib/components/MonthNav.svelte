<script lang="ts">
  export let month: string; // YYYY-MM format
  export let onPrev: () => void;
  export let onNext: () => void;

  const monthNames = [
    'January', 'February', 'March', 'April', 'May', 'June',
    'July', 'August', 'September', 'October', 'November', 'December'
  ];

  $: {
    const [year, monthNum] = month.split('-').map(Number);
    displayMonth = `${monthNames[monthNum - 1]} ${year}`;
  }

  let displayMonth: string;
</script>

<div class="month-nav">
  <button class="nav-btn" on:click={onPrev} aria-label="Previous month">
    <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <polyline points="15 18 9 12 15 6"></polyline>
    </svg>
  </button>
  <span class="month-display">{displayMonth}</span>
  <button class="nav-btn" on:click={onNext} aria-label="Next month">
    <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <polyline points="9 18 15 12 9 6"></polyline>
    </svg>
  </button>
</div>

<style>
  .month-nav {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .nav-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 4px;
    background: transparent;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    color: var(--text-secondary);
  }

  .nav-btn:hover {
    background: var(--bg-tertiary);
    color: var(--text-primary);
  }

  .month-display {
    font-size: 16px;
    font-weight: 500;
    color: var(--text-primary);
  }
</style>
