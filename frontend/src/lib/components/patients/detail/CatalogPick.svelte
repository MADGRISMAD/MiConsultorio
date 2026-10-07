<script lang="ts">
  import { onDestroy } from 'svelte';
  import { consult } from '$lib/api/consult';
  import { moneyCents } from '$lib/format';
  import type { ChargeCatalogItem } from '$lib/types/consult';
  import Icon from '../../ui/Icon.svelte';

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
  let open = $state(false);
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
    open = true;
    clearTimeout(timer);
    timer = setTimeout(run, 250);
  }
  function focus() {
    open = true;
    if (!results.length) void run();
  }
  function blur(ev: FocusEvent) {
    const next = ev.relatedTarget as Node | null;
    if (next && (ev.currentTarget as HTMLElement).parentElement?.contains(next)) return;
    open = false;
  }
  function pick(it: ChargeCatalogItem) {
    onpick(it);
    q = '';
    open = false;
  }
  const blocked = (it: ChargeCatalogItem) => needStock && (!it.track_stock || it.usable_stock <= 0);
  onDestroy(() => clearTimeout(timer));
</script>

<div class="relative">
  <label class="label" for={id}>{label}</label>
  <div class="relative">
    <Icon name="search" size={16} class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-app-muted" />
    <input {id} class="field pl-9" autocomplete="off" {placeholder} bind:value={q} oninput={input} onfocus={focus} onblur={blur} role="combobox" aria-expanded={open} aria-controls="{id}-list" />
  </div>
  {#if open}
    <ul id="{id}-list" class="absolute z-20 mt-1 max-h-64 w-full overflow-auto rounded-xl border border-app-ink/15 bg-app-panel p-1 shadow-lg" role="listbox" aria-label={label}>
      {#if loading && !results.length}
        <li class="px-3 py-2 text-sm text-app-muted">Buscando…</li>
      {:else if error}
        <li class="px-3 py-2 text-sm text-app-danger" role="alert">{error}</li>
      {:else if !results.length}
        <li class="px-3 py-2 text-sm text-app-muted">Sin resultados en el catálogo.</li>
      {/if}
      {#each results as it (it.id)}
        <li role="option" aria-selected="false" aria-disabled={blocked(it)}>
          <button type="button" class="flex w-full items-start justify-between gap-3 rounded-lg px-3 py-2 text-left hover:bg-app-ink/5 disabled:cursor-not-allowed disabled:opacity-55" disabled={blocked(it)} onmousedown={(e) => e.preventDefault()} onclick={() => pick(it)}>
            <span class="min-w-0">
              <span class="block truncate text-sm font-medium">{it.name}</span>
              {#if it.track_stock}
                <span class="block text-xs text-app-muted">Existencias: {it.usable_stock} {it.unit}{it.next_expiry ? ` · caduca ${it.next_expiry}` : ''}</span>
              {/if}
              {#if it.stock_warning}<span class="block text-xs text-app-warning">{it.stock_warning}</span>{/if}
            </span>
            <span class="shrink-0 text-sm tabular-nums text-app-muted">{moneyCents(it.price_cents)}</span>
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</div>
