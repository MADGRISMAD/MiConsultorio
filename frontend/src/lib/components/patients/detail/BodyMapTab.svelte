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
  import type { BodyFinding, BodymapData, BodyView, FindingKind, PatientChart } from '$lib/types/specialty';
  import BodyFigure from '../../specialty/BodyFigure.svelte';
  import { bodyDiff, intensityColor, KINDS, kindLabel, zoneLabel } from '../../specialty/bodyGeometry';
  import EmptyState from '../../ui/EmptyState.svelte';
  import VersionHistory from '../../specialty/VersionHistory.svelte';
  import ViewedVersionBanner from '../../specialty/ViewedVersionBanner.svelte';
  import Icon from '../../ui/Icon.svelte';

  let { patient, canWrite }: { patient: Patient; schema: PatientSchema | null; canWrite: boolean; isAdmin: boolean } = $props();

  let history = $state<PatientChart[]>([]);
  const ld = new Loader('No se pudo cargar el esquema corporal.');
  let work = $state<BodymapData>({ zones: [] });
  let base = $state('[]');
  let view = $state<BodyView>('front');
  let selected = $state<{ view: BodyView; zone: string } | null>(null);
  let viewing = $state<string | null>(null);
  let comparing = $state(false);
  let note = $state('');
  const saveOp = new Op();

  // form for a new finding
  let kind = $state<FindingKind>('dolor');
  let intensity = $state(5);
  let findingNote = $state('');

  const asData = (c: PatientChart) => c.data as BodymapData;

  async function load() {
    await ld.run(async () => {
        const r = await specialtyApi.charts(patient.id, 'bodymap');
        history = r.charts;
        work = r.latest ? JSON.parse(JSON.stringify(asData(r.latest))) : { zones: [] };
        base = JSON.stringify(work.zones);
    });
  }
  onMount(load);

  const viewed = $derived(viewing ? history.find((h) => h.id === viewing) : undefined);
  const viewedIdx = $derived(viewed ? history.indexOf(viewed) : -1);
  const shown = $derived<BodymapData>(viewed ? asData(viewed) : work);
  const readonly = $derived(!canWrite || !!viewed);
  const dirty = $derived(JSON.stringify(work.zones) !== base);
  const diff = $derived(
    viewed && comparing
      ? bodyDiff(history[viewedIdx + 1] ? asData(history[viewedIdx + 1]) : null, asData(viewed))
      : !viewed && history.length
        ? bodyDiff(asData(history[0]), work)
        : []
  );
  const here = $derived(selected ? shown.zones.filter((f) => f.view === selected!.view && f.zone === selected!.zone) : []);

  function pick(v: BodyView, zone: string) {
    selected = { view: v, zone };
  }
  function addFinding() {
    if (!selected) return;
    const f: BodyFinding = { zone: selected.zone, view: selected.view, kind, intensity, note: findingNote.trim() };
    const others = work.zones.filter((x) => !(x.view === f.view && x.zone === f.zone && x.kind === f.kind));
    work = { zones: [...others, f] };
    findingNote = '';
  }
  function removeFinding(f: BodyFinding) {
    work = { zones: work.zones.filter((x) => x !== f) };
  }
  async function save() {
    if (await saveOp.run(() => specialtyApi.saveChart(patient.id, 'bodymap', work, note.trim()))) {
      toast.show('Esquema corporal guardado');
      note = '';
      viewing = null;
      comparing = false;
      await load();
    }
  }
  function show(c: PatientChart | null, compare = false) {
    viewing = c?.id ?? null;
    comparing = compare;
    selected = null;
  }
  function useAsBase(c: PatientChart) {
    work = JSON.parse(JSON.stringify(asData(c)));
    show(null);
    toast.show('Cargado como borrador: guarda para crear una nueva versión');
  }
  async function print() {
    try {
      await printChart(patient, 'bodymap', { data: shown, note: viewed?.note ?? note, at: viewed?.created_at ?? new Date().toISOString(), by: viewed?.created_by_name ?? '' });
    } catch (e) {
      toast.show(e instanceof Error ? e.message : 'No se pudo imprimir.', 'error');
    }
  }
  const sortedFindings = $derived([...shown.zones].sort((a, b) => b.intensity - a.intensity));
</script>

