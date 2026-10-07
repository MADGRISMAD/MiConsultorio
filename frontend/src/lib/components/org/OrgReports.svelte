<script lang="ts">
  import { orgApi } from '$lib/api/org';
  import { moneyCents } from '$lib/format';
  import type { OrgBranchReport, OrgSummary } from '$lib/types/org';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import Donut from '$lib/components/reports/Donut.svelte';
  import HBars from '$lib/components/reports/HBars.svelte';
  import Line from '$lib/components/reports/Line.svelte';
  import RangeBar from '$lib/components/reports/RangeBar.svelte';
  import { MAX_RANGE_DAYS, rangeDays, rangeFor, type RangePreset } from '$lib/components/reports/range';

  interface Props {
    /** '' = every branch, otherwise one branch id (the report always covers the whole organization on the server) */
    branch?: string;
  }
  let { branch = '' }: Props = $props();

  let preset = $state<RangePreset>('30d');
  let from = $state(rangeFor('30d').from);
  let to = $state(rangeFor('30d').to);
  let report = $state<OrgSummary | null>(null);
  let loading = $state(true);
  let error = $state('');
  let seq = 0;

  $effect(() => {
    const f = from;
    const t = to;
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
    orgApi
      .summary(f, t)
      .then(
        (r) => {
          if (my === seq) report = r;
        },
        (e) => {
          if (my === seq) error = e instanceof Error ? e.message : 'No se pudo cargar el reporte.';
        }
      )
      .finally(() => {
        if (my === seq) loading = false;
      });
  });

  const STATUS: Record<string, string> = {
    scheduled: 'Agendadas', confirmed: 'Confirmadas', arrived: 'En sala', in_progress: 'En consulta',
    completed: 'Atendidas', no_show: 'No asistió', cancelled: 'Canceladas'
  };
  const pct = (v: number) => `${v.toLocaleString('es-MX', { maximumFractionDigits: 1 })}%`;

  const selected = $derived<OrgBranchReport | null>(report ? (branch ? (report.branches.find((b) => b.id === branch) ?? null) : report.total) : null);
  const salesOn = $derived(!!report?.total.sales.enabled);
  const shown = $derived(report ? (branch ? report.branches.filter((b) => b.id === branch) : report.branches) : []);
  const slices = $derived(
    selected ? Object.entries(selected.appointments.by_status).map(([k, v]) => ({ label: STATUS[k] ?? k, value: v })).filter((s) => s.value > 0) : []
  );
  const dayLabel = (d: string) => new Date(d + 'T12:00:00').toLocaleDateString('es-MX', { day: 'numeric', month: 'short' });
  const cards = $derived(
    selected
      ? [
          ...(salesOn ? [{ label: 'Ventas', value: moneyCents(selected.sales.total_cents), sub: `${selected.sales.count} cobros` }, { label: 'Ticket promedio', value: moneyCents(selected.sales.avg_ticket_cents), sub: 'Por cobro' }] : []),
          { label: 'Citas', value: String(selected.appointments.total), sub: `${pct(selected.appointments.cancellation_rate)} canceladas` },
          { label: 'Ausentismo', value: pct(selected.appointments.no_show_rate), sub: `${selected.appointments.no_show} de ${selected.appointments.eligible} citas pasadas` },
          { label: 'Consultas', value: String(selected.consultations), sub: 'Registradas en expediente' },
          { label: 'Pacientes nuevos', value: String(selected.new_patients), sub: 'Altas en el periodo' }
        ]
      : []
  );
</script>

<div class="mb-6 flex flex-wrap items-end justify-between gap-3">
  <div class="min-w-0 max-w-full"><RangeBar bind:from bind:to bind:preset /></div>
  <a href={orgApi.csvUrl(from, to)} download class="btn-secondary"><Icon name="download" size={18} />Exportar CSV</a>
</div>

