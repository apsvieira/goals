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
    gap: 4px;
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
    width: 6px;
    height: 6px;
  }

  .medium .dot {
    width: 10px;
    height: 10px;
  }

  .large .dot {
    width: 14px;
    height: 14px;
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
