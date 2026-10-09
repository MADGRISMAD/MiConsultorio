<script lang="ts">
  import { Loader } from '$lib/loader.svelte';
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { moneyCents } from '$lib/format';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { CatalogItem } from '$lib/types';
  import Modal from '$lib/components/Modal.svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import PageHeader from '$lib/components/ui/PageHeader.svelte';
  import AlertsPanel from './AlertsPanel.svelte';
  import { dayLabel, expiryTone } from './expiry';
  import HistoryModal from './HistoryModal.svelte';
  import StockModal, { type StockMode } from './StockModal.svelte';
  import { isLow, normalize, parsePesos, qty } from './helpers';

  const canManage = $derived(session.has('posManage'));

  let all = $state<CatalogItem[]>([]);
  const ld = new Loader('No se pudo cargar el inventario.');
  /** bumped on every reload so the alerts follow the stock */
  let alertsKey = $state(0);

  async function load() {
    alertsKey++;
    await ld.run(async () => {
      all = (await api.pos.items()).items;
    });
  }
  onMount(load);

  const tracked = $derived(all.filter((i) => i.kind === 'product' && i.track_stock && i.active));
  const untrackedCount = $derived(all.filter((i) => i.kind === 'product' && !i.track_stock && i.active).length);
  const lowCount = $derived(tracked.filter((i) => i.stock <= i.min_stock).length);
  const costValue = $derived(tracked.reduce((s, i) => s + Math.max(0, i.stock) * i.cost_cents, 0));
  const retailValue = $derived(tracked.reduce((s, i) => s + Math.max(0, i.stock) * i.price_cents, 0));

  let filter = $state<'all' | 'low' | 'out'>('all');
  let search = $state('');
  const visible = $derived.by(() => {
    const q = normalize(search);
    const raw = search.trim();
    return tracked
      .filter((i) => (filter === 'all' ? true : filter === 'low' ? i.stock <= i.min_stock : i.stock <= 0))
      .filter((i) => !q || normalize(i.name).includes(q) || normalize(i.sku).includes(q) || i.barcode.includes(raw))
      .sort((a, b) => a.name.localeCompare(b.name, 'es'));
  });

  /** A scanner types the code and presses Enter: if it matches exactly one barcode, jump to its adjust dialog. */
  function onSearchKey(e: KeyboardEvent) {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    const code = search.trim();
    const hit = code ? tracked.find((i) => i.barcode === code || i.sku === code) : undefined;
    if (hit && canManage && !counting) openStock(hit, 'in');
  }

  // ----- single adjustments -----
  let stockOpen = $state(false);
  let stockItem = $state<CatalogItem | null>(null);
  let stockMode = $state<StockMode>('in');
  function openStock(i: CatalogItem, m: StockMode) {
    stockItem = i;
    stockMode = m;
    stockOpen = true;
  }
  async function stockSaved() {
    stockOpen = false;
    search = '';
    await load();
  }
  let histOpen = $state(false);
  let histItem = $state<CatalogItem | null>(null);

  // ----- physical count mode -----
  let counting = $state(false);
  let counts = $state<Record<string, string>>({});
  let reviewOpen = $state(false);
  const saveOp = new Op();

  function startCount() {
    counts = {};
    counting = true;
    filter = 'all';
  }
  function cancelCount() {
    counting = false;
    counts = {};
  }
  const diffs = $derived.by(() => {
    const out: { item: CatalogItem; counted: number }[] = [];
    for (const i of tracked) {
      const raw = counts[i.id];
      if (raw === undefined || raw.trim() === '') continue;
      const n = parsePesos(raw);
      if (!Number.isNaN(n) && n >= 0 && n !== i.stock) out.push({ item: i, counted: n });
    }
    return out;
  });
  const invalidCount = $derived(Object.entries(counts).filter(([, v]) => v.trim() !== '' && (Number.isNaN(parsePesos(v)) || parsePesos(v) < 0)).length);

  async function applyCount() {
    const list = [...diffs];
    let failed: string[] = [];
    let ok = 0;
    await saveOp.run(async () => {
      for (const d of list) {
        try {
          await api.pos.adjustStock(d.item.id, { set_to: d.counted, reason: 'adjustment', note: 'Conteo físico' });
          ok++;
        } catch {
          failed.push(d.item.name);
        }
      }
      if (failed.length) throw new Error(`No se pudo ajustar: ${failed.join(', ')}.`);
    });
    if (ok) toast.show(`${ok} ${ok === 1 ? 'ajuste guardado' : 'ajustes guardados'}`);
    await load();
    if (!failed.length) {
      reviewOpen = false;
      cancelCount();
    } else {
      // keep only the rows that failed so the person can retry
      const keep = new Set(failed);
      counts = Object.fromEntries(Object.entries(counts).filter(([id]) => keep.has(all.find((a) => a.id === id)?.name ?? '')));
    }
  }

  const delta = (d: number) => `${d > 0 ? '+' : ''}${qty(d)}`;
