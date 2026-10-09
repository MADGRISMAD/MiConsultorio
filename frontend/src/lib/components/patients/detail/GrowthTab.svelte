<script lang="ts">
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onMount } from 'svelte';
  import { growthApi } from '$lib/api/lab';
  import type { Patient, PatientSchema } from '$lib/types';
  import type { GrowthIndicatorKey, PatientGrowth } from '$lib/types/lab';
  import GrowthChart from '../../lab/GrowthChart.svelte';
  import { dateFmt } from '../../lab/labUtil';
  import EmptyState from '../../ui/EmptyState.svelte';
  import Icon from '../../ui/Icon.svelte';

  let { patient }: { patient: Patient; schema: PatientSchema | null; canWrite: boolean; isAdmin: boolean } = $props();

  let data = $state<PatientGrowth | null>(null);
  let loading = $state(true);
  let error = $state('');
  let standard = $state('');

  async function load(std = '') {
    try {
      data = await growthApi.patient(patient.id, std);
      standard = data.standard;
      error = '';
    } catch (e) {
      error = e instanceof Error ? e.message : 'No se pudieron cargar las mediciones.';
    } finally {
      loading = false;
    }
  }
  onMount(() => load());

  const LABEL: Record<GrowthIndicatorKey, string> = {
    weight_for_age: 'Peso para la edad',
    length_height_for_age: 'Talla para la edad',
    bmi_for_age: 'IMC para la edad',
    head_circumference_for_age: 'Perímetro cefálico para la edad'
  };
  const ORDER: GrowthIndicatorKey[] = ['weight_for_age', 'length_height_for_age', 'bmi_for_age', 'head_circumference_for_age'];
  const shown = $derived(ORDER.map((k) => data?.indicators[k]).filter((x) => x && x.points.length > 0));
  // Without references the indicators come empty: build the series from the measurements.
  const raw = $derived.by(() => {
    if (!data || data.reference_status === 'ok') return [];
    const m = data.measurements;
    const mk = (key: GrowthIndicatorKey, unit: string, pick: (x: (typeof m)[number]) => number | null) => ({
      indicator: key,
      unit,
      has_reference: false,
      curves: {},
      ref_min_age_months: null,
      ref_max_age_months: null,
      points: m.flatMap((x) => (pick(x) == null ? [] : [{ date: x.date, age_months: x.age_months, value: pick(x) as number, z: null, percentile: null }]))
    });
    const all = [mk('weight_for_age', 'kg', (x) => x.weight_kg), ...(data.subject === 'person' ? [mk('length_height_for_age', 'cm', (x) => x.height_cm), mk('bmi_for_age', 'kg/m²', (x) => x.bmi), mk('head_circumference_for_age', 'cm', (x) => x.head_cm)] : [])];
    return all.filter((s) => s.points.length > 0);
  });
  const series = $derived(data?.reference_status === 'ok' ? (shown as NonNullable<(typeof shown)[number]>[]) : raw);
  const rawLabel = (k: GrowthIndicatorKey) => (data?.subject === 'animal' && k === 'weight_for_age' ? 'Peso' : data?.reference_status === 'ok' ? LABEL[k] : LABEL[k].replace(' para la edad', ''));

  const fmtAge = (m: number | null) => (m == null ? '—' : m < 24 ? `${Math.round(m)} meses` : `${Math.floor(m / 12)} a ${Math.round(m % 12)} m`);
  const rows = $derived([...(data?.measurements ?? [])].reverse());
  const src = $derived(data?.standards.find((s) => s.standard === standard));
  const lastOf = (k: GrowthIndicatorKey) => data?.indicators[k]?.points.at(-1);
</script>

