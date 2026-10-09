<script lang="ts">
  import Alert from '$lib/components/ui/Alert.svelte';
  import { reportsApi } from '$lib/api/reports';
  import type { OperationsReport, RptProfessional, RptRate } from '$lib/types/reports';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import Bars from './Bars.svelte';
  import Donut from './Donut.svelte';
  import HBars from './HBars.svelte';
  import Line from './Line.svelte';
  import RangeBar from './RangeBar.svelte';
  import { MAX_RANGE_DAYS, rangeDays, rangeFor, type RangePreset } from './range';

  let preset = $state<RangePreset>('30d');
  let from = $state(rangeFor('30d').from);
  let to = $state(rangeFor('30d').to);
  let professional = $state('');
  let known = $state<RptProfessional[]>([]);

  let report = $state<OperationsReport | null>(null);
  let loading = $state(true);
  let error = $state('');
  let seq = 0;

  $effect(() => {
    const f = from;
    const t = to;
    const pro = professional;
    if (!f || !t) return;
    if (f > t) {
      error = 'La fecha inicial no puede ser posterior a la final.';
      return;
    }
    if (rangeDays(f, t) > MAX_RANGE_DAYS) {
      error = 'El rango no puede ser mayor a 2 años.';
      return;
    }
    const my = ++seq;
    loading = true;
    error = '';
    reportsApi.operations(f, t, pro).then(
      (r) => {
        if (my !== seq) return;
        report = r;
        if (!pro && r.scope === 'clinic') known = r.professionals.filter((p) => p.id);
      },
      (e) => {
        if (my === seq) error = e instanceof Error ? e.message : 'No se pudo cargar el reporte.';
      }
    ).finally(() => {
      if (my === seq) loading = false;
    });
  });

  const csvUrl = $derived(reportsApi.operationsCsvUrl(from, to, professional));
  const fmtMin = (v: number | null) => (v === null ? '—' : `${v.toLocaleString('es-MX', { maximumFractionDigits: 1 })} min`);
  const fmtPct = (v: number | null) => (v === null ? '—' : `${v.toLocaleString('es-MX', { maximumFractionDigits: 1 })}%`);
  const periodLabel = (iso: string, g: string) => {
    const d = new Date(iso + 'T12:00:00');
    if (g === 'month') return d.toLocaleDateString('es-MX', { month: 'short', year: '2-digit' });
    return d.toLocaleDateString('es-MX', { day: 'numeric', month: 'short' });
  };
  const granLabel = { day: 'por día', week: 'por semana', month: 'por mes' };
  const noShowRows = (rows: RptRate[]) => rows.map((r) => ({ label: r.label, value: r.rate, text: `${fmtPct(r.rate)}`, sub: `${r.no_show} de ${r.total}` }));

  const topPeak = $derived.by(() => {
    const days = ['Lun', 'Mar', 'Mié', 'Jue', 'Vie', 'Sáb', 'Dom'];
    const out: { label: string; n: number }[] = [];
    (report?.peak_hours.heatmap ?? []).forEach((row, d) => row.forEach((n, h) => n > 0 && out.push({ label: `${days[d]} ${h}:00`, n })));
    return out.sort((a, b) => b.n - a.n).slice(0, 5);
  });
  const heatMax = $derived(Math.max(1, ...(report?.peak_hours.heatmap ?? []).flat()));
  const activeHours = $derived.by(() => {
    const hs = (report?.peak_hours.heatmap ?? []).flatMap((row) => row.map((n, h) => (n > 0 ? h : -1))).filter((h) => h >= 0);
    return hs.length ? { min: Math.min(...hs), max: Math.max(...hs) } : null;
  });
</script>

