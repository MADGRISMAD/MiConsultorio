<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { moneyCents } from '$lib/format';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { CatalogItem, ItemKind } from '$lib/types';
  import ConfirmModal from '$lib/components/ConfirmModal.svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import PageHeader from '$lib/components/ui/PageHeader.svelte';
  import ImportModal from './ImportModal.svelte';
  import ItemForm from './ItemForm.svelte';
  import MagicInventoryModal from './MagicInventoryModal.svelte';
  import MagicPriceModal from './MagicPriceModal.svelte';
  import { isLow, marginPct, normalize, parsePesos, pct, qty, toCents, toInput } from './helpers';

  const canManage = $derived(session.has('posManage'));

  let items = $state<CatalogItem[]>([]);
  let categories = $state<string[]>([]);
  let loading = $state(true);
  let loadError = $state('');

  async function load() {
    try {
      const r = await api.pos.items();
      items = r.items;
      categories = r.categories ?? [];
      loadError = '';
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'No se pudo cargar el catálogo.';
    } finally {
      loading = false;
    }
  }
  onMount(load);

  // ----- filters -----
  let tab = $state<'all' | ItemKind>('all');
  let search = $state('');
  let category = $state('');
  let showArchived = $state(false);
  let sortKey = $state<'name' | 'category' | 'price' | 'stock'>('name');
  let sortDir = $state<1 | -1>(1);

  const visible = $derived.by(() => {
    const q = normalize(search);
    const list = items.filter(
      (i) =>
        (showArchived || i.active) &&
        (tab === 'all' || i.kind === tab) &&
        (!category || i.category === category) &&
        (!q || normalize(i.name).includes(q) || normalize(i.sku).includes(q) || i.barcode.includes(search.trim()))
    );
    const val = (i: CatalogItem) => (sortKey === 'price' ? i.price_cents : sortKey === 'stock' ? (i.track_stock ? i.stock : -1) : normalize(i[sortKey]));
    return list.sort((a, b) => {
      const x = val(a);
      const y = val(b);
      return (x < y ? -1 : x > y ? 1 : 0) * sortDir || a.name.localeCompare(b.name, 'es');
    });
  });
  const counts = $derived({
    all: items.filter((i) => showArchived || i.active).length,
    service: items.filter((i) => (showArchived || i.active) && i.kind === 'service').length,
    product: items.filter((i) => (showArchived || i.active) && i.kind === 'product').length
  });
  const archivedCount = $derived(items.filter((i) => !i.active).length);

  function sortBy(k: typeof sortKey) {
    if (sortKey === k) sortDir = sortDir === 1 ? -1 : 1;
    else {
      sortKey = k;
      sortDir = 1;
    }
  }
  const ariaSort = (k: typeof sortKey) => (sortKey === k ? (sortDir === 1 ? 'ascending' : 'descending') : 'none');

  // ----- form -----
  let formOpen = $state(false);
  let editing = $state<CatalogItem | null>(null);
  let startKind = $state<ItemKind>('service');
  function openCreate() {
    editing = null;
    startKind = tab === 'product' ? 'product' : 'service';
    formOpen = true;
  }
  function openEdit(i: CatalogItem) {
    editing = i;
    formOpen = true;
  }
  async function saved() {
    formOpen = false;
    await load();
  }

  // ----- inline price -----
  let priceEditId = $state<string | null>(null);
  let priceDraft = $state('');
  let priceBusy = $state(false);
  function startPrice(i: CatalogItem) {
    if (!canManage || !i.active) return;
    priceEditId = i.id;
    priceDraft = String(i.price_cents / 100);
  }
  async function savePrice(i: CatalogItem) {
    if (priceBusy) return;
    const p = parsePesos(priceDraft);
    if (Number.isNaN(p) || p < 0) return toast.show('Escribe un precio válido.', 'error');
    if (toCents(p) === i.price_cents) return void (priceEditId = null);
    priceBusy = true;
    try {
      const u = await api.pos.updateItem(i.id, toInput(i, { price_cents: toCents(p) }));
      items = items.map((x) => (x.id === u.id ? u : x));
      priceEditId = null;
      toast.show('Precio actualizado');
    } catch (e) {
      toast.show(e instanceof Error ? e.message : 'No se pudo guardar el precio.', 'error');
    } finally {
      priceBusy = false;
    }
  }
  function focusSelect(node: HTMLInputElement) {
    node.focus();
    node.select();
  }
  function priceKey(e: KeyboardEvent, i: CatalogItem) {
    if (e.key === 'Enter') {
      e.preventDefault();
      savePrice(i);
    } else if (e.key === 'Escape') {
      e.stopPropagation();
      priceEditId = null;
    }
  }

  // ----- archive / restore -----
  let deleting = $state<CatalogItem | null>(null);
  const delOp = new Op();
  async function confirmDelete() {
    const it = deleting;
    if (!it) return;
    let archived = false;
    if (!(await delOp.run(async () => (archived = (await api.pos.deleteItem(it.id)).archived)))) return;
    deleting = null;
    toast.show(archived ? `“${it.name}” tiene ventas registradas, así que se archivó en lugar de borrarse. Ya no aparece al cobrar.` : `“${it.name}” se eliminó`);
    await load();
  }
  async function restore(i: CatalogItem) {
    try {
      await api.pos.updateItem(i.id, toInput(i, { active: true }));
      toast.show(`“${i.name}” se reactivó`);
      await load();
    } catch (e) {
      toast.show(e instanceof Error ? e.message : 'No se pudo reactivar.', 'error');
    }
  }

  let importOpen = $state(false);
  let magicInvOpen = $state(false);
  let magicPriceOpen = $state(false);
  const anyFilter = $derived(!!search || !!category || tab !== 'all');
  function clearFilters() {
    search = '';
    category = '';
    tab = 'all';
  }
