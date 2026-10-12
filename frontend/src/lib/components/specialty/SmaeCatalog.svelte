<script lang="ts">
  import { searchSmae, smaeLine, type SmaeFood, type SmaeGroup } from '$lib/specialty/smae';
  import Modal from '../Modal.svelte';

  /** The catalog of equivalents: picking a food hands its text to `onpick` (the meal being written). */
  let { open, onclose, onpick, target = '' }: { open: boolean; onclose: () => void; onpick: (line: string) => void; target?: string } = $props();

  let q = $state('');
  const found = $derived(searchSmae(q));
  const pick = (food: SmaeFood, group: SmaeGroup) => onpick(smaeLine(food, group));
</script>

<Modal {open} title="Equivalentes SMAE" {onclose}>
  <p class="text-sm text-app-muted">Cada alimento aparece con la porción que equivale a un equivalente de su grupo.{target ? ` Al elegirlo se agrega a: ${target}.` : ' Haz clic en una comida del menú para elegir a dónde se agrega.'} Valores de referencia: confirma con la edición vigente del SMAE.</p>
  <label class="label mt-3" for="smae-q">Buscar alimento o grupo</label>
  <input id="smae-q" class="field" type="search" bind:value={q} placeholder="Tortilla, pollo, fruta…" />
  <div class="mt-3 max-h-[55vh] overflow-y-auto pr-1">
    {#each found as { group, foods } (group.id)}
      <section class="mb-4">
        <h4 class="text-sm font-semibold">{group.name}</h4>
        <p class="text-xs text-app-muted">1 equivalente: {group.kcal} kcal · proteína {group.protein} g · hidratos {group.carb} g · grasa {group.fat} g</p>
        <ul class="mt-1.5 flex flex-wrap gap-1.5">
          {#each foods as food (food.name)}
            <li><button type="button" class="rounded-full border border-app-ink/15 px-3 py-1.5 text-xs hover:border-app-primary hover:bg-app-primary/8" onclick={() => pick(food, group)}>{food.name} · {food.portion}</button></li>
          {/each}
        </ul>
      </section>
    {:else}
      <p class="text-sm text-app-muted">Sin resultados.</p>
    {/each}
  </div>
</Modal>
