<script lang="ts">
  import { untrack } from 'svelte';
  import { api } from '$lib/api';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { InvoiceRequest } from '$lib/types';
  import Modal from '$lib/components/Modal.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { UUID_RE } from './fiscal';

  interface Props {
    invoice: InvoiceRequest | null;
    onclose: () => void;
    ondone: () => void;
  }
  let { invoice, onclose, ondone }: Props = $props();

  let uuid = $state('');
  let note = $state('');
  let touched = $state(false);
  const op = new Op();
  const uuidOk = $derived(UUID_RE.test(uuid.trim()));

  $effect(() => {
    if (!invoice) return;
    untrack(() => {
      uuid = note = '';
      touched = false;
      op.reset();
    });
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    touched = true;
    if (!invoice || !uuidOk) return;
    const id = invoice.id;
    if (await op.run(() => api.pos.updateInvoice(id, { status: 'issued', fiscal_uuid: uuid.trim().toLowerCase(), note: note.trim() }))) {
      toast.show('Factura marcada como emitida');
      ondone();
      onclose();
    }
  }
</script>

<Modal open={!!invoice} title="Marcar como emitida" {onclose}>
  {#if invoice}
    <p class="mb-4 text-sm text-app-muted">Venta #{invoice.folio} · {invoice.legal_name} ({invoice.rfc}). Captura el folio fiscal que te dio tu contador o tu proveedor al timbrar el CFDI.</p>
    <form id="issue-form" class="grid gap-4" onsubmit={submit} novalidate>
      <div>
        <label class="label" for="iss-uuid">Folio fiscal (UUID)</label>
        <input id="iss-uuid" class="field font-mono text-[13px]" bind:value={uuid} placeholder="xxxxxxxx-xxxx-4xxx-xxxx-xxxxxxxxxxxx" autocomplete="off" spellcheck="false" aria-invalid={touched && !uuidOk} />
        {#if touched && !uuidOk}<p class="mt-1 text-xs text-app-danger">El UUID debe tener el formato 8-4-4-4-12 (versión 4).</p>{/if}
      </div>
      <div>
        <label class="label" for="iss-note">Nota <span class="font-normal text-app-muted">(opcional)</span></label>
        <input id="iss-note" class="field" bind:value={note} maxlength="200" />
      </div>
      {#if op.phase === 'error'}<p class="alert" role="alert"><Icon name="alert" size={18} />{op.message}</p>{/if}
    </form>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
    <button type="submit" form="issue-form" class="btn-primary" disabled={op.phase === 'loading'}>
      {#if op.phase === 'loading'}<span class="spin"></span>{/if}Marcar como emitida
    </button>
  {/snippet}
</Modal>
