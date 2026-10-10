<script lang="ts">
  import { api } from '$lib/api';
  import { Op } from '$lib/op.svelte';
  import { printReceta } from '$lib/print';
  import { toast } from '$lib/toast.svelte';
  import type { Patient, PatientSchema, Prescription } from '$lib/types';
  import ConfirmModal from '../../ConfirmModal.svelte';
  import Modal from '../../Modal.svelte';
  import EmptyState from '../../ui/EmptyState.svelte';
  import Icon from '../../ui/Icon.svelte';
  import Pill from '../../ui/Pill.svelte';

  interface Props {
    patient: Patient;
    schema: PatientSchema;
    prescriptions: Prescription[];
    canWrite: boolean;
    userName: string;
    isAdmin: boolean;
    onnew: () => void;
    onchange: () => void;
  }
  let { patient, schema, prescriptions, canWrite, userName, isAdmin, onnew, onchange }: Props = $props();
  const instr = $derived(schema.rx_mode === 'instructions');
  const noun = $derived(instr ? 'hoja de indicaciones' : 'receta');

  const d = (iso: string | null) => (iso ? new Date(iso.length === 10 ? `${iso}T12:00:00` : iso).toLocaleDateString('es-MX', { day: 'numeric', month: 'short', year: 'numeric' }) : '—');
  const expired = (r: Prescription) => !!r.valid_until && new Date(`${r.valid_until.slice(0, 10)}T23:59:59`) < new Date();
  const sorted = $derived([...prescriptions].sort((a, b) => b.issued_at.localeCompare(a.issued_at)));

  let viewing = $state<Prescription | null>(null);
  let voiding = $state<Prescription | null>(null);
  let reason = $state('');
  const voidOp = new Op();
  let busy = $state('');

  async function print(r: Prescription) {
    busy = r.id;
    try {
      await printReceta(r.id);
    } catch (e) {
      toast.show(e instanceof Error ? e.message : 'No se pudo imprimir.', 'error');
    } finally {
      busy = '';
    }
  }
  function startVoid(r: Prescription) {
    voiding = r;
    reason = '';
    voidOp.reset();
  }
  async function confirmVoid() {
    const r = voiding;
    if (!r) return;
    if (!reason.trim()) return voidOp.fail('Escribe el motivo de la cancelación.');
    if (await voidOp.run(() => api.prescriptions.void(r.id, reason.trim()))) {
      voiding = null;
      toast.show('Cancelada');
      onchange();
    }
  }
  const canVoid = (r: Prescription) => !r.voided_at && canWrite && (isAdmin || r.author_name === userName);
</script>

