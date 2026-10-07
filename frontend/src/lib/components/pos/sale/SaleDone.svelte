<script lang="ts">
  import Icon from '$lib/components/ui/Icon.svelte';
  import { moneyCents } from '$lib/format';
  import type { Sale } from '$lib/types';
  import EmailTicket from './EmailTicket.svelte';

  interface Props {
    sale: Sale;
    change: number;
    printing: boolean;
    onprint: () => void;
    onnew: () => void;
  }
  let { sale, change, printing, onprint, onnew }: Props = $props();
  const open = $derived(sale.status === 'open' && (sale.balance_cents ?? 0) > 0);
  let newBtn = $state<HTMLButtonElement>();
  $effect(() => newBtn?.focus());
</script>

<div class="card mx-auto max-w-lg p-6 text-center sm:p-8" role="status" aria-live="polite">
  <span class="mx-auto grid h-16 w-16 place-items-center rounded-full bg-app-accent/15 text-app-accent"><Icon name="check" size={32} stroke={2.2} /></span>
  <h2 class="display mt-4 text-4xl">{open ? 'Venta a abonos registrada' : 'Venta registrada'}</h2>
  <p class="mt-1 text-app-muted">Folio <strong class="text-app-ink tabular-nums">#{sale.folio}</strong>{sale.customer_name ? ` · ${sale.customer_name}` : ''}</p>

  <dl class="mt-6 grid gap-3 text-left">
    <div class="flex items-baseline justify-between rounded-2xl bg-app-elevated px-5 py-4">
      <dt class="text-sm text-app-muted">{open ? 'Total de la cuenta' : 'Total cobrado'}</dt>
      <dd class="display text-3xl tabular-nums">{moneyCents(sale.total_cents)}</dd>
    </div>
    {#if open}
      <div class="flex items-baseline justify-between rounded-2xl bg-app-elevated px-5 py-3 text-sm">
        <dt class="text-app-muted">Abonado</dt>
        <dd class="font-medium tabular-nums">{moneyCents(sale.paid_cents ?? 0)}</dd>
      </div>
      <div class="flex items-baseline justify-between rounded-2xl bg-app-warning/12 px-5 py-4 text-app-warning" data-testid="sale-balance">
        <dt class="font-medium">Saldo pendiente</dt>
        <dd class="display text-4xl tabular-nums">{moneyCents(sale.balance_cents ?? 0)}</dd>
      </div>
    {/if}
    {#if change > 0}
      <div class="flex items-baseline justify-between rounded-2xl bg-app-accent/12 px-5 py-5 text-app-accent">
        <dt class="font-medium">Cambio a entregar</dt>
        <dd class="display text-5xl tabular-nums">{moneyCents(change)}</dd>
      </div>
    {/if}
  </dl>

  <div class="mt-6 grid gap-2 sm:grid-cols-2">
    <button type="button" class="btn-secondary min-h-12" disabled={printing} onclick={onprint}>
      {#if printing}<span class="spin"></span>{/if}<Icon name="receipt" size={18} />Imprimir ticket
    </button>
    <button bind:this={newBtn} type="button" class="btn-primary min-h-12" onclick={onnew}><Icon name="plus" size={18} />Nueva venta</button>
    <div class="sm:col-span-2"><EmailTicket saleId={sale.id} /></div>
  </div>
  {#if sale.warnings?.length}
    <ul class="mt-4 space-y-1 rounded-xl bg-app-warning/10 px-4 py-3 text-left text-xs text-app-warning" role="status">
      {#each sale.warnings as w}<li>{w}</li>{/each}
    </ul>
  {/if}
  {#if open}
    <a href="/pos/cuentas" class="btn-ghost mt-2 min-h-11">Ver cuentas por cobrar</a>
  {:else}
    <a href="/pos/facturacion?venta={sale.id}" class="btn-ghost mt-2 min-h-11">Solicitar factura</a>
  {/if}
</div>
