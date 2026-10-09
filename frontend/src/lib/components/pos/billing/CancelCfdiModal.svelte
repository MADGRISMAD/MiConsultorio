<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { untrack } from 'svelte';
  import { pos2 } from '$lib/api/pos2';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { InvoiceRequest } from '$lib/types';
  import Modal from '$lib/components/Modal.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { UUID_LOOSE_RE } from './fiscal';

  interface Props {
    invoice: InvoiceRequest | null;
    onclose: () => void;
    ondone: () => void;
  }
  let { invoice, onclose, ondone }: Props = $props();

  const MOTIVES = [
    { id: '02', label: '02 · Comprobante emitido con errores sin relación', hint: 'El caso más común: se cancela y, si hace falta, se vuelve a facturar.' },
    { id: '01', label: '01 · Comprobante emitido con errores con relación', hint: 'Indica el folio fiscal de la factura que lo sustituye.' },
    { id: '03', label: '03 · No se llevó a cabo la operación', hint: '' },
    { id: '04', label: '04 · Operación nominativa relacionada en una factura global', hint: '' }
  ];

  const uid = $props.id();
  const op = new Op();
  let motive = $state('02');
  let replacement = $state('');
  let error = $state('');

  $effect(() => {
    if (!invoice) return;
    untrack(() => {
      motive = '02';
      replacement = '';
      error = '';
      op.reset();
    });
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    const inv = invoice;
    if (!inv) return;
    if (motive === '01' && !UUID_LOOSE_RE.test(replacement.trim())) return void (error = 'Escribe el folio fiscal (UUID) de la factura que sustituye.');
    error = '';
    if (await op.run(() => pos2.cancelCfdi(inv.id, motive, motive === '01' ? replacement.trim() : undefined))) {
      toast.show('CFDI cancelado ante el SAT');
      ondone();
      onclose();
    }
  }
</script>

<Modal open={!!invoice} title="Cancelar CFDI" {onclose}>
  {#if invoice}
    <form id="{uid}-f" class="space-y-4" onsubmit={submit} novalidate>
      <p class="text-sm text-app-muted">Se cancelará la factura <span class="break-all font-mono text-[13px] text-app-ink">{invoice.fiscal_uuid}</span> de la venta #{invoice.folio} ante el SAT. No se puede deshacer.</p>
      <fieldset class="space-y-2">
        <legend class="label">Motivo de cancelación</legend>
        {#each MOTIVES as m (m.id)}
          <label class="flex min-h-11 cursor-pointer items-start gap-3 rounded-xl p-3 ring-1 ring-inset ring-app-ink/15 has-[:checked]:bg-app-primary/10 has-[:checked]:ring-app-primary">
            <input type="radio" class="mt-1" name="{uid}-m" value={m.id} bind:group={motive} />
            <span class="text-sm"><span class="block font-medium">{m.label}</span>{#if m.hint}<span class="block text-xs text-app-muted">{m.hint}</span>{/if}</span>
          </label>
        {/each}
      </fieldset>
      {#if motive === '01'}
        <div>
          <label class="label" for="{uid}-u">Folio fiscal que sustituye</label>
          <input id="{uid}-u" class="field font-mono text-[13px] uppercase" bind:value={replacement} maxlength="36" autocomplete="off" aria-invalid={!!error} />
        </div>
      {/if}
      {#if error}<Alert>{error}</Alert>{/if}
      <OpError op={op} />
    </form>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>No cancelar</button>
    <button type="submit" form="{uid}-f" class="btn-danger" disabled={op.phase === 'loading'}>{#if op.phase === 'loading'}<span class="spin"></span>{/if}Cancelar CFDI</button>
  {/snippet}
</Modal>
