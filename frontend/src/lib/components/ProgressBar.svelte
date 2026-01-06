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
    gap: 6px;
    padding: 0 8px;
  }

  .bar-container {
    flex: 1;
    height: 6px;
    background: var(--bg-tertiary);
    border-radius: 3px;
    overflow: hidden;
    min-width: 40px;
  }

  .bar-fill {
    height: 100%;
    border-radius: 3px;
    transition: width 0.3s ease;
  }

  .progress-text {
    font-size: 11px;
    color: var(--text-muted);
    white-space: nowrap;
  }
</style>
