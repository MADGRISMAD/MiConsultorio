<script lang="ts">
  import { reportsApi } from '$lib/api/reports';
  import { dateShort } from '$lib/format';
  import type { PatientsReport, RptContact } from '$lib/types/reports';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import Bars from './Bars.svelte';
  import Donut from './Donut.svelte';
  import HBars from './HBars.svelte';
  import RangeBar from './RangeBar.svelte';
  import { MAX_RANGE_DAYS, rangeDays, rangeFor, type RangePreset } from './range';

  let preset = $state<RangePreset>('90d');
  let from = $state(rangeFor('90d').from);
  let to = $state(rangeFor('90d').to);
  let inactiveDays = $state(180);
  let page = $state(1);

  let report = $state<PatientsReport | null>(null);
  let loading = $state(true);
  let error = $state('');
  let seq = 0;

  // changing the range or the threshold goes back to the first page
  $effect(() => {
    void from;
    void to;
    void inactiveDays;
    page = 1;
  });

  $effect(() => {
    const f = from;
    const t = to;
    const days = inactiveDays;
    const pg = page;
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
    reportsApi.patients(f, t, days, pg).then(
      (r) => {
        if (my === seq) report = r;
      },
      (e) => {
        if (my === seq) error = e instanceof Error ? e.message : 'No se pudo cargar el reporte.';
      }
    ).finally(() => {
      if (my === seq) loading = false;
    });
  });

  const monthLabel = (m: string) => new Date(m + '-15T12:00:00').toLocaleDateString('es-MX', { month: 'short', year: '2-digit' });
  const pages = $derived(report ? Math.max(1, Math.ceil(report.inactive.total / report.inactive.limit)) : 1);
  const hasVisits = $derived((report?.visits.by_month.length ?? 0) > 0);
  const phoneHref = (p: string) => 'tel:' + p.replace(/[^\d+]/g, '');
</script>

