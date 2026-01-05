<script lang="ts">
  import MonthNav from './MonthNav.svelte';

  export let month: string;
  export let onPrev: () => void;
  export let onNext: () => void;
  export let showAddForm: boolean;
  export let onToggleAddForm: () => void;
</script>

<header>
  <div class="header-content">
    <MonthNav {month} {onPrev} {onNext} />
    <button
      class="add-btn"
      on:click={onToggleAddForm}
      aria-label={showAddForm ? 'Close form' : 'Add goal'}
    >
      {showAddForm ? '×' : '+'}
    </button>
  </div>
</header>

<style>
  header {
    position: sticky;
    top: 0;
    z-index: 100;
    background: var(--bg-primary);
    border-bottom: 1px solid var(--border);
    padding: 16px 24px;
  }

  .header-content {
    display: flex;
    justify-content: center;
    align-items: center;
    gap: 16px;
    max-width: 1400px;
    margin: 0 auto;
  }

  .add-btn {
    width: 32px;
    height: 32px;
    padding: 0;
    font-size: 20px;
    line-height: 1;
    background: var(--accent);
    color: white;
    border: none;
    border-radius: 50%;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .add-btn:hover {
    background: var(--accent-hover);
  }
</style>