{#if error}<p class="alert mb-4" role="alert"><Icon name="alert" size={18} />{error}</p>{/if}

{#if loading && !report}
  <div class="card"><LoadingRows /></div>
{:else if report && selected}
  <div class="grid min-w-0 grid-cols-[minmax(0,1fr)] gap-4 transition-opacity {loading ? 'opacity-60' : ''}" aria-busy={loading}>
    <div class="grid grid-cols-2 gap-3 lg:grid-cols-3 xl:grid-cols-6">
      {#each cards as k}
        <div class="card p-4">
          <p class="section-title">{k.label}</p>
          <p class="display mt-1.5 text-[1.5rem] leading-tight">{k.value}</p>
          <p class="mt-0.5 text-xs text-app-muted">{k.sub}</p>
        </div>
      {/each}
    </div>

    {#if !salesOn}
      <p class="text-sm text-app-muted">El plan actual no incluye cobros, por eso las sucursales aparecen sin ventas.</p>
    {/if}

    {#if shown.length > 1 || !branch}
      <section class="card p-5">
        <h2 class="display text-2xl">Comparativo entre sucursales</h2>
        <div class="mt-4 grid gap-8 md:grid-cols-2">
          {#if salesOn}
            <div>
              <h3 class="section-title mb-3">Ventas</h3>
              <HBars rows={report.branches.map((b) => ({ label: b.name, value: b.sales.total_cents, text: moneyCents(b.sales.total_cents), sub: b.suspended ? 'de baja' : '' }))} />
            </div>
            <div>
              <h3 class="section-title mb-3">Ticket promedio</h3>
              <HBars rows={report.branches.map((b) => ({ label: b.name, value: b.sales.avg_ticket_cents, text: moneyCents(b.sales.avg_ticket_cents) }))} />
            </div>
          {/if}
          <div>
            <h3 class="section-title mb-3">Citas</h3>
            <HBars rows={report.branches.map((b) => ({ label: b.name, value: b.appointments.total, text: String(b.appointments.total) }))} />
          </div>
          <div>
            <h3 class="section-title mb-3">Ausentismo</h3>
            <HBars tone="warning" rows={report.branches.map((b) => ({ label: b.name, value: b.appointments.no_show_rate, text: pct(b.appointments.no_show_rate), sub: `${b.appointments.no_show} de ${b.appointments.eligible}` }))} />
          </div>
          <div>
            <h3 class="section-title mb-3">Consultas</h3>
            <HBars rows={report.branches.map((b) => ({ label: b.name, value: b.consultations, text: String(b.consultations) }))} />
          </div>
          <div>
            <h3 class="section-title mb-3">Pacientes nuevos</h3>
            <HBars rows={report.branches.map((b) => ({ label: b.name, value: b.new_patients, text: String(b.new_patients) }))} />
          </div>
        </div>
      </section>
    {/if}

    <div class="grid gap-4 lg:grid-cols-2">
      <section class="card p-5">
        <h2 class="display text-2xl">{salesOn ? 'Ventas por día' : 'Citas por día'}</h2>
        <p class="text-xs text-app-muted">Toda la organización</p>
        <div class="mt-3">
          {#if report.daily.length > 1}
            <Line
              ariaLabel={salesOn ? 'Ventas por día de toda la organización' : 'Citas por día de toda la organización'}
              points={report.daily.map((d) => ({ label: dayLabel(d.day), value: salesOn ? d.sales_cents / 100 : d.appointments }))}
            />
          {:else}
            <p class="text-sm text-app-muted">Elige un periodo de más de un día para ver la gráfica.</p>
          {/if}
        </div>
      </section>
      <section class="card p-5">
        <h2 class="display text-2xl">Citas por estado</h2>
        <p class="text-xs text-app-muted">{branch ? selected.name : 'Todas las sucursales'}</p>
        <div class="mt-3">
          {#if slices.length}<Donut {slices} ariaLabel="Citas por estado" />{:else}<p class="text-sm text-app-muted">Sin citas en este periodo.</p>{/if}
        </div>
      </section>
    </div>

    <section class="card overflow-hidden">
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <caption class="sr-only">Resumen por sucursal</caption>
          <thead>
            <tr>
              <th class="th text-left" scope="col">Sucursal</th>
              {#if salesOn}<th class="th text-right" scope="col">Ventas</th><th class="th text-right" scope="col">Ticket</th>{/if}
              <th class="th text-right" scope="col">Citas</th>
              <th class="th text-right" scope="col">Ausentismo</th>
              <th class="th text-right" scope="col">Consultas</th>
              <th class="th text-right" scope="col">Nuevos</th>
            </tr>
          </thead>
          <tbody>
            {#each shown as b (b.id)}
              <tr>
                <th scope="row" class="td text-left font-medium">{b.name}{#if b.suspended}<span class="ml-2 text-xs font-normal text-app-muted">de baja</span>{/if}</th>
                {#if salesOn}<td class="td text-right">{moneyCents(b.sales.total_cents)}</td><td class="td text-right">{moneyCents(b.sales.avg_ticket_cents)}</td>{/if}
                <td class="td text-right">{b.appointments.total}</td>
                <td class="td text-right">{pct(b.appointments.no_show_rate)}</td>
                <td class="td text-right">{b.consultations}</td>
                <td class="td text-right">{b.new_patients}</td>
              </tr>
            {/each}
            {#if !branch}
              <tr class="font-semibold">
                <th scope="row" class="td text-left">Total</th>
                {#if salesOn}<td class="td text-right">{moneyCents(report.total.sales.total_cents)}</td><td class="td text-right">{moneyCents(report.total.sales.avg_ticket_cents)}</td>{/if}
                <td class="td text-right">{report.total.appointments.total}</td>
                <td class="td text-right">{pct(report.total.appointments.no_show_rate)}</td>
                <td class="td text-right">{report.total.consultations}</td>
                <td class="td text-right">{report.total.new_patients}</td>
              </tr>
            {/if}
          </tbody>
        </table>
      </div>
    </section>
  </div>
{:else if !loading}
  <div class="card"><EmptyState icon="chart" title="Sin datos" text="No hay datos para esta sucursal en el periodo." /></div>
{/if}
