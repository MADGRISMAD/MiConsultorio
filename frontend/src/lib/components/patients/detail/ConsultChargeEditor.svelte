<script lang="ts">
  import { moneyCents } from '$lib/format';
  import type { ChargeCatalogItem, ChargeDraftLine } from '$lib/types/consult';
  import Icon from '../../ui/Icon.svelte';
  import CatalogPick from './CatalogPick.svelte';
  import { draftFree, draftFromCatalog, lineTotal } from './consultLines';

  interface Props {
    lines: ChargeDraftLine[];
    /** the plan includes cobros: catalog, prices and stock */
    cobros: boolean;
    /** unique prefix for element ids */
    uid: string;
  }
  let { lines = $bindable(), cobros, uid }: Props = $props();

  let free = $state('');
  const billLines = $derived(lines.filter((l) => !l.consumed));
  const supplyLines = $derived(lines.filter((l) => l.consumed));
  const total = $derived(lines.reduce((a, l) => a + lineTotal(l), 0));

  function addCatalog(it: ChargeCatalogItem, consumed: boolean) {
    const exist = lines.find((l) => !l.locked && l.catalog_item_id === it.id && l.consumed === consumed);
    if (exist) exist.qty = Math.round((exist.qty + 1) * 1000) / 1000;
    else lines.push(draftFromCatalog(it, consumed));
  }
  function addFree() {
    const n = free.trim();
    if (!n) return;
    lines.push(draftFree(n));
    free = '';
  }
  function remove(key: string) {
    lines = lines.filter((l) => l.key !== key);
  }
  const over = (l: ChargeDraftLine) => l.consumed && !l.locked && l.track_stock && l.usable_stock !== undefined && l.qty > l.usable_stock;
</script>

{#snippet qty(l: ChargeDraftLine)}
  <input
    type="number"
    class="field w-24 text-right"
    min="0.001"
    step="any"
    inputmode="decimal"
    aria-label="Cantidad de {l.name}"
    disabled={l.locked}
    value={l.qty}
    oninput={(e) => (l.qty = Number((e.currentTarget as HTMLInputElement).value) || 0)}
  />
{/snippet}

{#if cobros}
  <div class="space-y-6">
    <section aria-labelledby="{uid}-bill">
      <h3 id="{uid}-bill" class="section-title mb-2">Servicios y productos a cobrar</h3>
      <CatalogPick id="{uid}-svc" label="Agregar servicio o producto" onpick={(it) => addCatalog(it, false)} />
      {#if billLines.length === 0}
        <p class="hint mt-2">Busca en tu catálogo lo que realizaste. Pasará solo a caja con su precio.</p>
      {:else}
        <ul class="mt-3 divide-y divide-app-ink/10 rounded-xl border border-app-ink/10">
          {#each billLines as l (l.key)}
            <li class="flex flex-wrap items-center gap-3 px-3 py-2.5">
              <div class="min-w-0 flex-1">
                <p class="truncate text-sm font-medium">{l.name}</p>
                <p class="text-xs text-app-muted">{moneyCents(l.price_cents)} c/u{l.catalog_item_id ? '' : ' · concepto libre'}</p>
              </div>
              {@render qty(l)}
              <p class="w-24 text-right text-sm font-medium tabular-nums">{moneyCents(lineTotal(l))}</p>
              <button type="button" class="icon-btn danger" aria-label="Quitar {l.name}" onclick={() => remove(l.key)}><Icon name="trash" size={16} /></button>
            </li>
          {/each}
        </ul>
      {/if}
    </section>

    <section aria-labelledby="{uid}-sup">
      <h3 id="{uid}-sup" class="section-title mb-2">Insumos usados</h3>
      <CatalogPick id="{uid}-sup-pick" label="Agregar insumo usado" kind="product" needStock placeholder="Gasas, jeringas, anestésico…" onpick={(it) => addCatalog(it, true)} />
      {#if supplyLines.length === 0}
        <p class="hint mt-2">Lo que agregues aquí sale del inventario al guardar, una sola vez; caja ya no lo descuenta al cobrar.</p>
      {:else}
        <ul class="mt-3 divide-y divide-app-ink/10 rounded-xl border border-app-ink/10">
          {#each supplyLines as l (l.key)}
            <li class="px-3 py-2.5">
              <div class="flex flex-wrap items-center gap-3">
                <div class="min-w-0 flex-1">
                  <p class="truncate text-sm font-medium">{l.name}</p>
                  <p class="text-xs text-app-muted">
                    {#if l.locked}
                      <span class="inline-flex items-center gap-1"><Icon name="lock" size={12} />Ya descontado del inventario</span>
                    {:else if l.usable_stock !== undefined}
                      Existencias vigentes: {l.usable_stock} {l.unit}
                    {/if}
                    {#if !l.locked || !l.no_charge} · {moneyCents(l.price_cents)} c/u{/if}
                  </p>
                </div>
                {@render qty(l)}
                {#if !l.locked}<button type="button" class="icon-btn danger" aria-label="Quitar {l.name}" onclick={() => remove(l.key)}><Icon name="trash" size={16} /></button>{/if}
              </div>
              {#if !l.locked}
                <label class="mt-2 flex cursor-pointer items-center gap-2 text-sm">
                  <input type="checkbox" class="h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={l.no_charge} />
                  No cobrar al paciente (solo descontar del inventario)
                </label>
              {:else if l.no_charge}
                <p class="mt-1 text-xs text-app-muted">No se cobra al paciente.</p>
              {/if}
              {#if l.warning && !l.locked}<p class="mt-1.5 flex items-center gap-1.5 text-xs text-app-warning"><Icon name="alert" size={13} />{l.warning}</p>{/if}
              {#if over(l)}<p class="mt-1.5 flex items-center gap-1.5 text-xs text-app-danger" role="alert"><Icon name="alert" size={13} />Solo hay {l.usable_stock} {l.unit} vigentes; no alcanza para {l.qty}.</p>{/if}
            </li>
          {/each}
        </ul>
      {/if}
    </section>

    <p class="flex items-center justify-between rounded-xl bg-app-ink/5 px-4 py-3 text-sm">
      <span class="text-app-muted">Total a cobrar (IVA incluido)</span>
      <span class="display text-xl tabular-nums" aria-live="polite">{moneyCents(total)}</span>
    </p>
  </div>
{:else}
  <div>
    <h3 class="section-title mb-2">Conceptos realizados</h3>
    <div class="flex gap-2">
      <input id="{uid}-free" class="field" aria-label="Concepto realizado" placeholder="Curación, sutura, aplicación…" bind:value={free} onkeydown={(e) => e.key === 'Enter' && (e.preventDefault(), addFree())} />
      <button type="button" class="btn-secondary shrink-0" onclick={addFree}><Icon name="plus" size={16} />Agregar</button>
    </div>
    <p class="hint mt-2">Tu plan guarda la lista de lo realizado, sin precios. Los precios, el inventario y el envío a caja vienen con los planes Crecimiento y Pro.</p>
    {#if lines.length}
      <ul class="mt-3 divide-y divide-app-ink/10 rounded-xl border border-app-ink/10">
        {#each lines as l (l.key)}
          <li class="flex items-center gap-3 px-3 py-2.5">
            <p class="min-w-0 flex-1 truncate text-sm font-medium">{l.name}</p>
            {@render qty(l)}
            <button type="button" class="icon-btn danger" aria-label="Quitar {l.name}" onclick={() => remove(l.key)}><Icon name="trash" size={16} /></button>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
{/if}
