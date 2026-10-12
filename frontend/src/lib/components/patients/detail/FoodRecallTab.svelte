<script lang="ts">
  import { Loader } from '$lib/loader.svelte';
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onMount } from 'svelte';
  import { dateShort, dateTime } from '$lib/format';
  import { specialtyApi } from '$lib/api/specialty';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { Patient } from '$lib/types';
  import type { FoodRecallData, PatientChart } from '$lib/types/specialty';
  import DateField from '../../ui/DateField.svelte';
  import EmptyState from '../../ui/EmptyState.svelte';
  import Icon from '../../ui/Icon.svelte';

  let { patient, canWrite }: { patient: Patient; canWrite: boolean } = $props();

  const today = () => new Date().toLocaleDateString('en-CA');
  const yesterday = () => new Date(Date.now() - 86400000).toLocaleDateString('en-CA');
  const blank = (): FoodRecallData => ({
    date: yesterday(),
    kind: '24h',
    meals: [
      { name: 'Desayuno', time: '', items: '' },
      { name: 'Colación', time: '', items: '' },
      { name: 'Comida', time: '', items: '' },
      { name: 'Colación', time: '', items: '' },
      { name: 'Cena', time: '', items: '' }
    ],
    water: '',
    notes: ''
  });
  const asData = (c: PatientChart): FoodRecallData => ({ ...blank(), ...(c.data as unknown as FoodRecallData) });

  const ld = new Loader('No se pudo cargar el recordatorio.');
  const saveOp = new Op();
  let history = $state<PatientChart[]>([]);
  let work = $state<FoodRecallData>(blank());
  let viewing = $state<string | null>(null);

  async function load() {
    await ld.run(async () => {
      history = (await specialtyApi.charts(patient.id, 'food_recall')).charts;
    });
  }
  onMount(load);

  const viewed = $derived(viewing ? history.find((h) => h.id === viewing) : undefined);
  const dirty = $derived(work.meals.some((m) => m.items.trim()) || !!work.water.trim() || !!work.notes.trim());

  const addMeal = () => (work.meals = [...work.meals, { name: '', time: '', items: '' }]);
  const removeMeal = (i: number) => (work.meals = work.meals.filter((_, k) => k !== i));

  async function save() {
    const clean: FoodRecallData = {
      ...work,
      meals: work.meals.filter((m) => m.items.trim()).map((m) => ({ name: m.name.trim() || 'Comida', time: m.time.trim(), items: m.items.trim() })),
      water: work.water.trim(),
      notes: work.notes.trim()
    };
    if (!clean.meals.length) return saveOp.fail('Anota al menos una comida.');
    if (clean.date > today()) return saveOp.fail('La fecha no puede ser futura.');
    if (await saveOp.run(() => specialtyApi.saveChart(patient.id, 'food_recall', clean, ''))) {
      toast.show('Recordatorio guardado');
      work = blank();
      await load();
    }
  }
</script>

{#if ld.loading}
  <div class="card h-40 animate-pulse"></div>
{:else if ld.error}
  <Alert>{ld.error}</Alert>
{:else}
  {#if canWrite}
    <div class="card px-4 py-4 sm:px-5">
      <h3 class="display text-xl">Nuevo recordatorio</h3>
      <p class="mt-1 text-sm text-app-muted">Lo que el paciente recuerda haber comido: pídele hora, lugar, cantidades y cómo se preparó. Sirve para ver sus hábitos reales antes de armar el plan.</p>
      <div class="mt-4 grid gap-3 sm:grid-cols-2">
        <div><label class="label" for="rc-date">Día que recuerda</label><DateField id="rc-date" bind:value={work.date} max={today()} /></div>
        <label class="block"><span class="label">Tipo</span>
          <select class="field" bind:value={work.kind}><option value="24h">Recordatorio de 24 horas</option><option value="dia_tipico">Un día típico</option></select></label>
      </div>

      <div class="mt-4 grid gap-3">
        {#each work.meals as m, i (i)}
          <div class="rounded-xl border border-app-ink/12 p-3">
            <div class="grid gap-2 sm:grid-cols-[1fr_7rem_auto]">
              <label class="block"><span class="label">Comida</span><input class="field" maxlength="40" bind:value={m.name} /></label>
              <label class="block"><span class="label">Hora</span><input class="field" type="time" bind:value={m.time} /></label>
              <button type="button" class="icon-btn danger self-end" aria-label="Quitar la comida {i + 1}" onclick={() => removeMeal(i)}><Icon name="x" size={14} /></button>
            </div>
            <label class="mt-2 block"><span class="label">Qué comió y cuánto</span><textarea class="field min-h-16" maxlength="1200" bind:value={m.items} placeholder="2 huevos revueltos con aceite, 2 tortillas, 1 vaso de jugo de naranja"></textarea></label>
          </div>
        {/each}
      </div>
      <button type="button" class="btn-secondary mt-3" onclick={addMeal} disabled={work.meals.length >= 10}><Icon name="plus" size={16} />Agregar comida</button>

      <div class="mt-4 grid gap-3 sm:grid-cols-2">
        <label class="block"><span class="label">Agua y otras bebidas</span><input class="field" maxlength="200" bind:value={work.water} placeholder="1.5 litros de agua, 2 cafés con azúcar" /></label>
      </div>
      <label class="mt-3 block"><span class="label">Observaciones</span><textarea class="field min-h-16" maxlength="1500" bind:value={work.notes} placeholder="Come fuera de casa, salta el desayuno, mucha prisa…"></textarea></label>
      <OpError op={saveOp} class="mt-3" />
      <button type="button" class="btn-primary mt-3" disabled={!dirty || saveOp.phase === 'loading'} onclick={save}><Icon name="check" size={18} />Guardar recordatorio</button>
    </div>
  {/if}

  <h3 class="display mb-2 mt-6 text-xl">Recordatorios guardados</h3>
  {#if history.length === 0}
    <div class="card"><EmptyState icon="info" title="Sin recordatorios" text="Aquí quedan los recordatorios de alimentación para compararlos con el plan." /></div>
  {:else}
    <ul class="grid gap-3">
      {#each history as h (h.id)}
        {@const d = asData(h)}
        <li class="card px-4 py-3 text-sm">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <span class="font-medium">{d.kind === 'dia_tipico' ? 'Día típico' : `Día ${dateShort(d.date)}`}<span class="ml-2 text-xs font-normal text-app-muted">registrado {dateTime(h.created_at)} · {h.created_by_name}</span></span>
            <button type="button" class="btn-secondary" onclick={() => (viewing = viewing === h.id ? null : h.id)} aria-expanded={viewing === h.id}>{viewing === h.id ? 'Ocultar' : 'Ver'}</button>
          </div>
          {#if viewed?.id === h.id}
            <ul class="mt-3 grid gap-2">
              {#each d.meals as m}<li><span class="font-medium">{m.name}{m.time ? ` · ${m.time}` : ''}</span><span class="block whitespace-pre-wrap text-app-muted">{m.items}</span></li>{/each}
            </ul>
            {#if d.water}<p class="mt-2"><span class="font-medium">Bebidas:</span> {d.water}</p>{/if}
            {#if d.notes}<p class="mt-1 whitespace-pre-wrap text-app-muted">{d.notes}</p>{/if}
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
{/if}
