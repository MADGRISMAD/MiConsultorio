<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { api, ApiError } from '$lib/api';
  import { consult } from '$lib/api/consult';
  import { pos2 } from '$lib/api/pos2';
  import type { Charge } from '$lib/types/consult';
  import type { PlanPrefill, PlanPrefillItem, Professional } from '$lib/types/pos2';
  import { moneyCents } from '$lib/format';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import { printSale } from '$lib/printer/connection.svelte';
  import { PERMISSIONS, type CashSession, type CatalogItem, type PosSettings, type Sale, type SalePaymentInput } from '$lib/types';
  import AlertsSummary from './AlertsSummary.svelte';
  import ExpiredOverrideModal from './ExpiredOverrideModal.svelte';
  import PlanPicker from './PlanPicker.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import PageHeader from '$lib/components/ui/PageHeader.svelte';
  import OpenCashModal from '../cash/OpenCashModal.svelte';
  import { Cart, type Person } from './cart.svelte';
  import CatalogBrowser from './CatalogBrowser.svelte';
  import CartPanel from './CartPanel.svelte';
  import ConsultChargesPanel from './ConsultChargesPanel.svelte';
  import FreeLineModal from './FreeLineModal.svelte';
  import PaymentModal from './PaymentModal.svelte';
  import SaleDone from './SaleDone.svelte';

  const cart = new Cart();
  // Restore before the draft-saving effect first runs, or it would overwrite the draft with an empty cart.
  cart.restore();

  let settings = $state<PosSettings | null>(null);
  let items = $state<CatalogItem[]>([]);
  let categories = $state<string[]>([]);
  let cash = $state<CashSession | null>(null);
  let loading = $state(true);
  let loadError = $state('');
  let people = $state<Person[]>([]);
  let professionals = $state<Professional[]>([]);
  let planOpen = $state<PlanPrefill | null>(null);
  let chargesKey = $state(0);
  /** a sale refused because of expired lots, waiting for an administrator's reason */
  let expiredAsk = $state<{ payments: SalePaymentInput[]; onAccount: boolean; message: string } | null>(null);

  let payOpen = $state(false);
  let payKey = $state(0);
  let paying = $state(false);
  let payError = $state('');
  let freeOpen = $state(false);
  let cashOpen = $state(false);
  /** What to do once the register has been opened from here. */
  let afterCash = $state<'pay' | 'retry' | null>(null);
  let retry: SalePaymentInput[] | null = null;

  let done = $state<{ sale: Sale; change: number } | null>(null);
  let printing = $state(false);

  const canEditPrice = $derived(session.has(PERMISSIONS.posManage));
  const canSeeExpedients = $derived(session.has(PERMISSIONS.navHistorials) || session.has(PERMISSIONS.adminHistorials));
  const isAdmin = $derived(session.user?.role === 'admin');
  const creditBlocked = $derived(cart.customer.trim() || cart.patientId ? '' : 'Para cobrar a abonos indica el paciente o el nombre del cliente en la cuenta.');
  const needsCash = $derived(!!settings?.require_open_cash && !cash);
  const inCart = $derived(new Map(cart.lines.filter((l) => l.item_id).map((l) => [l.item_id!, cart.lines.filter((x) => x.item_id === l.item_id).reduce((a, x) => a + x.qty, 0)])));

  // Keep the draft in localStorage (the cart reads every field it saves, so this re-runs on any change)
  $effect(() => {
    cart.save();
  });

  async function loadItems() {
    const r = await api.pos.items({ active: true });
    items = r.items;
    categories = r.categories;
    cart.syncStock(items);
  }
  async function loadCash() {
    try {
      cash = await api.pos.cash();
    } catch {
      /* the banner simply will not show */
    }
  }

  async function load() {
    loading = true;
    loadError = '';
    try {
      const s = await api.pos.settings();
      settings = s.settings;
      cart.configure(settings);
      await Promise.all([loadItems(), loadCash()]);
      void prefill();
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'No se pudo cargar el punto de venta.';
    } finally {
      loading = false;
    }
  }

  /** Loads a consultation's pre-account (services and supplies) into the cart. */
  async function loadCharge(id: string) {
    if (!cart.empty) {
      toast.show('Hay una cuenta en curso: termínala o vacíala antes de abrir la pre-cuenta.', 'error');
      return;
    }
    try {
      const c = await consult.get(id);
      if (c.status !== 'sent' && c.status !== 'draft') {
        toast.show(c.status === 'charged' ? 'Esa pre-cuenta ya fue cobrada.' : 'Esa pre-cuenta fue cancelada.', 'error');
        chargesKey++;
        return;
      }
      let billed = 0;
      for (const i of c.items ?? []) {
        // supplies used but not billed to the patient are not on the ticket
        if (i.consumed && i.unit_price_cents === 0) continue;
        const item = i.catalog_item_id ? items.find((x) => x.id === i.catalog_item_id) : undefined;
        cart.addFromCharge(item, i.name, i.unit_price_cents, i.tax_rate, i.qty, i.consumed);
        billed++;
      }
      cart.consultChargeId = c.id;
      cart.patientId = c.patient_id;
      cart.customer = c.patient_name;
      cart.professionalId = c.professional_id ?? '';
      cart.appointmentId = c.appointment_id ?? '';
      cart.origin = `Pre-cuenta de la consulta de ${c.patient_name}${c.professional_name ? ` (${c.professional_name})` : ''}: ${billed} ${billed === 1 ? 'concepto' : 'conceptos'}. Los insumos que ya se descontaron del inventario no se descuentan otra vez.`;
    } catch (e) {
      toast.show(e instanceof ApiError && e.status === 404 ? 'No encontramos esa pre-cuenta.' : 'No se pudo cargar la pre-cuenta.', 'error');
    }
  }
  function openCharge(c: Charge) {
    void loadCharge(c.id);
  }

  /** ?cita=<id>, ?plan=<id> and ?precuenta=<id> load a visit, a treatment plan or a consultation's pre-account into the cart. */
  async function prefill() {
    const cita = page.url.searchParams.get('cita');
    const plan = page.url.searchParams.get('plan');
    const pre = page.url.searchParams.get('precuenta');
    if (!cita && !plan && !pre) return;
    void goto('/pos/cobros', { replaceState: true, noScroll: true, keepFocus: true });
    if (pre && cart.consultChargeId === pre) return; // the draft already holds it
    if (!cart.empty) {
      toast.show('Hay una cuenta en curso: termínala o vacíala antes de cargar la cita o el plan.', 'error');
      return;
    }
    if (pre) {
      await loadCharge(pre);
      return;
    }
    if (cita) {
      try {
        const a = await pos2.appointment(cita);
        cart.appointmentId = a.id;
        cart.patientId = a.patient_id ?? '';
        cart.customer = `${a.names} ${a.last_names}`.trim();
        cart.professionalId = a.professional_id ?? '';
        cart.origin = `Cobro de la cita de ${cart.customer}. Al cobrar, la cita se marca como completada.`;
        if (a.sale_id) toast.show('Esta cita ya tiene una venta registrada.', 'error');
        const svc = a.service_id ? items.find((i) => i.id === a.service_id) : undefined;
        if (svc) add(svc);
        else toast.show('La cita no tiene un servicio del catálogo: agrega los conceptos a cobrar.');
      } catch (e) {
        toast.show(e instanceof ApiError && e.status === 404 ? 'No encontramos esa cita.' : 'No se pudo cargar la cita.', 'error');
      }
    }
    if (plan) {
      try {
        planOpen = await pos2.plan(plan);
      } catch (e) {
        toast.show(e instanceof ApiError && e.status === 404 ? 'No encontramos ese plan de tratamiento.' : 'No se pudo cargar el plan de tratamiento.', 'error');
      }
    }
  }

  function addPlanItems(plan: PlanPrefill, picked: PlanPrefillItem[]) {
    for (const p of picked) {
      const item = p.catalog_item_id ? items.find((i) => i.id === p.catalog_item_id) : undefined;
      if (item) cart.addPlanned(item, p.unit_price_cents, p.qty, p.id);
      else cart.addFree(`${p.description}${p.tooth ? ` · diente ${p.tooth}` : ''}`, p.unit_price_cents, p.tax_rate, p.qty, p.id);
    }
    cart.patientId = plan.patient_id;
    cart.customer = plan.patient_name;
    cart.origin = `Plan de tratamiento «${plan.title}»: ${picked.length} ${picked.length === 1 ? 'concepto' : 'conceptos'}.`;
    planOpen = null;
  }

  onMount(() => {
    load();
    pos2
      .professionals()
      .then((r) => (professionals = r))
      .catch(() => {
        /* the selector simply does not show */
      });
    if (canSeeExpedients) {
      api
        .patients.list()
        .then((r) => (people = r.map((e) => ({ id: e.id, name: `${e.names} ${e.last_names}`.trim(), curp: '' }))))
        .catch(() => {
          /* plain text field then */
        });
    }
  });

  function add(item: CatalogItem) {
    const warn = cart.add(item);
    if (warn) toast.show(warn, 'error');
  }

  function checkoutClicked() {
    if (cart.problem) return;
    payError = '';
    if (needsCash) {
      afterCash = 'pay';
      cashOpen = true;
    } else payOpen = true;
  }

  async function submit(payments: SalePaymentInput[], onAccount = false, override?: string) {
    if (!settings) return;
    paying = true;
    payError = '';
    try {
      const extra = { on_account: onAccount || undefined, ...(override ? { allow_expired: true, expired_reason: override } : {}) };
      const sale = await api.pos.createSale(cart.toInput(payments, extra));
      const change = payments.reduce((a, p) => a + Math.max(0, (p.received_cents ?? p.amount_cents) - p.amount_cents), 0);
      cart.clear();
      payOpen = false;
      expiredAsk = null;
      payKey++;
      done = { sale, change };
      loadCash();
      loadItems().catch(() => {});
      chargesKey++;
      if (settings.printer.auto_print) print(sale);
    } catch (e) {
      if (e instanceof ApiError && e.code === 'LOT_EXPIRED' && isAdmin) {
        payOpen = false;
        expiredAsk = { payments, onAccount, message: e.message };
      } else if (e instanceof ApiError && e.code === 'CASH_CLOSED') {
        retry = payments;
        afterCash = 'retry';
        payOpen = false;
        cash = null;
        cashOpen = true;
      } else {
        payError = e instanceof Error ? e.message : 'No se pudo registrar la venta.';
        if (e instanceof ApiError && (e.code === 'NO_STOCK' || e.code === 'LOT_EXPIRED')) loadItems().catch(() => {});
      }
    } finally {
      paying = false;
    }
  }

  function cashOpened(s: CashSession) {
    cash = s;
    cashOpen = false;
    const next = afterCash;
    afterCash = null;
    if (next === 'retry' && retry) {
      payOpen = true;
      const p = retry;
      retry = null;
      submit(p);
    } else if (next === 'pay') payOpen = true;
  }
  function cashDismissed() {
    cashOpen = false;
    if (afterCash === 'retry') {
      payOpen = true;
      payError = 'Abre la caja para poder registrar la venta.';
    }
    afterCash = null;
    retry = null;
  }

  async function print(sale: Sale) {
    if (!settings) return;
    printing = true;
    try {
      const full = sale.lines ? sale : await api.pos.sale(sale.id);
      await printSale(full, settings);
    } catch (e) {
      toast.show(`No se pudo imprimir el ticket${e instanceof Error && e.message ? ': ' + e.message : ''}`, 'error');
    } finally {
      printing = false;
    }
  }

  const sinceText = (iso: string) => new Date(iso).toLocaleTimeString('es-MX', { hour: '2-digit', minute: '2-digit' });
