<script lang="ts">
  import { api } from '$lib/api';
  import { dateShort, moneyCents } from '$lib/format';
  import { PAY_METHODS, type PosReport, type Sale } from '$lib/types';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import PageHeader from '$lib/components/ui/PageHeader.svelte';
  import Pill from '$lib/components/ui/Pill.svelte';
  import InvoiceRequestForm from '../billing/InvoiceRequestForm.svelte';
  import SaleDetail from './SaleDetail.svelte';
  import { PRESETS, presetRange, type Preset } from './range';

  let preset = $state<Preset>('month');
  let from = $state(presetRange('month').from);
  let to = $state(presetRange('month').to);

  function pick(p: Preset) {
    preset = p;
    if (p !== 'custom') ({ from, to } = presetRange(p));
  }

  // bumped to refetch report and list after a sale changes
  let reload = $state(0);

  // ---- report ----
  let report = $state<PosReport | null>(null);
  let loading = $state(true);
  let error = $state('');
  let rseq = 0;
  $effect(() => {
    const f = from;
    const t = to;
    void reload;
    if (!f || !t) return;
    if (f > t) {
      error = 'La fecha inicial no puede ser posterior a la final.';
      return;
    }
    const my = ++rseq;
    loading = true;
    error = '';
    api.pos.report(f, t).then(
      (r) => {
        if (my === rseq) report = r.report;
      },
      (e) => {
        if (my === rseq) error = e instanceof Error ? e.message : 'No se pudo cargar el reporte.';
      }
    ).finally(() => {
      if (my === rseq) loading = false;
    });
  });

  const marginPct = $derived(report && report.total_cents > 0 ? (report.margin_cents / report.total_cents) * 100 : 0);
  const maxDay = $derived(Math.max(1, ...(report?.by_day ?? []).map((d) => d.total_cents)));
  const methodTotal = $derived((report?.by_method ?? []).reduce((a, m) => a + m.amount_cents, 0));
  const pct = (n: number, of: number) => (of > 0 ? `${((n / of) * 100).toFixed(n / of >= 0.1 ? 0 : 1)}%` : '0%');
  const dayLabel = (d: string) => new Date(d + 'T12:00:00').toLocaleDateString('es-MX', { day: 'numeric', month: 'short' });
  const csvUrl = $derived(api.pos.salesCsvUrl(from, to));

  // ---- sales list ----
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
    reload++;
  }
</script>

