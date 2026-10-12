<script lang="ts">
  import { specialtyApi } from '$lib/api/specialty';
  import { Op } from '$lib/op.svelte';
  import { printShopping } from '$lib/print';
  import type { Patient } from '$lib/types';
  import type { NutritionDay } from '$lib/types/specialty';
  import Modal from '../Modal.svelte';
  import OpError from '../ui/OpError.svelte';
  import Icon from '../ui/Icon.svelte';

  /** The week's menu added up into what to buy, grouped by section of the supermarket. */
  let { open, patient, days, onclose }: { open: boolean; patient: Patient; days: NutritionDay[]; onclose: () => void } = $props();

  const op = new Op();
  let persons = $state(1);
  let sections = $state<{ name: string; items: { name: string; quantity: string }[] }[]>([]);
  let bought = $state<Record<string, boolean>>({});

  async function make() {
    let got: typeof sections = [];
    if (await op.run(async () => (got = await specialtyApi.nutritionShopping(patient.id, { days, persons })))) {
      sections = got;
      bought = {};
    }
  }
</script>

<Modal {open} title="Lista del súper" {onclose}>
  <p class="text-sm text-app-muted">La IA suma los alimentos del menú de la semana en lo que hay que comprar. Usa <strong>1 uso de magia</strong> (se devuelve si falla). Revísala antes de dársela al paciente.</p>
  <div class="mt-3 flex flex-wrap items-end gap-3">
    <label class="block"><span class="label">Personas que comen este menú</span><input class="field w-24" type="number" min="1" max="12" bind:value={persons} /></label>
    <button type="button" class="btn-primary" disabled={op.phase === 'loading'} onclick={make}><Icon name="sparkles" size={18} />{sections.length ? 'Volver a armar' : 'Armar la lista'}</button>
    {#if sections.length}<button type="button" class="btn-secondary" onclick={() => printShopping(patient, sections, persons)}><Icon name="receipt" size={18} />Imprimir</button>{/if}
  </div>
  <OpError {op} class="mt-3" />
  {#if op.phase === 'loading'}<p class="mt-3 text-sm text-app-muted" role="status">Armando la lista…</p>{/if}
  <div class="mt-4 max-h-[50vh] overflow-y-auto pr-1">
    {#each sections as sec, si (sec.name)}
      <section class="mb-4">
        <h4 class="text-sm font-semibold">{sec.name}</h4>
        <ul class="mt-1 divide-y divide-app-ink/8">
          {#each sec.items as it, ii (it.name)}
            <li><label class="flex cursor-pointer items-center gap-3 py-2 text-sm"><input type="checkbox" class="h-4 w-4" bind:checked={bought[`${si}-${ii}`]} /><span class="min-w-0 flex-1 {bought[`${si}-${ii}`] ? 'text-app-muted line-through' : ''}">{it.name}</span><span class="text-app-muted">{it.quantity}</span></label></li>
          {/each}
        </ul>
      </section>
    {/each}
  </div>
</Modal>