</script>

<PageHeader title="Punto de venta" subtitle="Cobra servicios y productos, divide el pago y entrega el ticket.">
  {#snippet actions()}
    <a href="/pos/cuentas" class="btn-secondary min-h-9 px-3.5 text-[13px]"><Icon name="wallet" size={15} />Cuentas por cobrar</a>
    {#if cash}
      <a href="/pos/caja" class="pill pill-ok min-h-9 px-3.5 text-[13px]" title="Ver caja">
        <Icon name="cash" size={15} />Caja abierta · desde {sinceText(cash.opened_at)} · {cash.sales} {cash.sales === 1 ? 'venta' : 'ventas'}
      </a>
    {:else if settings && !loading}
      <a href="/pos/caja" class="pill pill-warn min-h-9 px-3.5 text-[13px]"><Icon name="lock" size={15} />Caja cerrada</a>
    {/if}
  {/snippet}
</PageHeader>

{#if loadError}
  <div class="card p-6">
    <p class="alert" role="alert"><Icon name="alert" size={18} />{loadError}</p>
    <button type="button" class="btn-primary mt-4" onclick={load}><Icon name="refresh" size={16} />Reintentar</button>
  </div>
{:else if done && settings}
  <SaleDone sale={done.sale} change={done.change} {printing} onprint={() => print(done!.sale)} onnew={() => (done = null)} />
{:else}
  {#if !loading}<AlertsSummary />{/if}
  {#if !loading && settings}<ConsultChargesPanel activeId={cart.consultChargeId} refreshKey={chargesKey} onopen={openCharge} />{/if}
  {#if needsCash && !loading}
    <div class="card mb-5 flex flex-wrap items-center justify-between gap-3 border-app-warning/40 bg-app-warning/10 px-4 py-3" role="status">
      <p class="flex items-center gap-2 text-sm font-medium text-app-warning"><Icon name="lock" size={18} />La caja está cerrada. Ábrela para poder cobrar.</p>
      <button type="button" class="btn-primary min-h-11" onclick={() => ((afterCash = null), (cashOpen = true))}>Abrir caja</button>
    </div>
  {/if}

  <div class="grid gap-5 lg:grid-cols-[minmax(0,1fr)_24rem] xl:grid-cols-[minmax(0,1fr)_27rem]">
    <CatalogBrowser {items} {loading} allowNegative={settings?.allow_negative_stock ?? false} {inCart} onadd={add} />

    <aside class="min-w-0 lg:sticky lg:top-4 lg:flex lg:max-h-[calc(100dvh-2rem)] lg:flex-col">
      <CartPanel {cart} {canEditPrice} {people} {professionals} showTax={settings?.show_tax_line ?? true} busy={loading || !settings} onfree={() => (freeOpen = true)} oncheckout={checkoutClicked} />
    </aside>
  </div>

  <!-- phones and tablets in portrait: the total and Cobrar stay within reach -->
  {#if !cart.empty}
    <div class="sticky bottom-0 z-10 -mx-4 mt-4 flex items-center gap-3 border-t border-app-ink/10 bg-app-panel/95 px-4 py-3 backdrop-blur sm:-mx-6 sm:px-6 lg:hidden">
      <a href="#cuenta" class="min-w-0 flex-1" aria-label="Ver la cuenta">
        <span class="block text-xs text-app-muted">{cart.count} {cart.count === 1 ? 'concepto' : 'conceptos'} · ver cuenta</span>
        <span class="display block text-2xl tabular-nums" aria-live="polite">{moneyCents(cart.total)}</span>
      </a>
      <button type="button" class="btn-primary min-h-12 px-8 text-base" disabled={!!cart.problem || loading} onclick={checkoutClicked}>Cobrar</button>
    </div>
  {/if}
{/if}

{#if settings}
  {#key payKey}
    <PaymentModal
      open={payOpen}
      total={cart.total}
      {settings}
      busy={paying}
      error={payError}
      allowCredit
      {creditBlocked}
      onclose={() => (payOpen = false)}
      onconfirm={(p, acc) => submit(p, acc)}
    />
  {/key}
  <ExpiredOverrideModal
    open={!!expiredAsk}
    message={expiredAsk?.message ?? ''}
    busy={paying}
    onclose={() => ((expiredAsk = null), (payOpen = true))}
    onconfirm={(reason) => expiredAsk && submit(expiredAsk.payments, expiredAsk.onAccount, reason)}
  />
  <PlanPicker plan={planOpen} onclose={() => (planOpen = null)} onadd={addPlanItems} />
  <FreeLineModal open={freeOpen} defaultTax={settings.default_tax_rate} onclose={() => (freeOpen = false)} onadd={(n, p, t) => cart.addFree(n, p, t)} />
{/if}
<OpenCashModal open={cashOpen} onclose={cashDismissed} onopened={cashOpened} />