{#if ld.loading}
  <div class="card h-64 animate-pulse"></div>
{:else if ld.error}
  <Alert>{ld.error}</Alert>
{:else}
  <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
    <p class="max-w-xl text-sm text-app-muted">Toca una zona del cuerpo para registrar dolor, contractura, subluxación o parestesia con su intensidad. Izquierdo y derecho son los del paciente.</p>
    <button type="button" class="btn-secondary" onclick={print}><Icon name="receipt" size={18} />Imprimir</button>
  </div>

  {#if viewed}
    <ViewedVersionBanner {viewed} back={() => show(null)} />
  {/if}

  <div class="grid gap-4 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)]">
    <section class="card p-4 sm:p-5" aria-label="Figura corporal">
      <div role="tablist" aria-label="Vista" class="mb-3 inline-flex rounded-full bg-app-ink/5 p-1 lg:hidden">
        {#each [['front', 'Frente'], ['back', 'Espalda']] as [v, l]}
          <button type="button" role="tab" aria-selected={view === v} class="min-h-9 rounded-full px-4 text-sm font-medium {view === v ? 'bg-app-panel shadow-sm' : 'text-app-muted'}" onclick={() => (view = v as BodyView)}>{l}</button>
        {/each}
      </div>
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div class="{view === 'front' ? '' : 'hidden'} lg:block">
          <p class="section-title mb-1 text-center">Frente</p>
          <BodyFigure view="front" findings={shown.zones} selected={selected?.view === 'front' ? selected.zone : null} onpick={(z) => pick('front', z)} />
        </div>
        <div class="{view === 'back' ? '' : 'hidden'} lg:block">
          <p class="section-title mb-1 text-center">Espalda</p>
          <BodyFigure view="back" findings={shown.zones} selected={selected?.view === 'back' ? selected.zone : null} onpick={(z) => pick('back', z)} />
        </div>
      </div>
      <div class="mt-3 flex items-center gap-2 text-xs text-app-muted" aria-hidden="true">
        <span>Intensidad</span>
        {#each [0, 2, 4, 6, 8, 10] as n}<span class="grid h-5 w-7 place-items-center rounded text-[10px] font-bold text-black" style="background:{intensityColor(n)}">{n}</span>{/each}
      </div>
    </section>

    <div class="space-y-4">
      <section class="card p-4 sm:p-5" aria-live="polite">
        <h3 class="display text-xl">{selected ? zoneLabel(selected.view, selected.zone) : 'Zona'}</h3>
        {#if !selected}
          <p class="mt-1 text-sm text-app-muted">Selecciona una zona de la figura.</p>
        {:else}
          {#if here.length === 0}<p class="mt-1 text-sm text-app-muted">Sin hallazgos en esta zona.</p>{/if}
          <ul class="mt-2 space-y-2">
            {#each here as f}
              <li class="flex items-start justify-between gap-2 rounded-xl bg-app-ink/5 p-2.5 text-sm">
                <span><span class="mr-1.5 inline-block rounded px-1.5 text-xs font-bold text-black" style="background:{intensityColor(f.intensity)}">{f.intensity}/10</span><strong>{kindLabel(f.kind)}</strong>{f.note ? ` · ${f.note}` : ''}</span>
                {#if !readonly}<button type="button" class="icon-btn danger -my-1" aria-label="Quitar hallazgo" onclick={() => removeFinding(f)}><Icon name="trash" size={16} /></button>{/if}
              </li>
            {/each}
          </ul>
          {#if !readonly}
            <fieldset class="mt-4 space-y-3 border-t border-app-ink/10 pt-3">
              <legend class="section-title">Agregar hallazgo</legend>
              <div>
                <label class="label" for="bm-kind">Tipo</label>
                <select id="bm-kind" class="field" bind:value={kind}>{#each KINDS as k}<option value={k.id}>{k.label}</option>{/each}</select>
              </div>
              <div>
                <label class="label" for="bm-int">Intensidad: <strong>{intensity}</strong> de 10</label>
                <input id="bm-int" type="range" min="0" max="10" step="1" class="w-full accent-[rgb(var(--app-primary))]" bind:value={intensity} />
              </div>
              <div>
                <label class="label" for="bm-note">Nota (opcional)</label>
                <input id="bm-note" class="field" maxlength="300" bind:value={findingNote} />
              </div>
              <button type="button" class="btn-secondary" onclick={addFinding}><Icon name="plus" size={18} />Agregar a la zona</button>
            </fieldset>
          {/if}
        {/if}
      </section>

      <section class="card p-4 sm:p-5">
        <h3 class="display text-xl">Hallazgos registrados</h3>
        {#if sortedFindings.length === 0}
          <p class="mt-1 text-sm text-app-muted">Aún no hay hallazgos en este esquema.</p>
        {:else}
          <ul class="mt-2 space-y-1 text-sm">
            {#each sortedFindings as f}
              <li><button type="button" class="text-left hover:underline" onclick={() => { view = f.view; pick(f.view, f.zone); }}><span class="mr-1 inline-block rounded px-1 text-xs font-bold text-black" style="background:{intensityColor(f.intensity)}">{f.intensity}</span>{zoneLabel(f.view, f.zone)} <span class="text-app-muted">({f.view === 'front' ? 'frente' : 'espalda'}) · {kindLabel(f.kind).toLowerCase()}</span></button></li>
            {/each}
          </ul>
        {/if}
        {#if diff.length}
          <p class="section-title mt-4">{viewed ? 'Cambios frente a la versión anterior' : 'Cambios sin guardar'}</p>
          <ul class="mt-1 list-disc space-y-0.5 pl-5 text-sm">{#each diff as d}<li>{d}</li>{/each}</ul>
        {:else if viewed && comparing}
          <p class="mt-3 text-sm text-app-muted">No hay diferencias con la versión anterior.</p>
        {/if}
        {#if canWrite && !viewed}
          <label class="label mt-4" for="bm-vnote">Nota de esta versión (opcional)</label>
          <input id="bm-vnote" class="field" maxlength="500" bind:value={note} placeholder="Ej. Evaluación inicial" />
          <OpError op={saveOp} class="mt-3" />
          <button type="button" class="btn-primary mt-3" disabled={saveOp.phase === 'loading' || (!dirty && history.length > 0)} onclick={save}>
            {#if saveOp.phase === 'loading'}<span class="spin"></span>{/if}<Icon name="check" size={18} />Guardar versión
          </button>
        {/if}
      </section>
    </div>
  </div>

  <section class="mt-6" aria-labelledby="bm-history">
    <VersionHistory id="bm-history" title="Historial de versiones" {history} {viewing} {canWrite} icon="spine" emptyText="Cuando guardes el primer esquema aparecerá aquí." summary={(h) => `${asData(h).zones.length} hallazgos`} onview={show} onbase={useAsBase} compare />
  </section>
{/if}
