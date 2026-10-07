<script lang="ts">
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { specialtyApi } from '$lib/api/specialty';
  import { moneyCents } from '$lib/format';
  import { Op } from '$lib/op.svelte';
  import { printPlan } from '$lib/print';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { Encounter, Patient, PatientSchema } from '$lib/types';
  import type { PlanItem, PlanStatus, SignatureInput, TreatmentPlan } from '$lib/types/specialty';
  import ConfirmModal from '../../ConfirmModal.svelte';
  import Modal from '../../Modal.svelte';
  import ConsentSection from '../../specialty/ConsentSection.svelte';
  import PlanEditor from '../../specialty/PlanEditor.svelte';
  import PlanHistory from '../../specialty/PlanHistory.svelte';
  import SignBlock from '../../specialty/SignBlock.svelte';
  import EmptyState from '../../ui/EmptyState.svelte';
  import Icon from '../../ui/Icon.svelte';
  import Pill from '../../ui/Pill.svelte';

  let { patient, schema, canWrite }: { patient: Patient; schema: PatientSchema | null; canWrite: boolean; isAdmin: boolean } = $props();

  const animal = $derived(patient.subject === 'animal');
  const dental = $derived((schema?.kinds ?? []).includes('DENTAL'));
  const patientName = $derived(`${patient.names} ${patient.last_names}`.trim());
  const canCharge = $derived(session.cobros && session.has('pos'));

  let plans = $state<TreatmentPlan[]>([]);
  let loading = $state(true);
  let error = $state('');
  let openId = $state<string | null>(null);
  let encounters = $state<Encounter[]>([]);
  let consentVersion = $state(0);

  const STATUS: Record<PlanStatus, { label: string; tone: 'info' | 'ok' | 'warn' | 'bad' | 'muted' }> = {
    draft: { label: 'Borrador', tone: 'muted' },
    proposed: { label: 'Propuesto', tone: 'info' },
    accepted: { label: 'Aceptado', tone: 'ok' },
    in_progress: { label: 'En curso', tone: 'warn' },
    completed: { label: 'Completado', tone: 'ok' },
    cancelled: { label: 'Cancelado', tone: 'bad' }
  };
  const d = (iso: string | null) => (iso ? new Date(iso).toLocaleDateString('es-MX', { day: 'numeric', month: 'short', year: 'numeric' }) : '—');

  async function load() {
    try {
      plans = await specialtyApi.plans(patient.id);
      error = '';
    } catch (e) {
      error = e instanceof Error ? e.message : 'No se pudieron cargar los planes.';
    } finally {
      loading = false;
    }
  }
  onMount(() => {
    load();
    if (canWrite) api.patients.encounters(patient.id).then((l) => (encounters = l.filter((x) => !x.hidden && !x.addendum_of))).catch(() => {});
  });
  /** a response from the server replaces the plan in the list */
  function replace(p: TreatmentPlan) {
    plans = plans.some((x) => x.id === p.id) ? plans.map((x) => (x.id === p.id ? p : x)) : [p, ...plans];
  }
  const phases = (p: TreatmentPlan) => [...new Set(p.items.map((i) => i.phase))].sort((a, b) => a - b);
  const pct = (p: TreatmentPlan) => (p.done_cents + p.pending_cents > 0 ? Math.round((p.done_cents / (p.done_cents + p.pending_cents)) * 100) : 0);
  const needsSignature = (p: TreatmentPlan) => p.status === 'draft' || p.status === 'proposed' || ((p.status === 'accepted' || p.status === 'in_progress') && p.version > p.accepted_version);

  // ---- editor ----
  let editor = $state<{ mode: 'create' | 'edit' | 'add'; plan: TreatmentPlan | null } | null>(null);

  // ---- generic action with feedback ----
  let busy = $state('');
  async function act(key: string, fn: () => Promise<TreatmentPlan>, ok: string) {
    busy = key;
    try {
      replace(await fn());
      toast.show(ok);
    } catch (e) {
      toast.show(e instanceof Error ? e.message : 'No se pudo completar la acción.', 'error');
    } finally {
      busy = '';
    }
  }

  // ---- accept with signature ----
  let accepting = $state<TreatmentPlan | null>(null);
  let sig = $state<SignatureInput>({ signer_name: '', signer_role: 'paciente', signature_png: '', witness1: '', witness2: '' });
  const acceptOp = new Op();
  function startAccept(p: TreatmentPlan) {
    sig = { signer_name: '', signer_role: animal ? 'propietario' : 'paciente', signature_png: '', witness1: '', witness2: '' };
    acceptOp.reset();
    accepting = p;
  }
  async function confirmAccept() {
    const p = accepting;
    if (!p) return;
    if (!sig.signer_name.trim()) return acceptOp.fail('Escribe el nombre de quien firma.');
    if (!sig.signature_png) return acceptOp.fail('Falta la firma.');
    if (await acceptOp.run(async () => replace(await specialtyApi.acceptPlan(p.id, sig)))) {
      accepting = null;
      consentVersion++;
      toast.show('Plan aceptado y firmado');
    }
  }

  // ---- item done / cancel ----
  let doing = $state<{ plan: TreatmentPlan; item: PlanItem } | null>(null);
  let encounterId = $state('');
  const doneOp = new Op();
  async function confirmDone() {
    const t = doing;
    if (!t) return;
    if (await doneOp.run(async () => replace(await specialtyApi.doneItem(t.plan.id, t.item.id, encounterId || undefined)))) {
      doing = null;
      toast.show('Concepto marcado como realizado');
    }
  }
  let dropping = $state<{ plan: TreatmentPlan; item: PlanItem } | null>(null);
  let dropReason = $state('');
  const dropOp = new Op();
  async function confirmDrop() {
    const t = dropping;
    if (!t) return;
    if (!dropReason.trim()) return dropOp.fail('Escribe el motivo.');
    if (await dropOp.run(async () => replace(await specialtyApi.cancelItem(t.plan.id, t.item.id, dropReason.trim())))) {
      dropping = null;
      toast.show('Concepto cancelado');
    }
  }
  let cancelling = $state<TreatmentPlan | null>(null);
  let cancelReason = $state('');
  const cancelOp = new Op();
  async function confirmCancel() {
    const p = cancelling;
    if (!p) return;
    if (!cancelReason.trim()) return cancelOp.fail('Escribe el motivo.');
    if (await cancelOp.run(async () => replace(await specialtyApi.cancelPlan(p.id, cancelReason.trim())))) {
      cancelling = null;
      toast.show('Plan cancelado');
    }
  }
  async function print(p: TreatmentPlan) {
    busy = `print-${p.id}`;
    try {
      await printPlan(patient, p.id);
    } catch (e) {
      toast.show(e instanceof Error ? e.message : 'No se pudo imprimir.', 'error');
    } finally {
      busy = '';
    }
  }
