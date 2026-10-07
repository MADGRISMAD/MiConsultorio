<script lang="ts">
  import type { CatalogMed, DoseResult } from '$lib/types/rx';
  import Icon from '../ui/Icon.svelte';

  interface Props {
    med: CatalogMed;
    /** kg, as typed by the prescriber */
    weight: string;
    onapply: (r: DoseResult) => void;
    idPrefix: string;
  }
  let { med, weight, onapply, idPrefix }: Props = $props();

  const concs = $derived(med.concentrations ?? []);
  let mgPerKg = $state('');
  let concKey = $state('0');
  let manualConc = $state('');
  let perDay = $state('3');

  // Start from the catalog value; the prescriber can change every number.
  $effect(() => {
    mgPerKg = med.mg_per_kg ? String(med.mg_per_kg) : '';
    concKey = concs.length ? '0' : 'manual';
    manualConc = '';
  });

  const num = (s: string) => {
    const n = parseFloat(s.replace(',', '.'));
    return Number.isFinite(n) ? n : 0;
  };
  const fmt = (n: number, d = 2) => String(parseFloat(n.toFixed(d))).replace('.', ',');

  const w = $derived(num(weight));
  const kg = $derived(num(mgPerKg));
  const n = $derived(Math.round(num(perDay)));
  const mgDose = $derived(w * kg);
  const conc = $derived(concKey === 'manual' ? num(manualConc) : (concs[Number(concKey)]?.mg_per_ml ?? 0));
  const mlDose = $derived(conc > 0 ? Math.round((mgDose / conc) * 10) / 10 : 0);
  const mgDay = $derived(mgDose * n);
  const maxDay = $derived((med.max_mg_per_kg_day ?? 0) * w);
  const over = $derived(maxDay > 0 && mgDay > maxDay * 1.0000001);
  const ready = $derived(w > 0 && kg > 0 && n >= 1 && n <= 24);
  const hours = $derived(n > 0 ? 24 / n : 0);

  function apply() {
    const dose = conc > 0 ? `${fmt(mlDose, 1)} mL (${fmt(mgDose)} mg)` : `${fmt(mgDose)} mg`;
    const frequency = Number.isInteger(hours) ? `Cada ${hours} horas` : `${n} veces al día`;
    onapply({ dose, frequency, dose_mg: Math.round(mgDose * 1000) / 1000, doses_per_day: n });
  }
</script>

<div class="rounded-xl border border-app-primary/25 bg-app-primary/5 p-4" role="group" aria-label="Calculadora de dosis por peso">
  <p class="text-sm font-medium">Dosis por peso: {med.name}</p>
  <p class="hint">Es una ayuda de cálculo: revisa cada paso y confirma tú la dosis. No se agrega nada a la receta hasta que la uses.</p>
  <div class="mt-3 grid gap-3 sm:grid-cols-4">
    <div>
      <label class="label" for="{idPrefix}-kg">mg por kg (por toma)</label>
      <input id="{idPrefix}-kg" class="field" inputmode="decimal" bind:value={mgPerKg} />
    </div>
    <div class="sm:col-span-2">
      <label class="label" for="{idPrefix}-conc">Concentración</label>
      {#if concs.length}
        <select id="{idPrefix}-conc" class="field" bind:value={concKey}>
          {#each concs as c, i}<option value={String(i)}>{c.label}</option>{/each}
          <option value="manual">Otra (escribir mg/mL)</option>
        </select>
      {:else}
        <input id="{idPrefix}-conc" class="field" inputmode="decimal" placeholder="mg por mL (opcional)" bind:value={manualConc} oninput={() => (concKey = 'manual')} />
      {/if}
      {#if concKey === 'manual' && concs.length}
        <input class="field mt-2" inputmode="decimal" aria-label="mg por mL" placeholder="mg por mL" bind:value={manualConc} />
      {/if}
    </div>
    <div>
      <label class="label" for="{idPrefix}-n">Tomas al día</label>
      <input id="{idPrefix}-n" class="field" type="number" min="1" max="24" bind:value={perDay} />
    </div>
  </div>

  {#if !(w > 0)}
    <p class="hint mt-3">Captura el peso del paciente arriba para calcular.</p>
  {:else if ready}
    <ol class="mt-4 space-y-1.5 text-sm" aria-live="polite">
      <li>1. Peso: <strong>{fmt(w)} kg</strong></li>
      <li>2. mg por toma = {fmt(w)} kg × {fmt(kg, 3)} mg/kg = <strong>{fmt(mgDose)} mg</strong></li>
      {#if conc > 0}
        <li>3. mL por toma = {fmt(mgDose)} mg ÷ {fmt(conc, 3)} mg/mL = {fmt(mgDose / conc, 3)} mL, redondeado a 0,1 mL = <strong>{fmt(mlDose, 1)} mL</strong></li>
      {:else}
        <li class="text-app-muted">3. Sin concentración: se indicará solo en mg.</li>
      {/if}
      <li>4. Total al día = {fmt(mgDose)} mg × {n} tomas = <strong>{fmt(mgDay)} mg</strong> (cada {fmt(hours, 1)} h)</li>
      {#if maxDay > 0}
        <li class={over ? 'font-medium text-app-danger' : 'text-app-muted'}>
          5. Máximo de referencia: {fmt(med.max_mg_per_kg_day ?? 0, 3)} mg/kg/día × {fmt(w)} kg = {fmt(maxDay)} mg al día.
          {#if over}<Icon name="alert" size={14} class="inline" /> La dosis diaria lo supera.{/if}
        </li>
      {/if}
    </ol>
    <button type="button" class="btn-primary mt-4" onclick={apply}><Icon name="check" size={16} />Usar este cálculo en la receta</button>
  {/if}
</div>
