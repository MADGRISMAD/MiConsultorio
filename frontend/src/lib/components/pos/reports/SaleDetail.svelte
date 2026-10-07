<script lang="ts">
  import EmailTicket from '$lib/components/pos/sale/EmailTicket.svelte';
  import { api } from '$lib/api';
  import { moneyCents } from '$lib/format';
  import { Op } from '$lib/op.svelte';
  import { printSale } from '$lib/printer/connection.svelte';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import { PAY_METHODS, type Sale } from '$lib/types';
  import Modal from '$lib/components/Modal.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import Pill from '$lib/components/ui/Pill.svelte';

  interface Props {
    saleId: string | null;
    onclose: () => void;
    /** Opens the invoice request form for this sale. */
    oninvoice: (saleId: string) => void;
    /** The sale changed (voided): refresh lists. */
    onchanged: () => void;
  }
  let { saleId, onclose, oninvoice, onchanged }: Props = $props();

  let sale = $state<Sale | null>(null);
  let loading = $state(false);
  let error = $state('');
  let voiding = $state(false);
  let reason = $state('');
  const voidOp = new Op();
  const printOp = new Op();

  $effect(() => {
    if (!saleId) return;
    const id = saleId;
    sale = null;
    error = '';
    voiding = false;
    reason = '';
    voidOp.reset();
    printOp.reset();
    loading = true;
    api.pos.sale(id).then(
      (s) => {
        if (id === saleId) sale = s;
      },
      (e) => (error = e instanceof Error ? e.message : 'No se pudo cargar la venta.')
    ).finally(() => (loading = false));
  });

  const when = (iso: string) => new Date(iso).toLocaleString('es-MX', { dateStyle: 'medium', timeStyle: 'short' });
  const canVoid = $derived(session.has('posManage') && sale?.status === 'paid');
  const canInvoice = $derived(sale?.status === 'paid' && !sale.invoice_status);

  async function reprint() {
    const s = sale;
    if (!s) return;
    if (await printOp.run(async () => printSale(s, (await api.pos.settings()).settings, { reprint: true }))) toast.show('Ticket enviado a imprimir');
  }

  async function doVoid(e: SubmitEvent) {
    e.preventDefault();
    const s = sale;
    if (!s || reason.trim().length < 3) return voidOp.fail('Escribe el motivo de la cancelación.');
    if (await voidOp.run(() => api.pos.voidSale(s.id, reason.trim()))) {
      toast.show(`Venta #${s.folio} cancelada`);
      voiding = false;
      onchanged();
      sale = await api.pos.sale(s.id).catch(() => s);
    }
  }
</script>

