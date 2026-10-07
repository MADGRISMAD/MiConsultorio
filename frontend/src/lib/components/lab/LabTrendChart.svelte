<script lang="ts">
  import type { LabTrendPoint } from '$lib/types/lab';
  import { dateFmt, FLAG_LABEL, FLAG_MARK, rangeText } from './labUtil';

  let { points, analyte, unit }: { points: LabTrendPoint[]; analyte: string; unit: string } = $props();

  const W = 640;
  const H = 240;
  const pad = { l: 52, r: 16, t: 16, b: 32 };

  // The band is the range printed with the latest result; older results keep their own range in the table.
  const last = $derived(points[points.length - 1]);
  const geo = $derived.by(() => {
    if (!points.length) return null;
    const vals = points.map((p) => p.value);
    const low = last.ref_low;
    const high = last.ref_high;
    let lo = Math.min(...vals, ...(low != null ? [low] : []));
    let hi = Math.max(...vals, ...(high != null ? [high] : []));
    if (hi === lo) {
      lo -= 1;
      hi += 1;
    }
    const margin = (hi - lo) * 0.12;
    lo -= margin;
    hi += margin;
    const ts = points.map((p) => new Date(p.resulted_at).getTime());
    const t0 = Math.min(...ts);
    const t1 = Math.max(...ts);
    const x = (t: number) => (t1 === t0 ? (pad.l + W - pad.r) / 2 : pad.l + ((t - t0) / (t1 - t0)) * (W - pad.l - pad.r));
    const y = (v: number) => pad.t + (1 - (v - lo) / (hi - lo)) * (H - pad.t - pad.b);
    const coords = points.map((p, i) => ({ x: x(ts[i]), y: y(p.value), p }));
    const ticks = [0, 0.5, 1].map((f) => ({ v: lo + (hi - lo) * f, y: y(lo + (hi - lo) * f) }));
    const band = low != null || high != null ? { top: high != null ? y(high) : pad.t, bottom: low != null ? y(low) : H - pad.b } : null;
    return { coords, ticks, band, t0, t1 };
  });
  const fmtTick = (v: number) => (Math.abs(v) >= 100 ? v.toFixed(0) : Math.abs(v) >= 10 ? v.toFixed(1) : v.toFixed(2));
  const dotClass = (f: string) => (f === 'critico' ? 'fill-app-danger stroke-app-danger' : f === 'alto' || f === 'bajo' || f === 'anormal' ? 'fill-app-warning stroke-app-warning' : 'fill-app-panel stroke-app-primary');
</script>

{#if geo}
  <figure class="m-0">
    <svg viewBox="0 0 {W} {H}" class="h-auto w-full" role="img" aria-label="Tendencia de {analyte}: {points.length} resultados de {dateFmt(points[0].resulted_at)} a {dateFmt(last.resulted_at)}. Último valor {last.value} {unit}.">
      {#if geo.band}
        <rect x={pad.l} y={geo.band.top} width={W - pad.l - pad.r} height={Math.max(0, geo.band.bottom - geo.band.top)} class="fill-app-accent/15" />
      {/if}
      {#each geo.ticks as k}
        <line x1={pad.l} x2={W - pad.r} y1={k.y} y2={k.y} class="stroke-app-ink/10" />
        <text x={pad.l - 6} y={k.y + 4} text-anchor="end" font-size="11" class="fill-app-muted">{fmtTick(k.v)}</text>
      {/each}
      {#if geo.coords.length > 1}
        <polyline fill="none" class="stroke-app-primary" stroke-width="2" stroke-linejoin="round" points={geo.coords.map((c) => `${c.x},${c.y}`).join(' ')} />
      {/if}
      {#each geo.coords as c}
        <circle cx={c.x} cy={c.y} r="5" class={dotClass(c.p.flag)} stroke-width="2.5">
          <title>{dateFmt(c.p.resulted_at)}: {c.p.value} {c.p.unit} ({FLAG_LABEL[c.p.flag]}){rangeText(c.p.ref_low, c.p.ref_high) ? ` · referencia ${rangeText(c.p.ref_low, c.p.ref_high)}` : ''}</title>
        </circle>
        {#if FLAG_MARK[c.p.flag]}<text x={c.x} y={c.y - 9} text-anchor="middle" font-size="12" font-weight="700" class="fill-app-ink">{FLAG_MARK[c.p.flag]}</text>{/if}
      {/each}
      <text x={geo.coords[0].x} y={H - 10} font-size="11" text-anchor={geo.coords.length > 1 ? 'start' : 'middle'} class="fill-app-muted">{dateFmt(points[0].resulted_at)}</text>
      {#if geo.coords.length > 1}<text x={geo.coords[geo.coords.length - 1].x} y={H - 10} font-size="11" text-anchor="end" class="fill-app-muted">{dateFmt(last.resulted_at)}</text>{/if}
      {#if unit}<text x={W - pad.r} y="11" font-size="11" text-anchor="end" class="fill-app-muted">{unit}</text>{/if}
    </svg>
    {#if geo.band}
      <p class="mt-1 flex items-center gap-2 text-xs text-app-muted"><span class="inline-block h-3 w-5 rounded-sm bg-app-accent/25"></span>Rango de referencia del último resultado: {rangeText(last.ref_low, last.ref_high)} {unit}</p>
    {:else}
      <p class="mt-1 text-xs text-app-muted">Estos resultados no traen rango de referencia.</p>
    {/if}
    <details class="mt-2 text-sm">
      <summary class="cursor-pointer text-app-muted">Ver resultados como tabla</summary>
      <div class="mt-2 overflow-x-auto">
        <table class="w-full text-left text-sm">
          <thead><tr><th class="th">Fecha</th><th class="th">Valor</th><th class="th">Referencia</th><th class="th">Laboratorio</th></tr></thead>
          <tbody>
            {#each [...points].reverse() as p (p.id)}
              <tr class="border-t border-app-ink/10">
                <td class="td">{dateFmt(p.resulted_at)}</td>
                <td class="td font-medium">{p.value} {p.unit} <span class="text-app-muted">{FLAG_MARK[p.flag]} {p.flag === 'na' || p.flag === 'normal' ? '' : FLAG_LABEL[p.flag]}</span></td>
                <td class="td">{rangeText(p.ref_low, p.ref_high) || '—'}</td>
                <td class="td">{p.lab_name || '—'}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </details>
  </figure>
{/if}