<PageHeader title="Reportes" subtitle="Lo que vendiste, cuánto ganaste y qué tienes en existencia.">
  {#snippet actions()}
    <a href={csvUrl} download class="btn-secondary"><Icon name="receipt" size={18} />Exportar CSV</a>
  {/snippet}
</PageHeader>

<div class="mb-6 flex flex-wrap items-end gap-3">
  <div class="flex gap-1 overflow-x-auto rounded-full bg-app-ink/5 p-1" role="group" aria-label="Periodo">
    {#each PRESETS as p}
      <button type="button" aria-pressed={preset === p.id} class="whitespace-nowrap rounded-full px-3.5 py-1.5 text-sm font-medium transition {preset === p.id ? 'bg-app-panel text-app-ink shadow-sm' : 'text-app-muted hover:text-app-ink'}" onclick={() => pick(p.id)}>{p.label}</button>
    {/each}
  </div>
  {#if preset === 'custom'}
    <div class="flex items-end gap-2">
      <div><label class="label" for="rep-from">Desde</label><input id="rep-from" type="date" class="field" bind:value={from} max={to} /></div>
      <div><label class="label" for="rep-to">Hasta</label><input id="rep-to" type="date" class="field" bind:value={to} min={from} /></div>
    </div>
  {/if}
</div>

{#if error}
  <p class="alert mb-4" role="alert"><Icon name="alert" size={18} />{error}</p>
{/if}

{#if loading && !report}
  <div class="card"><LoadingRows /></div>
{:else if report}
  <div class="grid gap-4 transition-opacity {loading ? 'opacity-60' : ''}" aria-busy={loading}>
    <!-- KPIs -->
    <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
      {#each [
        { label: 'Total vendido', value: moneyCents(report.total_cents), big: true },
        { label: 'Ventas', value: String(report.sales) },
        { label: 'Ticket promedio', value: moneyCents(report.avg_ticket_cents) },
        { label: 'Utilidad bruta', value: moneyCents(report.margin_cents), sub: `${marginPct.toFixed(1)}% de margen` },
        { label: 'Descuentos', value: moneyCents(report.discount_cents) },
        { label: 'IVA incluido', value: moneyCents(report.tax_cents) },
        { label: 'Costo de lo vendido', value: moneyCents(report.cost_cents) },
        { label: 'Cancelaciones', value: String(report.void_sales), sub: report.void_sales ? 'No cuentan en los totales' : '' }
      ] as k}
        <div class="card p-4 {k.big ? 'col-span-2 lg:col-span-1' : ''}">
          <p class="section-title">{k.label}</p>
          <p class="display mt-1.5 text-[1.75rem] leading-tight {k.big ? 'text-app-primary' : ''}">{k.value}</p>
          {#if k.sub}<p class="mt-0.5 text-xs text-app-muted">{k.sub}</p>{/if}
        </div>
      {/each}
    </div>

    <!-- Daily chart -->
    <section class="card p-5">
      <h2 class="display text-2xl">Ventas por día</h2>
      {#if !report.by_day.length}
        <p class="mt-3 text-sm text-app-muted">Sin ventas en este periodo.</p>
      {:else}
        <div class="mt-4 overflow-x-auto pb-1">
          <ul class="flex h-44 items-end gap-1.5 text-app-primary" style="min-width: {Math.min(report.by_day.length, 62) * 26}px" aria-label="Total vendido por día">
            {#each report.by_day as d (d.day)}
              <li class="group relative flex h-full min-w-[18px] max-w-[72px] flex-1 flex-col justify-end" title="{dayLabel(d.day)}: {moneyCents(d.total_cents)} · {d.sales} venta(s)">
                <span class="rounded-t-md bg-current opacity-80 transition group-hover:opacity-100" style="height: {Math.max(3, (d.total_cents / maxDay) * 100)}%"></span>
                <span class="sr-only">{dayLabel(d.day)}: {moneyCents(d.total_cents)}, {d.sales} ventas</span>
              </li>
            {/each}
          </ul>
          <div class="mt-1.5 flex justify-between text-xs text-app-muted"><span>{dayLabel(report.by_day[0].day)}</span><span>Máximo {moneyCents(maxDay)}</span><span>{dayLabel(report.by_day[report.by_day.length - 1].day)}</span></div>
        </div>
      {/if}
    </section>

    <div class="grid gap-4 lg:grid-cols-2">
      <!-- by method -->
      <section class="card p-5">
        <h2 class="display text-2xl">Por método de pago</h2>
        {#if !report.by_method.length}
          <p class="mt-3 text-sm text-app-muted">Sin cobros en este periodo.</p>
        {:else}
          <ul class="mt-4 grid gap-3.5">
            {#each report.by_method as m (m.method)}
              <li>
                <div class="flex justify-between gap-3 text-sm"><span>{PAY_METHODS[m.method]?.label ?? m.method} <span class="text-app-muted">· {m.count}</span></span><span class="font-medium">{moneyCents(m.amount_cents)} <span class="text-app-muted">({pct(m.amount_cents, methodTotal)})</span></span></div>
                <div class="mt-1.5 h-2 overflow-hidden rounded-full bg-app-ink/8"><div class="h-full rounded-full bg-app-accent" style="width: {methodTotal ? (m.amount_cents / methodTotal) * 100 : 0}%"></div></div>
              </li>
            {/each}
          </ul>
        {/if}
      </section>

      <!-- by cashier -->
      <section class="card p-5">
        <h2 class="display text-2xl">Por cajero</h2>
        {#if !report.by_user.length}
          <p class="mt-3 text-sm text-app-muted">Sin ventas en este periodo.</p>
        {:else}
          <table class="mt-3 w-full">
            <thead><tr><th class="th pl-0">Persona</th><th class="th text-right">Ventas</th><th class="th pr-0 text-right">Total</th></tr></thead>
            <tbody class="divide-y divide-app-ink/10">
              {#each report.by_user as u}
                <tr><td class="td pl-0">{u.name || '—'}</td><td class="td text-right">{u.sales}</td><td class="td pr-0 text-right font-medium">{moneyCents(u.total_cents)}</td></tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </section>
    </div>

    <!-- top items -->
    <section class="card p-5">
      <h2 class="display text-2xl">Lo más vendido</h2>
      {#if !report.top_items.length}
        <p class="mt-3 text-sm text-app-muted">Sin ventas en este periodo.</p>
      {:else}
        <div class="mt-3 overflow-x-auto">
          <table class="w-full min-w-[460px]">
            <thead><tr><th class="th pl-0">Concepto</th><th class="th text-right">Cantidad</th><th class="th text-right">Total</th><th class="th pr-0 text-right">Utilidad</th></tr></thead>
            <tbody class="divide-y divide-app-ink/10">
              {#each report.top_items.slice(0, 10) as it}
                <tr>
                  <td class="td pl-0">{it.name} <span class="ml-1 text-xs text-app-muted">{it.kind === 'service' ? 'Servicio' : 'Producto'}</span></td>
                  <td class="td text-right">{it.qty}</td>
                  <td class="td text-right font-medium">{moneyCents(it.total_cents)}</td>
                  <td class="td pr-0 text-right">{moneyCents(it.margin_cents)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </section>

    <!-- inventory -->
    <section class="card p-5">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <h2 class="display text-2xl">Inventario hoy</h2>
        <a href="/pos/inventario" class="btn-ghost">Ver inventario<Icon name="arrow-right" size={16} /></a>
      </div>
      <dl class="mt-3 grid gap-4 sm:grid-cols-4">
        <div><dt class="section-title">Con control de existencias</dt><dd class="display mt-1 text-2xl">{report.inventory.items}</dd></div>
        <div>
          <dt class="section-title">Bajo el mínimo</dt>
          <dd class="display mt-1 text-2xl {report.inventory.low_stock ? 'text-app-warning' : ''}">{report.inventory.low_stock}</dd>
          {#if report.inventory.low_stock}<a href="/pos/inventario" class="text-xs font-medium text-app-primary hover:underline">Revisar</a>{/if}
        </div>
        <div><dt class="section-title">Valor al costo</dt><dd class="display mt-1 text-2xl">{moneyCents(report.inventory.value_cents)}</dd></div>
        <div><dt class="section-title">Valor a precio público</dt><dd class="display mt-1 text-2xl">{moneyCents(report.inventory.retail_cents)}</dd></div>
      </dl>
    </section>
  </div>
{/if}

<!-- sales list -->
<section class="mt-8" aria-labelledby="sales-h">
  <div class="mb-3 flex flex-wrap items-end justify-between gap-3">
    <h2 id="sales-h" class="display text-3xl">Ventas</h2>
    <div class="flex flex-wrap gap-2">
      <div class="relative">
        <span class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-app-muted"><Icon name="search" size={16} /></span>
        <input class="field w-56 pl-9" type="search" placeholder="Folio o paciente" aria-label="Buscar ventas por folio o paciente" bind:value={q} />
      </div>
      <select class="field w-auto" aria-label="Estado" bind:value={status}>
        <option value="">Todas</option>
        <option value="paid">Pagadas</option>
        <option value="void">Canceladas</option>
      </select>
    </div>
  </div>
  <div class="card overflow-hidden">
    {#if sLoading && !sales.length}
      <LoadingRows />
    {:else if sError}
      <p class="alert m-4" role="alert"><Icon name="alert" size={18} />{sError}</p>
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
                <td class="td"><span class="flex flex-wrap gap-1"><Pill tone={s.status === 'paid' ? 'ok' : 'bad'}>{s.status === 'paid' ? 'Pagada' : 'Cancelada'}</Pill>{#if s.invoice_status}<Pill tone="info">Factura</Pill>{/if}</span></td>
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
