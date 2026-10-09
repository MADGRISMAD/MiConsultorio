<script lang="ts">
  import { Loader } from '$lib/loader.svelte';
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onMount } from 'svelte';
  import { specialtyApi } from '$lib/api/specialty';
  import { Op } from '$lib/op.svelte';
  import { printCarnet } from '$lib/print';
  import { toast } from '$lib/toast.svelte';
  import type { Patient, PatientSchema } from '$lib/types';
  import type { Vaccination, VaccinationKind, VaccineSuggestion, WeightPoint } from '$lib/types/specialty';
  import ConfirmModal from '../../ConfirmModal.svelte';
  import Modal from '../../Modal.svelte';
  import EmptyState from '../../ui/EmptyState.svelte';
  import Icon from '../../ui/Icon.svelte';
  import Pill from '../../ui/Pill.svelte';
  import WeightChart from '../../specialty/WeightChart.svelte';

  let { patient, canWrite }: { patient: Patient; schema: PatientSchema | null; canWrite: boolean; isAdmin: boolean } = $props();

  const animal = $derived(patient.subject === 'animal');
  let list = $state<Vaccination[]>([]);
  let suggestions = $state<VaccineSuggestion[]>([]);
  let weights = $state<WeightPoint[]>([]);
  const ld = new Loader('No se pudo cargar el carnet.');
  let showVoided = $state(false);

  const KINDS: { v: VaccinationKind; label: string }[] = [
    { v: 'vaccine', label: 'Vacuna' },
    { v: 'deworming_internal', label: 'Desparasitación interna' },
    { v: 'deworming_external', label: 'Desparasitación externa' },
    { v: 'other', label: 'Otro' }
  ];
  const kindLabel = (k: string) => KINDS.find((x) => x.v === k)?.label ?? k;
  const d = (iso: string | null) => (iso ? new Date(`${iso.slice(0, 10)}T12:00:00`).toLocaleDateString('es-MX', { day: 'numeric', month: 'short', year: 'numeric' }) : '—');
  const iso = (dt: Date) => `${dt.getFullYear()}-${String(dt.getMonth() + 1).padStart(2, '0')}-${String(dt.getDate()).padStart(2, '0')}`;
  const todayIso = () => iso(new Date());
  const addDays = (date: string, n: number) => {
    const x = new Date(`${date}T12:00:00`);
    x.setDate(x.getDate() + n);
    return iso(x);
  };

  async function load() {
    await ld.run(async () => {
        const [v, w] = await Promise.all([specialtyApi.vaccinations(patient.id), specialtyApi.weights(patient.id)]);
        list = v.vaccinations;
        suggestions = v.suggestions;
        weights = w;
    });
  }
  onMount(load);

  // The latest application of each product decides its state.
  const key = (v: Vaccination) => `${v.kind}|${v.name.trim().toLowerCase()}`;
  const current = $derived.by(() => {
    const m = new Map<string, Vaccination>();
    for (const v of list) {
      if (v.voided_at) continue;
      const c = m.get(key(v));
      if (!c || v.applied_on > c.applied_on) m.set(key(v), v);
    }
    return m;
  });
  type Status = 'ok' | 'soon' | 'late' | 'none';
  function status(v: Vaccination): Status {
    if (v.voided_at || current.get(key(v))?.id !== v.id || !v.next_due) return 'none';
    const t = todayIso();
    if (v.next_due < t) return 'late';
    return v.next_due <= addDays(t, 30) ? 'soon' : 'ok';
  }
  const rows = $derived(list.filter((v) => showVoided || !v.voided_at));
  const upcoming = $derived([...current.values()].filter((v) => status(v) === 'late' || status(v) === 'soon').sort((a, b) => (a.next_due ?? '').localeCompare(b.next_due ?? '')));
  const voidedCount = $derived(list.filter((v) => v.voided_at).length);

  // ---- new application ----
  let adding = $state(false);
  let form = $state({ kind: 'vaccine' as VaccinationKind, name: '', applied_on: todayIso(), next_due: '', lot: '', dose: '', notes: '' });
  let nextTouched = false;
  const addOp = new Op();
  const shownSuggestions = $derived(suggestions.filter((s) => (form.kind === 'other' ? true : s.kind === form.kind)));

  function openAdd() {
    form = { kind: 'vaccine', name: '', applied_on: todayIso(), next_due: '', lot: '', dose: '', notes: '' };
    nextTouched = false;
    addOp.reset();
    adding = true;
  }
  function suggest(s: VaccineSuggestion) {
    form.kind = s.kind;
    form.name = s.name;
    nextTouched = false;
    refreshNext();
  }
  function refreshNext() {
    if (nextTouched) return;
    const s = suggestions.find((x) => x.name.toLowerCase() === form.name.trim().toLowerCase());
    form.next_due = s && s.interval_days > 0 && form.applied_on ? addDays(form.applied_on, s.interval_days) : '';
  }
  async function submit() {
    if (!form.name.trim()) return addOp.fail('Escribe el nombre de la vacuna o desparasitante.');
    if (!form.applied_on) return addOp.fail('Indica la fecha de aplicación.');
    if (await addOp.run(() => specialtyApi.addVaccination(patient.id, { ...form, name: form.name.trim(), next_due: form.next_due || undefined }))) {
      adding = false;
      toast.show('Aplicación registrada');
      load();
    }
  }

  // ---- void ----
  let voiding = $state<Vaccination | null>(null);
  let reason = $state('');
  const voidOp = new Op();
  async function confirmVoid() {
    const v = voiding;
    if (!v) return;
    if (!reason.trim()) return voidOp.fail('Escribe el motivo de la anulación.');
    if (await voidOp.run(() => specialtyApi.voidVaccination(v.id, reason.trim()))) {
      voiding = null;
      toast.show('Registro anulado');
      load();
    }
  }
  let printing = $state(false);
  async function print() {
    printing = true;
    try {
      await printCarnet(patient);
    } catch (e) {
      toast.show(e instanceof Error ? e.message : 'No se pudo imprimir.', 'error');
    } finally {
      printing = false;
    }
  }
