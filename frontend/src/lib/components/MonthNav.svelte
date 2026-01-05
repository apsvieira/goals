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
    ‹
  </button>
  <span class="month-display">{displayMonth}</span>
  <button class="nav-btn" on:click={onNext} aria-label="Next month">
    ›
  </button>
</div>

<style>
  .month-nav {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .nav-btn {
    padding: 4px 8px;
    font-size: 16px;
    background: transparent;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    color: var(--text-secondary);
    line-height: 1;
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
