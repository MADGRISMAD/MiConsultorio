<script lang="ts">
  import { Loader } from '$lib/loader.svelte';
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onMount } from 'svelte';
  import { specialtyApi } from '$lib/api/specialty';
  import { Op } from '$lib/op.svelte';
  import { printChart } from '$lib/print';
  import { toast } from '$lib/toast.svelte';
  import type { Patient, PatientSchema } from '$lib/types';
  import type { Dentition, OdontogramData, PatientChart, Surface, ToothState } from '$lib/types/specialty';
  import OdontogramChart from '../../specialty/OdontogramChart.svelte';
  import {
    autoDentition,
    cleanTeeth,
    diffOdontograms,
    emptyOdontogram,
    STATES,
    stateDef,
    SURFACE_NAMES,
    surfaceShapes,
    toothKind,
    toothSummary
  } from '../../specialty/odontoGeometry';
  import EmptyState from '../../ui/EmptyState.svelte';
  import VersionHistory from '../../specialty/VersionHistory.svelte';
  import ViewedVersionBanner from '../../specialty/ViewedVersionBanner.svelte';
  import Icon from '../../ui/Icon.svelte';

  let { patient, canWrite }: { patient: Patient; schema: PatientSchema | null; canWrite: boolean; isAdmin: boolean } = $props();

  let history = $state<PatientChart[]>([]);
  const ld = new Loader('No se pudo cargar el odontograma.');
  let work = $state<OdontogramData>(emptyOdontogram());
  let base = $state<OdontogramData>(emptyOdontogram()); // what is stored (to detect unsaved changes)
  let tool = $state<ToothState>('caries');
  let selected = $state<number | null>(null);
  let viewing = $state<string | null>(null); // history entry shown read-only
  let comparing = $state(false);
  let note = $state('');
  const saveOp = new Op();

  const asData = (c: PatientChart) => c.data as OdontogramData;
  const copy = (d: OdontogramData): OdontogramData => JSON.parse(JSON.stringify(d));

  async function load() {
    await ld.run(async () => {
        const r = await specialtyApi.charts(patient.id, 'odontogram');
        history = r.charts;
        const d = r.latest ? copy(asData(r.latest)) : defaultFor(patient);
        if (r.latest && patient.age != null) d.dentition = autoDentition(patient.age, d);
        work = copy(d);
        base = copy(d);
    });
  }
  /** children start on the deciduous chart, teenagers on the mixed one */
  function defaultFor(p: Patient): OdontogramData {
    return emptyOdontogram(autoDentition(p.age, emptyOdontogram()));
  }
  onMount(load);

  const viewed = $derived(viewing ? history.find((h) => h.id === viewing) : undefined);
  const viewedIdx = $derived(viewed ? history.indexOf(viewed) : -1);
  const shown = $derived<OdontogramData>(viewed ? asData(viewed) : work);
  const previous = $derived(viewedIdx >= 0 ? history[viewedIdx + 1] : undefined);
  const diff = $derived(
    viewed && comparing
      ? diffOdontograms(previous ? asData(previous) : null, asData(viewed))
      : !viewed && history.length
        ? diffOdontograms(asData(history[0]), cleanTeeth(work))
        : []
  );
  const highlight = $derived(new Set(diff.map((d) => d.tooth)));
  const dirty = $derived(JSON.stringify(cleanTeeth(work)) !== JSON.stringify(cleanTeeth(base)));
  const readonly = $derived(!canWrite || !!viewed);
  const tooth = $derived(selected != null ? shown.teeth[String(selected)] : undefined);

  function touch(n: number, fn: (t: NonNullable<OdontogramData['teeth'][string]>) => void) {
    const k = String(n);
    const t = { ...work.teeth[k], surfaces: { ...work.teeth[k]?.surfaces } };
    fn(t);
    work = { ...work, teeth: { ...work.teeth, [k]: t } };
  }
  function toggleWhole(n: number, st: ToothState) {
    touch(n, (t) => (t.state = t.state === st ? undefined : st));
  }
  function toggleSurface(n: number, s: Surface, st: ToothState) {
    touch(n, (t) => {
      t.surfaces = { ...t.surfaces, [s]: t.surfaces?.[s] === st ? undefined : st };
    });
  }
  function applyTool(n: number, s: Surface | null) {
    const def = stateDef(tool);
    if (def?.whole) toggleWhole(n, tool);
    else if (s) toggleSurface(n, s, tool);
  }
  function pick(n: number, s: Surface | null) {
    selected = n;
    if (readonly) return;
    if (s) applyTool(n, s);
  }
  function key(n: number, k: string) {
    selected = n;
    if (k === 'clear') return clearTooth(n);
    const def = STATES.find((s) => s.key === k);
    if (!def) return;
    tool = def.id;
    if (def.whole) toggleWhole(n, def.id);
  }
  function clearTooth(n: number) {
    const { [String(n)]: _, ...rest } = work.teeth;
    work = { ...work, teeth: rest };
  }
  function setNote(n: number, v: string) {
    touch(n, (t) => (t.note = v));
  }
  function setDentition(d: Dentition) {
    work = { ...work, dentition: d };
    selected = null;
  }

  async function save() {
    const data = cleanTeeth(work);
    if (await saveOp.run(() => specialtyApi.saveChart(patient.id, 'odontogram', data, note.trim()))) {
      toast.show('Odontograma guardado');
      note = '';
      viewing = null;
      comparing = false;
      await load();
    }
  }
  function view(c: PatientChart | null, compare = false) {
    viewing = c?.id ?? null;
    comparing = compare;
    selected = null;
  }
  function useAsBase(c: PatientChart) {
    work = copy(asData(c));
    view(null);
    toast.show('Cargado como borrador: guarda para crear una nueva versión');
  }
  async function print() {
    try {
      await printChart(patient, 'odontogram', { data: cleanTeeth(shown), note: viewed?.note ?? note, at: viewed?.created_at ?? new Date().toISOString(), by: viewed?.created_by_name ?? '' }, highlight);
    } catch (e) {
      toast.show(e instanceof Error ? e.message : 'No se pudo imprimir.', 'error');
    }
  }
  /** Options that make sense for the age: kids get baby teeth + mixed, adults permanent + mixed, the transition years all three.
   *  The one in use is always offered so it never disappears. */
  const offered = $derived.by(() => {
    const age = patient.age;
    const keep = shown.dentition;
    return dentitions.filter((d) => {
      if (age == null || d.v === keep || d.v === 'mixed') return true;
      if (age >= 6 && age < 13) return true;
      return d.v === (age < 6 ? 'child' : 'adult');
    });
  });
  const dentitions: { v: Dentition; label: string }[] = [
    { v: 'adult', label: 'Adulto (32)' },
    { v: 'child', label: 'Niño (20)' },
    { v: 'mixed', label: 'Mixta' }
  ];
