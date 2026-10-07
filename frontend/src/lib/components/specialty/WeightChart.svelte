<script lang="ts">
  import type { WeightPoint } from '$lib/types/specialty';

  let { points }: { points: WeightPoint[] } = $props();

  const W = 560;
  const H = 200;
  const pad = { l: 44, r: 14, t: 14, b: 28 };
  const t = (d: string) => new Date(`${d}T12:00:00`).getTime();

  const geo = $derived.by(() => {
    if (!points.length) return null;
    const ws = points.map((p) => p.weight_kg);
    let lo = Math.min(...ws);
    let hi = Math.max(...ws);
    if (hi - lo < 0.5) {
      lo -= 0.5;
      hi += 0.5;
    }
    const margin = (hi - lo) * 0.12;
    lo = Math.max(0, lo - margin);
    hi += margin;
    const t0 = t(points[0].date);
    const t1 = t(points[points.length - 1].date);
    const x = (d: string) => (t1 === t0 ? (W + pad.l - pad.r) / 2 : pad.l + ((t(d) - t0) / (t1 - t0)) * (W - pad.l - pad.r));
    const y = (v: number) => pad.t + (1 - (v - lo) / (hi - lo)) * (H - pad.t - pad.b);
    const coords = points.map((p) => ({ x: x(p.date), y: y(p.weight_kg), p }));
    const ticks = [0, 0.5, 1].map((f) => ({ v: lo + (hi - lo) * f, y: y(lo + (hi - lo) * f) }));
    return { coords, ticks };
  });
  const fmt = (d: string) => new Date(`${d}T12:00:00`).toLocaleDateString('es-MX', { day: 'numeric', month: 'short', year: '2-digit' });
  const first = $derived(points[0]);
  const last = $derived(points[points.length - 1]);
</script>

{#if geo}
  <figure class="m-0">
    <svg viewBox="0 0 {W} {H}" class="h-auto w-full" role="img" aria-label="Peso de {fmt(first.date)} a {fmt(last.date)}: de {first.weight_kg} a {last.weight_kg} kilogramos, {points.length} mediciones">
      {#each geo.ticks as k}
        <line x1={pad.l} x2={W - pad.r} y1={k.y} y2={k.y} class="stroke-app-ink/10" />
        <text x={pad.l - 6} y={k.y + 4} text-anchor="end" font-size="11" class="fill-app-muted">{k.v.toFixed(1)}</text>
      {/each}
      {#if geo.coords.length > 1}
        <polyline fill="none" class="stroke-app-primary" stroke-width="2.5" stroke-linejoin="round" points={geo.coords.map((c) => `${c.x},${c.y}`).join(' ')} />
      {/if}
      {#each geo.coords as c}
        <circle cx={c.x} cy={c.y} r="4.5" class="fill-app-panel stroke-app-primary" stroke-width="2.5"><title>{fmt(c.p.date)}: {c.p.weight_kg} kg</title></circle>
      {/each}
      <text x={geo.coords[0].x} y={H - 8} font-size="11" text-anchor={geo.coords.length > 1 ? 'start' : 'middle'} class="fill-app-muted">{fmt(first.date)}</text>
      {#if geo.coords.length > 1}<text x={geo.coords[geo.coords.length - 1].x} y={H - 8} font-size="11" text-anchor="end" class="fill-app-muted">{fmt(last.date)}</text>{/if}
      <text x={W - pad.r} y="11" font-size="11" text-anchor="end" class="fill-app-muted">kg</text>
    </svg>
    <details class="mt-1 text-sm">
      <summary class="cursor-pointer text-app-muted">Ver mediciones como tabla</summary>
      <table class="mt-2 w-full text-left text-sm">
        <thead><tr><th class="th">Fecha</th><th class="th">Peso</th></tr></thead>
        <tbody>{#each [...points].reverse() as p}<tr class="border-t border-app-ink/10"><td class="td">{fmt(p.date)}</td><td class="td">{p.weight_kg} kg</td></tr>{/each}</tbody>
      </table>
    </details>
  </figure>
{:else}
  <p class="text-sm text-app-muted">Aún no hay pesos registrados. Se toman del campo «Peso» de la bitácora.</p>
{/if}
