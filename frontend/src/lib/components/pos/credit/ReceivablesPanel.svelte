<script lang="ts">
  import { Loader } from '$lib/loader.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onMount } from 'svelte';
  import { api, ApiError } from '$lib/api';
  import { pos2 } from '$lib/api/pos2';
  import { moneyCents } from '$lib/format';
  import { printSale } from '$lib/printer/connection.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { PosSettings, SalePaymentInput } from '$lib/types';
  import type { ReceivableCustomer, ReceivableSale, Receivables } from '$lib/types/pos2';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import PageHeader from '$lib/components/ui/PageHeader.svelte';
  import PaymentModal from '../sale/PaymentModal.svelte';

  let data = $state<Receivables | null>(null);
  let settings = $state<PosSettings | null>(null);
  const ld = new Loader('No se pudieron cargar las cuentas por cobrar.');
  let open = $state<Record<string, boolean>>({});

  let target = $state<{ customer: string; sale: ReceivableSale } | null>(null);
  let payKey = $state(0);
  let busy = $state(false);
  let payError = $state('');

  async function load() {
    await ld.run(async () => {
        const [r, s] = await Promise.all([pos2.receivables(), settings ? Promise.resolve({ settings }) : api.pos.settings()]);
        data = r;
        settings = s.settings;
    });
  }
  onMount(load);

  const AGING: [keyof Receivables['aging'], string][] = [
    ['0_30', '0 a 30 días'],
    ['31_60', '31 a 60 días'],
    ['61_90', '61 a 90 días'],
    ['over_90', 'Más de 90 días']
  ];
  const ageTone = (d: number) => (d > 90 ? 'pill-bad' : d > 30 ? 'pill-warn' : '');
  const ageText = (d: number) => (d === 0 ? 'hoy' : `${d} ${d === 1 ? 'día' : 'días'}`);

  function pay(c: ReceivableCustomer, s: ReceivableSale) {
    payError = '';
    payKey++;
    target = { customer: c.name, sale: s };
  }

  async function submit(payments: SalePaymentInput[]) {
    const t = target;
    if (!t) return;
    busy = true;
    payError = '';
    try {
      const sale = await pos2.addPayments(t.sale.id, payments);
      target = null;
      toast.show(sale.status === 'paid' ? `Venta #${sale.folio} saldada` : `Abono registrado. Saldo: ${moneyCents(sale.balance_cents ?? 0)}`);
      if (settings?.printer.auto_print) {
        try {
          await printSale(sale, settings);
        } catch {
          /* printing is optional */
        }
      }
      await load();
    } catch (e) {
      payError = e instanceof ApiError && e.code === 'CASH_CLOSED' ? 'Abre la caja para registrar el abono (Caja).' : e instanceof Error ? e.message : 'No se pudo registrar el abono.';
    } finally {
      busy = false;
    }
  }
</script>

<PageHeader title="Cuentas por cobrar" subtitle="Ventas con anticipo que aún tienen saldo.">
  {#snippet actions()}
    <a class="btn-secondary" href="/pos/cobros"><Icon name="arrow-left" size={16} />Punto de venta</a>
  {/snippet}
</PageHeader>

{#if ld.loading}
  <div class="card overflow-hidden"><LoadingRows /></div>
{:else if ld.error}
  <Alert>{ld.error}</Alert>
{:else if data}
  {#if !data.customers.length}
    <div class="card">
      <EmptyState icon="wallet" title="Nada por cobrar" text="Cuando cobres una cuenta “a abonos”, el saldo pendiente aparecerá aquí para registrar los siguientes pagos.">
        <a class="btn-primary" href="/pos/cobros">Ir al punto de venta</a>
      </EmptyState>
    </div>
  {:else}
    <div class="mb-5 grid grid-cols-2 gap-3 lg:grid-cols-5">
      <div class="card col-span-2 p-4 lg:col-span-1">
        <p class="section-title">Total por cobrar</p>
        <p class="display mt-1 text-[1.75rem] leading-tight tabular-nums" data-testid="receivables-total">{moneyCents(data.total_cents)}</p>
      </div>
      {#each AGING as [k, label]}
        <div class="card p-4">
          <p class="section-title">{label}</p>
          <p class="display mt-1 text-[1.5rem] leading-tight tabular-nums {k === 'over_90' && data.aging[k] ? 'text-app-danger' : ''}">{moneyCents(data.aging[k])}</p>
        </div>
      {/each}
    </div>

    <ul class="space-y-3">
      {#each data.customers as c (c.key)}
        <li class="card overflow-hidden">
          <button
            type="button"
            class="flex min-h-14 w-full items-center justify-between gap-3 px-4 py-3 text-left"
            aria-expanded={!!open[c.key]}
            onclick={() => (open[c.key] = !open[c.key])}
          >
            <span class="min-w-0">
              <span class="block truncate font-medium">{c.name}</span>
              <span class="text-xs text-app-muted">{c.sales.length} {c.sales.length === 1 ? 'venta' : 'ventas'} · la más antigua, {ageText(c.oldest_days)}</span>
            </span>
            <span class="flex items-center gap-3">
              <span class="display text-2xl tabular-nums">{moneyCents(c.balance_cents)}</span>
              <Icon name="chevron-down" size={18} class="transition {open[c.key] ? 'rotate-180' : ''}" />
            </span>
          </button>
          {#if open[c.key]}
            <ul class="divide-y divide-app-ink/10 border-t border-app-ink/10">
              {#each c.sales as s (s.id)}
                <li class="flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3 text-sm">
                  <span class="font-medium">Venta #{s.folio}</span>
                  <span class="pill {ageTone(s.age_days)}">{ageText(s.age_days)}</span>
                  <span class="text-xs text-app-muted">Total {moneyCents(s.total_cents)} · abonado {moneyCents(s.total_cents - s.balance_cents)}</span>
                  <span class="ml-auto font-medium tabular-nums">Saldo {moneyCents(s.balance_cents)}</span>
                  <button type="button" class="btn-primary min-h-10" onclick={() => pay(c, s)}><Icon name="cash" size={16} />Abonar</button>
                </li>
              {/each}
            </ul>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
{/if}

{#if settings && target}
  {#key payKey}
    <PaymentModal
      open={!!target}
      total={target.sale.balance_cents}
      {settings}
      {busy}
      error={payError}
      partial
      title="Registrar abono"
      totalLabel="Saldo de la venta #{target.sale.folio}"
      onclose={() => (target = null)}
      onconfirm={(p) => submit(p)}
    />
  {/key}
{/if}
