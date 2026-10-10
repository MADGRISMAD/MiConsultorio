<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import PageHeader from '$lib/components/ui/PageHeader.svelte';
  import Stars from '$lib/components/ui/Stars.svelte';
  import Kpi from '$lib/components/reports/Kpi.svelte';
  import Line from '$lib/components/reports/Line.svelte';
  import RangeBar from '$lib/components/reports/RangeBar.svelte';
  import { MAX_RANGE_DAYS, rangeDays, rangeFor, type RangePreset } from '$lib/components/reports/range';
  import { indicatorsApi, type Indicators } from '$lib/api/reports';
  import { dateShort, money } from '$lib/format';

  let preset = $state<RangePreset>('30d');
  let from = $state(rangeFor('30d').from);
  let to = $state(rangeFor('30d').to);
  let data = $state<Indicators | null>(null);
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
    if (rangeDays(f, t) > MAX_RANGE_DAYS / 2) {
      error = 'El rango no puede ser mayor a 1 año, para poder compararlo con el periodo anterior.';
      return;
    }
    const my = ++seq;
    loading = true;
    error = '';
    indicatorsApi
      .get(f, t)
      .then((r) => my === seq && (data = r))
      .catch((e) => my === seq && (error = e instanceof Error ? e.message : 'No se pudieron cargar los indicadores.'))
      .finally(() => my === seq && (loading = false));
  });

  const pct = (v: number) => `${v.toFixed(1)}%`;
  const mins = (v: number | null) => (v == null ? '—' : `${Math.round(v)} min`);
  const c = $derived(data?.current);
  const p = $derived(data?.previous);
  const gran = { day: 'por día', week: 'por semana', month: 'por mes' };
  const label = (iso: string, g: string) => (g === 'month' ? new Date(`${iso}T12:00:00`).toLocaleDateString('es-MX', { month: 'short', year: '2-digit' }) : dateShort(iso));
</script>

<svelte:head><title>Indicadores · Caresia</title></svelte:head>

<Guard title="Indicadores" permissions={['adminUsers']}>
  <PageHeader title="Indicadores" subtitle="Cómo va tu consultorio frente al periodo anterior de la misma duración." />
  <div class="mb-6"><RangeBar bind:from bind:to bind:preset /></div>

  {#if error}
    <Alert>{error}</Alert>
  {:else if loading && !data}
    <LoadingRows />
  {:else if data && c && p}
    <p class="mb-3 text-xs text-app-muted">{dateShort(data.from)} – {dateShort(data.to)} · comparado con {dateShort(data.prev_from)} – {dateShort(data.prev_to)}</p>
    <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4 {loading ? 'opacity-60' : ''}">
      <Kpi label="Consultas realizadas" value={String(c.encounters)} now={c.encounters} before={p.encounters} />
      <Kpi label="Citas agendadas" value={String(c.appointments)} now={c.appointments} before={p.appointments} hint="{c.completed} atendidas" />
      <Kpi label="Pacientes nuevos" value={String(c.new_patients)} now={c.new_patients} before={p.new_patients} />
      <Kpi label="Pacientes que regresaron" value={String(c.returning_patients)} now={c.returning_patients} before={p.returning_patients} hint="Ya se habían atendido antes" />
      <Kpi label="Inasistencias" value={pct(c.no_show_rate)} now={c.no_show_rate} before={p.no_show_rate} unit="pts" lowerIsBetter />
      <Kpi label="Cancelaciones" value={pct(c.cancellation_rate)} now={c.cancellation_rate} before={p.cancellation_rate} unit="pts" lowerIsBetter />
      <Kpi label="Ocupación de la agenda" value={c.occupancy_rate == null ? '—' : pct(c.occupancy_rate)} now={c.occupancy_rate} before={p.occupancy_rate} unit="pts" />
      <Kpi label="Espera promedio" value={mins(c.avg_wait_min)} now={c.avg_wait_min} before={p.avg_wait_min} lowerIsBetter hint="Desde que llegan hasta que pasan" />
      {#if data.cobros}
        <Kpi label="Ingresos" value={money(c.revenue_cents / 100)} now={c.revenue_cents} before={p.revenue_cents} hint="{c.sales} venta{c.sales === 1 ? '' : 's'}" />
        <Kpi label="Ticket promedio" value={money(c.avg_ticket_cents / 100)} now={c.avg_ticket_cents} before={p.avg_ticket_cents} />
      {/if}
      <div class="card p-4">
        <p class="text-xs text-app-muted">Satisfacción</p>
        {#if c.satisfaction_responses > 0}
          <p class="display mt-1 text-3xl">{c.satisfaction_avg.toFixed(1)} <span class="align-middle"><Stars value={c.satisfaction_avg} size={16} /></span></p>
          <p class="mt-1 text-xs text-app-muted">{c.satisfaction_responses} respuesta{c.satisfaction_responses === 1 ? '' : 's'}{p.satisfaction_responses > 0 ? ` · antes ${p.satisfaction_avg.toFixed(1)}` : ''}</p>
        {:else}
          <p class="mt-1 text-sm text-app-muted">Sin respuestas todavía. Activa la encuesta en <a class="underline" href="/ajustes?s=perfil">Ajustes › Página pública y encuesta</a>.</p>
        {/if}
      </div>
    </div>

    <section class="card mt-6 p-5 sm:p-6">
      <h2 class="display text-2xl">Consultas {gran[data.granularity]}</h2>
      {#if data.trend.length}
        <div class="mt-4"><Line ariaLabel="Consultas {gran[data.granularity]}" points={data.trend.map((t) => ({ label: label(t.key, data!.granularity), value: t.count }))} /></div>
      {:else}
        <p class="mt-3 text-sm text-app-muted">No hubo consultas en este periodo.</p>
      {/if}
    </section>

    {#if data.professionals.length}
      <section class="card mt-6 overflow-x-auto p-5 sm:p-6">
        <h2 class="display mb-3 text-2xl">Por profesional</h2>
        <table class="w-full min-w-[32rem] text-sm">
          <thead class="text-left text-xs text-app-muted"><tr><th class="py-2 pr-3 font-medium">Profesional</th><th class="py-2 pr-3 font-medium">Consultas</th><th class="py-2 pr-3 font-medium">Citas</th><th class="py-2 pr-3 font-medium">Inasistencias</th><th class="py-2 font-medium">Duración media</th></tr></thead>
          <tbody>
            {#each data.professionals as r (r.id)}
              <tr class="border-t border-app-ink/8"><td class="py-2 pr-3 font-medium">{r.name || 'Sin asignar'}</td><td class="py-2 pr-3">{r.encounters}</td><td class="py-2 pr-3">{r.appointments}</td><td class="py-2 pr-3">{r.no_show}</td><td class="py-2">{mins(r.avg_attention_min)}</td></tr>
            {/each}
          </tbody>
        </table>
      </section>
    {/if}
  {/if}
</Guard>