{#if loading}
  <div class="card h-48 animate-pulse"></div>
{:else if error}
  <Alert>{error}</Alert>
{:else if data}
  {#if data.measurements.length === 0}
    <div class="card"><EmptyState icon="activity" title="Sin mediciones todavía" text="Las gráficas se arman con el peso, la talla y el perímetro cefálico que captures en las consultas de la bitácora." /></div>
  {:else}
    {#if data.reference_status === 'no_references'}
      <p class="mb-4 flex items-start gap-2 rounded-xl bg-app-warning/10 px-3.5 py-3 text-sm"><Icon name="info" size={18} />Carga las tablas oficiales de la OMS/CDC en Ajustes para ver percentiles. Mientras tanto se muestran solo las mediciones del paciente.</p>
    {:else if data.reference_status === 'no_birth_date'}
      <p class="mb-4 flex items-start gap-2 rounded-xl bg-app-warning/10 px-3.5 py-3 text-sm"><Icon name="info" size={18} />Falta la fecha de nacimiento: sin ella no se puede calcular la edad ni ubicar las mediciones en las curvas.</p>
    {:else if data.reference_status === 'no_sex'}
      <p class="mb-4 flex items-start gap-2 rounded-xl bg-app-warning/10 px-3.5 py-3 text-sm"><Icon name="info" size={18} />Las curvas de referencia dependen del sexo (Mujer u Hombre) del paciente: complétalo en sus datos para verlas.</p>
    {:else if data.reference_status === 'animal'}
      <p class="mb-4 flex items-start gap-2 rounded-xl bg-app-ink/5 px-3.5 py-3 text-sm text-app-muted"><Icon name="info" size={18} />Las tablas OMS/CDC son para personas; para animales se muestra la evolución del peso sin curvas de referencia.</p>
    {:else if data.standards.length > 0}
      <div class="mb-4 flex flex-wrap items-end gap-3">
        <div>
          <label class="label" for="gr-std">Estándar de referencia</label>
          <select id="gr-std" class="field" bind:value={standard} onchange={() => load(standard)} disabled={data.standards.length < 2}>
            {#each data.standards as s (s.standard)}<option value={s.standard}>{s.standard} (versión {s.version})</option>{/each}
          </select>
        </div>
        {#if src}<p class="max-w-xl text-xs text-app-muted">Fuente cargada por el consultorio: {src.source_name}. Curvas P3, P15, P50, P85 y P97.</p>{/if}
      </div>
    {/if}

    <div class="grid gap-4 lg:grid-cols-2">
      {#each series as s (s.indicator)}
        {@const lp = s.points.at(-1)}
        <section class="card p-4 sm:p-5" aria-labelledby="gr-{s.indicator}">
          <div class="mb-2 flex flex-wrap items-baseline justify-between gap-2">
            <h3 id="gr-{s.indicator}" class="text-base font-medium">{rawLabel(s.indicator)}</h3>
            {#if lp}
              <p class="text-sm text-app-muted">
                Último: <strong class="text-app-ink">{lp.value} {s.unit}</strong>
                {#if lp.percentile != null}· percentil <strong class="text-app-ink">{lp.percentile}</strong> · Z {lp.z}{:else if data.reference_status === 'ok' && s.has_reference}· fuera del rango de edad de la tabla{/if}
              </p>
            {/if}
          </div>
          <GrowthChart ind={s} label={rawLabel(s.indicator)} />
        </section>
      {/each}
    </div>

    <section class="card mt-4 p-4 sm:p-5" aria-labelledby="gr-table">
      <h3 id="gr-table" class="section-title mb-2">Mediciones</h3>
      <div class="overflow-x-auto">
        <table class="w-full min-w-[34rem] text-left text-sm">
          <thead>
            <tr><th class="th !px-2">Fecha</th><th class="th !px-2">Edad</th><th class="th !px-2">Peso (kg)</th>{#if data.subject === 'person'}<th class="th !px-2">Talla (cm)</th><th class="th !px-2">IMC</th><th class="th !px-2">P. cefálico (cm)</th>{/if}</tr>
          </thead>
          <tbody>
            {#each rows as m (m.encounter_id)}
              <tr class="border-t border-app-ink/10">
                <td class="td !px-2 !py-2">{dateFmt(m.date)}</td>
                <td class="td !px-2 !py-2">{fmtAge(m.age_months)}</td>
                <td class="td !px-2 !py-2">{m.weight_kg ?? '—'}</td>
                {#if data.subject === 'person'}
                  <td class="td !px-2 !py-2">{m.height_cm ?? '—'}</td>
                  <td class="td !px-2 !py-2">{m.bmi ?? '—'}</td>
                  <td class="td !px-2 !py-2">{m.head_cm ?? '—'}</td>
                {/if}
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
      <p class="mt-3 text-xs text-app-muted">El percentil y el valor Z orientan la valoración: la interpretación clínica es del profesional. Las mediciones salen de la bitácora.</p>
    </section>
  {/if}
{/if}