</script>

{#if ld.loading}
  <div class="card h-48 animate-pulse"></div>
{:else if ld.error}
  <Alert>{ld.error}</Alert>
{:else}
  <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
    <p class="max-w-xl text-sm text-app-muted">
      Carnet de {animal ? 'vacunación y desparasitación' : 'vacunación'}. Los registros no se borran: si hay un error se anulan con un motivo y quedan visibles.
    </p>
    <div class="flex flex-wrap gap-2">
      <button type="button" class="btn-secondary" disabled={printing || list.length === 0} onclick={print}><Icon name="receipt" size={18} />Imprimir carnet</button>
      {#if canWrite}<button type="button" class="btn-primary" onclick={openAdd}><Icon name="plus" size={18} />Registrar aplicación</button>{/if}
    </div>
  </div>

  {#if upcoming.length}
    <div class="mb-4 rounded-2xl bg-app-warning/12 p-4" role="status">
      <p class="flex items-center gap-2 text-sm font-semibold"><Icon name="alert" size={18} />Refuerzos pendientes</p>
      <ul class="mt-1 space-y-0.5 text-sm">
        {#each upcoming as v (v.id)}<li>{v.name}: <strong>{d(v.next_due)}</strong> {status(v) === 'late' ? '(vencida)' : '(por vencer)'}</li>{/each}
      </ul>
    </div>
  {/if}

  {#if list.length === 0}
    <div class="card"><EmptyState icon="shield" title="Carnet vacío" text="Registra la primera vacuna{animal ? ' o desparasitación' : ''} de {patient.names}.">
      {#if canWrite}<button type="button" class="btn-primary" onclick={openAdd}><Icon name="plus" size={18} />Registrar aplicación</button>{/if}
    </EmptyState></div>
  {:else}
    <div class="card hidden overflow-hidden md:block">
      <table class="w-full">
        <thead class="border-b border-app-ink/10"><tr><th class="th">Aplicada</th><th class="th">Producto</th><th class="th">Dosis y lote</th><th class="th">Próximo refuerzo</th><th class="th">Aplicó</th><th class="th"><span class="sr-only">Acciones</span></th></tr></thead>
        <tbody>
          {#each rows as v (v.id)}
            {@const s = status(v)}
            <tr class="border-t border-app-ink/8 align-top {v.voided_at ? 'opacity-60' : ''}">
              <td class="td whitespace-nowrap">{d(v.applied_on)}</td>
              <td class="td"><span class="font-medium {v.voided_at ? 'line-through' : ''}">{v.name}</span><br /><span class="text-xs text-app-muted">{kindLabel(v.kind)}</span>
                {#if v.notes}<p class="mt-0.5 text-xs text-app-muted">{v.notes}</p>{/if}
                {#if v.voided_at}<p class="mt-0.5 text-xs text-app-danger">Anulada {d(v.voided_at)} por {v.voided_by_name}: {v.void_reason}</p>{/if}</td>
              <td class="td">{[v.dose, v.lot && `Lote ${v.lot}`].filter(Boolean).join(' · ') || '—'}</td>
              <td class="td whitespace-nowrap">{d(v.next_due)}
                {#if s === 'late'}<Pill tone="bad">Vencida</Pill>{:else if s === 'soon'}<Pill tone="warn">Por vencer</Pill>{:else if s === 'ok'}<Pill tone="ok">Al día</Pill>{/if}</td>
              <td class="td">{v.administered_by_name || '—'}</td>
              <td class="td text-right">{#if canWrite && !v.voided_at}<button type="button" class="btn-ghost text-app-danger" onclick={() => { voiding = v; reason = ''; voidOp.reset(); }}><Icon name="ban" size={16} />Anular</button>{/if}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
    <ul class="space-y-3 md:hidden">
      {#each rows as v (v.id)}
        {@const s = status(v)}
        <li class="card p-4 {v.voided_at ? 'opacity-60' : ''}">
          <div class="flex flex-wrap items-center gap-2">
            <span class="font-medium {v.voided_at ? 'line-through' : ''}">{v.name}</span>
            {#if s === 'late'}<Pill tone="bad">Vencida</Pill>{:else if s === 'soon'}<Pill tone="warn">Por vencer</Pill>{:else if s === 'ok'}<Pill tone="ok">Al día</Pill>{/if}
          </div>
          <p class="text-xs text-app-muted">{kindLabel(v.kind)} · aplicada {d(v.applied_on)}{v.next_due ? ` · refuerzo ${d(v.next_due)}` : ''}</p>
          {#if v.dose || v.lot}<p class="text-sm">{[v.dose, v.lot && `Lote ${v.lot}`].filter(Boolean).join(' · ')}</p>{/if}
          {#if v.notes}<p class="text-sm text-app-muted">{v.notes}</p>{/if}
          {#if v.voided_at}<p class="mt-1 text-xs text-app-danger">Anulada por {v.voided_by_name}: {v.void_reason}</p>{/if}
          {#if canWrite && !v.voided_at}<button type="button" class="btn-ghost mt-1 text-app-danger" onclick={() => { voiding = v; reason = ''; voidOp.reset(); }}><Icon name="ban" size={16} />Anular</button>{/if}
        </li>
      {/each}
    </ul>
    {#if voidedCount}
      <label class="mt-3 inline-flex items-center gap-2 text-sm text-app-muted"><input type="checkbox" bind:checked={showVoided} />Mostrar {voidedCount} anulada{voidedCount === 1 ? '' : 's'}</label>
    {/if}
  {/if}

  <section class="card mt-6 p-4 sm:p-5" aria-labelledby="weights-h">
    <h3 id="weights-h" class="display mb-2 text-xl">Peso</h3>
    <WeightChart points={weights} />
  </section>
{/if}

<Modal open={adding} title="Registrar aplicación" onclose={() => (adding = false)} wide>
  <form id="vacc-form" class="space-y-4" onsubmit={(e) => { e.preventDefault(); submit(); }}>
    <div>
      <label class="label" for="v-kind">Tipo</label>
      <select id="v-kind" class="field" bind:value={form.kind}>{#each KINDS.filter((k) => animal || k.v === 'vaccine' || k.v === 'other') as k}<option value={k.v}>{k.label}</option>{/each}</select>
    </div>
    {#if shownSuggestions.length}
      <div>
        <p class="section-title mb-1.5">Sugerencias{animal && patient.profile?.species ? ` para ${patient.profile.species}` : ''}</p>
        <div class="flex flex-wrap gap-1.5">
          {#each shownSuggestions as s (s.name)}
            <button type="button" class="rounded-full px-3 py-1.5 text-sm ring-1 ring-inset ring-app-ink/15 hover:bg-app-elevated {form.name === s.name ? 'bg-app-primary/10 ring-app-primary' : ''}" title={s.note} onclick={() => suggest(s)}>{s.name}</button>
          {/each}
        </div>
        <p class="hint">Son sugerencias editables; el criterio clínico es tuyo.</p>
      </div>
    {/if}
    <div class="grid gap-4 sm:grid-cols-2">
      <div class="sm:col-span-2">
        <label class="label" for="v-name">Nombre</label>
        <input id="v-name" class="field" bind:value={form.name} maxlength="120" required oninput={refreshNext} />
      </div>
      <div>
        <label class="label" for="v-on">Fecha de aplicación</label>
        <input id="v-on" type="date" class="field" max={todayIso()} bind:value={form.applied_on} required onchange={refreshNext} />
      </div>
      <div>
        <label class="label" for="v-next">Próximo refuerzo</label>
        <input id="v-next" type="date" class="field" min={form.applied_on ? addDays(form.applied_on, 1) : undefined} bind:value={form.next_due} oninput={() => (nextTouched = true)} />
        <p class="hint">Se calcula con el esquema sugerido; puedes cambiarlo o dejarlo vacío.</p>
      </div>
      <div>
        <label class="label" for="v-dose">Dosis</label>
        <input id="v-dose" class="field" bind:value={form.dose} maxlength="60" placeholder="Ej. 1 ml" />
      </div>
      <div>
        <label class="label" for="v-lot">Lote</label>
        <input id="v-lot" class="field" bind:value={form.lot} maxlength="60" />
      </div>
      <div class="sm:col-span-2">
        <label class="label" for="v-notes">Notas</label>
        <input id="v-notes" class="field" bind:value={form.notes} maxlength="500" />
      </div>
    </div>
    <OpError op={addOp} />
  </form>
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={() => (adding = false)}>Cancelar</button>
    <button type="submit" form="vacc-form" class="btn-primary" disabled={addOp.phase === 'loading'}>{#if addOp.phase === 'loading'}<span class="spin"></span>{/if}Guardar</button>
  {/snippet}
</Modal>

<ConfirmModal open={!!voiding} title="Anular aplicación" op={voidOp} onconfirm={confirmVoid} onclose={() => (voiding = null)} confirmLabel="Anular">
  <p>El registro seguirá en el carnet marcado como anulado, con tu nombre y el motivo. No se puede deshacer.</p>
  <label class="label mt-4" for="vv-reason">Motivo</label>
  <input id="vv-reason" class="field" bind:value={reason} autocomplete="off" maxlength="300" />
</ConfirmModal>