<div class="mb-6 flex flex-wrap items-end justify-between gap-3">
  <div class="flex min-w-0 max-w-full flex-wrap items-end gap-3">
    <RangeBar bind:from bind:to bind:preset />
    {#if report?.scope === 'clinic' && known.length > 1}
      <div>
        <label class="label" for="rpt-pro">Profesional</label>
        <select id="rpt-pro" class="field w-auto" bind:value={professional}>
          <option value="">Todo el consultorio</option>
          {#each known as p (p.id)}<option value={p.id}>{p.name}</option>{/each}
        </select>
      </div>
    {/if}
  </div>
  <a href={csvUrl} download class="btn-secondary"><Icon name="download" size={18} />Exportar CSV</a>
</div>

{#if report?.scope === 'own'}
  <p class="mb-4 text-sm text-app-muted">Estás viendo solo tus propias consultas y citas.</p>
{/if}

{#if error}
  <Alert class="mb-4">{error}</Alert>
{/if}

{#if loading && !report}
  <div class="card"><LoadingRows /></div>
{:else if report}
  <div class="grid grid-cols-1 gap-4 transition-opacity {loading ? 'opacity-60' : ''}" aria-busy={loading}>
    <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
      {#each [
        { label: 'Consultas', value: String(report.encounters.total), sub: 'Consulta, seguimiento y procedimiento' },
        { label: 'Citas', value: String(report.appointments.total), sub: `${fmtPct(report.appointments.cancellation_rate)} canceladas` },
        { label: 'Ausentismo', value: fmtPct(report.no_show.rate), sub: `${report.no_show.count} de ${report.no_show.eligible} citas pasadas` },
        { label: 'Ocupación de agenda', value: fmtPct(report.occupancy.rate), sub: 'Aproximada, según horarios y bloqueos' },
        { label: 'Atención promedio', value: fmtMin(report.times.avg_attention_min), sub: report.times.samples ? `${report.times.samples} citas atendidas` : 'Sin citas con hora de inicio y fin' },
        { label: 'Espera promedio', value: fmtMin(report.times.avg_wait_min), sub: 'De llegada a inicio de la consulta' }
      ] as k}
        <div class="card p-4">
          <p class="section-title">{k.label}</p>
          <p class="display mt-1.5 text-[1.75rem] leading-tight">{k.value}</p>
          <p class="mt-0.5 text-xs text-app-muted">{k.sub}</p>
        </div>
      {/each}
    </div>

    <section class="card p-5">
      <h2 class="display text-2xl">Consultas {granLabel[report.granularity]}</h2>
      {#if !report.encounters.by_period.length}
        <p class="mt-3 text-sm text-app-muted">Sin consultas en este periodo.</p>
      {:else if report.encounters.by_period.length > 3}
        <div class="mt-4">
          <Line ariaLabel="Consultas {granLabel[report.granularity]}" points={report.encounters.by_period.map((p) => ({ label: periodLabel(p.key, report!.granularity), value: p.count }))} />
        </div>
      {:else}
        <div class="mt-4">
          <Bars ariaLabel="Consultas {granLabel[report.granularity]}" items={report.encounters.by_period.map((p) => ({ label: periodLabel(p.key, report!.granularity), value: p.count }))} />
        </div>
      {/if}
    </section>

    <section class="card p-5">
      <h2 class="display text-2xl">Por profesional</h2>
      {#if !report.professionals.length}
        <p class="mt-3 text-sm text-app-muted">Sin actividad en este periodo.</p>
      {:else}
        <div class="mt-3 overflow-x-auto">
          <table class="w-full min-w-[620px]">
            <thead><tr><th class="th pl-0">Profesional</th><th class="th text-right">Consultas</th><th class="th text-right">Citas</th><th class="th text-right">No asistió</th><th class="th text-right">Atención</th><th class="th pr-0 text-right">Espera</th></tr></thead>
            <tbody class="divide-y divide-app-ink/10">
              {#each report.professionals as p (p.id)}
                <tr><td class="td pl-0">{p.name}</td><td class="td text-right font-medium">{p.encounters}</td><td class="td text-right">{p.appointments}</td><td class="td text-right">{p.no_show}</td><td class="td text-right">{fmtMin(p.avg_attention_min)}</td><td class="td pr-0 text-right">{fmtMin(p.avg_wait_min)}</td></tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </section>

    <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
      <section class="card p-5">
        <h2 class="display text-2xl">Citas por estado</h2>
        {#if !report.appointments.total}
          <p class="mt-3 text-sm text-app-muted">Sin citas en este periodo.</p>
        {:else}
          <div class="mt-4"><Donut ariaLabel="Citas por estado" slices={report.appointments.by_status.filter((s) => s.count > 0).map((s) => ({ label: s.label, value: s.count }))} /></div>
        {/if}
      </section>
      <section class="card p-5">
        <h2 class="display text-2xl">Origen de las citas</h2>
        {#if !report.appointments.total}
          <p class="mt-3 text-sm text-app-muted">Sin citas en este periodo.</p>
        {:else}
          <div class="mt-4"><Donut ariaLabel="Origen de las citas" slices={report.appointments.by_source.filter((s) => s.count > 0).map((s) => ({ label: s.label, value: s.count }))} /></div>
        {/if}
      </section>
    </div>

    <section class="card p-5">
      <h2 class="display text-2xl">Ausentismo</h2>
      <p class="mt-1 text-sm text-app-muted">Citas pasadas, sin contar las canceladas, que terminaron en “No asistió”.</p>
      {#if !report.no_show.eligible}
        <p class="mt-3 text-sm text-app-muted">Todavía no hay citas pasadas en este periodo.</p>
      {:else}
        <div class="mt-4 grid gap-6 lg:grid-cols-2">
          <div><h3 class="section-title mb-3">Por día de la semana</h3><HBars tone="warning" rows={noShowRows(report.no_show.by_weekday)} /></div>
          <div><h3 class="section-title mb-3">Por hora</h3><HBars tone="warning" rows={noShowRows(report.no_show.by_hour)} /></div>
          {#if report.no_show.by_professional.length > 1}
            <div><h3 class="section-title mb-3">Por profesional</h3><HBars tone="warning" rows={noShowRows(report.no_show.by_professional)} /></div>
          {/if}
          <div><h3 class="section-title mb-3">Por servicio</h3><HBars tone="warning" rows={noShowRows(report.no_show.by_service)} /></div>
        </div>
      {/if}
    </section>

    <section class="card p-5">
      <h2 class="display text-2xl">Horas pico</h2>
      {#if !topPeak.length}
        <p class="mt-3 text-sm text-app-muted">Sin citas en este periodo.</p>
      {:else}
        <p class="mt-1 text-sm text-app-muted">Más concurridas: {topPeak.map((p) => `${p.label} (${p.n})`).join(' · ')}</p>
        <div class="mt-4 overflow-x-auto">
          <table class="w-full min-w-[520px] border-separate border-spacing-1 text-center text-xs">
            <caption class="sr-only">Citas por día de la semana y hora de inicio</caption>
            <thead><tr><td></td>{#each ['Lun', 'Mar', 'Mié', 'Jue', 'Vie', 'Sáb', 'Dom'] as d}<th scope="col" class="font-medium text-app-muted">{d}</th>{/each}</tr></thead>
            <tbody>
              {#each Array.from({ length: (activeHours?.max ?? 0) - (activeHours?.min ?? 0) + 1 }, (_, i) => (activeHours?.min ?? 0) + i) as h}
                <tr>
                  <th scope="row" class="pr-2 text-right font-medium text-app-muted">{h}:00</th>
                  {#each report.peak_hours.heatmap as row, d}
                    {@const n = row[h]}
                    <td class="rounded-md py-1.5 {n ? 'text-app-ink' : 'text-app-muted/50'}" style="background: rgb(var(--app-primary) / {n ? 0.1 + (n / heatMax) * 0.6 : 0.04})" title="{n} citas">{n || '·'}</td>
                  {/each}
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </section>

    <section class="card p-5">
      <h2 class="display text-2xl">Ocupación de agenda</h2>
      {#if report.occupancy.available_min === 0}
        <EmptyState icon="calendar" title="Sin horario disponible" text="No hay horarios abiertos en este periodo. Revisa el horario del consultorio y el de cada profesional." />
      {:else}
        <p class="mt-3 text-sm">
          <span class="font-medium">{Math.round(report.occupancy.booked_min / 60)} h</span> agendadas de
          <span class="font-medium">{Math.round(report.occupancy.available_min / 60)} h</span> disponibles ({fmtPct(report.occupancy.rate)}).
        </p>
        <p class="mt-1 text-xs text-app-muted">Aproximado: horas disponibles según el horario de cada profesional (o del consultorio) menos bloqueos, durante todo el periodo, incluidos los días que aún no llegan.</p>
      {/if}
    </section>

    <p class="text-sm text-app-muted">Los reportes de ventas y cobros están en <a href="/pos/reportes" class="font-medium text-app-primary hover:underline">Cobros · Reportes</a>.</p>
  </div>
{/if}
