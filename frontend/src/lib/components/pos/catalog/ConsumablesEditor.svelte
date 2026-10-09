<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { untrack } from 'svelte';
  import { api } from '$lib/api';
  import { pos2 } from '$lib/api/pos2';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { CatalogItem } from '$lib/types';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { parsePesos, qty as qtyText } from './helpers';

  interface Props {
    /** Saved service whose supplies are edited; null while the service is not saved yet. */
    serviceId: string | null;
    open: boolean;
  }
  let { serviceId, open }: Props = $props();

  const uid = $props.id();
  const op = new Op();
  interface Row {
    key: number;
    product_id: string;
    qty: string;
  }
  let seq = 0;
  let rows = $state<Row[]>([]);
  let products = $state<CatalogItem[]>([]);
  let loading = $state(false);
  let loadError = $state('');
  let dirty = $state(false);

  $effect(() => {
    if (!open || !serviceId) return;
    const id = serviceId;
    untrack(() => load(id));
  });

  async function load(id: string) {
    loading = true;
    loadError = '';
    dirty = false;
    op.reset();
    try {
      const [cons, items] = await Promise.all([pos2.consumables(id), api.pos.items({ kind: 'product', active: true })]);
      products = items.items.filter((p) => p.track_stock);
      rows = cons.map((c) => ({ key: ++seq, product_id: c.product_id, qty: String(c.qty) }));
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'No se pudieron cargar los consumibles.';
    } finally {
      loading = false;
    }
  }

  const unitOf = (id: string) => products.find((p) => p.id === id)?.unit ?? '';
  const used = $derived(new Set(rows.map((r) => r.product_id)));
  const available = $derived(products.filter((p) => !used.has(p.id)));
  let toAdd = $state('');

  function add() {
    if (!toAdd) return;
    rows.push({ key: ++seq, product_id: toAdd, qty: '1' });
    toAdd = '';
    dirty = true;
  }

  const invalid = $derived(rows.some((r) => !(parsePesos(r.qty) > 0)));

  async function save() {
    if (!serviceId || invalid) return;
    const id = serviceId;
    const body = rows.map((r) => ({ product_id: r.product_id, qty: parsePesos(r.qty) }));
    if (await op.run(() => pos2.saveConsumables(id, body))) {
      dirty = false;
      toast.show('Consumibles guardados');
    }
  }
</script>

<div class="rounded-2xl border border-app-ink/10 bg-app-elevated p-4">
  <h3 class="text-sm font-medium">Consumibles del servicio</h3>
  <p class="text-xs text-app-muted">Insumos que se descuentan del inventario cada vez que vendes este servicio (el que caduca primero sale primero).</p>

  {#if !serviceId}
    <p class="mt-3 text-sm text-app-muted">Guarda el servicio y vuelve a editarlo para agregar sus consumibles.</p>
  {:else if loading}
    <p class="mt-3 text-sm text-app-muted" role="status">Cargando…</p>
  {:else if loadError}
    <Alert class="mt-3">{loadError}</Alert>
  {:else}
    {#if rows.length}
      <ul class="mt-3 space-y-2">
        {#each rows as r (r.key)}
          <li class="flex items-center gap-2">
            <span class="min-w-0 flex-1 truncate text-sm">{products.find((p) => p.id === r.product_id)?.name ?? 'Producto'}</span>
            <label class="sr-only" for="{uid}-{r.key}">Cantidad de {products.find((p) => p.id === r.product_id)?.name}</label>
            <input
              id="{uid}-{r.key}"
              class="field !min-h-9 !w-20 !px-2 text-right"
              inputmode="decimal"
              autocomplete="off"
              bind:value={r.qty}
              aria-invalid={!(parsePesos(r.qty) > 0)}
              oninput={() => (dirty = true)}
            />
            <span class="w-12 text-xs text-app-muted">{unitOf(r.product_id)}</span>
            <button
              type="button"
              class="icon-btn danger"
              aria-label="Quitar consumible"
              onclick={() => ((rows = rows.filter((x) => x.key !== r.key)), (dirty = true))}><Icon name="x" size={16} /></button
            >
          </li>
        {/each}
      </ul>
    {:else}
      <p class="mt-3 text-sm text-app-muted">Sin consumibles.</p>
    {/if}
    {#if products.length}
      <div class="mt-3 flex flex-wrap items-center gap-2">
        <label class="sr-only" for="{uid}-add">Producto a agregar</label>
        <select id="{uid}-add" class="field !min-h-9 !w-auto min-w-0 flex-1 !py-1" bind:value={toAdd} disabled={!available.length}>
          <option value="">{available.length ? 'Agregar un producto…' : 'Ya agregaste todos'}</option>
          {#each available as p (p.id)}<option value={p.id}>{p.name} ({qtyText(p.stock)} {p.unit})</option>{/each}
        </select>
        <button type="button" class="btn-secondary min-h-9" disabled={!toAdd} onclick={add}><Icon name="plus" size={16} />Agregar</button>
      </div>
    {:else}
      <p class="hint mt-3">Aún no tienes productos con control de existencias.</p>
    {/if}
    <OpError op={op} class="mt-3" />
    <div class="mt-3 flex justify-end">
      <button type="button" class="btn-secondary min-h-9" disabled={!dirty || invalid || op.phase === 'loading'} onclick={save}>
        {#if op.phase === 'loading'}<span class="spin"></span>{/if}Guardar consumibles
      </button>
    </div>
  {/if}
</div>
