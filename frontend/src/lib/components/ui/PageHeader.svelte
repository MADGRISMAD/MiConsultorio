<script lang="ts">
  import type { Snippet } from 'svelte';

  interface Props {
    title: string;
    subtitle?: string;
    actions?: Snippet;
  }
  let { title, subtitle, actions }: Props = $props();
</script>

<div class="mb-7 flex flex-wrap items-end justify-between gap-4">
  <div>
    <h1 class="display text-[2.25rem] leading-[1] sm:text-5xl">{title}</h1>
    {#if subtitle}<p class="mt-2 text-[15px] text-app-muted">{subtitle}</p>{/if}
  </div>
  {#if actions}<div class="flex flex-wrap items-center gap-2">{@render actions()}</div>{/if}
</div>
