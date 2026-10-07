<script lang="ts">
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { api, ApiError } from '$lib/api';
  import { pos2 } from '$lib/api/pos2';
  import { dateShort, moneyCents } from '$lib/format';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { InvoiceRequest } from '$lib/types';
  import ConfirmModal from '$lib/components/ConfirmModal.svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import PageHeader from '$lib/components/ui/PageHeader.svelte';
  import Pill from '$lib/components/ui/Pill.svelte';
  import CancelCfdiModal from './CancelCfdiModal.svelte';
  import InvoiceDetail from './InvoiceDetail.svelte';
  import InvoiceIssueModal from './InvoiceIssueModal.svelte';
  import InvoiceRequestForm from './InvoiceRequestForm.svelte';
  import { INVOICE_STATUS } from './fiscal';

  const TABS: { id: InvoiceRequest['status']; label: string; empty: string }[] = [
    { id: 'pending', label: 'Pendientes', empty: 'No hay solicitudes pendientes. Cuando un paciente pida factura, aparecerá aquí.' },
    { id: 'issued', label: 'Emitidas', empty: 'Aún no marcas facturas como emitidas.' },
    { id: 'cancelled', label: 'Canceladas', empty: 'No hay solicitudes canceladas.' }
  ];

  const canManage = $derived(session.has('posReports'));
  const canCancelCfdi = $derived(session.has('posManage'));
  /** the server has PAC credentials, so invoices can be stamped from here */
  let stampEnabled = $state(false);
  let stampingId = $state('');
  let cfdiCancelling = $state<InvoiceRequest | null>(null);
  let notConfigured = $state(false);
  let tab = $state<InvoiceRequest['status']>('pending');
  let rows = $state<InvoiceRequest[]>([]);
  let loading = $state(true);
  let error = $state('');
  let seq = 0;

  let formOpen = $state(false);
  let formSale = $state('');
  let detail = $state<InvoiceRequest | null>(null);
  let issuing = $state<InvoiceRequest | null>(null);
  let cancelling = $state<InvoiceRequest | null>(null);
  const cancelOp = new Op();

  async function load() {
    const my = ++seq;
    loading = true;
    error = '';
    try {
      const r = await pos2.invoices(tab);
      if (my === seq) (rows = r.invoices), (stampEnabled = r.stamping);
    } catch (e) {
      if (my === seq) error = e instanceof Error ? e.message : 'No se pudieron cargar las solicitudes.';
    } finally {
      if (my === seq) loading = false;
    }
  }
  $effect(() => {
    void tab;
    void load();
  });

  // ?venta=<id> opens the request form with that sale already chosen
  $effect(() => {
    const id = page.url.searchParams.get('venta');
    if (!id) return;
    formSale = id;
    formOpen = true;
    void goto('/pos/facturacion', { replaceState: true, noScroll: true, keepFocus: true });
  });

  function openNew() {
    formSale = '';
    formOpen = true;
  }
  function startIssue(i: InvoiceRequest) {
    detail = null;
    issuing = i;
  }
  function startCancel(i: InvoiceRequest) {
    detail = null;
    cancelOp.reset();
    cancelling = i;
  }
  async function stamp(i: InvoiceRequest) {
    if (!stampEnabled) {
      notConfigured = true;
      return;
    }
    stampingId = i.id;
    try {
      const r = await pos2.stamp(i.id);
      detail = null;
      toast.show(r.emailed ? 'CFDI timbrado y enviado por correo' : 'CFDI timbrado');
      for (const w of r.warnings) toast.show(w, 'error');
      tab = 'issued';
      void load();
    } catch (e) {
      toast.show(e instanceof ApiError || e instanceof Error ? e.message : 'No se pudo timbrar.', 'error');
    } finally {
      stampingId = '';
    }
  }
  async function confirmCancel() {
    const i = cancelling;
    if (!i) return;
    if (await cancelOp.run(() => api.pos.updateInvoice(i.id, { status: 'cancelled' }))) {
      cancelling = null;
      toast.show('Solicitud cancelada');
      void load();
    }
  }
</script>

