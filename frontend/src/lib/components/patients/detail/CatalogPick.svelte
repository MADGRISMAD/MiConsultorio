<script lang="ts">
  import { onDestroy } from 'svelte';
  import { consult } from '$lib/api/consult';
  import { moneyCents } from '$lib/format';
  import type { ChargeCatalogItem } from '$lib/types/consult';
  import Combobox from '../../ui/Combobox.svelte';

  interface Props {
    id: string;
    label: string;
    placeholder?: string;
    /** '' = services and products; 'product' = supplies only */
    kind?: '' | 'service' | 'product';
    /** supplies are taken out of stock, so a line without valid stock cannot be picked */
    needStock?: boolean;
    onpick: (item: ChargeCatalogItem) => void;
  }
  let { id, label, placeholder = 'Buscar en tu catálogo…', kind = '', needStock = false, onpick }: Props = $props();

  let q = $state('');
  let results = $state<ChargeCatalogItem[]>([]);
  let loading = $state(false);
  let error = $state('');
  let timer: ReturnType<typeof setTimeout> | undefined;
  let seq = 0;

  async function run() {
    const my = ++seq;
    loading = true;
    error = '';
    try {
      const r = await consult.catalog(q.trim(), kind);
      if (my === seq) results = r;
    } catch (e) {
      if (my === seq) error = e instanceof Error ? e.message : 'No se pudo consultar el catálogo.';
    } finally {
      if (my === seq) loading = false;
    }
  }
  function input() {
    clearTimeout(timer);
    timer = setTimeout(run, 250);
  }
  function focus() {
    if (!results.length) void run();
  }
  function pick(it: ChargeCatalogItem) {
    onpick(it);
    q = '';
  }
  const blocked = (it: ChargeCatalogItem) => needStock && (!it.track_stock || it.usable_stock <= 0);
  onDestroy(() => clearTimeout(timer));
</script>

<Combobox
  {id}
  {label}
  {placeholder}
  bind:query={q}
  options={results.map((it) => ({ key: it.id, value: it, disabled: blocked(it) }))}
  status={loading && !results.length ? 'Buscando…' : error ? error : !results.length ? 'Sin resultados en el catálogo.' : ''}
  statusTone={error ? 'danger' : 'muted'}
  oninput={input}
  onopen={focus}
  onpick={(o) => pick(o.value)}
>
  {#snippet option(o)}
    {@const it = o.value}
    <span class="flex items-start justify-between gap-3 px-3 py-2">
      <span class="min-w-0">
        <span class="block truncate text-sm font-medium">{it.name}</span>
        {#if it.track_stock}
          <span class="block text-xs text-app-muted">Existencias: {it.usable_stock} {it.unit}{it.next_expiry ? ` · caduca ${it.next_expiry}` : ''}</span>
        {/if}
        {#if it.stock_warning}<span class="block text-xs text-app-warning">{it.stock_warning}</span>{/if}
      </span>
      <span class="shrink-0 text-sm tabular-nums text-app-muted">{moneyCents(it.price_cents)}</span>
    </span>
  {/snippet}
</Combobox>