{#snippet contactCell(c: RptContact)}
  <div class="text-sm">{c.contact_name}</div>
  <div class="flex flex-wrap gap-x-3 text-xs text-app-muted">
    {#if c.contact_phone}<a class="hover:text-app-primary hover:underline" href={phoneHref(c.contact_phone)}>{c.contact_phone}</a>{/if}
    {#if c.contact_email}<a class="hover:text-app-primary hover:underline" href="mailto:{c.contact_email}">{c.contact_email}</a>{/if}
    {#if !c.contact_phone && !c.contact_email}Sin datos de contacto{/if}
  </div>
{/snippet}

<div class="mb-6 flex flex-wrap items-end justify-between gap-3">
  <RangeBar bind:from bind:to bind:preset />
  <a href={reportsApi.patientsCsvUrl(from, to, inactiveDays)} download class="btn-secondary"><Icon name="download" size={18} />Exportar CSV</a>
</div>

{#if error}
  <p class="alert mb-4" role="alert"><Icon name="alert" size={18} />{error}</p>
{/if}

{#if loading && !report}
  <div class="card"><LoadingRows /></div>
{:else if report}
  <div class="grid gap-4 transition-opacity {loading ? 'opacity-60' : ''}" aria-busy={loading}>
    <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
      {#each [
        { label: 'Pacientes atendidos', value: report.visits.patients, sub: 'Con consulta en el periodo' },
        { label: 'Con 2 o más consultas', value: report.visits.repeat, sub: 'Pacientes recurrentes en el periodo' },
        { label: 'Altas nuevas', value: report.signups.total, sub: `${report.signups.person} personas · ${report.signups.animal} animales` },
        { label: `Sin visita en ${report.inactive.days} días`, value: report.inactive.total, sub: 'Para invitarlos a volver' }
      ] as k}
        <div class="card p-4">
          <p class="section-title">{k.label}</p>
          <p class="display mt-1.5 text-[1.75rem] leading-tight">{k.value}</p>
          <p class="mt-0.5 text-xs text-app-muted">{k.sub}</p>
        </div>
      {/each}
    </div>

    <div class="grid gap-4 lg:grid-cols-2">
      <section class="card p-5">
        <h2 class="display text-2xl">Nuevos y recurrentes por mes</h2>
        <p class="mt-1 text-sm text-app-muted">Nuevo: su primera consulta fue ese mes. Recurrente: ya tenía consultas antes.</p>
        {#if !hasVisits}
          <p class="mt-3 text-sm text-app-muted">Sin consultas en este periodo.</p>
        {:else}
          <div class="mt-4 overflow-x-auto">
            <table class="w-full">
              <thead><tr><th class="th pl-0">Mes</th><th class="th text-right">Nuevos</th><th class="th pr-0 text-right">Recurrentes</th></tr></thead>
              <tbody class="divide-y divide-app-ink/10">
                {#each report.visits.by_month as m (m.month)}
                  <tr><td class="td pl-0 capitalize">{monthLabel(m.month)}</td><td class="td text-right font-medium">{m.new}</td><td class="td pr-0 text-right font-medium">{m.returning}</td></tr>
                {/each}
              </tbody>
            </table>
          </div>
          <div class="mt-4"><Donut ariaLabel="Pacientes nuevos y recurrentes" slices={[{ label: 'Nuevos', value: report.visits.new_total }, { label: 'Recurrentes', value: report.visits.returning_total }]} /></div>
        {/if}
      </section>

      <section class="card p-5">
        <h2 class="display text-2xl">Altas por mes</h2>
        {#if !report.signups.by_month.length}
          <p class="mt-3 text-sm text-app-muted">No se registraron pacientes en este periodo.</p>
        {:else}
          <div class="mt-4">
            <Bars ariaLabel="Altas de pacientes por mes" items={report.signups.by_month.map((m) => ({ label: monthLabel(m.month), value: m.person + m.animal, text: `${m.person + m.animal} (${m.person} personas, ${m.animal} animales)` }))} />
          </div>
        {/if}
      </section>
    </div>

    <section class="card p-5">
      <h2 class="display text-2xl">Motivos de consulta más frecuentes</h2>
      <p class="mt-1 text-sm text-app-muted">Se agrupan sin importar mayúsculas ni acentos. No se incluyen notas privadas.</p>
      {#if !report.reasons.length}
        <p class="mt-3 text-sm text-app-muted">Aún no hay motivos de consulta capturados en este periodo.</p>
      {:else}
        <div class="mt-4"><HBars rows={report.reasons.map((r) => ({ label: r.label, value: r.count }))} /></div>
      {/if}
    </section>

    <section class="card p-5">
      <div class="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 class="display text-2xl">Pacientes que no han vuelto</h2>
          <p class="mt-1 text-sm text-app-muted">Sin visita desde hace al menos el tiempo elegido (los que nunca han venido cuentan desde su alta). Contacto del tutor o responsable.</p>
        </div>
        <div class="flex items-end gap-2">
          <div>
            <label class="label" for="rpt-inactive">Sin visita en</label>
            <select id="rpt-inactive" class="field w-auto" bind:value={inactiveDays}>
              {#each [60, 90, 180, 365, 730] as d}<option value={d}>{d} días</option>{/each}
            </select>
          </div>
          <a href={reportsApi.inactiveCsvUrl(inactiveDays)} download class="btn-secondary"><Icon name="download" size={18} />Lista CSV</a>
        </div>
      </div>
      {#if !report.inactive.items.length}
        <EmptyState icon="heart" title="Nadie se ha ausentado" text="Todos tus pacientes activos han venido dentro de este plazo." />
      {:else}
        <div class="mt-3 overflow-x-auto">
          <table class="w-full min-w-[620px]">
            <thead><tr><th class="th pl-0">Paciente</th><th class="th">Última visita</th><th class="th pr-0">Contacto</th></tr></thead>
            <tbody class="divide-y divide-app-ink/10">
              {#each report.inactive.items as c (c.id)}
                <tr>
                  <td class="td pl-0"><a href="/pacientes/{c.id}" class="font-medium hover:text-app-primary hover:underline">{c.name}</a><div class="text-xs text-app-muted">Exp. {c.file_number}</div></td>
                  <td class="td whitespace-nowrap">{c.last_visit ? dateShort(c.last_visit) : 'Nunca'}<div class="text-xs text-app-muted">{c.days_since ?? '—'} días</div></td>
                  <td class="td pr-0">{@render contactCell(c)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
        <nav class="mt-3 flex items-center justify-between gap-3 text-sm" aria-label="Paginación">
          <span class="text-app-muted">Página {report.inactive.page} de {pages} · {report.inactive.total} pacientes</span>
          <span class="flex gap-2">
            <button type="button" class="btn-secondary" disabled={page <= 1} onclick={() => (page -= 1)}>Anterior</button>
            <button type="button" class="btn-secondary" disabled={page >= pages} onclick={() => (page += 1)}>Siguiente</button>
          </span>
        </nav>
      {/if}
    </section>

    <section class="card p-5">
      <h2 class="display text-2xl">Vacunas y desparasitaciones pendientes</h2>
      <p class="mt-1 text-sm text-app-muted">Próxima dosis vencida o dentro de los próximos 30 días (se toma la última aplicación de cada una).</p>
      {#if !report.vaccines.items.length}
        <p class="mt-3 text-sm text-app-muted">No hay dosis pendientes.</p>
      {:else}
        <div class="mt-3 overflow-x-auto">
          <table class="w-full min-w-[620px]">
            <thead><tr><th class="th pl-0">Paciente</th><th class="th">Pendiente</th><th class="th">Fecha</th><th class="th pr-0">Contacto</th></tr></thead>
            <tbody class="divide-y divide-app-ink/10">
              {#each report.vaccines.items as v (v.id + v.vaccine)}
                <tr>
                  <td class="td pl-0"><a href="/pacientes/{v.id}" class="font-medium hover:text-app-primary hover:underline">{v.name}</a></td>
                  <td class="td">{v.vaccine}</td>
                  <td class="td whitespace-nowrap {v.overdue ? 'text-app-danger' : ''}">{dateShort(v.next_due)}<div class="text-xs">{v.overdue ? `Vencida hace ${v.days_late} días` : 'Próxima'}</div></td>
                  <td class="td pr-0">{@render contactCell(v)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
        {#if report.vaccines.total > report.vaccines.items.length}
          <p class="mt-2 text-xs text-app-muted">Se muestran las {report.vaccines.items.length} más próximas de {report.vaccines.total}.</p>
        {/if}
      {/if}
    </section>

    <div class="grid gap-4 lg:grid-cols-2">
      <section class="card p-5">
        <h2 class="display text-2xl">Edad y sexo (personas)</h2>
        {#if !report.people.total}
          <p class="mt-3 text-sm text-app-muted">Aún no hay pacientes personas activos.</p>
        {:else}
          <div class="mt-3 overflow-x-auto">
            <table class="w-full min-w-[360px]">
              <thead><tr><th class="th pl-0">Edad</th><th class="th text-right">Mujeres</th><th class="th text-right">Hombres</th><th class="th text-right">Otro / s.d.</th><th class="th pr-0 text-right">Total</th></tr></thead>
              <tbody class="divide-y divide-app-ink/10">
                {#each report.people.by_age.filter((a) => a.total > 0) as a (a.key)}
                  <tr><td class="td pl-0">{a.label}</td><td class="td text-right">{a.female}</td><td class="td text-right">{a.male}</td><td class="td text-right">{a.other + a.unknown}</td><td class="td pr-0 text-right font-medium">{a.total}</td></tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </section>
      <section class="card p-5">
        <h2 class="display text-2xl">Animales por especie</h2>
        {#if !report.animals.total}
          <p class="mt-3 text-sm text-app-muted">Aún no hay pacientes animales activos.</p>
        {:else}
          <div class="mt-4"><Donut ariaLabel="Animales por especie" slices={report.animals.by_species.map((s) => ({ label: s.label, value: s.count }))} /></div>
        {/if}
      </section>
    </div>
  </div>
{/if}