</script>

{#snippet priceCell(i: CatalogItem)}
  {#if priceEditId === i.id}
    <input
      class="field !min-h-9 !w-28 !px-2 text-right"
      inputmode="decimal"
      aria-label="Nuevo precio de {i.name}"
      bind:value={priceDraft}
      use:focusSelect
      onkeydown={(e) => priceKey(e, i)}
      onblur={() => !priceBusy && (priceEditId = null)}
    />
  {:else if canManage && i.active}
    <button type="button" class="rounded-lg px-2 py-1 tabular-nums transition hover:bg-app-primary/10 hover:text-app-primary" title="Clic para cambiar el precio" onclick={() => startPrice(i)}>{moneyCents(i.price_cents)}</button>
  {:else}
    <span class="tabular-nums">{moneyCents(i.price_cents)}</span>
  {/if}
{/snippet}

{#snippet stockCell(i: CatalogItem)}
  {#if i.kind === 'service'}<span class="text-app-muted">—</span>
  {:else if !i.track_stock}<span class="text-xs text-app-muted">Sin control</span>
  {:else}<span class="tabular-nums {isLow(i) ? 'font-medium text-app-danger' : ''}">{qty(i.stock)} <span class="text-xs text-app-muted">{i.unit}</span></span>{/if}
{/snippet}

{#snippet statusCell(i: CatalogItem)}
  {#if !i.active}<span class="pill">Archivado</span>
  {:else if isLow(i)}<span class="pill pill-bad">{i.stock <= 0 ? 'Agotado' : 'Bajo mínimo'}</span>
  {:else}<span class="pill pill-ok">Activo</span>{/if}
{/snippet}

{#snippet rowActions(i: CatalogItem)}
  {#if i.active}
    <button type="button" class="icon-btn" aria-label="Editar {i.name}" onclick={() => openEdit(i)}><Icon name="edit" size={17} /></button>
    <button type="button" class="icon-btn danger" aria-label="Archivar o eliminar {i.name}" onclick={() => ((deleting = i), delOp.reset())}><Icon name="trash" size={17} /></button>
  {:else}
    <button type="button" class="btn-ghost min-h-8 px-3 text-xs" onclick={() => restore(i)}><Icon name="refresh" size={14} />Reactivar</button>
  {/if}
{/snippet}

<PageHeader title="Servicios y precios" subtitle="Lo que cobras en tu consultorio: servicios, productos y su precio.">
  {#snippet actions()}
    {#if canManage}
      <button type="button" class="btn-secondary" onclick={() => (magicInvOpen = true)}><Icon name="sparkles" size={16} />Inventario Mágico</button>
      <button type="button" class="btn-secondary" onclick={() => (magicPriceOpen = true)}><Icon name="sparkles" size={16} />Precio Mágico</button>
      <button type="button" class="btn-secondary" onclick={() => (importOpen = true)}><Icon name="receipt" size={16} />Importar</button>
      <button type="button" class="btn-primary" onclick={openCreate}><Icon name="plus" size={16} />Nuevo</button>
    {/if}
  {/snippet}
</PageHeader>

<div class="card overflow-hidden">
  <div class="space-y-3 border-b border-app-ink/10 p-4">
    <div class="flex flex-wrap items-center gap-3">
      <div role="tablist" aria-label="Tipo" class="inline-flex rounded-full bg-app-ink/6 p-1">
        {#each [['all', 'Todo'], ['service', 'Servicios'], ['product', 'Productos']] as [k, label]}
          <button type="button" role="tab" aria-selected={tab === k} class="rounded-full px-4 py-1.5 text-sm font-medium transition {tab === k ? 'bg-app-panel shadow-sm' : 'text-app-muted hover:text-app-ink'}" onclick={() => (tab = k as typeof tab)}>
            {label} <span class="text-xs text-app-muted">{counts[k as keyof typeof counts]}</span>
          </button>
        {/each}
      </div>
      {#if archivedCount}
        <label class="ml-auto flex cursor-pointer items-center gap-2 text-sm text-app-muted">
          <input type="checkbox" class="h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={showArchived} />Mostrar archivados ({archivedCount})
        </label>
      {/if}
    </div>
    <div class="flex flex-wrap gap-3">
      <div class="relative min-w-[14rem] flex-1">
        <label class="sr-only" for="cat-search">Buscar por nombre, SKU o código de barras</label>
        <Icon name="search" size={17} class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-app-muted" />
        <input id="cat-search" type="search" class="field pl-10" placeholder="Buscar por nombre, SKU o código de barras" bind:value={search} autocomplete="off" />
      </div>
      {#if categories.length}
        <div class="sm:w-56">
          <label class="sr-only" for="cat-filter">Categoría</label>
          <select id="cat-filter" class="field" bind:value={category}>
            <option value="">Todas las categorías</option>
            {#each categories as c}<option value={c}>{c}</option>{/each}
          </select>
        </div>
      {/if}
    </div>
  </div>

  {#if loading}
    <LoadingRows />
  {:else if loadError}
    <p class="alert m-5" role="alert"><Icon name="alert" size={18} />{loadError}</p>
  {:else if !items.length}
    <EmptyState icon="tag" title="Tu catálogo está vacío" text={canManage ? 'Agrega tus servicios y productos para poder cobrarlos. Puedes capturarlos uno por uno, importarlos de una hoja de cálculo o dejar que la magia los arme por ti.' : 'Todavía no hay servicios ni productos. Pide a un administrador que los agregue.'}>
      {#if canManage}
        <div class="flex flex-wrap justify-center gap-2">
          <button type="button" class="btn-primary" onclick={openCreate}><Icon name="plus" size={16} />Agregar el primero</button>
          <button type="button" class="btn-secondary" onclick={() => (magicInvOpen = true)}><Icon name="sparkles" size={16} />Inventario Mágico</button>
          <button type="button" class="btn-secondary" onclick={() => (importOpen = true)}>Importar</button>
        </div>
      {/if}
    </EmptyState>
  {:else if !visible.length}
    <EmptyState icon="search" title="Sin resultados" text="Ningún artículo coincide con tu búsqueda o filtros.">
      {#if anyFilter}<button type="button" class="btn-secondary" onclick={clearFilters}>Quitar filtros</button>{/if}
    </EmptyState>
  {:else}
    <!-- table (tablet and up) -->
    <div class="hidden overflow-x-auto md:block">
      <table class="w-full min-w-[44rem]">
        <thead class="border-b border-app-ink/10 bg-app-elevated">
          <tr>
            {#each [['name', 'Nombre', ''], ['category', 'Categoría', ''], ['price', 'Precio', 'text-right']] as [k, label, cls]}
              <th class="th {cls}" aria-sort={ariaSort(k as typeof sortKey)}>
                <button type="button" class="inline-flex items-center gap-1 uppercase tracking-[0.12em] hover:text-app-ink" onclick={() => sortBy(k as typeof sortKey)}>{label}{#if sortKey === k}<span aria-hidden="true">{sortDir === 1 ? '↑' : '↓'}</span>{/if}</button>
              </th>
            {/each}
            {#if canManage}<th class="th text-right">Costo</th><th class="th text-right">Margen</th>{/if}
            <th class="th text-right">IVA</th>
            <th class="th text-right" aria-sort={ariaSort('stock')}>
              <button type="button" class="inline-flex items-center gap-1 uppercase tracking-[0.12em] hover:text-app-ink" onclick={() => sortBy('stock')}>Existencia{#if sortKey === 'stock'}<span aria-hidden="true">{sortDir === 1 ? '↑' : '↓'}</span>{/if}</button>
            </th>
            <th class="th">Estado</th>
            {#if canManage}<th class="th"><span class="sr-only">Acciones</span></th>{/if}
          </tr>
        </thead>
        <tbody class="divide-y divide-app-ink/8">
          {#each visible as i (i.id)}
            {@const m = marginPct(i.price_cents, i.cost_cents)}
            <tr class="hover:bg-app-ink/[0.025] {i.active ? '' : 'opacity-60'}">
              <td class="td">
                <span class="font-medium">{i.name}</span>
                <span class="block text-xs text-app-muted">{i.kind === 'product' ? 'Producto' : 'Servicio'}{i.sku ? ` · ${i.sku}` : ''}{i.barcode ? ` · ${i.barcode}` : ''}</span>
              </td>
              <td class="td text-app-muted">{i.category || '—'}</td>
              <td class="td text-right">{@render priceCell(i)}</td>
              {#if canManage}
                <td class="td text-right tabular-nums text-app-muted">{i.cost_cents ? moneyCents(i.cost_cents) : '—'}</td>
                <td class="td text-right tabular-nums {m !== null && m < 0 ? 'text-app-danger' : 'text-app-muted'}">{m !== null ? pct(m) : '—'}</td>
              {/if}
              <td class="td text-right tabular-nums text-app-muted">{i.tax_rate} %</td>
              <td class="td text-right">{@render stockCell(i)}</td>
              <td class="td">{@render statusCell(i)}</td>
              {#if canManage}<td class="td"><div class="flex justify-end gap-1">{@render rowActions(i)}</div></td>{/if}
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <!-- cards (phones) -->
    <ul class="divide-y divide-app-ink/8 md:hidden">
      {#each visible as i (i.id)}
        {@const m = marginPct(i.price_cents, i.cost_cents)}
        <li class="space-y-2 p-4 {i.active ? '' : 'opacity-60'}">
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <p class="font-medium">{i.name}</p>
              <p class="text-xs text-app-muted">{i.kind === 'product' ? 'Producto' : 'Servicio'}{i.category ? ` · ${i.category}` : ''}{i.sku ? ` · ${i.sku}` : ''}</p>
            </div>
            <div class="text-right">{@render priceCell(i)}</div>
          </div>
          <div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-app-muted">
            {@render statusCell(i)}
            {#if i.kind === 'product'}<span>Existencia: {@render stockCell(i)}</span>{/if}
            <span>IVA {i.tax_rate} %</span>
            {#if canManage && i.cost_cents}<span>Costo {moneyCents(i.cost_cents)}{m !== null ? ` · ${pct(m)}` : ''}</span>{/if}
          </div>
          {#if canManage}<div class="flex justify-end gap-1">{@render rowActions(i)}</div>{/if}
        </li>
      {/each}
    </ul>
  {/if}
</div>

{#if canManage}
  <ItemForm open={formOpen} item={editing} {categories} initialKind={startKind} onsaved={saved} onclose={() => (formOpen = false)} />
  <ImportModal open={importOpen} ondone={load} onclose={() => (importOpen = false)} />
  <MagicInventoryModal open={magicInvOpen} ondone={load} onclose={() => (magicInvOpen = false)} />
  <MagicPriceModal open={magicPriceOpen} {items} ondone={load} onclose={() => (magicPriceOpen = false)} />
  <ConfirmModal open={!!deleting} title="Quitar artículo" op={delOp} confirmLabel="Quitar" onconfirm={confirmDelete} onclose={() => (deleting = null)}>
    <p>¿Quitar <strong class="text-app-ink">{deleting?.name}</strong> de tu catálogo?</p>
    <p class="mt-2 text-sm">Si ya tiene ventas registradas se archivará (no se pierde el historial) y podrás reactivarlo; si no, se elimina por completo.</p>
  </ConfirmModal>
{/if}
