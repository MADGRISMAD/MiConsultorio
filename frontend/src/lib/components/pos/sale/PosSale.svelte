<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '$lib/api';
  import { moneyCents } from '$lib/format';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import { printSale } from '$lib/printer/connection.svelte';
  import { PERMISSIONS, type CashSession, type CatalogItem, type PosSettings, type Sale, type SalePaymentInput } from '$lib/types';
  import Icon from '$lib/components/ui/Icon.svelte';
  import PageHeader from '$lib/components/ui/PageHeader.svelte';
  import OpenCashModal from '../cash/OpenCashModal.svelte';
  import { Cart, type Person } from './cart.svelte';
  import CatalogBrowser from './CatalogBrowser.svelte';
  import CartPanel from './CartPanel.svelte';
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
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'No se pudo cargar el punto de venta.';
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    load();
    if (canSeeExpedients) {
      api
        .expedients()
        .then((r) => (people = r.map((e) => ({ name: `${e.names} ${e.last_names}`.trim(), curp: e.CURP }))))
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

  async function submit(payments: SalePaymentInput[]) {
    if (!settings) return;
    paying = true;
    payError = '';
    try {
      const sale = await api.pos.createSale(cart.toInput(payments));
      const change = payments.reduce((a, p) => a + Math.max(0, (p.received_cents ?? p.amount_cents) - p.amount_cents), 0);
      cart.clear();
      payOpen = false;
      payKey++;
      done = { sale, change };
      loadCash();
      loadItems().catch(() => {});
      if (settings.printer.auto_print) print(sale);
    } catch (e) {
      if (e instanceof ApiError && e.code === 'CASH_CLOSED') {
        retry = payments;
        afterCash = 'retry';
        payOpen = false;
        cash = null;
        cashOpen = true;
      } else {
        payError = e instanceof Error ? e.message : 'No se pudo registrar la venta.';
        if (e instanceof ApiError && e.code === 'NO_STOCK') loadItems().catch(() => {});
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
  {#if needsCash && !loading}
    <div class="card mb-5 flex flex-wrap items-center justify-between gap-3 border-app-warning/40 bg-app-warning/10 px-4 py-3" role="status">
      <p class="flex items-center gap-2 text-sm font-medium text-app-warning"><Icon name="lock" size={18} />La caja está cerrada. Ábrela para poder cobrar.</p>
      <button type="button" class="btn-primary min-h-11" onclick={() => ((afterCash = null), (cashOpen = true))}>Abrir caja</button>
    </div>
  {/if}

  <div class="grid gap-5 lg:grid-cols-[minmax(0,1fr)_24rem] xl:grid-cols-[minmax(0,1fr)_27rem]">
    <CatalogBrowser {items} {loading} allowNegative={settings?.allow_negative_stock ?? false} {inCart} onadd={add} />

    <aside class="min-w-0 lg:sticky lg:top-4 lg:flex lg:max-h-[calc(100dvh-2rem)] lg:flex-col">
      <CartPanel {cart} {canEditPrice} {people} showTax={settings?.show_tax_line ?? true} busy={loading || !settings} onfree={() => (freeOpen = true)} oncheckout={checkoutClicked} />
    </aside>
  </div>

  <!-- phones and tablets in portrait: the total and Cobrar stay within reach -->
  {#if !cart.empty}
    <div class="sticky bottom-0 z-30 -mx-4 mt-4 flex items-center gap-3 border-t border-app-ink/10 bg-app-panel/95 px-4 py-3 backdrop-blur sm:-mx-6 sm:px-6 lg:hidden">
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
    <PaymentModal open={payOpen} total={cart.total} {settings} busy={paying} error={payError} onclose={() => (payOpen = false)} onconfirm={submit} />
  {/key}
  <FreeLineModal open={freeOpen} defaultTax={settings.default_tax_rate} onclose={() => (freeOpen = false)} onadd={(n, p, t) => cart.addFree(n, p, t)} />
{/if}
<OpenCashModal open={cashOpen} onclose={cashDismissed} onopened={cashOpened} />