<Modal open={!!saleId} title={sale ? `Venta #${sale.folio}` : 'Venta'} {onclose} wide>
  {#if loading}
    <div class="space-y-3" role="status" aria-label="Cargando">{#each [0, 1, 2] as i}<div class="h-10 animate-pulse rounded-lg bg-app-ink/8"></div>{/each}</div>
  {:else if error}
    <p class="alert" role="alert"><Icon name="alert" size={18} />{error}</p>
  {:else if sale}
    <div class="mb-4 flex flex-wrap items-center gap-2 text-sm text-app-muted">
      <Pill tone={sale.status === 'paid' ? 'ok' : 'bad'}>{sale.status === 'paid' ? 'Pagada' : 'Cancelada'}</Pill>
      {#if sale.invoice_status}<Pill tone={sale.invoice_status === 'issued' ? 'ok' : 'warn'}>{sale.invoice_status === 'issued' ? 'Factura emitida' : 'Factura pendiente'}</Pill>{/if}
      <span>{when(sale.created_at)}{sale.created_by ? ` · atendió ${sale.created_by}` : ''}</span>
    </div>
    {#if sale.customer_name}<p class="mb-3 text-sm"><span class="text-app-muted">Paciente:</span> {sale.customer_name}</p>{/if}
    {#if sale.status === 'void'}
      <p class="mb-4 rounded-xl bg-app-danger/10 px-3.5 py-3 text-sm text-app-danger">
        <strong>Cancelada{sale.voided_by ? ` por ${sale.voided_by}` : ''}.</strong> {sale.void_reason}
      </p>
    {/if}

    <div class="overflow-x-auto rounded-xl border border-app-ink/10">
      <table class="w-full min-w-[420px]">
        <thead class="border-b border-app-ink/10"><tr><th class="th">Concepto</th><th class="th text-right">Cant.</th><th class="th text-right">Precio</th><th class="th text-right">Importe</th></tr></thead>
        <tbody class="divide-y divide-app-ink/10">
          {#each sale.lines ?? [] as l (l.id)}
            <tr>
              <td class="td">{l.name}{#if l.discount_cents}<span class="block text-xs text-app-muted">Descuento {moneyCents(l.discount_cents)}</span>{/if}</td>
              <td class="td text-right">{l.qty}</td>
              <td class="td text-right">{moneyCents(l.unit_price_cents)}</td>
              <td class="td text-right font-medium">{moneyCents(l.total_cents)}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <div class="mt-4 grid gap-6 sm:grid-cols-2">
      <div>
        <p class="section-title mb-2">Pagos</p>
        <ul class="grid gap-1.5 text-sm">
          {#each sale.payments ?? [] as p}
            <li class="flex justify-between gap-3">
              <span>{PAY_METHODS[p.method]?.label ?? p.method}{#if p.reference}<span class="text-app-muted"> · {p.reference}</span>{/if}</span>
              <span class="font-medium">{moneyCents(p.amount_cents)}</span>
            </li>
            {#if p.received_cents && p.change_cents > 0}<li class="flex justify-between text-xs text-app-muted"><span>Recibido {moneyCents(p.received_cents)}</span><span>Cambio {moneyCents(p.change_cents)}</span></li>{/if}
          {/each}
        </ul>
      </div>
      <dl class="grid gap-1.5 text-sm">
        <div class="flex justify-between"><dt class="text-app-muted">Subtotal</dt><dd>{moneyCents(sale.subtotal_cents)}</dd></div>
        {#if sale.discount_cents}<div class="flex justify-between"><dt class="text-app-muted">Descuento</dt><dd>-{moneyCents(sale.discount_cents)}</dd></div>{/if}
        <div class="flex justify-between"><dt class="text-app-muted">IVA incluido</dt><dd>{moneyCents(sale.tax_cents)}</dd></div>
        <div class="flex justify-between border-t border-app-ink/10 pt-1.5 text-base font-semibold"><dt>Total</dt><dd>{moneyCents(sale.total_cents)}</dd></div>
      </dl>
    </div>
    {#if sale.note}<p class="mt-4 text-sm text-app-muted">Nota: {sale.note}</p>{/if}

    {#if sale.status === 'paid'}<div class="mt-4"><EmailTicket saleId={sale.id} /></div>{/if}

    {#if printOp.phase === 'error'}<p class="alert mt-4" role="alert"><Icon name="alert" size={18} />{printOp.message}</p>{/if}

    {#if voiding}
      <form class="mt-5 grid gap-3 rounded-xl border border-app-danger/30 bg-app-danger/5 p-4" onsubmit={doVoid}>
        <p class="text-sm">Al cancelar, la venta deja de contar en reportes y caja, y los productos con control de existencias <strong>regresan al inventario</strong>. Si tiene una solicitud de factura pendiente, también se cancela. Esto no se puede deshacer.</p>
        <div>
          <label class="label" for="void-reason">Motivo de la cancelación</label>
          <input id="void-reason" class="field" bind:value={reason} maxlength="200" required />
        </div>
        {#if voidOp.phase === 'error'}<p class="alert" role="alert"><Icon name="alert" size={18} />{voidOp.message}</p>{/if}
        <div class="flex flex-wrap justify-end gap-2">
          <button type="button" class="btn-secondary" onclick={() => (voiding = false)}>No cancelar</button>
          <button type="submit" class="btn-danger" disabled={voidOp.phase === 'loading'}>{#if voidOp.phase === 'loading'}<span class="spin"></span>{/if}Cancelar venta</button>
        </div>
      </form>
    {/if}
  {/if}
  {#snippet footer()}
    {#if sale}
      {#if canVoid && !voiding}<button type="button" class="btn-ghost mr-auto text-app-danger" onclick={() => (voiding = true)}>Cancelar venta</button>{/if}
      {#if canInvoice}<button type="button" class="btn-secondary" onclick={() => oninvoice(sale!.id)}><Icon name="receipt" size={18} />Solicitar factura</button>{/if}
      <button type="button" class="btn-primary" disabled={printOp.phase === 'loading'} onclick={reprint}>
        {#if printOp.phase === 'loading'}<span class="spin"></span>{/if}Reimprimir ticket
      </button>
    {/if}
  {/snippet}
</Modal>
