<script lang="ts">
  export let filled = false;
  export let color = '#4CAF50';
  export let day: number;
  export let onClick: () => void = () => {};

  // Hexagon path for a flat-topped hexagon
  const size = 20;
  const h = size * Math.sqrt(3) / 2;
  const points = [
    [size / 2, 0],
    [size, h / 2],
    [size, h * 1.5],
    [size / 2, h * 2],
    [0, h * 1.5],
    [0, h / 2],
  ].map(([x, y]) => `${x},${y}`).join(' ');
</script>

<svg
  class="hex"
  width={size}
  height={h * 2}
  viewBox="0 0 {size} {h * 2}"
  on:click={onClick}
  on:keydown={(e) => e.key === 'Enter' && onClick()}
  role="button"
  tabindex="0"
  aria-label="Day {day}"
>
  <title>Day {day}</title>
  <polygon
    {points}
    fill={filled ? color : 'transparent'}
    stroke={color}
    stroke-width="1.5"
  />
</svg>

<style>
  .hex {
    cursor: pointer;
    transition: transform 0.1s ease;
  }
  .hex:hover {
    transform: scale(1.1);
  }
  .hex:focus {
    outline: none;
  }
  .hex:focus polygon {
    stroke-width: 2.5;
  }
</style>
