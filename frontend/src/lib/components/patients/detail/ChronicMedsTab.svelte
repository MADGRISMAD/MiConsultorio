<script lang="ts">
  import { Loader } from '$lib/loader.svelte';
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onMount } from 'svelte';
  import { rxApi } from '$lib/api/rx';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { Patient } from '$lib/types';
  import type { ChronicMed, RxCheck } from '$lib/types/rx';
  import ConfirmModal from '../../ConfirmModal.svelte';
  import Modal from '../../Modal.svelte';
  import EmptyState from '../../ui/EmptyState.svelte';
  import Icon from '../../ui/Icon.svelte';
  import Pill from '../../ui/Pill.svelte';

  let { patient, canWrite }: { patient: Patient; canWrite: boolean } = $props();

  let list = $state<ChronicMed[]>([]);
  const ld = new Loader('No se pudo cargar la medicación crónica.');
  let showStopped = $state(false);

  const d = (iso: string | null) => (iso ? new Date(`${iso.slice(0, 10)}T12:00:00`).toLocaleDateString('es-MX', { day: 'numeric', month: 'short', year: 'numeric' }) : '—');
  const iso = (dt: Date) => `${dt.getFullYear()}-${String(dt.getMonth() + 1).padStart(2, '0')}-${String(dt.getDate()).padStart(2, '0')}`;
  const todayIso = () => iso(new Date());
  const inDays = (n: number) => {
    const x = new Date();
    x.setDate(x.getDate() + n);
    return iso(x);
  };

  async function load() {
    await ld.run(async () => {
      list = await rxApi.chronic.list(patient.id);
    });
  }
  onMount(load);

  const active = $derived(list.filter((m) => m.active));
  const stopped = $derived(list.filter((m) => !m.active));
  const renewal = (m: ChronicMed): 'late' | 'soon' | 'ok' | 'none' => {
    if (!m.next_renewal) return 'none';
    if (m.next_renewal < todayIso()) return 'late';
    return m.next_renewal <= inDays(15) ? 'soon' : 'ok';
  };

  // ---- add ----
  let adding = $state(false);
  let form = $state({ name: '', dose: '', frequency: '', indication: '', started_on: '', next_renewal: '' });
  const addOp = new Op();
  let alerts = $state<{ allergies: RxCheck['allergies']; interactions: RxCheck['interactions'] } | null>(null);

  function openAdd() {
    form = { name: '', dose: '', frequency: '', indication: '', started_on: '', next_renewal: '' };
    addOp.reset();
    adding = true;
  }
  async function submit() {
    if (!form.name.trim()) return addOp.fail('Escribe el nombre del medicamento.');
    let found: typeof alerts = null;
    const ok = await addOp.run(async () => {
      const r = await rxApi.chronic.add(patient.id, {
        name: form.name.trim(),
        dose: form.dose.trim(),
        frequency: form.frequency.trim(),
        indication: form.indication.trim(),
        started_on: form.started_on || null,
        next_renewal: form.next_renewal || null
      });
      found = r.alerts;
      return true;
    });
    if (!ok) return;
    adding = false;
    toast.show('Medicamento agregado');
    const a = found as typeof alerts;
    alerts = a && (a.allergies.length || a.interactions.length) ? a : null;
    load();
  }

  // ---- renew ----
  let renewing = $state<ChronicMed | null>(null);
  let renewDate = $state('');
  const renewOp = new Op();
  async function confirmRenew() {
    const m = renewing;
    if (!m) return;
    if (await renewOp.run(() => rxApi.chronic.update(patient.id, m.id, { next_renewal: renewDate || null }))) {
      renewing = null;
      toast.show('Renovación registrada');
      load();
    }
  }

  // ---- stop ----
  let stopping = $state<ChronicMed | null>(null);
  let reason = $state('');
  const stopOp = new Op();
  async function confirmStop() {
    const m = stopping;
    if (!m) return;
    if (!reason.trim()) return stopOp.fail('Escribe el motivo de la suspensión.');
    if (await stopOp.run(() => rxApi.chronic.update(patient.id, m.id, { stop: true, stopped_reason: reason.trim() }))) {
      stopping = null;
      toast.show('Medicamento suspendido');
      load();
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
      Medicamentos que {patient.names} toma de forma continua. Se revisan contra las alergias y contra lo que se receta, para avisarte de interacciones. Los suspendidos quedan en el historial.
    </p>
    {#if canWrite}<button type="button" class="btn-primary" onclick={openAdd}><Icon name="plus" size={18} />Agregar medicamento</button>{/if}
  </div>

  {#if alerts}
    <div class="mb-4 rounded-2xl bg-app-warning/12 p-4" role="status">
      <p class="flex items-center gap-2 text-sm font-semibold"><Icon name="alert" size={18} />Revisa antes de continuar</p>
      <ul class="mt-1 space-y-0.5 text-sm">
        {#each alerts.allergies as a}<li>{a.message}</li>{/each}
        {#each alerts.interactions as h}<li><strong>{h.severity === 'grave' ? 'Grave' : 'Moderada'}:</strong> {h.message}</li>{/each}
      </ul>
      <button type="button" class="btn-ghost mt-1" onclick={() => (alerts = null)}>Entendido</button>
    </div>
  {/if}

  {#if list.length === 0}
    <div class="card"><EmptyState icon="droplet" title="Sin medicación crónica" text="Registra lo que {patient.names} toma de forma continua para detectar interacciones al recetar.">
      {#if canWrite}<button type="button" class="btn-primary" onclick={openAdd}><Icon name="plus" size={18} />Agregar medicamento</button>{/if}
    </EmptyState></div>
  {:else}
    <ul class="space-y-3">
      {#each active as m (m.id)}
        {@const r = renewal(m)}
        <li class="card p-4">
          <div class="flex flex-wrap items-center gap-2">
            <span class="font-medium">{m.name}</span>
            {#if r === 'late'}<Pill tone="bad">Renovación vencida</Pill>{:else if r === 'soon'}<Pill tone="warn">Por renovar</Pill>{/if}
          </div>
          <p class="text-sm">{[m.dose, m.frequency].filter(Boolean).join(' · ') || 'Sin dosis registrada'}</p>
          {#if m.indication}<p class="text-sm text-app-muted">Para: {m.indication}</p>{/if}
          <p class="text-xs text-app-muted">{m.started_on ? `Desde ${d(m.started_on)}` : ''}{m.started_on && m.next_renewal ? ' · ' : ''}{m.next_renewal ? `Renovar ${d(m.next_renewal)}` : ''}</p>
          {#if canWrite}
            <div class="mt-1 flex flex-wrap gap-1">
              <button type="button" class="btn-ghost" onclick={() => { renewing = m; renewDate = m.next_renewal ?? ''; renewOp.reset(); }}><Icon name="refresh" size={16} />Renovar</button>
              <button type="button" class="btn-ghost text-app-danger" onclick={() => { stopping = m; reason = ''; stopOp.reset(); }}><Icon name="ban" size={16} />Suspender</button>
            </div>
          {/if}
        </li>
      {/each}
    </ul>
    {#if stopped.length}
      <label class="mt-3 inline-flex items-center gap-2 text-sm text-app-muted"><input type="checkbox" bind:checked={showStopped} />Mostrar {stopped.length} suspendido{stopped.length === 1 ? '' : 's'}</label>
      {#if showStopped}
        <ul class="mt-3 space-y-3">
          {#each stopped as m (m.id)}
            <li class="card p-4 opacity-70">
              <span class="font-medium line-through">{m.name}</span>
              <p class="text-sm">{[m.dose, m.frequency].filter(Boolean).join(' · ')}</p>
              <p class="text-xs text-app-muted">Suspendido {d(m.stopped_at)}: {m.stopped_reason}</p>
            </li>
          {/each}
        </ul>
      {/if}
    {/if}
  {/if}
{/if}

<Modal open={adding} title="Agregar medicamento crónico" onclose={() => (adding = false)} wide>
  <form id="chronic-form" class="space-y-4" onsubmit={(e) => { e.preventDefault(); submit(); }}>
    <div class="grid gap-4 sm:grid-cols-2">
      <div class="sm:col-span-2">
        <label class="label" for="c-name">Medicamento</label>
        <input id="c-name" class="field" bind:value={form.name} maxlength="120" required />
      </div>
      <div>
        <label class="label" for="c-dose">Dosis</label>
        <input id="c-dose" class="field" bind:value={form.dose} maxlength="60" placeholder="Ej. 850 mg" />
      </div>
      <div>
        <label class="label" for="c-freq">Frecuencia</label>
        <input id="c-freq" class="field" bind:value={form.frequency} maxlength="60" placeholder="Ej. cada 12 horas" />
      </div>
      <div class="sm:col-span-2">
        <label class="label" for="c-ind">Para qué lo toma</label>
        <input id="c-ind" class="field" bind:value={form.indication} maxlength="200" />
      </div>
      <div>
        <label class="label" for="c-start">Desde</label>
        <input id="c-start" type="date" class="field" max={todayIso()} bind:value={form.started_on} />
      </div>
      <div>
        <label class="label" for="c-renew">Próxima renovación</label>
        <input id="c-renew" type="date" class="field" bind:value={form.next_renewal} />
      </div>
    </div>
    <OpError op={addOp} />
  </form>
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={() => (adding = false)}>Cancelar</button>
    <button type="submit" form="chronic-form" class="btn-primary" disabled={addOp.phase === 'loading'}>{#if addOp.phase === 'loading'}<span class="spin"></span>{/if}Guardar</button>
  {/snippet}
</Modal>

<ConfirmModal open={!!renewing} title="Renovar medicamento" op={renewOp} onconfirm={confirmRenew} onclose={() => (renewing = null)} confirmLabel="Guardar">
  <p>Indica la fecha de la próxima renovación de {renewing?.name}.</p>
  <label class="label mt-4" for="c-rd">Próxima renovación</label>
  <input id="c-rd" type="date" class="field" bind:value={renewDate} />
</ConfirmModal>

<ConfirmModal open={!!stopping} title="Suspender medicamento" op={stopOp} onconfirm={confirmStop} onclose={() => (stopping = null)} confirmLabel="Suspender">
  <p>Dejará de revisarse contra nuevas recetas y quedará en el historial con tu nombre y el motivo.</p>
  <label class="label mt-4" for="c-reason">Motivo</label>
  <input id="c-reason" class="field" bind:value={reason} autocomplete="off" maxlength="300" />
</ConfirmModal>
