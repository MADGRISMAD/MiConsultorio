<script lang="ts">
  // Vertical bar chart drawn with plain markup. Each bar also has a screen-reader label.
  interface Item {
    label: string;
    value: number;
    /** shown in the tooltip and read aloud instead of the bare number */
    text?: string;
  }
  interface Props {
    items: Item[];
    ariaLabel: string;
    height?: number;
    /** show every Nth label to avoid crowding */
    labelEvery?: number;
  }
  let { items, ariaLabel, height = 176, labelEvery = 1 }: Props = $props();
  const max = $derived(Math.max(1, ...items.map((i) => i.value)));
  const minW = $derived(Math.min(items.length, 62) * 26);
</script>

<div class="overflow-x-auto pb-1">
  <ul class="flex items-end gap-1.5 text-app-primary" style="height: {height}px; min-width: {minW}px" aria-label={ariaLabel}>
    {#each items as it, i (i)}
      <li class="group relative flex h-full min-w-[18px] max-w-[72px] flex-1 flex-col justify-end" title="{it.label}: {it.text ?? it.value}">
        <span class="rounded-t-md bg-current opacity-80 transition group-hover:opacity-100" style="height: {it.value > 0 ? Math.max(3, (it.value / max) * 100) : 0}%"></span>
        <span class="sr-only">{it.label}: {it.text ?? it.value}</span>
      </li>
    {/each}
  </ul>
  <ul class="mt-1.5 flex gap-1.5 text-[11px] text-app-muted" style="min-width: {minW}px" aria-hidden="true">
    {#each items as it, i (i)}
      <li class="min-w-[18px] max-w-[72px] flex-1 truncate text-center">{i % labelEvery === 0 ? it.label : ''}</li>
    {/each}
  </ul>
</div>
