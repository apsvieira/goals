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
    <div class="spacer"></div>
    <MonthNav {month} {onPrev} {onNext} />
    <button
      class="add-btn"
      on:click={onToggleAddForm}
      aria-label={showAddForm ? 'Close form' : 'Add goal'}
    >
      {showAddForm ? 'Cancel' : 'New Goal'}
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
    justify-content: space-between;
    align-items: center;
    max-width: 1400px;
    margin: 0 auto;
  }

  .spacer {
    width: 90px;
  }

  .add-btn {
    padding: 8px 16px;
    font-size: 14px;
    font-weight: 500;
    background: var(--accent);
    color: white;
    border: none;
    border-radius: 6px;
    cursor: pointer;
    min-width: 90px;
  }

  .add-btn:hover {
    background: var(--accent-hover);
  }
</style>
