<script lang="ts">
  import type { Encounter, Patient } from '$lib/types';
  import MetricChart, { type Series } from '../../specialty/MetricChart.svelte';
  import Icon from '../../ui/Icon.svelte';

  let { patient, encounters, only }: { patient: Patient; encounters: Encounter[]; only?: string[] } = $props();

  interface Metric {
    key: string;
    title: string;
    unit: string;
    decimals?: number;
    series: { label: string; get: (e: Encounter) => number | null }[];
  }
  const n = (v: unknown) => {
    const x = Number(String(v ?? '').replace(',', '.'));
    return Number.isFinite(x) && x > 0 ? x : null;
  };
  const m = (k: string) => (e: Encounter) => n(e.measures?.[k]);
  const METRICS: Metric[] = [
    { key: 'weight', title: 'Peso', unit: 'kg', series: [{ label: 'Peso', get: m('weight_kg') }] },
    {
      key: 'bmi',
      title: 'Índice de masa corporal',
      unit: '',
      series: [{ label: 'IMC', get: (e) => {
        const w = n(e.measures?.weight_kg);
        const h = n(e.measures?.height_cm) ?? lastHeight;
        return w && h ? Math.round((w / (h / 100) ** 2) * 10) / 10 : null;
      } }]
    },
    { key: 'bp', title: 'Presión arterial', unit: 'mmHg', decimals: 0, series: [{ label: 'Sistólica', get: m('bp_sys') }, { label: 'Diastólica', get: m('bp_dia') }] },
    { key: 'glucose', title: 'Glucosa', unit: 'mg/dL', decimals: 0, series: [{ label: 'Glucosa', get: m('glucose') }] },
    { key: 'hr', title: 'Frecuencia cardiaca', unit: 'lpm', decimals: 0, series: [{ label: 'FC', get: m('heart_rate') }] },
    { key: 'waist', title: 'Cintura', unit: 'cm', series: [{ label: 'Cintura', get: m('waist_cm') }] },
    { key: 'fat', title: 'Grasa corporal', unit: '%', series: [{ label: '% grasa', get: m('body_fat') }] },
    { key: 'muscle', title: 'Masa muscular', unit: 'kg', series: [{ label: 'Músculo', get: m('muscle_kg') }] },
    { key: 'eva', title: 'Dolor (escala EVA 0 a 10)', unit: '', decimals: 0, series: [{ label: 'EVA', get: (e) => { const v = Number(e.measures?.pain_eva); return Number.isFinite(v) && e.measures?.pain_eva !== '' && e.measures?.pain_eva != null ? v : null; } }] },
    { key: 'rom', title: 'Rango de movimiento', unit: '°', decimals: 0, series: [{ label: 'Grados', get: m('rom_deg') }] },
    { key: 'fundal', title: 'Altura del fondo uterino', unit: 'cm', series: [{ label: 'AFU', get: m('fundal_height') }] },
    { key: 'temp', title: 'Temperatura', unit: '°C', series: [{ label: '°C', get: m('temp_c') }] },
    { key: 'spo2', title: 'Saturación de oxígeno', unit: '%', decimals: 0, series: [{ label: 'SpO₂', get: m('spo2') }] }
  ];

  const ordered = $derived([...encounters].filter((e) => !e.hidden && !e.addendum_of).sort((a, b) => a.occurred_at.localeCompare(b.occurred_at)));
  const lastHeight = $derived.by(() => {
    for (let i = ordered.length - 1; i >= 0; i--) {
      const h = n(ordered[i].measures?.height_cm);
      if (h) return h;
    }
    return null;
  });

  /** only the measures with at least two readings are worth a line */
  const charts = $derived(
    METRICS.filter((mt) => !only || only.includes(mt.key))
      .map((mt) => ({
        mt,
        series: mt.series.map((s): Series => ({
          label: s.label,
          points: ordered.flatMap((e) => {
            const v = s.get(e);
            return v == null ? [] : [{ date: e.occurred_at.slice(0, 10), value: v }];
          })
        }))
      }))
      .filter((c) => c.series.some((s) => s.points.length >= 2))
  );
</script>

{#if charts.length}
  <section class="card p-5 sm:p-6" aria-label="Evolución">
    <h2 class="display mb-4 flex items-center gap-2 text-2xl"><Icon name="activity" size={20} />Evolución</h2>
    <div class="grid gap-6 lg:grid-cols-2">
      {#each charts as c (c.mt.key)}
        <MetricChart title={c.mt.title} unit={c.mt.unit} decimals={c.mt.decimals ?? 1} series={c.series} />
      {/each}
    </div>
  </section>
{/if}
