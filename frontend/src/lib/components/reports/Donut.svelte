<script lang="ts">
  // Donut chart in SVG with a legend (the legend doubles as the accessible text version).
  interface Slice {
    label: string;
    value: number;
  }
  interface Props {
    slices: Slice[];
    ariaLabel: string;
  }
  let { slices, ariaLabel }: Props = $props();
  const COLORS = ['rgb(var(--app-primary))', 'rgb(var(--app-accent))', 'rgb(var(--app-warning))', 'rgb(var(--app-danger))', 'rgb(var(--app-muted))', 'rgb(var(--app-ink) / 0.45)', 'rgb(var(--app-ink) / 0.2)'];
  const total = $derived(slices.reduce((a, s) => a + s.value, 0));
  const R = 40;
  const C = 2 * Math.PI * R;
  const arcs = $derived.by(() => {
    let acc = 0;
    return slices.map((s, i) => {
      const len = total > 0 ? (s.value / total) * C : 0;
      const a = { len, off: -acc, color: COLORS[i % COLORS.length] };
      acc += len;
      return a;
    });
  });
</script>

<div class="flex flex-wrap items-center gap-5 text-app-ink">
  <svg viewBox="0 0 100 100" class="h-36 w-36 shrink-0 -rotate-90" role="img" aria-label={ariaLabel}>
    <circle cx="50" cy="50" r={R} fill="none" stroke="currentColor" stroke-opacity="0.08" stroke-width="14" />
    {#each arcs as a, i (i)}
      {#if a.len > 0}
        <circle cx="50" cy="50" r={R} fill="none" stroke={a.color} stroke-width="14" stroke-dasharray="{a.len} {C - a.len}" stroke-dashoffset={a.off}><title>{slices[i].label}: {slices[i].value}</title></circle>
      {/if}
    {/each}
  </svg>
  <ul class="grid min-w-[10rem] flex-1 gap-1.5 text-sm">
    {#each slices as s, i (i)}
      <li class="flex items-center justify-between gap-3">
        <span class="flex items-center gap-2"><span class="h-2.5 w-2.5 shrink-0 rounded-sm" style="background: {COLORS[i % COLORS.length]}"></span>{s.label}</span>
        <span class="font-medium">{s.value}{#if total > 0}<span class="ml-1 text-app-muted">({Math.round((s.value / total) * 100)}%)</span>{/if}</span>
      </li>
    {/each}
  </ul>
</div>
