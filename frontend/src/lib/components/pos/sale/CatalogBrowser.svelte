<script lang="ts">
  import { onMount } from 'svelte';
  import { moneyCents } from '$lib/format';
  import type { CatalogItem } from '$lib/types';
  import Icon from '$lib/components/ui/Icon.svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import { qtyText } from './money';

  interface Props {
    items: CatalogItem[];
    loading: boolean;
    allowNegative: boolean;
    /** Quantity of each item already in the cart. */
    inCart: Map<string, number>;
    onadd: (item: CatalogItem) => void;
  }
  let { items, loading, allowNegative, inCart, onadd }: Props = $props();

  type Tab = 'service' | 'product' | 'all';
  let tab = $state<Tab>('all');
  let category = $state('');
  let q = $state('');
  let search = $state<HTMLInputElement>();

  onMount(() => search?.focus());

  const norm = (s: string) => s.toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '');

  const soldOut = (i: CatalogItem) => i.track_stock && !allowNegative && i.stock <= 0;
  const low = (i: CatalogItem) => i.track_stock && i.stock > 0 && i.stock <= i.min_stock;

  const shownCategories = $derived(
    [...new Set(items.filter((i) => tab === 'all' || i.kind === tab).map((i) => i.category).filter(Boolean))].sort((a, b) => a.localeCompare(b, 'es'))
  );

  const filtered = $derived.by(() => {
    const needle = norm(q.trim());
    return items.filter((i) => {
      if (tab !== 'all' && i.kind !== tab) return false;
      if (category && i.category !== category) return false;
      if (!needle) return true;
      return norm(i.name).includes(needle) || norm(i.sku).includes(needle) || i.barcode.includes(needle) || norm(i.category).includes(needle);
    });
  });

  $effect(() => {
    // The chosen category may not exist in the new tab
    if (category && !shownCategories.includes(category)) category = '';
  });

  function pick(i: CatalogItem) {
    if (soldOut(i)) return;
    onadd(i);
    search?.focus();
  }

  /** Scanners type the code then Enter: an exact barcode/SKU match goes straight to the cart. */
  function onEnter(e: KeyboardEvent) {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    const code = q.trim().toLowerCase();
    if (!code) return;
    const exact = items.find((i) => (i.barcode && i.barcode.toLowerCase() === code) || (i.sku && i.sku.toLowerCase() === code));
    const target = exact ?? (filtered.length === 1 ? filtered[0] : undefined);
    if (target) {
      pick(target);
      q = '';
    }
  }

  const tabs: { id: Tab; label: string }[] = [
    { id: 'service', label: 'Servicios' },
    { id: 'product', label: 'Productos' },
    { id: 'all', label: 'Todo' }
  ];
</script>

<section aria-label="Catálogo" class="min-w-0">
  <div class="flex flex-wrap items-center gap-3">
    <div class="relative min-w-[14rem] flex-1">
      <Icon name="search" size={18} class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-app-muted" />
      <input
        bind:this={search}
        bind:value={q}
        onkeydown={onEnter}
        type="search"
        class="field pl-10"
        placeholder="Buscar o escanear código de barras"
        aria-label="Buscar en el catálogo o escanear código de barras"
        autocomplete="off"
        enterkeyhint="search"
      />
    </div>
    <div class="flex rounded-full bg-app-ink/5 p-1" role="tablist" aria-label="Tipo de concepto">
      {#each tabs as t (t.id)}
        <button
          type="button"
          role="tab"
          aria-selected={tab === t.id}
          class="min-h-10 rounded-full px-4 text-sm font-medium transition {tab === t.id ? 'bg-app-panel text-app-ink shadow-sm' : 'text-app-muted hover:text-app-ink'}"
          onclick={() => (tab = t.id)}>{t.label}</button
        >
      {/each}
    </div>
  </div>

  {#if shownCategories.length}
    <div class="-mx-1 mt-3 flex gap-2 overflow-x-auto px-1 pb-1" role="group" aria-label="Categorías">
      <button type="button" class="chip {category === '' ? 'chip-on' : ''}" aria-pressed={category === ''} onclick={() => (category = '')}>Todas</button>
      {#each shownCategories as c (c)}
        <button type="button" class="chip {category === c ? 'chip-on' : ''}" aria-pressed={category === c} onclick={() => (category = category === c ? '' : c)}>{c}</button>
      {/each}
    </div>
  {/if}

  {#if loading}
    <div class="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-4" aria-busy="true">
      {#each Array(8) as _, n (n)}<div class="h-28 animate-pulse rounded-2xl bg-app-ink/5"></div>{/each}
    </div>
  {:else if filtered.length === 0}
    <div class="card mt-4">
      <EmptyState
        icon="search"
        title={items.length ? 'Sin resultados' : 'Aún no hay conceptos'}
        text={items.length ? 'Prueba con otra búsqueda o categoría. También puedes agregar un concepto libre en la cuenta.' : 'Da de alta tus servicios y productos en Inventario, o usa “Concepto libre” para cobrar de inmediato.'}
      />
    </div>
  {:else}
    <ul class="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-4">
      {#each filtered as i (i.id)}
        {@const out = soldOut(i)}
        {@const n = inCart.get(i.id) ?? 0}
        <li>
          <button
            type="button"
            disabled={out}
            onclick={() => pick(i)}
            class="card relative flex min-h-28 w-full flex-col items-start justify-between gap-2 p-3.5 text-left transition hover:border-app-primary/40 hover:bg-app-elevated active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-55"
            aria-label="{i.name}, {moneyCents(i.price_cents)}{out ? ', agotado' : ''}{n ? `, ${qtyText(n)} en la cuenta` : ''}"
          >
            <span class="line-clamp-2 text-[15px] font-medium leading-snug">{i.name}</span>
            <span class="flex w-full flex-wrap items-end justify-between gap-1.5">
              <span class="display text-xl tabular-nums">{moneyCents(i.price_cents)}</span>
              {#if out}
                <span class="pill pill-bad">Agotado</span>
              {:else if low(i)}
                <span class="pill pill-warn">Quedan {qtyText(i.stock)}</span>
              {:else if i.track_stock}
                <span class="pill">{qtyText(i.stock)} {i.unit || 'pza'}</span>
              {:else if i.kind === 'service'}
                <span class="pill pill-info">Servicio</span>
              {/if}
            </span>
            {#if n}
              <span class="absolute -right-1.5 -top-1.5 grid h-6 min-w-6 place-items-center rounded-full bg-app-primary px-1.5 text-xs font-semibold text-app-on-primary tabular-nums">{qtyText(n)}</span>
            {/if}
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  .chip {
    display: inline-flex;
    min-height: 2.5rem;
    flex: none;
    align-items: center;
    border-radius: 9999px;
    padding: 0 0.95rem;
    font-size: 0.85rem;
    font-weight: 500;
    color: rgb(var(--app-muted));
    box-shadow: inset 0 0 0 1px rgb(var(--app-ink) / 0.15);
    transition: background-color 0.15s, color 0.15s;
  }
  .chip:hover {
    background: rgb(var(--app-ink) / 0.05);
    color: rgb(var(--app-ink));
  }
  .chip-on {
    background: rgb(var(--app-primary) / 0.12);
    color: rgb(var(--app-primary));
    box-shadow: inset 0 0 0 1px rgb(var(--app-primary) / 0.4);
  }
</style>
