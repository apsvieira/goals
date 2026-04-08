<script lang="ts">
  import { _ } from 'svelte-i18n';

  // Decorative header — day cells carry semantic date labels.
  $: labels = [
    $_('weekday.sun'),
    $_('weekday.mon'),
    $_('weekday.tue'),
    $_('weekday.wed'),
    $_('weekday.thu'),
    $_('weekday.fri'),
    $_('weekday.sat'),
  ];
</script>

<div class="weekday-header" aria-hidden="true">
  {#each labels as label}
    <div class="weekday-cell">{label}</div>
  {/each}
</div>

<style>
  .weekday-header {
    display: grid;
    grid-template-columns: repeat(7, 1fr);
    gap: 1px;
    flex: 1;
    min-width: 0;
    position: sticky;
    top: 0;
    z-index: 2;
    background: var(--bg-primary);
    border-bottom: 1px solid var(--border);
    padding: 0.25rem 0;
  }

  .weekday-cell {
    text-align: center;
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.03125rem;
  }
</style>
