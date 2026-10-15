<script lang="ts">
  import Alert from '$lib/components/ui/Alert.svelte';
  import { api } from '$lib/api';
  import { dateShort, moneyCents } from '$lib/format';
  import type { Sale } from '$lib/types';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import Pill from '$lib/components/ui/Pill.svelte';
  import InvoiceRequestForm from '../billing/InvoiceRequestForm.svelte';
  import SaleDetail from './SaleDetail.svelte';

  /** Sales of a period: search, open one (reprint, invoice, return) and refresh. Used by Ventas and by Reportes. */
  interface Props {
    from: string;
    to: string;
    /** bump to refetch after a sale changes elsewhere */
    reload?: number;
    /** a sale changed here (return, void, invoice): let the owner refresh what depends on it */
    onchanged?: () => void;
    /** show the «Ventas» title (Reportes does; the Ventas page already has one) */
    heading?: boolean;
  }
  let { from, to, reload = 0, onchanged, heading = true }: Props = $props();

  let q = $state('');
  let status = $state('');
  let sales = $state<Sale[]>([]);
  let sLoading = $state(true);
  let sError = $state('');
  let sseq = 0;
  $effect(() => {
    const f = from;
    const t = to;
    const term = q.trim();
    const st = status;
    void reload;
    if (!f || !t || f > t) return;
    const my = ++sseq;
    const timer = setTimeout(
      () => {
        sLoading = true;
        sError = '';
        api.pos.sales({ from: f, to: t, q: term, status: st, limit: 200 }).then(
          (r) => {
            if (my === sseq) sales = r;
          },
          (e) => {
            if (my === sseq) sError = e instanceof Error ? e.message : 'No se pudieron cargar las ventas.';
          }
        ).finally(() => {
          if (my === sseq) sLoading = false;
        });
      },
      term ? 300 : 0
    );
    return () => clearTimeout(timer);
  });

  let detailId = $state<string | null>(null);
  let invoiceSale = $state('');
  let invoiceOpen = $state(false);
  function startInvoice(id: string) {
    detailId = null;
    invoiceSale = id;
    invoiceOpen = true;
  }
  function refresh() {
    onchanged?.();
  }
</script>

<section class={heading ? 'mt-8' : 'mt-5'} aria-label="Ventas">
  <div class="mb-3 flex flex-wrap items-end justify-between gap-3">
    {#if heading}<h2 class="display text-3xl">Ventas</h2>{:else}<span></span>{/if}
    <div class="flex flex-wrap gap-2">
      <div class="relative">
        <span class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-app-muted"><Icon name="search" size={16} /></span>
        <input class="field w-56 pl-9" type="search" placeholder="Folio o paciente" aria-label="Buscar ventas por folio o paciente" bind:value={q} />
      </div>
      <select class="field w-auto" aria-label="Estado" bind:value={status}>
        <option value="">Todas</option>
        <option value="paid">Pagadas</option>
        <option value="open">Con saldo</option>
        <option value="void">Canceladas</option>
      </select>
    </div>
  </div>
  <div class="card overflow-hidden">
    {#if sLoading && !sales.length}
      <LoadingRows />
    {:else if sError}
      <Alert class="m-4">{sError}</Alert>
    {:else if !sales.length}
      <EmptyState icon="receipt" title="Sin ventas" text="No hay ventas con estos filtros en el periodo elegido." />
    {:else}
      <div class="overflow-x-auto">
        <table class="w-full min-w-[640px]">
          <thead class="border-b border-app-ink/10"><tr><th class="th">Folio</th><th class="th">Fecha</th><th class="th">Paciente</th><th class="th">Atendió</th><th class="th text-right">Total</th><th class="th">Estado</th></tr></thead>
          <tbody class="divide-y divide-app-ink/10">
            {#each sales as s (s.id)}
              <tr class="cursor-pointer hover:bg-app-elevated/50" onclick={() => (detailId = s.id)}>
                <td class="td font-medium"><button type="button" class="text-left hover:text-app-primary hover:underline" onclick={(e) => { e.stopPropagation(); detailId = s.id; }}>#{s.folio}</button></td>
                <td class="td whitespace-nowrap text-app-muted">{dateShort(s.created_at)} · {new Date(s.created_at).toLocaleTimeString('es-MX', { hour: '2-digit', minute: '2-digit' })}</td>
                <td class="td max-w-[14rem] truncate">{s.customer_name || '—'}</td>
                <td class="td text-app-muted">{s.created_by || '—'}</td>
                <td class="td text-right font-medium {s.status === 'void' ? 'text-app-muted line-through' : ''}">{moneyCents(s.total_cents)}</td>
                <td class="td"><span class="flex flex-wrap gap-1"><Pill tone={s.status === 'paid' ? 'ok' : s.status === 'open' ? 'warn' : 'bad'}>{s.status === 'paid' ? 'Pagada' : s.status === 'open' ? `Saldo ${moneyCents(s.balance_cents ?? 0)}` : 'Cancelada'}</Pill>{#if s.invoice_status}<Pill tone="info">Factura</Pill>{/if}</span></td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>
</section>

<SaleDetail saleId={detailId} onclose={() => (detailId = null)} oninvoice={startInvoice} onchanged={refresh} />
<InvoiceRequestForm open={invoiceOpen} saleId={invoiceSale} onclose={() => (invoiceOpen = false)} onsaved={refresh} />
