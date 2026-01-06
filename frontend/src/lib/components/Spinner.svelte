<script lang="ts">
  export let size: 'small' | 'medium' | 'large' = 'medium';
  export let color: string = 'var(--text-muted)';
</script>

<div class="spinner {size}" style="--spinner-color: {color}">
  <div class="dot"></div>
  <div class="dot"></div>
  <div class="dot"></div>
</div>

<style>
  .spinner {
    display: flex;
    gap: 0.25rem;
    align-items: center;
    justify-content: center;
  }

  .dot {
    background-color: var(--spinner-color);
    border-radius: 50%;
    animation: bounce 1.4s ease-in-out infinite both;
  }

  .dot:nth-child(1) { animation-delay: -0.32s; }
  .dot:nth-child(2) { animation-delay: -0.16s; }
  .dot:nth-child(3) { animation-delay: 0s; }

  .small .dot {
    width: 0.375rem;
    height: 0.375rem;
  }

  .medium .dot {
    width: 0.625rem;
    height: 0.625rem;
  }

  .large .dot {
    width: 0.875rem;
    height: 0.875rem;
  }

  @keyframes bounce {
    0%, 80%, 100% {
      transform: scale(0);
      opacity: 0.5;
    }
    40% {
      transform: scale(1);
      opacity: 1;
    }
  }
</style>
