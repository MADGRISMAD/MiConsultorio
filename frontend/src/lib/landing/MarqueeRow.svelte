<script lang="ts">
  import { onMount } from 'svelte';
  import { clamp, onFrame, reducedMotion, Spring } from './motion';

  interface Props {
    base: number;
    items: string[];
    outline?: boolean;
  }
  let { base, items, outline = false }: Props = $props();

  const wrap = (min: number, max: number, v: number) => {
    const r = max - min;
    return ((((v - min) % r) + r) % r) + min;
  };

  let track: HTMLDivElement;

  // Drifts sideways on its own and speeds up (or reverses) with scroll velocity.
  onMount(() => {
    if (reducedMotion()) return;
    const velocity = new Spring(0, { stiffness: 400, damping: 50 });
    let x = 0;
    let dir = 1;
    let prevY = scrollY;
    return onFrame((dt) => {
      const y = scrollY;
      const raw = dt > 0 ? (y - prevY) / dt : 0;
      prevY = y;
      const boost = clamp((velocity.step(raw, dt) / 1500) * 4, -4, 4);
      if (boost < 0) dir = -1;
      else if (boost > 0) dir = 1;
      x += dir * base * dt * (1 + Math.abs(boost));
      track.style.transform = `translate3d(${wrap(-50, 0, x)}%, 0, 0)`;
    });
  });
</script>

{#snippet content()}
  {#each items as s, i}
    <span class="flex items-center">
      <span class="px-6 sm:px-10 {i % 2 ? 'italic' : ''} {outline ? 'text-transparent [-webkit-text-stroke:1.2px_#0B2540]' : ''}">{s}</span>
      <svg viewBox="0 0 24 24" class="h-6 w-6 flex-none text-signal sm:h-9 sm:w-9" aria-hidden="true"><path d="M12 3v18M3 12h18" stroke="currentColor" stroke-width="3" stroke-linecap="round" /></svg>
    </span>
  {/each}
{/snippet}

<div class="overflow-hidden whitespace-nowrap">
  <div bind:this={track} class="flex w-max font-display text-[clamp(3rem,9vw,8rem)] leading-[1.05] tracking-[-0.02em]">
    <span class="flex">{@render content()}</span>
    <span class="flex" aria-hidden="true">{@render content()}</span>
  </div>
</div>
