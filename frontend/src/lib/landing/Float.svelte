<script module lang="ts">
  export interface FloatCtx {
    /** hero scroll progress 0..1 */
    p: number;
    /** pointer offset from centre, smoothed, in [-0.5, 0.5] */
    mx: number;
    my: number;
  }
</script>

<script lang="ts">
  import type { Snippet } from 'svelte';
  import { onMount } from 'svelte';
  import { onFrame, reducedMotion } from './motion';

  interface Props {
    depth: number;
    ctx: FloatCtx;
    delay: number;
    class: string;
    children: Snippet;
  }
  let { depth, ctx, delay, class: cls, children }: Props = $props();

  let el: HTMLDivElement;
  onMount(() => {
    const k = reducedMotion() ? 0 : depth;
    return onFrame(() => {
      el.style.transform = `translate3d(${ctx.mx * k * 36}px, ${-ctx.p * k * 220 + ctx.my * k * 36}px, 0)`;
    });
  });
</script>

<div bind:this={el} aria-hidden="true" class="absolute z-20 {cls}">
  <div class="enter-float" style="--d: {delay}s">
    {@render children()}
  </div>
</div>
