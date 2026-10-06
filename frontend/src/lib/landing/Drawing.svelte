<script lang="ts">
  import { onMount } from 'svelte';
  import { EASE, interp, observeOnce, reducedMotion, track } from './motion';

  interface Props {
    paths: string[];
    viewBox: string;
    class: string;
  }
  let { paths, viewBox, class: cls }: Props = $props();

  let box: HTMLDivElement;
  let drawn = $state(false);

  onMount(() => {
    const reduce = reducedMotion();
    const stopView = observeOnce(box, () => (drawn = true), '-20% 0px');
    const stopScroll = track(box, ['start end', 'end start'], (p) => {
      box.style.transform = reduce ? '' : `translate3d(0, ${interp(p, [0, 1], [60, -60])}px, 0) rotate(${interp(p, [0, 1], [-6, 6])}deg)`;
    });
    return () => {
      stopView();
      stopScroll();
    };
  });
</script>

<div bind:this={box} aria-hidden="true" class={cls}>
  <svg {viewBox} fill="none" class="h-full w-full">
    {#each paths as d, i}
      <path
        {d}
        pathLength="1"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        style="stroke-dasharray: 1; stroke-dashoffset: {drawn ? 0 : 1}; transition: stroke-dashoffset 2.2s {EASE} {0.2 + i * 0.3}s"
      />
    {/each}
  </svg>
</div>
