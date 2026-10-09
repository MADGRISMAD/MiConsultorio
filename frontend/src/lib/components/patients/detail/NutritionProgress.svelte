<script lang="ts">
  import { api } from '$lib/api';
  import { BODY_MEASURES, bmiLabel, bmiOf, latestMeasures, measureRows, numOf } from '$lib/nutritionMeasures';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { Encounter, Patient } from '$lib/types';
  import Icon from '../../ui/Icon.svelte';

  let { patient, canWrite, encounters, onsaved }: { patient: Patient; canWrite: boolean; encounters: Encounter[]; onsaved: () => void | Promise<void> } = $props();

  const rows = $derived(measureRows(encounters));
  const dt = (iso: string) => new Date(iso).toLocaleDateString('es-MX', { day: 'numeric', month: 'short', year: 'numeric' });
  const fmt = (v: unknown) => (numOf(v) > 0 ? String(numOf(v)) : '—');
  /** height of a row, or the last known one before it */
  const heightAt = (i: number) => {
    for (let j = i; j >= 0; j--) if (numOf(rows[j].measures?.height_cm) > 0) return numOf(rows[j].measures.height_cm);
    return numOf(latestMeasures(encounters).height_cm);
  };
  const bmiAt = (i: number) => bmiOf(numOf(rows[i].measures?.weight_kg), heightAt(i));
  /** change against the previous row that has the same measure */
  function delta(i: number, key: string): number | null {
    const cur = numOf(rows[i].measures?.[key]);
    if (!cur) return null;
    for (let j = i - 1; j >= 0; j--) {
      const prev = numOf(rows[j].measures?.[key]);
      if (prev) return Math.round((cur - prev) * 10) / 10;
    }
    return null;
  }
  const first = $derived(rows[0]);
  const last = $derived(rows[rows.length - 1]);
  const totalWeight = $derived(first && last && first !== last ? Math.round((numOf(last.measures.weight_kg) - numOf(first.measures.weight_kg)) * 10) / 10 : null);

  let open = $state(false);
  let vals = $state<Record<string, string>>({});
  let note = $state('');
  let error = $state('');
  const op = new Op();

  function start(prefill: Record<string, string> = {}) {
    vals = { ...prefill };
    note = '';
    error = '';
    open = true;
  }
  const bmiNow = $derived(bmiOf(numOf(vals.weight_kg), numOf(vals.height_cm) || numOf(latestMeasures(encounters).height_cm)));

  async function save() {
    error = '';
    const measures: Record<string, number> = {};
    for (const m of BODY_MEASURES) if (numOf(vals[m.key]) > 0) measures[m.key] = numOf(vals[m.key]);
    if (!measures.weight_kg) {
      error = 'Escribe el peso para registrar la medición.';
      return;
    }
    if (await op.run(() => api.patients.createEncounter(patient.id, { kind: 'seguimiento', reason: 'Medición de seguimiento', subjective: '', measures, exam: '', assessment: '', diagnosis_codes: [], plan: '', notes: note.trim(), private: false }))) {
      toast.show('Medición registrada en el avance');
      open = false;
      await onsaved();
    }
  }
</script>

<section class="card mb-4 p-4 sm:p-5" aria-labelledby="np-progress">
  <div class="flex flex-wrap items-baseline justify-between gap-2">
    <h3 id="np-progress" class="display text-xl">Avance del paciente</h3>
    {#if canWrite && !open}<button type="button" class="btn-secondary" onclick={() => start(rows.length ? {} : latestMeasures(encounters))}><Icon name="plus" size={18} />Registrar medición</button>{/if}
  </div>

  {#if open}
    <form class="mt-3 rounded-xl border border-app-ink/10 p-3 sm:p-4" onsubmit={(e) => { e.preventDefault(); save(); }}>
      <p class="text-sm text-app-muted">Mide al paciente hoy. Los datos que no tengas déjalos vacíos: con más datos (sobre todo el % de grasa) el cálculo del menú es más preciso.</p>
      <div class="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-3">
        {#each BODY_MEASURES as m}
          <div><label class="label" for="pm-{m.key}">{m.label}{m.unit ? ` (${m.unit})` : ''}</label>
            <input id="pm-{m.key}" class="field" inputmode="decimal" bind:value={vals[m.key]} /></div>
        {/each}
      </div>
      {#if bmiNow}<p class="hint">Índice de masa corporal: <strong>{bmiNow}</strong> ({bmiLabel(bmiNow)}).</p>{/if}
      <label class="label mt-3" for="pm-note">Observaciones (opcional)</label>
      <input id="pm-note" class="field" maxlength="500" bind:value={note} />
      {#if error}<p class="alert mt-3" role="alert"><Icon name="alert" size={18} />{error}</p>{/if}
      {#if op.phase === 'error'}<p class="alert mt-3" role="alert"><Icon name="alert" size={18} />{op.message}</p>{/if}
      <div class="mt-3 flex flex-wrap gap-2">
        <button type="submit" class="btn-primary" disabled={op.phase === 'loading'}>{#if op.phase === 'loading'}<span class="spin"></span>{/if}<Icon name="check" size={18} />Guardar medición</button>
        <button type="button" class="btn-ghost" onclick={() => (open = false)}>Cancelar</button>
      </div>
    </form>
  {/if}

  {#if rows.length === 0}
    <p class="mt-3 text-sm text-app-muted">Todavía no hay mediciones. {canWrite ? 'Registra la primera para ver aquí la evolución del peso, la cintura y la grasa.' : ''}</p>
  {:else}
    {#if totalWeight !== null}
      <p class="mt-2 text-sm text-app-muted">Desde el {dt(first.occurred_at)}: <strong class="{totalWeight <= 0 ? 'text-app-success' : 'text-app-warning'}">{totalWeight > 0 ? '+' : ''}{totalWeight} kg</strong> de peso.</p>
    {/if}
    <div class="mt-3 overflow-x-auto rounded-xl border border-app-ink/10">
      <table class="w-full min-w-[46rem] border-collapse text-sm">
        <caption class="sr-only">Mediciones del paciente por fecha</caption>
        <thead>
          <tr class="bg-app-elevated text-xs uppercase tracking-wide">
            <th scope="col" class="px-2 py-2 text-left">Fecha</th>
            {#each BODY_MEASURES as m}
              <th scope="col" class="px-2 py-2 text-right">{m.short}{m.unit ? ` (${m.unit})` : ''}{#if m.key === 'height_cm'}<span class="sr-only"> y</span>{/if}</th>
              {#if m.key === 'height_cm'}<th scope="col" class="px-2 py-2 text-right">IMC</th>{/if}
            {/each}
          </tr>
        </thead>
        <tbody>
          {#each rows as r, i (r.id)}
            {@const b = bmiAt(i)}
            <tr class="border-t border-app-ink/10">
              <th scope="row" class="whitespace-nowrap px-2 py-2 text-left font-medium">{dt(r.occurred_at)}</th>
              {#each BODY_MEASURES as m}
                {@const d = delta(i, m.key)}
                <td class="px-2 py-2 text-right tabular-nums">
                  {fmt(r.measures?.[m.key])}
                  {#if d}<span class="block text-[11px] {(m.lowerIsBetter ? d < 0 : d > 0) ? 'text-app-success' : 'text-app-muted'}">{d > 0 ? '+' : ''}{d}</span>{/if}
                </td>
                {#if m.key === 'height_cm'}<td class="px-2 py-2 text-right tabular-nums" title={b ? bmiLabel(b) : ''}>{b ?? '—'}</td>{/if}
              {/each}
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</section>