<PageHeader title="Facturación" subtitle="Solicitudes de factura de tus ventas.">
  {#snippet actions()}
    <button type="button" class="btn-primary" onclick={openNew}><Icon name="plus" size={18} />Nueva solicitud</button>
  {/snippet}
</PageHeader>

<p class="mb-5 flex items-start gap-2.5 rounded-xl bg-app-primary/10 px-3.5 py-3 text-sm text-app-primary">
  <Icon name="info" size={18} />
    {#if stampEnabled}
    <span>Caresia guarda los datos fiscales de cada solicitud y puede <strong>timbrar el CFDI 4.0</strong> por ti. También puedes emitirla por tu cuenta y marcarla como emitida con su folio fiscal (UUID).</span>
  {:else}
    <span>Caresia guarda los datos fiscales de cada solicitud; el timbrado del CFDI lo haces con tu contador o proveedor y aquí marcas la factura como emitida con su folio fiscal (UUID).</span>
  {/if}
</p>
{#if notConfigured}
  <p class="alert mb-5" role="alert">
    <Icon name="alert" size={18} />
    <span>El timbrado automático aún no está configurado en el servidor (faltan las credenciales del proveedor de facturación). Mientras tanto marca la factura como emitida con su folio fiscal.</span>
    <button type="button" class="btn-ghost ml-auto min-h-9" onclick={() => (notConfigured = false)}>Entendido</button>
  </p>
{/if}

<div class="mb-4 flex gap-1 overflow-x-auto rounded-full bg-app-ink/5 p-1 sm:w-fit" role="tablist" aria-label="Estado de la solicitud">
  {#each TABS as t}
    <button type="button" role="tab" aria-selected={tab === t.id} class="whitespace-nowrap rounded-full px-4 py-1.5 text-sm font-medium transition {tab === t.id ? 'bg-app-panel text-app-ink shadow-sm' : 'text-app-muted hover:text-app-ink'}" onclick={() => (tab = t.id)}>{t.label}</button>
  {/each}
</div>

<div class="card overflow-hidden">
  {#if loading && !rows.length}
    <LoadingRows />
  {:else if error}
    <p class="alert m-4" role="alert"><Icon name="alert" size={18} />{error}</p>
  {:else if !rows.length}
    <EmptyState icon="receipt" title="Sin solicitudes" text={TABS.find((t) => t.id === tab)?.empty}>
      {#if tab === 'pending'}<button type="button" class="btn-primary" onclick={openNew}>Nueva solicitud</button>{/if}
    </EmptyState>
  {:else}
    <div class="overflow-x-auto">
      <table class="w-full min-w-[720px]">
        <thead class="border-b border-app-ink/10">
          <tr><th class="th">Venta</th><th class="th">Fecha</th><th class="th">RFC</th><th class="th">Razón social</th><th class="th text-right">Total</th><th class="th">Estado</th><th class="th"><span class="sr-only">Acciones</span></th></tr>
        </thead>
        <tbody class="divide-y divide-app-ink/10">
          {#each rows as i (i.id)}
            <tr class="hover:bg-app-elevated/50">
              <td class="td font-medium">#{i.folio}</td>
              <td class="td whitespace-nowrap text-app-muted">{dateShort(i.created_at)}</td>
              <td class="td font-mono text-[13px]">{i.rfc}</td>
              <td class="td max-w-[16rem] truncate">{i.legal_name}</td>
              <td class="td text-right font-medium">{moneyCents(i.total_cents)}</td>
              <td class="td"><Pill tone={INVOICE_STATUS[i.status].tone}>{INVOICE_STATUS[i.status].label}</Pill></td>
              <td class="td">
                <div class="flex justify-end gap-1">
                  <button type="button" class="btn-ghost" onclick={() => (detail = i)}>Ver</button>
                  {#if canManage && i.status === 'pending'}
                    <button type="button" class="btn-secondary" disabled={stampingId === i.id} onclick={() => stamp(i)}>{#if stampingId === i.id}<span class="spin"></span>{/if}Timbrar</button>
                    <button type="button" class="btn-ghost" onclick={() => startIssue(i)}>Marcar como emitida</button>
                    <button type="button" class="icon-btn danger" aria-label="Cancelar solicitud de la venta #{i.folio}" onclick={() => startCancel(i)}><Icon name="ban" size={18} /></button>
                  {/if}
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<InvoiceRequestForm open={formOpen} saleId={formSale} onclose={() => (formOpen = false)} onsaved={() => { tab = 'pending'; void load(); }} />
<InvoiceDetail
  invoice={detail}
  {canManage}
  {canCancelCfdi}
  stamping={!!detail && stampingId === detail.id}
  onclose={() => (detail = null)}
  onissue={startIssue}
  oncancel={startCancel}
  onstamp={stamp}
  oncancelcfdi={(i) => ((detail = null), (cfdiCancelling = i))}
/>
<CancelCfdiModal invoice={cfdiCancelling} onclose={() => (cfdiCancelling = null)} ondone={() => void load()} />
<InvoiceIssueModal invoice={issuing} onclose={() => (issuing = null)} ondone={() => void load()} />
<ConfirmModal open={!!cancelling} title="Cancelar solicitud" op={cancelOp} confirmLabel="Cancelar solicitud" onconfirm={confirmCancel} onclose={() => (cancelling = null)}>
  {#if cancelling}La solicitud de la venta #{cancelling.folio} ({cancelling.legal_name}) quedará cancelada. La venta podrá volver a solicitarse.{/if}
</ConfirmModal>
