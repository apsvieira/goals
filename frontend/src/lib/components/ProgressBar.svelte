<script lang="ts">
  export let current: number;
  export let target: number;
  export let period: 'week' | 'month';
  export let color: string;

  $: percentage = Math.min(100, Math.round((current / target) * 100));
  $: periodShort = period === 'week' ? '/wk' : '/mo';
</script>

<div
  class="progress-bar"
  role="progressbar"
  aria-valuenow={current}
  aria-valuemin={0}
  aria-valuemax={target}
  aria-label="{current} of {target} {period === 'week' ? 'weekly' : 'monthly'} goal completed"
>
  <div class="bar-container">
    <div
      class="bar-fill"
      style="width: {percentage}%; background-color: {color}"
    ></div>
  </div>
  <span class="progress-text" aria-hidden="true">{current}/{target}{periodShort}</span>
</div>

<style>
  .progress-bar {
    display: flex;
    align-items: center;
    gap: 0.375rem;
    padding: 0 0.5rem;
  }

  .bar-container {
    flex: 1;
    height: 0.375rem;
    background: var(--bg-tertiary);
    border-radius: 0.1875rem;
    overflow: hidden;
    min-width: 2.5rem;
  }

  .bar-fill {
    height: 100%;
    border-radius: 0.1875rem;
    transition: width 0.3s ease;
  }

  .progress-text {
    font-size: 0.6875rem;
    color: var(--text-muted);
    white-space: nowrap;
  }
</style>
