<script lang="ts">
  import { Loader } from '$lib/loader.svelte';
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onMount } from 'svelte';
  import { dateTime } from '$lib/format';
  import { specialtyApi } from '$lib/api/specialty';
  import { Op } from '$lib/op.svelte';
  import { printExercises } from '$lib/print';
  import { toast } from '$lib/toast.svelte';
  import type { Patient } from '$lib/types';
  import type { ExerciseItem, ExercisesData, PatientChart } from '$lib/types/specialty';
  import ViewedVersionBanner from '../../specialty/ViewedVersionBanner.svelte';
  import EmptyState from '../../ui/EmptyState.svelte';
  import Icon from '../../ui/Icon.svelte';

  let { patient, canWrite }: { patient: Patient; canWrite: boolean } = $props();

  /** Common exercises to start from; anything else is typed by hand. */
  const SUGGESTED = [
    'Puente de glúteo', 'Estiramiento de isquiotibiales', 'Estiramiento de cuádriceps', 'Estiramiento de pantorrilla', 'Plancha abdominal', 'Plancha lateral',
    'Sentadilla asistida', 'Elevación de talones', 'Extensión de rodilla sentado', 'Elevación de pierna recta', 'Rotación externa de hombro con banda', 'Péndulo de Codman',
    'Escalera de dedos en pared', 'Retracción cervical (mentón al pecho)', 'Inclinación lateral de cuello', 'Gato-camello', 'Rotación de tronco acostado', 'Bird-dog (perro de caza)',
    'Estiramiento de piriforme', 'Equilibrio en un pie', 'Marcha con elevación de rodillas', 'Remo con banda elástica', 'Curl de bíceps con banda', 'Respiración diafragmática'
  ];

  const empty = (): ExercisesData => ({ items: [], frequency: '', notes: '' });
  const copy = <T,>(v: T): T => JSON.parse(JSON.stringify(v));
  const asData = (c: PatientChart): ExercisesData => ({ ...empty(), ...(c.data as unknown as ExercisesData) });

  const ld = new Loader('No se pudo cargar la hoja de ejercicios.');
  const saveOp = new Op();
  let history = $state<PatientChart[]>([]);
  let work = $state<ExercisesData>(empty());
  let base = $state('');
  let viewing = $state<string | null>(null);
  let note = $state('');

  async function load() {
    await ld.run(async () => {
      const r = await specialtyApi.charts(patient.id, 'exercises');
      history = r.charts;
      work = r.latest ? asData(r.latest) : empty();
      base = JSON.stringify(work);
    });
  }
  onMount(load);

  const viewed = $derived(viewing ? history.find((h) => h.id === viewing) : undefined);
  const shown = $derived<ExercisesData>(viewed ? asData(viewed) : work);
  const readonly = $derived(!canWrite || !!viewed);
  const dirty = $derived(JSON.stringify(work) !== base);

  const add = (name = '') => (work.items = [...work.items, { name, sets: '3', reps: '10', notes: '' } satisfies ExerciseItem]);
  const remove = (i: number) => (work.items = work.items.filter((_, k) => k !== i));
  const move = (i: number, d: number) => {
    const j = i + d;
    if (j < 0 || j >= work.items.length) return;
    const next = [...work.items];
    [next[i], next[j]] = [next[j], next[i]];
    work.items = next;
  };

  async function save() {
    const clean: ExercisesData = {
      items: work.items.filter((x) => x.name.trim()).map((x) => ({ name: x.name.trim(), sets: x.sets.trim(), reps: x.reps.trim(), notes: x.notes.trim() })),
      frequency: work.frequency.trim(),
      notes: work.notes.trim()
    };
    if (!clean.items.length) return saveOp.fail('Agrega al menos un ejercicio.');
    if (await saveOp.run(() => specialtyApi.saveChart(patient.id, 'exercises', clean, note.trim()))) {
      toast.show('Hoja de ejercicios guardada');
      note = '';
      await load();
    }
  }

  async function print(c?: PatientChart) {
    const data = c ? asData(c) : shown;
    const src = c ?? viewed ?? history[0];
    await printExercises(patient, { data, at: src?.created_at ?? new Date().toISOString(), by: src?.created_by_name ?? '' });
  }
</script>

