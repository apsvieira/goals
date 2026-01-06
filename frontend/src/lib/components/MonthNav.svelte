<script lang="ts">
  export let month: string; // YYYY-MM format
  export let onPrev: () => void;
  export let onNext: () => void;

  const monthNames = [
    'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun',
    'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'
  ];

  $: {
    const [year, monthNum] = month.split('-').map(Number);
    displayMonth = monthNames[monthNum - 1];
  }

  let displayMonth: string;
</script>

<div class="month-nav">
  <button class="nav-btn" on:click={onPrev} aria-label="Previous month">
    <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
      <path d="M15.41 7.41L14 6l-6 6 6 6 1.41-1.41L10.83 12z"/>
    </svg>
  </button>
  <span class="month-display">{displayMonth}</span>
  <button class="nav-btn" on:click={onNext} aria-label="Next month">
    <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
      <path d="M10 6L8.59 7.41 13.17 12l-4.58 4.59L10 18l6-6z"/>
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
