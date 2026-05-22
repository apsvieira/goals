<script lang="ts">
  import { _ } from 'svelte-i18n';
  import { viewSettings, updateViewSettings, type CalendarView } from '../view-settings';

  function select(view: CalendarView) {
    if ($viewSettings.calendarView === view) return;
    void updateViewSettings({ calendarView: view });
  }
</script>

<div class="view-toggle" role="group" aria-label="Calendar view">
  <button
    class="toggle-btn"
    class:active={$viewSettings.calendarView === 'month'}
    on:click={() => select('month')}
    aria-label={$_('viewToggle.month')}
    title={$_('viewToggle.month')}
    aria-pressed={$viewSettings.calendarView === 'month'}
  >
    {$_('viewToggle.monthShort')}
  </button>
  <button
    class="toggle-btn"
    class:active={$viewSettings.calendarView === 'week'}
    on:click={() => select('week')}
    aria-label={$_('viewToggle.week')}
    title={$_('viewToggle.week')}
    aria-pressed={$viewSettings.calendarView === 'week'}
  >
    {$_('viewToggle.weekShort')}
  </button>
</div>

<style>
  .view-toggle {
    display: flex;
    align-items: center;
    border: 1px solid var(--border);
    border-radius: 0.375rem;
    overflow: hidden;
  }

  .toggle-btn {
    padding: 0.25rem 0.5rem;
    background: transparent;
    border: none;
    border-radius: 0;
    cursor: pointer;
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--text-secondary);
    line-height: 1;
    min-width: 1.5rem;
    transition: background-color 0.15s, color 0.15s;
  }

  .toggle-btn + .toggle-btn {
    border-left: 1px solid var(--border);
  }

  .toggle-btn:hover:not(.active) {
    background: var(--bg-secondary);
    color: var(--text-primary);
  }

  .toggle-btn.active {
    background: var(--accent);
    color: white;
  }
</style>
