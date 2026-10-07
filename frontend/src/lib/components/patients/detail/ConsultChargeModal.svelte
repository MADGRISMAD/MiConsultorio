<script lang="ts">
  import { consult } from '$lib/api/consult';
  import { moneyCents } from '$lib/format';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { Charge, ChargeDraftLine } from '$lib/types/consult';
  import Modal from '../../Modal.svelte';
  import Icon from '../../ui/Icon.svelte';
  import Pill from '../../ui/Pill.svelte';
  import ConsultChargeEditor from './ConsultChargeEditor.svelte';
  import { draftsFromCharge, toItems } from './consultLines';

  interface Props {
    open: boolean;
    patientId: string;
    encounterId?: string;
    /** an existing pre-account of the encounter (summary row is enough; its lines are loaded here) */
    charge?: Charge | null;
    onclose: () => void;
    onchanged: () => void;
  }
  let { open, patientId, encounterId, charge = null, onclose, onchanged }: Props = $props();

  const STATUS: Record<string, { label: string; tone: 'info' | 'ok' | 'warn' | 'bad' | 'muted' }> = {
    draft: { label: 'Borrador', tone: 'muted' },
    sent: { label: 'Enviada a caja', tone: 'info' },
    charged: { label: 'Cobrada', tone: 'ok' },
    cancelled: { label: 'Cancelada', tone: 'bad' }
  };

  const cobros = $derived(session.cobros);
  let lines = $state<ChargeDraftLine[]>([]);
  let current = $state<Charge | null>(null);
  let loading = $state(false);
  let confirmCancel = $state(false);
  const op = new Op();
  let wasOpen = false;

  const editable = $derived(!current || current.status === 'draft' || current.status === 'sent');

  $effect(() => {
    if (open && !wasOpen) {
      op.reset();
      confirmCancel = false;
      current = null;
      lines = [];
      if (charge) void load(charge.id);
    }
    wasOpen = open;
  });

  async function load(id: string) {
    loading = true;
    try {
      current = await consult.get(id);
      lines = draftsFromCharge(current);
    } catch (e) {
      op.fail(e instanceof Error ? e.message : 'No se pudo abrir la pre-cuenta.');
    } finally {
      loading = false;
    }
  }

  async function save(send: boolean) {
    if (!lines.length) return op.fail('Agrega al menos un concepto.');
    if (lines.some((l) => !(l.qty > 0))) return op.fail('Revisa las cantidades.');
    let saved: Charge | undefined;
    const ok = await op.run(async () => {
      const body = { items: toItems(lines), send, ...(encounterId ? { encounter_id: encounterId } : {}) };
      saved = current ? await consult.update(current.id, { ...body, patient_id: patientId }) : await consult.create({ ...body, patient_id: patientId });
    });
    if (ok && saved) {
      for (const w of saved.warnings ?? []) toast.show(w, 'error');
      toast.show(send ? 'Pre-cuenta enviada a caja' : 'Pre-cuenta guardada');
      onchanged();
      onclose();
    }
  }

  async function cancel() {
    if (!current) return;
    const c = current;
    const ok = await op.run(async () => {
      const r = await consult.cancel(c.id);
      if (r.consumed_kept) toast.show('Los insumos ya descontados siguen fuera del inventario.');
    });
    if (ok) {
      toast.show('Pre-cuenta cancelada');
      onchanged();
      onclose();
    }
  }
</script>

<Modal {open} title="Pre-cuenta de la consulta" {onclose} wide>
  {#if loading}
    <p class="text-sm text-app-muted" role="status">Cargando…</p>
  {:else}
    {#if current}
      <p class="mb-4 flex flex-wrap items-center gap-2 text-sm">
        <Pill tone={STATUS[current.status].tone}>{STATUS[current.status].label}</Pill>
        <span class="text-app-muted">{current.created_by}</span>
        {#if current.status === 'cancelled' && current.cancel_reason}<span class="text-app-muted">· {current.cancel_reason}</span>{/if}
      </p>
    {/if}
    {#if editable}
      <ConsultChargeEditor bind:lines {cobros} uid="enc-modal" />
    {:else if current}
      <ul class="divide-y divide-app-ink/10 rounded-xl border border-app-ink/10">
        {#each current.items ?? [] as i (i.id)}
          <li class="flex items-center justify-between gap-3 px-3 py-2.5 text-sm">
            <span class="min-w-0 truncate">{i.qty} × {i.name}{i.consumed ? ' · insumo descontado' : ''}</span>
            {#if cobros}<span class="tabular-nums">{moneyCents(i.total_cents)}</span>{/if}
          </li>
        {/each}
      </ul>
    {/if}
    {#if op.phase === 'error'}<p class="alert mt-4" role="alert"><Icon name="alert" size={18} />{op.message}</p>{/if}
    {#if confirmCancel}
      <p class="alert mt-4" role="alertdialog">
        <Icon name="alert" size={18} />
        <span>¿Cancelar esta pre-cuenta? Caja ya no la verá. Los insumos que ya se descontaron siguen fuera del inventario.</span>
        <button type="button" class="btn-ghost ml-auto min-h-9 text-app-danger" onclick={cancel}>Sí, cancelar</button>
        <button type="button" class="btn-ghost min-h-9" onclick={() => (confirmCancel = false)}>No</button>
      </p>
    {/if}
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>Cerrar</button>
    {#if editable && !loading}
      {#if current}<button type="button" class="btn-ghost text-app-danger" disabled={op.phase === 'loading'} onclick={() => (confirmCancel = true)}>Cancelar pre-cuenta</button>{/if}
      <button type="button" class="btn-secondary" disabled={op.phase === 'loading'} onclick={() => save(false)}>Guardar</button>
      {#if cobros}
        <button type="button" class="btn-primary" disabled={op.phase === 'loading'} onclick={() => save(true)}>
          {#if op.phase === 'loading'}<span class="spin"></span>{/if}{current?.status === 'sent' ? 'Guardar cambios' : 'Enviar a caja'}
        </button>
      {/if}
    {/if}
  {/snippet}
</Modal>
