<script module lang="ts">
  export interface Pt {
    date: string; // YYYY-MM-DD
    value: number;
  }
  export interface Series {
    label: string;
    points: Pt[];
  }
</script>

<script lang="ts">
  /** A line chart of one or two measures over time (blood pressure draws two lines). Accessible: a table sits under it. */
  let { title, unit = '', series, decimals = 1 }: { title: string; unit?: string; series: Series[]; decimals?: number } = $props();

  const W = 560;
  const H = 190;
  const pad = { l: 44, r: 14, t: 16, b: 28 };
  const t = (d: string) => new Date(`${d}T12:00:00`).getTime();
  const fmt = (d: string) => new Date(`${d}T12:00:00`).toLocaleDateString('es-MX', { day: 'numeric', month: 'short', year: '2-digit' });
  const STROKES = ['stroke-app-primary', 'stroke-app-accent'];
  const FILLS = ['fill-app-primary', 'fill-app-accent'];

  const all = $derived(series.flatMap((s) => s.points));
  const geo = $derived.by(() => {
    if (!all.length) return null;
    let lo = Math.min(...all.map((p) => p.value));
    let hi = Math.max(...all.map((p) => p.value));
    if (hi - lo < 0.001) {
      lo -= 1;
      hi += 1;
    }
    const m = (hi - lo) * 0.14;
    lo -= m;
    hi += m;
    const ts = all.map((p) => t(p.date));
    const t0 = Math.min(...ts);
    const t1 = Math.max(...ts);
    const x = (d: string) => (t1 === t0 ? (W + pad.l - pad.r) / 2 : pad.l + ((t(d) - t0) / (t1 - t0)) * (W - pad.l - pad.r));
    const y = (v: number) => pad.t + (1 - (v - lo) / (hi - lo)) * (H - pad.t - pad.b);
    return {
      lines: series.map((s) => s.points.map((p) => ({ x: x(p.date), y: y(p.value), p }))),
      ticks: [0, 0.5, 1].map((f) => ({ v: lo + (hi - lo) * f, y: y(lo + (hi - lo) * f) })),
      from: new Date(t0).toISOString().slice(0, 10),
      to: new Date(t1).toISOString().slice(0, 10),
      x0: x(new Date(t0).toISOString().slice(0, 10)),
      x1: x(new Date(t1).toISOString().slice(0, 10))
    };
  });
  const dates = $derived([...new Set(all.map((p) => p.date))].sort());
  const at = (s: Series, d: string) => s.points.find((p) => p.date === d)?.value;
</script>

{#if geo}
  <figure class="m-0">
    <figcaption class="mb-1 flex flex-wrap items-baseline justify-between gap-2 text-sm">
      <strong>{title}</strong>
      {#if series.length > 1}
        <span class="flex gap-3 text-xs text-app-muted">{#each series as s, i}<span class="flex items-center gap-1"><i class="inline-block h-2 w-3 rounded-sm {i ? 'bg-app-accent' : 'bg-app-primary'}"></i>{s.label}</span>{/each}</span>
      {/if}
    </figcaption>
    <svg viewBox="0 0 {W} {H}" class="h-auto w-full" role="img" aria-label="{title}: {series.map((s) => `${s.label} de ${s.points[0].value} a ${s.points[s.points.length - 1].value}`).join('; ')} {unit}">
      {#each geo.ticks as k}
        <line x1={pad.l} x2={W - pad.r} y1={k.y} y2={k.y} class="stroke-app-ink/10" />
        <text x={pad.l - 6} y={k.y + 4} text-anchor="end" font-size="11" class="fill-app-muted">{k.v.toFixed(decimals)}</text>
      {/each}
      {#each geo.lines as line, i}
        {#if line.length > 1}<polyline fill="none" class={STROKES[i % 2]} stroke-width="2.5" stroke-linejoin="round" points={line.map((c) => `${c.x},${c.y}`).join(' ')} />{/if}
        {#each line as c}
          <circle cx={c.x} cy={c.y} r="4.5" class="fill-app-panel {STROKES[i % 2]}" stroke-width="2.5"><title>{fmt(c.p.date)}: {c.p.value} {unit}</title></circle>
        {/each}
      {/each}
      <text x={geo.x0} y={H - 8} font-size="11" text-anchor={geo.from === geo.to ? 'middle' : 'start'} class="fill-app-muted">{fmt(geo.from)}</text>
      {#if geo.from !== geo.to}<text x={geo.x1} y={H - 8} font-size="11" text-anchor="end" class="fill-app-muted">{fmt(geo.to)}</text>{/if}
      {#if unit}<text x={W - pad.r} y="11" font-size="11" text-anchor="end" class="fill-app-muted">{unit}</text>{/if}
    </svg>
    <details class="mt-1 text-sm">
      <summary class="cursor-pointer text-app-muted">Ver como tabla</summary>
      <table class="mt-2 w-full text-left text-sm">
        <thead><tr><th class="th">Fecha</th>{#each series as s}<th class="th">{s.label}</th>{/each}</tr></thead>
        <tbody>{#each [...dates].reverse() as d}<tr class="border-t border-app-ink/10"><td class="td">{fmt(d)}</td>{#each series as s}<td class="td">{at(s, d) ?? '—'}</td>{/each}</tr>{/each}</tbody>
      </table>
    </details>
  </figure>
{/if}
