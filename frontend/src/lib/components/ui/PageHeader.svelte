<script lang="ts">
  import type { Snippet } from 'svelte';

  interface Props {
    title: string;
    subtitle?: string;
    actions?: Snippet;
  }
  let { title, subtitle, actions }: Props = $props();
</script>

<div class="mb-6 flex flex-wrap items-end justify-between gap-4">
  <div>
    <h1 class="text-2xl font-extrabold tracking-tight sm:text-3xl">{title}</h1>
    {#if subtitle}<p class="mt-1 text-sm text-app-muted">{subtitle}</p>{/if}
  </div>
  {#if actions}<div class="flex flex-wrap items-center gap-2">{@render actions()}</div>{/if}
</div>
