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
    &larr;
  </button>
  <h2 class="month-display">{displayMonth}</h2>
  <button class="nav-btn" on:click={onNext} aria-label="Next month">
    &rarr;
  </button>
</div>

<style>
  .month-nav {
    display: flex;
    align-items: center;
    gap: 16px;
    margin-bottom: 24px;
  }

  .nav-btn {
    padding: 8px 16px;
    font-size: 18px;
    background: #f0f0f0;
    border: 1px solid #ccc;
    border-radius: 4px;
    cursor: pointer;
  }

  .nav-btn:hover {
    background: #e0e0e0;
  }

  .month-display {
    margin: 0;
    font-size: 24px;
    font-weight: 600;
    min-width: 200px;
    text-align: center;
  }
</style>
