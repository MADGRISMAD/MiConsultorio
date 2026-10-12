<script lang="ts">
  import { specialtyApi } from '$lib/api/specialty';
  import { Op } from '$lib/op.svelte';
  import { printShopping } from '$lib/print';
  import type { Patient } from '$lib/types';
  import type { NutritionDay, ShoppingSection } from '$lib/types/specialty';
  import Modal from '../Modal.svelte';
  import OpError from '../ui/OpError.svelte';
  import Icon from '../ui/Icon.svelte';

  /**
   * The week's groceries. The list comes with the menu (the same AI use) and follows it when a meal is changed with the AI.
   * Only a plan made by hand has none: then it can be made here, for one more use.
   */
  let { open, patient, days, list, persons, canMake, onmade, onclose }: {
    open: boolean;
    patient: Patient;
    days: NutritionDay[];
    list: ShoppingSection[];
    persons: number;
    canMake: boolean;
    onmade: (list: ShoppingSection[], persons: number) => void;
    onclose: () => void;
  } = $props();

  const op = new Op();
  let people = $state(1);
  $effect(() => {
    people = persons || 1;
  });
  let bought = $state<Record<string, boolean>>({});

  async function make() {
    let got: ShoppingSection[] = [];
    if (await op.run(async () => (got = await specialtyApi.nutritionShopping(patient.id, { days, persons: Math.min(12, Math.max(1, Math.round(people) || 1)) })))) {
      bought = {};
      onmade(got, people);
    }
  }
</script>

<Modal {open} title="Lista del súper" {onclose}>
  {#if list.length}
    <p class="text-sm text-app-muted">Para la semana del menú{persons > 1 ? `, ${persons} personas` : ''}. Se arma junto con el menú y se actualiza sola cuando cambias una comida con IA (sin gastar otro uso). Si editas las comidas a mano, revisa la lista. Se guarda con el plan.</p>
    <div class="mt-3 flex flex-wrap gap-2">
      <button type="button" class="btn-secondary" onclick={() => printShopping(patient, list, persons)}><Icon name="receipt" size={18} />Imprimir</button>
    </div>
    <div class="mt-4 max-h-[50vh] overflow-y-auto pr-1">
      {#each list as sec, si (sec.name)}
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
  {:else}
    <p class="text-sm text-app-muted">Este plan no tiene lista del súper (los planes que genera la IA la traen incluida). Puedes armarla con lo que escribiste en el menú: usa <strong>1 uso de magia</strong> y se devuelve si falla.</p>
    {#if canMake}
      <div class="mt-3 flex flex-wrap items-end gap-3">
        <label class="block"><span class="label">Personas que comen este menú</span><input class="field w-24" type="number" min="1" max="12" bind:value={people} /></label>
        <button type="button" class="btn-primary" disabled={op.phase === 'loading'} onclick={make}><Icon name="sparkles" size={18} />Armar la lista</button>
      </div>
      <OpError {op} class="mt-3" />
      {#if op.phase === 'loading'}<p class="mt-3 text-sm text-app-muted" role="status">Armando la lista…</p>{/if}
    {/if}
  {/if}
</Modal>
