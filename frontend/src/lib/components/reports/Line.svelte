<script lang="ts">
  // Line/area chart in SVG; the data is also available as a visually hidden list.
  interface Pt {
    label: string;
    value: number;
  }
  interface Props {
    points: Pt[];
    ariaLabel: string;
  }
  let { points, ariaLabel }: Props = $props();
  const W = 600;
  const H = 180;
  const PAD = 8;
  const max = $derived(Math.max(1, ...points.map((p) => p.value)));
  const xy = $derived(
    points.map((p, i) => ({
      x: points.length < 2 ? W / 2 : PAD + (i / (points.length - 1)) * (W - PAD * 2),
      y: H - PAD - (p.value / max) * (H - PAD * 2)
    }))
  );
  const line = $derived(xy.map((p, i) => `${i ? 'L' : 'M'}${p.x.toFixed(1)},${p.y.toFixed(1)}`).join(' '));
  const area = $derived(xy.length ? `${line} L${xy[xy.length - 1].x.toFixed(1)},${H - PAD} L${xy[0].x.toFixed(1)},${H - PAD} Z` : '');
</script>

<figure class="text-app-primary">
  <svg viewBox="0 0 {W} {H}" class="h-44 w-full" role="img" aria-label={ariaLabel} preserveAspectRatio="none">
    <line x1={PAD} x2={W - PAD} y1={H - PAD} y2={H - PAD} stroke="currentColor" stroke-opacity="0.2" />
    <path d={area} fill="currentColor" fill-opacity="0.12" />
    <path d={line} fill="none" stroke="currentColor" stroke-width="2.5" stroke-linejoin="round" stroke-linecap="round" vector-effect="non-scaling-stroke" />
    {#if points.length <= 40}
      {#each xy as p, i (i)}
        <circle cx={p.x} cy={p.y} r="3" fill="currentColor"><title>{points[i].label}: {points[i].value}</title></circle>
      {/each}
    {/if}
  </svg>
  {#if points.length}
    <div class="mt-1 flex justify-between text-xs text-app-muted"><span>{points[0].label}</span><span>Máximo {max}</span><span>{points[points.length - 1].label}</span></div>
  {/if}
  <ul class="sr-only">
    {#each points as p}<li>{p.label}: {p.value}</li>{/each}
  </ul>
</figure>