</script>

<div class="mb-4 flex flex-wrap items-center justify-between gap-3">
  <p class="max-w-xl text-sm text-app-muted">Planes por fases con costo. Al aceptarlos con firma quedan conservados: después solo se agregan conceptos (nueva versión) o se cancelan los pendientes.</p>
  {#if canWrite}<button type="button" class="btn-primary" onclick={() => (editor = { mode: 'create', plan: null })}><Icon name="plus" size={18} />Nuevo plan</button>{/if}
</div>

{#if loading}
  <div class="card h-32 animate-pulse"></div>
{:else if error}
  <p class="alert" role="alert"><Icon name="alert" size={18} />{error}</p>
{:else if plans.length === 0}
  <div class="card"><EmptyState icon="clock-plus" title="Sin planes de tratamiento" text="Arma un plan por fases para {patient.names}, preséntalo y regístralo con firma.">
    {#if canWrite}<button type="button" class="btn-primary" onclick={() => (editor = { mode: 'create', plan: null })}><Icon name="plus" size={18} />Nuevo plan</button>{/if}
  </EmptyState></div>
{:else}
  <ul class="space-y-3">
    {#each plans as p (p.id)}
      {@const open = openId === p.id}
      <li class="card p-4 sm:p-5">
        <button type="button" class="flex w-full flex-wrap items-center justify-between gap-2 text-left" aria-expanded={open} aria-controls="plan-{p.id}" onclick={() => (openId = open ? null : p.id)}>
          <span class="min-w-0">
            <span class="display block truncate text-xl">{p.title}</span>
            <span class="block text-xs text-app-muted">{d(p.created_at)} · {p.created_by_name} · versión {p.version}</span>
          </span>
          <span class="flex items-center gap-2">
            <Pill tone={STATUS[p.status].tone}>{STATUS[p.status].label}</Pill>
            <strong class="text-sm">{moneyCents(p.total_cents)}</strong>
            <Icon name="chevron-down" size={18} class="transition {open ? 'rotate-180' : ''}" />
          </span>
        </button>
        {#if p.status !== 'draft' && p.status !== 'proposed' && p.status !== 'cancelled'}
          <div class="mt-3 h-1.5 overflow-hidden rounded-full bg-app-ink/10" role="progressbar" aria-label="Avance del plan" aria-valuemin="0" aria-valuemax="100" aria-valuenow={pct(p)}>
            <div class="h-full bg-app-primary" style="width:{pct(p)}%"></div>
          </div>
        {/if}

        {#if open}
          <div id="plan-{p.id}" class="mt-4 space-y-4">
            {#if p.notes}<p class="whitespace-pre-line text-sm text-app-muted">{p.notes}</p>{/if}
            {#if p.status === 'cancelled'}<p class="text-sm text-app-danger">Cancelado: {p.cancel_reason}</p>{/if}
            {#if p.accepted_at}<p class="text-xs text-app-muted">Aceptado por {p.accepted_by_name} el {d(p.accepted_at)} (versión {p.accepted_version}).</p>{/if}
            {#if needsSignature(p) && p.accepted_version > 0}<p class="rounded-xl bg-app-warning/12 p-3 text-sm" role="status">Hay conceptos nuevos (versión {p.version}) que aún no tienen firma.</p>{/if}

            {#each phases(p) as ph}
              {@const list = p.items.filter((i) => i.phase === ph)}
              <section aria-label="Fase {ph}">
                <h4 class="section-title mb-1">Fase {ph} · {moneyCents(list.filter((i) => i.status !== 'cancelled').reduce((s, i) => s + i.total_cents, 0))}</h4>
                <ul class="divide-y divide-app-ink/8 rounded-2xl border border-app-ink/10">
                  {#each list as it (it.id)}
                    <li class="flex flex-wrap items-center justify-between gap-2 px-3 py-2.5 {it.status === 'cancelled' ? 'opacity-60' : ''}">
                      <div class="min-w-0 flex-1">
                        <p class="text-sm {it.status === 'cancelled' ? 'line-through' : ''}">{it.description}{#if it.tooth}<span class="ml-1 rounded bg-app-ink/8 px-1.5 text-xs">{it.tooth}</span>{/if}{#if it.version_added > 1}<span class="ml-1 text-xs text-app-muted">v{it.version_added}</span>{/if}</p>
                        <p class="text-xs text-app-muted">{it.qty} × {moneyCents(it.unit_price_cents)} = {moneyCents(it.total_cents)}
                          {#if it.status === 'done'}· realizado {d(it.done_at)} por {it.done_by_name}{/if}
                          {#if it.status === 'cancelled'}· cancelado: {it.cancel_reason}{/if}
                          {#if it.sale_id}· cobrado{/if}</p>
                      </div>
                      <div class="flex items-center gap-1">
                        {#if it.status === 'done'}<Pill tone="ok"><Icon name="check" size={14} />Hecho</Pill>
                        {:else if it.status === 'cancelled'}<Pill tone="bad">Cancelado</Pill>
                        {:else if canWrite && (p.status === 'accepted' || p.status === 'in_progress' || p.status === 'completed')}
                          <button type="button" class="btn-secondary !min-h-9 !px-3" onclick={() => { doing = { plan: p, item: it }; encounterId = ''; doneOp.reset(); }}><Icon name="check" size={16} />Realizado</button>
                          <button type="button" class="icon-btn danger" aria-label="Cancelar concepto {it.description}" onclick={() => { dropping = { plan: p, item: it }; dropReason = ''; dropOp.reset(); }}><Icon name="ban" size={16} /></button>
                        {:else}<Pill>Pendiente</Pill>{/if}
                      </div>
                    </li>
                  {/each}
                </ul>
              </section>
            {/each}
            <div class="flex flex-wrap items-center justify-between gap-2 rounded-2xl bg-app-ink/5 px-4 py-3 text-sm">
              <span>Realizado <strong>{moneyCents(p.done_cents)}</strong> · Pendiente <strong>{moneyCents(p.pending_cents)}</strong></span>
              <span class="text-base">Total <strong>{moneyCents(p.total_cents)}</strong></span>
            </div>

            <div class="flex flex-wrap gap-2 border-t border-app-ink/8 pt-3">
              {#if canWrite && (p.status === 'draft' || p.status === 'proposed')}<button type="button" class="btn-secondary" onclick={() => (editor = { mode: 'edit', plan: p })}><Icon name="edit" size={16} />Editar</button>{/if}
              {#if canWrite && p.status === 'draft'}<button type="button" class="btn-secondary" disabled={busy === `p-${p.id}` || p.items.length === 0} onclick={() => act(`p-${p.id}`, () => specialtyApi.proposePlan(p.id), 'Plan marcado como propuesto')}>Marcar como propuesto</button>{/if}
              {#if canWrite && needsSignature(p) && p.items.length > 0}<button type="button" class="btn-primary" onclick={() => startAccept(p)}><Icon name="edit" size={16} />{p.accepted_version > 0 ? 'Firmar nueva versión' : 'Aceptar con firma'}</button>{/if}
              {#if canWrite && (p.status === 'accepted' || p.status === 'in_progress' || p.status === 'completed')}<button type="button" class="btn-secondary" onclick={() => (editor = { mode: 'add', plan: p })}><Icon name="plus" size={16} />Agregar conceptos</button>{/if}
              {#if canCharge && p.pending_cents > 0 && (p.status === 'accepted' || p.status === 'in_progress')}<button type="button" class="btn-secondary" onclick={() => goto(`/pos/cobros?plan=${p.id}`)}><Icon name="cash" size={16} />Cobrar</button>{/if}
              <button type="button" class="btn-ghost" disabled={busy === `print-${p.id}`} onclick={() => print(p)}><Icon name="receipt" size={16} />Imprimir {p.status === 'draft' || p.status === 'proposed' ? 'presupuesto' : 'plan'}</button>
              {#if canWrite && p.status !== 'completed' && p.status !== 'cancelled'}<button type="button" class="btn-ghost text-app-danger" onclick={() => { cancelling = p; cancelReason = ''; cancelOp.reset(); }}><Icon name="ban" size={16} />Cancelar plan</button>{/if}
            </div>

            <details>
              <summary class="cursor-pointer text-sm text-app-muted">Historial del plan</summary>
              <PlanHistory id={p.id} />
            </details>
          </div>
        {/if}
      </li>
    {/each}
  </ul>
{/if}

<ConsentSection {patient} {canWrite} version={consentVersion} />

{#if editor}
  <PlanEditor
    open
    patientId={patient.id}
    mode={editor.mode}
    plan={editor.plan}
    {dental}
    onclose={() => (editor = null)}
    onsaved={(p) => {
      replace(p);
      openId = p.id;
      editor = null;
      toast.show('Plan guardado');
    }}
  />
{/if}

<Modal open={!!accepting} title="Aceptación del plan" onclose={() => (accepting = null)} wide>
  {#if accepting}
    <div class="space-y-4">
      <div class="rounded-2xl bg-app-ink/5 p-3 text-sm">
        <p class="font-medium">{accepting.title} · versión {accepting.version}</p>
        <ul class="mt-1 list-disc space-y-0.5 pl-5">
          {#each accepting.items.filter((i) => i.status !== 'cancelled') as i (i.id)}<li>Fase {i.phase}: {i.description}{i.tooth ? ` [${i.tooth}]` : ''} — {moneyCents(i.total_cents)}</li>{/each}
        </ul>
        <p class="mt-2 font-semibold">Total {moneyCents(accepting.total_cents)}</p>
      </div>
      <p class="text-sm text-app-muted">Quien firma acepta el plan, sus costos y fases, y que puede modificarse con su autorización. Se guarda este resumen exacto junto con la firma.</p>
      <SignBlock bind:sig {animal} {patientName} />
      {#if acceptOp.phase === 'error'}<p class="alert" role="alert"><Icon name="alert" size={18} />{acceptOp.message}</p>{/if}
    </div>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={() => (accepting = null)}>Cancelar</button>
    <button type="button" class="btn-primary" disabled={acceptOp.phase === 'loading'} onclick={confirmAccept}>{#if acceptOp.phase === 'loading'}<span class="spin"></span>{/if}Firmar y aceptar</button>
  {/snippet}
</Modal>

<Modal open={!!doing} title="Marcar como realizado" onclose={() => (doing = null)}>
  {#if doing}
    <p class="text-sm">{doing.item.description}{doing.item.tooth ? ` [${doing.item.tooth}]` : ''}</p>
    <label class="label mt-4" for="done-enc">Vincular con una consulta de la bitácora (opcional)</label>
    <select id="done-enc" class="field" bind:value={encounterId}>
      <option value="">Sin vincular</option>
      {#each encounters.slice(0, 40) as e (e.id)}<option value={e.id}>{new Date(e.occurred_at).toLocaleDateString('es-MX', { day: 'numeric', month: 'short', year: 'numeric' })} · {e.reason || e.kind}</option>{/each}
    </select>
    {#if doneOp.phase === 'error'}<p class="alert mt-3" role="alert"><Icon name="alert" size={18} />{doneOp.message}</p>{/if}
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={() => (doing = null)}>Cancelar</button>
    <button type="button" class="btn-primary" disabled={doneOp.phase === 'loading'} onclick={confirmDone}>Confirmar</button>
  {/snippet}
</Modal>

<ConfirmModal open={!!dropping} title="Cancelar concepto" op={dropOp} onconfirm={confirmDrop} onclose={() => (dropping = null)} confirmLabel="Cancelar concepto">
  <p>El concepto queda en el plan marcado como cancelado y deja de contar en el total.</p>
  <label class="label mt-4" for="drop-reason">Motivo</label>
  <input id="drop-reason" class="field" bind:value={dropReason} maxlength="300" autocomplete="off" />
</ConfirmModal>

<ConfirmModal open={!!cancelling} title="Cancelar plan" op={cancelOp} onconfirm={confirmCancel} onclose={() => (cancelling = null)} confirmLabel="Cancelar plan">
  <p>El plan y su historial se conservan, marcados como cancelados. No se puede deshacer.</p>
  <label class="label mt-4" for="cancel-reason">Motivo</label>
  <input id="cancel-reason" class="field" bind:value={cancelReason} maxlength="300" autocomplete="off" />
</ConfirmModal>