<div class="mb-4 flex flex-wrap items-center justify-between gap-3">
  <p class="max-w-xl text-sm text-app-muted">{instr ? 'Hojas de indicaciones emitidas.' : 'Recetas emitidas, con vigencia máxima de 30 días.'} Una receta nueva deja vencidas las anteriores (las complementarias no); solo se cancelan las que tú canceles. Todas se conservan en la lista.</p>
  {#if canWrite}<button type="button" class="btn-primary" onclick={onnew}><Icon name="plus" size={18} />{instr ? 'Nueva hoja' : 'Nueva receta'}</button>{/if}
</div>

{#if sorted.length === 0}
  <div class="card"><EmptyState icon="receipt" title="Sin {instr ? 'hojas de indicaciones' : 'recetas'}" text="Aquí aparecerán las que emitas para {patient.names}.">
    {#if canWrite}<button type="button" class="btn-primary" onclick={onnew}><Icon name="plus" size={18} />{instr ? 'Nueva hoja' : 'Nueva receta'}</button>{/if}
  </EmptyState></div>
{:else}
  <ul class="space-y-3">
    {#each sorted as r (r.id)}
      <li class="card p-4 sm:p-5 {r.voided_at ? 'opacity-80' : ''}">
        <div class="flex flex-wrap items-center gap-2">
          <span class="font-mono text-sm font-semibold">Folio {String(r.folio).padStart(6, '0')}</span>
          {#if r.voided_at}<Pill tone="bad">Cancelada</Pill>{:else if r.superseded_at || expired(r)}<Pill tone="warn">Vencida</Pill>{:else}<Pill tone="ok">Vigente</Pill>{/if}
          <span class="text-sm text-app-muted">{d(r.issued_at)}{r.valid_until ? ` · vigente hasta ${d(r.valid_until)}` : ''}</span>
        </div>
        <p class="mt-2 break-words text-sm {r.voided_at ? 'line-through' : ''}">
          {r.diagnosis || 'Sin diagnóstico'}
          <span class="text-app-muted">· {r.mode === 'instructions' ? 'Indicaciones' : `${r.items.length} ${r.items.length === 1 ? 'medicamento' : 'medicamentos'}`}</span>
        </p>
        {#if r.superseded_at && !r.voided_at}<p class="mt-1 text-xs text-app-muted">Vencida el {d(r.superseded_at)}: la reemplazó la receta {r.superseded_by_folio ? `folio ${String(r.superseded_by_folio).padStart(6, '0')}` : 'más reciente'}.</p>{/if}
        {#if r.voided_at}<p class="mt-1 text-xs text-app-danger">Cancelada el {d(r.voided_at)}{r.voided_by ? ` por ${r.voided_by}` : ''}{r.void_reason ? `: ${r.void_reason}` : ''}</p>{/if}
        <div class="mt-3 flex flex-wrap gap-1 border-t border-app-ink/8 pt-3">
          <button type="button" class="btn-ghost" onclick={() => (viewing = r)}><Icon name="eye" size={16} />Ver</button>
          <button type="button" class="btn-ghost" disabled={busy === r.id} onclick={() => print(r)}><Icon name="receipt" size={16} />Imprimir receta</button>
          {#if canVoid(r)}<button type="button" class="btn-ghost text-app-danger" onclick={() => startVoid(r)}><Icon name="ban" size={16} />Cancelar</button>{/if}
        </div>
      </li>
    {/each}
  </ul>
{/if}

<Modal open={!!viewing} title={viewing ? `Folio ${String(viewing.folio).padStart(6, '0')}` : ''} onclose={() => (viewing = null)}>
  {#if viewing}
    {@const r = viewing}
    <div class="space-y-3 text-sm">
      <p class="text-app-muted">{d(r.issued_at)} · {r.author_name}{r.author_title ? `, ${r.author_title}` : ''} · Cédula {r.author_license}</p>
      {#if r.diagnosis}<p><span class="section-title block">Diagnóstico</span>{r.diagnosis}</p>{/if}
      {#each r.items as it}
        <div class="rounded-xl bg-app-ink/5 p-3">
          <p class="font-medium">{it.medicine}{it.brand ? ` (${it.brand})` : ''}</p>
          <p class="text-app-muted">{[it.presentation, it.dose, it.route, it.frequency, it.duration, it.quantity && `Cantidad: ${it.quantity}`].filter(Boolean).join(' · ')}</p>
          {#if it.notes}<p>{it.notes}</p>{/if}
          {#if it.control !== 'No'}<p class="mt-1"><Pill tone="warn">{it.control}</Pill></p>{/if}
        </div>
      {/each}
      {#if r.instructions}<p class="whitespace-pre-line"><span class="section-title block">Indicaciones</span>{r.instructions}</p>{/if}
      {#if r.next_visit}<p><span class="section-title block">Próxima cita</span>{d(r.next_visit)}</p>{/if}
    </div>
  {/if}
</Modal>

<ConfirmModal open={!!voiding} title="Cancelar {noun}" op={voidOp} onconfirm={confirmVoid} onclose={() => (voiding = null)} confirmLabel="Cancelar {noun}">
  <p>Seguirá en la lista marcada como cancelada, con tu nombre y el motivo. No se puede deshacer.</p>
  <label class="label mt-4" for="void-reason">Motivo</label>
  <input id="void-reason" class="field" bind:value={reason} autocomplete="off" />
</ConfirmModal>
