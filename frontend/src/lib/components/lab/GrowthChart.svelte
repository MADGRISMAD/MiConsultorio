<script lang="ts">
  import type { GrowthIndicator } from '$lib/types/lab';
  import { dateFmt } from './labUtil';

  let { ind, label }: { ind: GrowthIndicator; label: string } = $props();

  const W = 640;
  const H = 300;
  const pad = { l: 50, r: 40, t: 16, b: 34 };
  const CURVES = ['p3', 'p15', 'p50', 'p85', 'p97'] as const;

  const ageMode = $derived(ind.points.length > 0 && ind.points.every((p) => p.age_months != null));
  const fmtAge = (m: number) => (m < 24 ? `${Math.round(m)} m` : `${(m / 12) % 1 === 0 ? m / 12 : (m / 12).toFixed(1)} a`);
  const fmtVal = (v: number) => (Math.abs(v) >= 100 ? v.toFixed(0) : v.toFixed(1));

  const geo = $derived.by(() => {
    if (!ind.points.length) return null;
    const x = (p: { age_months: number | null; date: string }) => (ageMode ? (p.age_months as number) : new Date(`${p.date}T12:00:00`).getTime() / 2.6298e9);
    const xs = ind.points.map(x);
    let lo = Math.min(...xs);
    let hi = Math.max(...xs);
    const span = Math.max(hi - lo, ageMode ? 12 : 1);
    lo -= ageMode ? Math.min(lo, span * 0.15) : span * 0.1;
    hi += span * 0.15;
    // Curves are drawn only where the patient's data is, one neighbour past each end so the lines reach the frame.
    const curves = ageMode && ind.has_reference
      ? CURVES.map((k) => {
          const all = ind.curves[k] ?? [];
          const inside = all.filter((c) => c.age_months >= lo && c.age_months <= hi);
          const before = all.filter((c) => c.age_months < lo).at(-1);
          const after = all.find((c) => c.age_months > hi);
          return { key: k, pts: [...(before ? [before] : []), ...inside, ...(after ? [after] : [])] };
        }).filter((c) => c.pts.length > 0)
      : [];
    const vals = [...ind.points.map((p) => p.value), ...curves.flatMap((c) => c.pts.filter((p) => p.age_months >= lo && p.age_months <= hi).map((p) => p.value))];
    let vlo = Math.min(...vals);
    let vhi = Math.max(...vals);
    if (vhi === vlo) {
      vlo -= 1;
      vhi += 1;
    }
    const m = (vhi - vlo) * 0.08;
    vlo -= m;
    vhi += m;
    const px = (v: number) => pad.l + ((v - lo) / (hi - lo)) * (W - pad.l - pad.r);
    const py = (v: number) => pad.t + (1 - (v - vlo) / (vhi - vlo)) * (H - pad.t - pad.b);
    const yTicks = [0, 0.25, 0.5, 0.75, 1].map((f) => ({ v: vlo + (vhi - vlo) * f, y: py(vlo + (vhi - vlo) * f) }));
    const xTicks = ageMode ? [0, 0.25, 0.5, 0.75, 1].map((f) => ({ v: lo + (hi - lo) * f, x: px(lo + (hi - lo) * f) })) : [];
    return {
      pts: ind.points.map((p, i) => ({ x: px(xs[i]), y: py(p.value), p })),
      curves: curves.map((c) => ({ key: c.key, d: c.pts.map((q) => `${Math.min(W - pad.r, Math.max(pad.l, px(q.age_months)))},${Math.min(H - pad.b, Math.max(pad.t, py(q.value)))}`).join(' '), end: c.pts.at(-1) ? { x: Math.min(W - pad.r, px(c.pts.at(-1)!.age_months)), y: Math.min(H - pad.b, Math.max(pad.t, py(c.pts.at(-1)!.value))) } : null })),
      yTicks,
      xTicks
    };
  });
  const last = $derived(ind.points.at(-1));
</script>

{#if geo && last}
  <figure class="m-0">
    <svg viewBox="0 0 {W} {H}" class="h-auto w-full" role="img" aria-label="{label}: {ind.points.length} mediciones, la última {last.value} {ind.unit}{last.percentile != null ? `, percentil ${last.percentile}` : ''}.{ind.has_reference ? ' Incluye curvas de percentiles de referencia.' : ''}">
      {#each geo.yTicks as k}
        <line x1={pad.l} x2={W - pad.r} y1={k.y} y2={k.y} class="stroke-app-ink/10" />
        <text x={pad.l - 6} y={k.y + 4} text-anchor="end" font-size="11" class="fill-app-muted">{fmtVal(k.v)}</text>
      {/each}
      {#each geo.xTicks as k}
        <text x={k.x} y={H - 12} text-anchor="middle" font-size="11" class="fill-app-muted">{fmtAge(k.v)}</text>
      {/each}
      {#each geo.curves as c (c.key)}
        <polyline fill="none" points={c.d} stroke-linejoin="round" stroke-width={c.key === 'p50' ? 2 : 1.4} stroke-dasharray={c.key === 'p50' ? undefined : '5 4'} class="stroke-app-accent" opacity={c.key === 'p50' ? 0.9 : 0.6} />
        {#if c.end}<text x={c.end.x + 4} y={c.end.y + 4} font-size="10" class="fill-app-muted">{c.key.toUpperCase()}</text>{/if}
      {/each}
      {#if geo.pts.length > 1}
        <polyline fill="none" class="stroke-app-primary" stroke-width="2.5" stroke-linejoin="round" points={geo.pts.map((c) => `${c.x},${c.y}`).join(' ')} />
      {/if}
      {#each geo.pts as c}
        <circle cx={c.x} cy={c.y} r="5" class="fill-app-panel stroke-app-primary" stroke-width="2.5">
          <title>{dateFmt(c.p.date)}{c.p.age_months != null ? ` · ${fmtAge(c.p.age_months)}` : ''}: {c.p.value} {ind.unit}{c.p.percentile != null ? ` · percentil ${c.p.percentile} (Z ${c.p.z})` : ''}</title>
        </circle>
      {/each}
      {#if !ageMode}
        <text x={geo.pts[0].x} y={H - 12} font-size="11" text-anchor="start" class="fill-app-muted">{dateFmt(ind.points[0].date)}</text>
        {#if geo.pts.length > 1}<text x={geo.pts.at(-1)!.x} y={H - 12} font-size="11" text-anchor="end" class="fill-app-muted">{dateFmt(last.date)}</text>{/if}
      {/if}
      <text x={pad.l} y="11" font-size="11" class="fill-app-muted">{ind.unit}</text>
      {#if ageMode}<text x={W - pad.r} y={H - 1} font-size="10" text-anchor="end" class="fill-app-muted">edad (m = meses, a = años)</text>{/if}
    </svg>
  </figure>
{/if}