</script>

{#snippet stockNum(i: CatalogItem)}
  <span class="tabular-nums {isLow(i) ? 'font-medium text-app-danger' : ''}">{qty(i.stock)} <span class="text-xs font-normal text-app-muted">{i.unit}</span></span>
{/snippet}

<PageHeader title="Inventario" subtitle="Existencias de los productos que controlas.">
  {#snippet actions()}
    {#if canManage && tracked.length}
      {#if counting}
        <button type="button" class="btn-secondary" onclick={cancelCount}>Cancelar conteo</button>
        <button type="button" class="btn-primary" disabled={!diffs.length || !!invalidCount} onclick={() => ((reviewOpen = true), saveOp.reset())}>Revisar {diffs.length} {diffs.length === 1 ? 'diferencia' : 'diferencias'}</button>
      {:else}
        <button type="button" class="btn-secondary" onclick={startCount}><Icon name="check" size={16} />Conteo físico</button>
      {/if}
    {/if}
    <a class="btn-secondary" href="/pos/servicios"><Icon name="tag" size={16} />Catálogo</a>
  {/snippet}
</PageHeader>

{#if !ld.loading && !ld.error}
  <AlertsPanel refresh={alertsKey} onitem={(id) => (search = all.find((x) => x.id === id)?.name ?? '')} />
  <div class="mb-5 grid grid-cols-2 gap-3 lg:grid-cols-4">
    {#each [['Artículos controlados', String(tracked.length), ''], ['Bajo mínimo', String(lowCount), lowCount ? 'text-app-danger' : ''], ['Valor al costo', moneyCents(costValue), ''], ['Valor a precio público', moneyCents(retailValue), '']] as [label, val, cls]}
      <div class="card p-4">
        <p class="section-title">{label}</p>
        <p class="display mt-1 text-[1.75rem] leading-tight tabular-nums {cls}">{val}</p>
      </div>
    {/each}
  </div>
{/if}

<div class="card overflow-hidden">
  <div class="flex flex-wrap items-center gap-3 border-b border-app-ink/10 p-4">
    <div role="tablist" aria-label="Filtro" class="inline-flex rounded-full bg-app-ink/6 p-1">
      {#each [['all', 'Todos'], ['low', 'Bajo mínimo'], ['out', 'Agotados']] as [k, label]}
        <button type="button" role="tab" aria-selected={filter === k} class="rounded-full px-4 py-1.5 text-sm font-medium transition {filter === k ? 'bg-app-panel shadow-sm' : 'text-app-muted hover:text-app-ink'}" onclick={() => (filter = k as typeof filter)}>{label}</button>
      {/each}
    </div>
    <div class="relative min-w-[14rem] flex-1">
      <label class="sr-only" for="inv-search">Buscar o escanear código de barras</label>
      <Icon name="search" size={17} class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-app-muted" />
      <input id="inv-search" type="search" class="field pl-10" placeholder="Buscar o escanear código de barras" bind:value={search} onkeydown={onSearchKey} autocomplete="off" />
    </div>
  </div>

  {#if counting}
    <p class="flex items-center gap-2 bg-app-primary/8 px-4 py-2.5 text-sm text-app-primary" role="status"><Icon name="info" size={16} />Escribe la cantidad contada de cada producto. Deja en blanco lo que no cuentes; solo se guardan las diferencias.</p>
  {/if}

  {#if ld.loading}
    <LoadingRows />
  {:else if ld.error}
    <Alert class="m-5">{ld.error}</Alert>
  {:else if !tracked.length}
    <EmptyState icon="box" title="Aún no controlas existencias" text="Marca “Controlar existencias” al editar un producto en tu catálogo y aquí verás cuántas piezas te quedan, con avisos de bajo mínimo.">
      <a class="btn-primary" href="/pos/servicios">Ir al catálogo</a>
    </EmptyState>
  {:else if !visible.length}
    <EmptyState icon="search" title="Sin resultados" text={filter === 'all' ? 'Ningún producto coincide con tu búsqueda.' : 'Ningún producto cumple este filtro con tu búsqueda.'} />
  {:else}
    <div class="hidden overflow-x-auto md:block">
      <table class="w-full min-w-[52rem]">
        <thead class="border-b border-app-ink/10 bg-app-elevated">
          <tr>
            <th class="th">Producto</th><th class="th">SKU / código</th><th class="th text-right">Existencia</th><th class="th">Caducidad</th>
            {#if counting}<th class="th">Conteo</th>{/if}
            <th class="th text-right">Mínimo</th><th class="th text-right">Costo</th><th class="th text-right">Valor</th>
            {#if canManage && !counting}<th class="th"><span class="sr-only">Acciones</span></th>{/if}
          </tr>
        </thead>
        <tbody class="divide-y divide-app-ink/8">
          {#each visible as i (i.id)}
            {@const c = counts[i.id]}
            {@const cn = c === undefined || c.trim() === '' ? null : parsePesos(c)}
            <tr class="hover:bg-app-ink/[0.025]">
              <td class="td font-medium">{i.name}{#if i.category}<span class="block text-xs font-normal text-app-muted">{i.category}</span>{/if}</td>
              <td class="td font-mono text-xs text-app-muted">{i.sku || '—'}{#if i.barcode}<span class="block">{i.barcode}</span>{/if}</td>
              <td class="td text-right">{@render stockNum(i)}{#if i.stock <= 0}<span class="pill pill-bad ml-2">Agotado</span>{:else if isLow(i)}<span class="pill pill-warn ml-2">Bajo</span>{/if}</td>
              <td class="td whitespace-nowrap">
                {#if i.next_expiry}<span class="pill {expiryTone(i.next_expiry) === 'muted' ? '' : `pill-${expiryTone(i.next_expiry)}`}">{dayLabel(i.next_expiry)}</span>{:else}<span class="text-app-muted">—</span>{/if}
              </td>
              {#if counting}
                <td class="td">
                  <div class="flex items-center gap-2">
                    <input class="field !min-h-9 !w-24 !px-2 text-right" inputmode="decimal" aria-label="Cantidad contada de {i.name}" aria-invalid={cn !== null && (Number.isNaN(cn) || cn < 0)} bind:value={counts[i.id]} autocomplete="off" />
                    {#if cn !== null && !Number.isNaN(cn) && cn !== i.stock}<span class="text-xs font-medium tabular-nums {cn > i.stock ? 'text-app-accent' : 'text-app-danger'}">{delta(cn - i.stock)}</span>{/if}
                  </div>
                </td>
              {/if}
              <td class="td text-right tabular-nums text-app-muted">{qty(i.min_stock)}</td>
              <td class="td text-right tabular-nums text-app-muted">{i.cost_cents ? moneyCents(i.cost_cents) : '—'}</td>
              <td class="td text-right tabular-nums">{moneyCents(Math.max(0, i.stock) * i.cost_cents)}</td>
              {#if canManage && !counting}
                <td class="td">
                  <div class="flex justify-end gap-1">
                    <button type="button" class="btn-ghost min-h-8 px-3 text-xs" onclick={() => openStock(i, 'in')}>Entrada</button>
                    <button type="button" class="btn-ghost min-h-8 px-3 text-xs" onclick={() => openStock(i, 'out')}>Salida</button>
                    <button type="button" class="btn-ghost min-h-8 px-3 text-xs" onclick={() => openStock(i, 'count')}>Contar</button>
                    <button type="button" class="icon-btn" aria-label="Historial de {i.name}" onclick={() => ((histItem = i), (histOpen = true))}><Icon name="clock" size={17} /></button>
                  </div>
                </td>
              {/if}
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <ul class="divide-y divide-app-ink/8 md:hidden">
      {#each visible as i (i.id)}
        {@const c = counts[i.id]}
        <li class="space-y-2 p-4">
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <p class="font-medium">{i.name}</p>
              <p class="text-xs text-app-muted">{i.sku || i.barcode || i.category || ''}</p>
            </div>
            <div class="text-right">{@render stockNum(i)}<span class="block text-xs text-app-muted">mín. {qty(i.min_stock)}</span></div>
          </div>
          <div class="flex flex-wrap items-center gap-2 text-xs text-app-muted">
            {#if i.stock <= 0}<span class="pill pill-bad">Agotado</span>{:else if isLow(i)}<span class="pill pill-warn">Bajo mínimo</span>{/if}
            {#if i.next_expiry}<span class="pill {expiryTone(i.next_expiry) === 'muted' ? '' : `pill-${expiryTone(i.next_expiry)}`}">Caduca {dayLabel(i.next_expiry)}</span>{/if}
            <span>Valor {moneyCents(Math.max(0, i.stock) * i.cost_cents)}</span>
          </div>
          {#if counting}
            <div>
              <label class="label" for="cnt-{i.id}">Cantidad contada</label>
              <input id="cnt-{i.id}" class="field" inputmode="decimal" bind:value={counts[i.id]} autocomplete="off" />
            </div>
          {:else if canManage}
            <div class="flex flex-wrap justify-end gap-1">
              <button type="button" class="btn-ghost min-h-8 px-3 text-xs" onclick={() => openStock(i, 'in')}>Entrada</button>
              <button type="button" class="btn-ghost min-h-8 px-3 text-xs" onclick={() => openStock(i, 'out')}>Salida</button>
              <button type="button" class="btn-ghost min-h-8 px-3 text-xs" onclick={() => openStock(i, 'count')}>Contar</button>
              <button type="button" class="icon-btn" aria-label="Historial de {i.name}" onclick={() => ((histItem = i), (histOpen = true))}><Icon name="clock" size={17} /></button>
            </div>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
</div>

{#if !ld.loading && !ld.error && untrackedCount}
  <p class="mt-4 flex items-start gap-2 text-sm text-app-muted">
    <Icon name="info" size={16} class="mt-0.5 shrink-0" />
    <span>Tienes {untrackedCount} {untrackedCount === 1 ? 'producto' : 'productos'} sin control de existencias. {canManage ? 'Edítalos en' : 'Se activa en'} <a class="font-medium text-app-primary underline-offset-2 hover:underline" href="/pos/servicios">Servicios y precios</a> con la opción “Controlar existencias”.</span>
  </p>
{/if}

{#if canManage}
  <StockModal open={stockOpen} item={stockItem} mode={stockMode} onsaved={stockSaved} onclose={() => (stockOpen = false)} />
  <Modal open={reviewOpen} title="Confirmar conteo físico" onclose={() => (reviewOpen = false)}>
    <p class="mb-3 text-sm text-app-muted">Estas existencias se ajustarán a lo que contaste. Cada cambio queda en el historial.</p>
    <ul class="divide-y divide-app-ink/8 rounded-2xl border border-app-ink/10">
      {#each diffs as d (d.item.id)}
        <li class="flex items-center justify-between gap-3 px-4 py-2.5 text-sm">
          <span class="min-w-0 truncate font-medium">{d.item.name}</span>
          <span class="shrink-0 tabular-nums text-app-muted">{qty(d.item.stock)} → <strong class="text-app-ink">{qty(d.counted)}</strong>
            <span class="ml-1 font-medium {d.counted > d.item.stock ? 'text-app-accent' : 'text-app-danger'}">{delta(d.counted - d.item.stock)}</span></span>
        </li>
      {/each}
    </ul>
    <OpError op={saveOp} class="mt-4" />
    {#snippet footer()}
      <button type="button" class="btn-secondary" onclick={() => (reviewOpen = false)}>Seguir contando</button>
      <button type="button" class="btn-primary" disabled={!diffs.length || saveOp.phase === 'loading'} onclick={applyCount}>{#if saveOp.phase === 'loading'}<span class="spin"></span>{/if}Guardar {diffs.length} {diffs.length === 1 ? 'ajuste' : 'ajustes'}</button>
    {/snippet}
  </Modal>
{/if}
<HistoryModal open={histOpen} item={histItem} onclose={() => (histOpen = false)} />
