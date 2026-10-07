<script lang="ts">
  import { dateShort, moneyCents } from '$lib/format';
  import { pos2 } from '$lib/api/pos2';
  import { toast } from '$lib/toast.svelte';
  import type { InvoiceRequest } from '$lib/types';
  import Modal from '$lib/components/Modal.svelte';
  import Pill from '$lib/components/ui/Pill.svelte';
  import { cfdiUseLabel, INVOICE_STATUS, invoiceText, regimeLabel } from './fiscal';

  interface Props {
    invoice: InvoiceRequest | null;
    canManage: boolean;
    onclose: () => void;
    onissue: (i: InvoiceRequest) => void;
    oncancel: (i: InvoiceRequest) => void;
    /** Stamp through the PAC; the button always shows and explains when it is not set up. */
    onstamp?: (i: InvoiceRequest) => void;
    stamping?: boolean;
    /** Cancel a stamped CFDI (administrators). */
    oncancelcfdi?: (i: InvoiceRequest) => void;
    canCancelCfdi?: boolean;
  }
  let { invoice, canManage, onclose, onissue, oncancel, onstamp, stamping = false, oncancelcfdi, canCancelCfdi = false }: Props = $props();
  const stamped = $derived(invoice?.cfdi_state === 'stamped');

  async function copy() {
    if (!invoice) return;
    try {
      await navigator.clipboard.writeText(invoiceText(invoice));
      toast.show('Datos copiados');
    } catch {
      toast.show('No se pudo copiar. Selecciona el texto y cópialo.', 'error');
    }
  }

  const rows = $derived(
    invoice
      ? ([
          ['RFC', invoice.rfc],
          ['Razón social', invoice.legal_name],
          ['Régimen fiscal', regimeLabel(invoice.tax_regime)],
          ['Código postal fiscal', invoice.zip_code],
          ['Uso de CFDI', cfdiUseLabel(invoice.cfdi_use)],
          ['Correo', invoice.email || '—'],
          ['Total (IVA incluido)', moneyCents(invoice.total_cents)],
          ['Solicitada', `${dateShort(invoice.created_at)}${invoice.created_by ? ` por ${invoice.created_by}` : ''}`]
        ] as [string, string][])
      : []
  );
</script>

<Modal open={!!invoice} title={invoice ? `Factura · venta #${invoice.folio}` : ''} {onclose}>
  {#if invoice}
    <p class="mb-4 flex flex-wrap gap-2">
      <Pill tone={INVOICE_STATUS[invoice.status].tone}>{INVOICE_STATUS[invoice.status].label}</Pill>
      {#if invoice.cfdi_state === 'stamped'}<Pill tone="ok">CFDI timbrado</Pill>{:else if invoice.cfdi_state === 'cancelled'}<Pill tone="bad">CFDI cancelado ante el SAT</Pill>{/if}
    </p>
    <dl class="grid gap-x-6 gap-y-3 text-sm sm:grid-cols-2">
      {#each rows as [k, v]}
        <div><dt class="section-title">{k}</dt><dd class="mt-1 break-words">{v}</dd></div>
      {/each}
      {#if invoice.fiscal_uuid}<div class="sm:col-span-2"><dt class="section-title">Folio fiscal (UUID)</dt><dd class="mt-1 break-all font-mono text-[13px]">{invoice.fiscal_uuid}</dd></div>{/if}
      {#if invoice.note}<div class="sm:col-span-2"><dt class="section-title">Nota</dt><dd class="mt-1">{invoice.note}</dd></div>{/if}
    </dl>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={copy}>Copiar datos</button>
    {#if invoice && invoice.cfdi_state && invoice.cfdi_state !== 'stamping'}
      <a class="btn-secondary" href={pos2.cfdiFileUrl(invoice.id, 'xml')} download>XML</a>
      <a class="btn-secondary" href={pos2.cfdiFileUrl(invoice.id, 'pdf')} download>PDF</a>
    {/if}
    {#if invoice && canCancelCfdi && stamped}
      <button type="button" class="btn-ghost text-app-danger" onclick={() => oncancelcfdi?.(invoice)}>Cancelar CFDI</button>
    {/if}
    {#if invoice && canManage && invoice.status === 'pending'}
      <button type="button" class="btn-ghost text-app-danger" onclick={() => oncancel(invoice)}>Cancelar solicitud</button>
      <button type="button" class="btn-secondary" onclick={() => onissue(invoice)}>Marcar como emitida</button>
      {#if onstamp}<button type="button" class="btn-primary" disabled={stamping} onclick={() => onstamp(invoice)}>{#if stamping}<span class="spin"></span>{/if}Timbrar CFDI</button>{/if}
    {/if}
  {/snippet}
</Modal>