{#if ld.loading}
  <div class="card h-40 animate-pulse"></div>
{:else if ld.error}
  <Alert>{ld.error}</Alert>
{:else}
  {#if viewed}<ViewedVersionBanner {viewed} back={() => (viewing = null)} backLabel="Volver a la hoja actual" />{/if}

  <div class="card px-4 py-4 sm:px-5">
    <p class="text-sm text-app-muted">Los ejercicios que el paciente hace en casa. Se imprimen en una hoja con casillas para marcar lo que cumplió.</p>

    {#if shown.items.length === 0}
      <div class="mt-3"><EmptyState icon="plus" title="Sin ejercicios" text={canWrite ? 'Agrega el primero o elige uno de los sugeridos.' : 'Todavía no hay una hoja de ejercicios.'} /></div>
    {:else}
      <ol class="mt-4 grid gap-3">
        {#each shown.items as it, i (i)}
          <li class="rounded-xl border border-app-ink/12 p-3">
            {#if readonly}
              <p class="font-medium">{i + 1}. {it.name}</p>
              <p class="text-sm text-app-muted">{it.sets} series · {it.reps}{it.notes ? ` · ${it.notes}` : ''}</p>
            {:else}
              <div class="grid gap-2 sm:grid-cols-[1fr_5rem_8rem]">
                <label class="block"><span class="label">Ejercicio {i + 1}</span><input class="field" list="ex-suggest" maxlength="120" bind:value={work.items[i].name} /></label>
                <label class="block"><span class="label">Series</span><input class="field" maxlength="12" bind:value={work.items[i].sets} /></label>
                <label class="block"><span class="label">Repeticiones o tiempo</span><input class="field" maxlength="24" placeholder="10 o 30 s" bind:value={work.items[i].reps} /></label>
              </div>
              <label class="mt-2 block"><span class="label">Indicaciones</span><input class="field" maxlength="300" bind:value={work.items[i].notes} placeholder="Cómo hacerlo, qué evitar" /></label>
              <div class="mt-2 flex gap-1">
                <button type="button" class="icon-btn" aria-label="Subir" disabled={i === 0} onclick={() => move(i, -1)}><Icon name="arrow-left" size={14} class="rotate-90" /></button>
                <button type="button" class="icon-btn" aria-label="Bajar" disabled={i === work.items.length - 1} onclick={() => move(i, 1)}><Icon name="arrow-right" size={14} class="rotate-90" /></button>
                <button type="button" class="icon-btn danger" aria-label="Quitar el ejercicio {i + 1}" onclick={() => remove(i)}><Icon name="x" size={14} /></button>
              </div>
            {/if}
          </li>
        {/each}
      </ol>
    {/if}
    <datalist id="ex-suggest">{#each SUGGESTED as s}<option value={s}></option>{/each}</datalist>

    {#if !readonly}
      <div class="mt-3 flex flex-wrap gap-2">
        <button type="button" class="btn-secondary" onclick={() => add()} disabled={work.items.length >= 30}><Icon name="plus" size={16} />Agregar ejercicio</button>
      </div>
    {/if}

    {#if readonly}
      {#if shown.frequency}<p class="mt-4 text-sm"><span class="font-medium">Frecuencia:</span> {shown.frequency}</p>{/if}
      {#if shown.notes}<p class="mt-2 whitespace-pre-wrap text-sm">{shown.notes}</p>{/if}
    {:else}
      <label class="mt-4 block"><span class="label">Frecuencia</span><input class="field" maxlength="120" bind:value={work.frequency} placeholder="3 veces por semana, 15 minutos" /></label>
      <label class="mt-3 block"><span class="label">Indicaciones generales</span><textarea class="field min-h-20" maxlength="1500" bind:value={work.notes} placeholder="Calentar 5 minutos antes; aplicar hielo después si hay molestia"></textarea></label>
    {/if}

    {#if canWrite && !viewed}
      <label class="label mt-4" for="ex-note">Nota de esta versión (opcional)</label>
      <input id="ex-note" class="field" maxlength="300" bind:value={note} />
      <OpError op={saveOp} class="mt-3" />
      <div class="mt-3 flex flex-wrap gap-2">
        <button type="button" class="btn-primary" disabled={!dirty || saveOp.phase === 'loading'} onclick={save}><Icon name="check" size={18} />Guardar hoja</button>
        {#if shown.items.length}<button type="button" class="btn-secondary" onclick={() => print()}><Icon name="receipt" size={18} />Imprimir</button>{/if}
      </div>
    {:else if shown.items.length}
      <button type="button" class="btn-secondary mt-3" onclick={() => print()}><Icon name="receipt" size={18} />Imprimir</button>
    {/if}
  </div>

  <h3 class="display mb-2 mt-6 text-xl">Versiones anteriores</h3>
  {#if history.length === 0}
    <div class="card"><EmptyState icon="info" title="Sin versiones" text="Cada vez que cambies la hoja y la guardes, queda la versión anterior aquí." /></div>
  {:else}
    <ul class="card divide-y divide-app-ink/8">
      {#each history as h (h.id)}
        <li class="flex flex-wrap items-center justify-between gap-2 px-4 py-3 text-sm">
          <span class="min-w-0"><span class="font-medium">{dateTime(h.created_at)}</span><span class="block text-xs text-app-muted">{h.created_by_name} · {asData(h).items.length} ejercicios{h.note ? ` · ${h.note}` : ''}</span></span>
          <span class="flex gap-2">
            <button type="button" class="btn-secondary" onclick={() => (viewing = viewing === h.id ? null : h.id)}>{viewing === h.id ? 'Cerrar' : 'Ver'}</button>
            <button type="button" class="btn-ghost" onclick={() => print(h)}>Imprimir</button>
          </span>
        </li>
      {/each}
    </ul>
  {/if}
{/if}
