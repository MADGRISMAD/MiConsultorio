<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import { labApi } from '$lib/api/lab';
  import { Op } from '$lib/op.svelte';
  import type { Patient } from '$lib/types';
  import type { LabCatalog, LabOrder } from '$lib/types/lab';
  import Modal from '../Modal.svelte';
  import Icon from '../ui/Icon.svelte';

  interface Props {
    open: boolean;
    patient: Patient;
    catalog: LabCatalog | null;
    onclose: () => void;
    /** the order saved; `print` when the professional asked to print the sheet right away */
    onsaved: (o: LabOrder, print: boolean) => void;
  }
  let { open, patient, catalog, onclose, onsaved }: Props = $props();

  interface Asked {
    name: string;
    /** what a catalog panel includes, so the request says exactly what is being asked */
    includes: string[];
  }
  const op = new Op();
  let title = $state('');
  let labName = $state('');
  let notes = $state('');
  let asked = $state<Asked[]>([]);
  let panelPick = $state('');
  let custom = $state('');

  const animal = $derived(patient.subject === 'animal');
  const species = $derived(String(patient.profile?.species ?? patient.profile?.especie ?? ''));
  const panels = $derived(
    (catalog?.panels ?? []).filter((p) => (animal ? p.audience === 'animal' && (!species || !p.species || p.species === species || !['Perro', 'Gato'].includes(species)) : p.audience === 'person'))
  );

  $effect(() => {
    if (!open) return;
    op.reset();
    title = '';
    labName = '';
    notes = '';
    asked = [];
    panelPick = '';
    custom = '';
  });

  function addPanel() {
    const p = panels.find((x) => x.id === panelPick);
    if (p && !asked.some((a) => a.name === p.name)) asked.push({ name: p.name, includes: p.analytes.map((a) => a.name) });
    panelPick = '';
  }
  function addCustom() {
    const name = custom.trim();
    if (name && !asked.some((a) => a.name.toLowerCase() === name.toLowerCase())) asked.push({ name, includes: [] });
    custom = '';
  }
  const remove = (name: string) => (asked = asked.filter((a) => a.name !== name));

  async function save(print: boolean) {
    if (asked.length === 0) return op.fail('Agrega al menos un estudio que vas a solicitar.');
    const finalTitle = title.trim() || (asked.length === 1 ? asked[0].name : 'Estudios de laboratorio');
    let saved: LabOrder | undefined;
    const ok = await op.run(async () => {
      saved = await labApi.createOrder(patient.id, { title: finalTitle, lab_name: labName.trim(), notes: notes.trim(), requested: asked.map((a) => a.name) });
    });
    if (ok && saved) onsaved(saved, print);
  }
</script>

<Modal {open} title="Nueva orden de laboratorio" {onclose} wide>
  <form id="lab-request" onsubmit={(e) => { e.preventDefault(); void save(false); }} class="grid gap-4">
    <p class="text-sm text-app-muted">Aquí solo solicitas los estudios y entregas la hoja al paciente. Cuando traiga sus resultados, los capturas o los escaneas dentro de la orden.</p>

    <div class="grid items-end gap-3 sm:grid-cols-[1fr_auto]">
      <div>
        <label class="label" for="lr-panel">Estudios que solicitas</label>
        <select id="lr-panel" class="field" bind:value={panelPick} onchange={addPanel}>
          <option value="">Elige un panel para agregarlo…</option>
          {#each panels as p (p.id)}<option value={p.id} disabled={asked.some((a) => a.name === p.name)}>{p.name}</option>{/each}
        </select>
      </div>
    </div>

    <div class="flex flex-wrap items-end gap-2">
      <div class="min-w-48 flex-1">
        <label class="label" for="lr-custom">Otro estudio (si no está en la lista)</label>
        <input id="lr-custom" class="field" bind:value={custom} maxlength="160" placeholder="Ej. Perfil tiroideo, Hemoglobina glucosilada…" autocomplete="off" onkeydown={(e) => { if (e.key === 'Enter') { e.preventDefault(); addCustom(); } }} />
      </div>
      <button type="button" class="btn-secondary" disabled={!custom.trim()} onclick={addCustom}><Icon name="plus" size={16} />Agregar</button>
    </div>

    {#if asked.length === 0}
      <p class="rounded-xl border border-dashed border-app-ink/15 px-4 py-6 text-center text-sm text-app-muted">Aún no has agregado estudios. Elige un panel o escribe el estudio que necesitas.</p>
    {:else}
      <ul class="grid gap-2" aria-label="Estudios solicitados">
        {#each asked as a (a.name)}
          <li class="rounded-2xl border border-app-ink/10 p-3.5">
            <div class="flex items-start justify-between gap-3">
              <p class="font-medium">{a.name}</p>
              <button type="button" class="icon-btn danger -mr-1 -mt-1" aria-label="Quitar {a.name}" onclick={() => remove(a.name)}><Icon name="x" size={16} /></button>
            </div>
            {#if a.includes.length}
              <p class="mt-1 text-xs font-medium uppercase tracking-wide text-app-muted">Incluye {a.includes.length} análisis</p>
              <ul class="mt-1.5 flex flex-wrap gap-1.5">
                {#each a.includes as x}<li class="rounded-full bg-app-ink/6 px-2.5 py-0.5 text-xs">{x}</li>{/each}
              </ul>
            {/if}
          </li>
        {/each}
      </ul>
    {/if}

    <div class="grid gap-3 sm:grid-cols-2">
      <div>
        <label class="label" for="lr-lab">Laboratorio sugerido (opcional)</label>
        <input id="lr-lab" class="field" bind:value={labName} maxlength="120" autocomplete="off" />
      </div>
      <div>
        <label class="label" for="lr-title">Nombre de la orden (opcional)</label>
        <input id="lr-title" class="field" bind:value={title} maxlength="160" placeholder={asked.length === 1 ? asked[0].name : 'Estudios de laboratorio'} autocomplete="off" />
      </div>
      <div class="sm:col-span-2">
        <label class="label" for="lr-notes">Indicaciones para el paciente (opcional)</label>
        <input id="lr-notes" class="field" bind:value={notes} maxlength="1000" placeholder="Ej. Ayuno de 8 a 12 horas" autocomplete="off" />
      </div>
    </div>
    <OpError {op} />
  </form>
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
    <button type="button" class="btn-secondary" disabled={op.phase === 'loading' || asked.length === 0} onclick={() => save(true)}><Icon name="receipt" size={16} />Guardar e imprimir</button>
    <button type="submit" form="lab-request" class="btn-primary" disabled={op.phase === 'loading' || asked.length === 0}>
      {#if op.phase === 'loading'}<span class="spin"></span>{/if}Guardar orden
    </button>
  {/snippet}
</Modal>