</script>

{#if ld.loading}
  <div class="card h-64 animate-pulse"></div>
{:else if ld.error}
  <Alert>{ld.error}</Alert>
{:else}
  <div class="mb-4 flex min-w-0 flex-wrap items-center justify-between gap-3">
    <p class="min-w-0 max-w-xl flex-1 basis-64 text-sm text-app-muted">Notación FDI. Elige un estado y toca la superficie o la pieza. Cada versión guardada se conserva en el historial.</p>
    <div class="flex flex-wrap gap-2">
      <div role="radiogroup" aria-label="Dentición" class="inline-flex rounded-full bg-app-ink/5 p-1">
        {#each offered as d (d.v)}
          <button
            type="button"
            role="radio"
            aria-checked={shown.dentition === d.v}
            disabled={readonly}
            class="min-h-9 rounded-full px-3.5 text-sm font-medium transition disabled:cursor-default {shown.dentition === d.v ? 'bg-app-panel text-app-ink shadow-sm' : 'text-app-muted hover:text-app-ink'}"
            onclick={() => setDentition(d.v)}>{d.label}</button
          >
        {/each}
      </div>
      <button type="button" class="btn-secondary" onclick={print}><Icon name="receipt" size={18} />Imprimir</button>
    </div>
  </div>

  {#if !viewed && patient.age != null}
    {@const auto = dentitions.find((d) => d.v === autoDentition(patient.age, work))?.label.replace(/ \(.*\)/, '').toLowerCase()}
    <p class="hint mb-3">
      {#if work.dentition === autoDentition(patient.age, work)}
        Según su edad ({patient.age} años) se muestra la dentición {auto}.
      {:else}
        Por su edad ({patient.age} años) correspondería la dentición {auto}; elegiste otra manualmente.
      {/if}
      «Mixta» es solo para cuando el paciente aún conserva algunos dientes de leche junto a los definitivos; en un adulto normal usa «Adulto».
    </p>
  {/if}

  {#if viewed}
    <ViewedVersionBanner {viewed} back={() => view(null)} />
  {/if}

  {#if !readonly}
    <fieldset class="mb-3">
      <legend class="section-title mb-2">Estado a aplicar</legend>
      <div class="flex flex-wrap gap-1.5">
        {#each STATES as s (s.id)}
          <button
            type="button"
            aria-pressed={tool === s.id}
            title="Atajo: {s.key}"
            class="inline-flex min-h-9 items-center gap-1.5 rounded-full px-3 text-sm ring-1 ring-inset transition {tool === s.id ? 'bg-app-ink text-app-surface ring-app-ink' : 'bg-app-panel ring-app-ink/15 hover:bg-app-elevated'}"
            onclick={() => (tool = s.id)}
          >
            <span class="grid h-5 w-5 place-items-center rounded text-[11px] font-bold text-white" style="background:{s.color}" aria-hidden="true">{s.letter}</span>
            {s.label}
            <kbd class="hidden font-mono text-[10px] opacity-60 sm:inline">{s.key}</kbd>
          </button>
        {/each}
      </div>
      <p class="hint">{stateDef(tool)?.whole ? 'Este estado se aplica a toda la pieza.' : 'Este estado se aplica a la superficie que toques.'} Con el teclado: flechas para moverte, números para aplicar, Supr para limpiar la pieza.</p>
    </fieldset>
  {/if}

  <OdontogramChart data={shown} {selected} {highlight} {readonly} onpick={pick} onkey={key} />

  <div class="mt-4 grid grid-cols-[minmax(0,1fr)] gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
    <section class="card p-4 sm:p-5" aria-live="polite">
      <h3 class="display text-xl">{selected != null ? `Pieza ${selected}` : 'Pieza'}</h3>
      {#if selected == null}
        <p class="mt-1 text-sm text-app-muted">Selecciona una pieza para ver sus hallazgos y agregar una nota.</p>
      {:else}
        <p class="text-sm text-app-muted">{toothKind(selected)} · {toothSummary(tooth) || 'Sin hallazgos'}</p>
        {#if !readonly}
          <p class="section-title mt-3">Superficies</p>
          <div class="mt-1 flex flex-wrap gap-1.5">
            {#each surfaceShapes(selected) as sh (sh.surface)}
              {@const st = tooth?.surfaces?.[sh.surface]}
              <button
                type="button"
                class="btn-secondary !min-h-9 !px-3"
                disabled={stateDef(tool)?.whole}
                onclick={() => toggleSurface(selected!, sh.surface, tool)}
              >
                {SURFACE_NAMES[sh.surface]}{#if st}<span class="rounded px-1 text-[11px] font-bold text-white" style="background:{stateDef(st)?.color}">{stateDef(st)?.letter}</span>{/if}
              </button>
            {/each}
          </div>
          <label class="label mt-3" for="tooth-note">Nota de la pieza</label>
          <textarea id="tooth-note" class="field" rows="2" maxlength="300" value={tooth?.note ?? ''} oninput={(e) => setNote(selected!, e.currentTarget.value)}></textarea>
          <button type="button" class="btn-ghost mt-2 text-app-danger" onclick={() => clearTooth(selected!)}><Icon name="trash" size={16} />Limpiar pieza</button>
        {:else if tooth?.note}
          <p class="mt-2 text-sm">Nota: {tooth.note}</p>
        {/if}
      {/if}
    </section>

    <section class="card p-4 sm:p-5">
      <h3 class="display text-xl">{viewed && comparing ? 'Cambios frente a la versión anterior' : viewed ? 'Hallazgos' : 'Cambios sin guardar'}</h3>
      {#if diff.length === 0}
        <p class="mt-1 text-sm text-app-muted">{viewed && comparing ? 'No hay diferencias con la versión anterior.' : viewed ? 'Usa «Comparar» en el historial para ver qué cambió.' : history.length ? 'Sin cambios respecto a la última versión.' : 'Aún no hay versiones guardadas.'}</p>
      {:else}
        <ul class="mt-2 space-y-1.5 text-sm">
          {#each diff as d (d.tooth)}
            <li><button type="button" class="font-semibold underline-offset-2 hover:underline" onclick={() => (selected = d.tooth)}>Pieza {d.tooth}</button>: <span class="text-app-muted">{d.before}</span> → {d.after}</li>
          {/each}
        </ul>
        <p class="hint">Las piezas con cambios llevan un borde punteado ámbar en el esquema.</p>
      {/if}
      {#if canWrite && !viewed}
        <label class="label mt-4" for="chart-note">Nota de esta versión (opcional)</label>
        <input id="chart-note" class="field" maxlength="500" bind:value={note} placeholder="Ej. Revisión de los 6 meses" />
        <OpError op={saveOp} class="mt-3" />
        <button type="button" class="btn-primary mt-3" disabled={saveOp.phase === 'loading' || (!dirty && history.length > 0)} onclick={save}>
          {#if saveOp.phase === 'loading'}<span class="spin"></span>{/if}<Icon name="check" size={18} />Guardar versión
        </button>
      {/if}
    </section>
  </div>

  <section class="mt-6" aria-labelledby="odo-history">
    <VersionHistory id="odo-history" title="Historial de versiones" {history} {viewing} {canWrite} icon="tooth" emptyText="Cuando guardes el primer odontograma aparecerá aquí." summary={(h) => ''} onview={view} onbase={useAsBase} compare />
  </section>
{/if}
