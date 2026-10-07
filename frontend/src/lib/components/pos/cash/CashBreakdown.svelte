<script lang="ts">
  import { moneyCents } from '$lib/format';
  import { PAY_METHODS, type CashSession } from '$lib/types';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';

  let { session }: { session: CashSession } = $props();

  const time = (iso: string) => new Date(iso).toLocaleTimeString('es-MX', { hour: '2-digit', minute: '2-digit' });
  const closed = $derived(session.closed_at != null);
  const diff = $derived(session.diff_cents ?? 0);
</script>

<div class="space-y-6">
  <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
    <div class="rounded-2xl bg-app-elevated p-4">
      <p class="section-title">Fondo inicial</p>
      <p class="display mt-1 text-2xl tabular-nums sm:text-3xl">{moneyCents(session.opening_cents)}</p>
    </div>
    <div class="rounded-2xl bg-app-elevated p-4">
      <p class="section-title">Ventas del turno</p>
      <p class="display mt-1 text-2xl tabular-nums sm:text-3xl">{moneyCents(session.sales_cents)}</p>
      <p class="text-xs text-app-muted">{session.sales} {session.sales === 1 ? 'venta' : 'ventas'}</p>
    </div>
    <div class="rounded-2xl bg-app-primary/10 p-4">
      <p class="section-title text-app-primary">Efectivo esperado</p>
      <p class="display mt-1 text-2xl tabular-nums sm:text-3xl">{moneyCents(session.expected_cents)}</p>
      <p class="text-xs text-app-muted">Fondo + efectivo + entradas − salidas</p>
    </div>
    <div class="rounded-2xl bg-app-elevated p-4">
      <p class="section-title">Entradas / salidas</p>
      <p class="mt-1 text-lg font-medium tabular-nums text-app-accent">+ {moneyCents(session.moves_in_cents)}</p>
      <p class="text-lg font-medium tabular-nums text-app-danger">− {moneyCents(session.moves_out_cents)}</p>
    </div>
  </div>

  {#if closed}
    <div class="grid grid-cols-2 gap-3">
      <div class="rounded-2xl bg-app-elevated p-4">
        <p class="section-title">Efectivo contado</p>
        <p class="display mt-1 text-2xl tabular-nums sm:text-3xl">{moneyCents(session.counted_cents ?? 0)}</p>
      </div>
      <div class="rounded-2xl p-4 {diff === 0 ? 'bg-app-accent/12 text-app-accent' : diff > 0 ? 'bg-app-warning/12 text-app-warning' : 'bg-app-danger/10 text-app-danger'}">
        <p class="section-title text-current">{diff === 0 ? 'Caja cuadrada' : diff > 0 ? 'Sobrante' : 'Faltante'}</p>
        <p class="display mt-1 text-2xl tabular-nums sm:text-3xl">{moneyCents(Math.abs(diff))}</p>
      </div>
    </div>
    {#if session.note}<p class="rounded-xl bg-app-elevated px-4 py-3 text-sm"><span class="text-app-muted">Nota:</span> {session.note}</p>{/if}
  {/if}

  <div class="grid gap-6 md:grid-cols-2">
    <section aria-label="Ventas por método de pago">
      <h3 class="section-title mb-2">Por método de pago</h3>
      {#if session.by_method.length}
        <ul class="divide-y divide-app-ink/10 rounded-2xl ring-1 ring-inset ring-app-ink/10">
          {#each session.by_method as m (m.method)}
            <li class="flex items-center justify-between gap-3 px-4 py-3 text-sm">
              <span>{PAY_METHODS[m.method]?.label ?? m.method}<span class="ml-1.5 text-xs text-app-muted">{m.count} {m.count === 1 ? 'cobro' : 'cobros'}</span></span>
              <span class="font-medium tabular-nums">{moneyCents(m.amount_cents)}</span>
            </li>
          {/each}
        </ul>
      {:else}
        <p class="rounded-2xl bg-app-elevated px-4 py-6 text-center text-sm text-app-muted">Todavía no hay ventas en este turno.</p>
      {/if}
    </section>

    <section aria-label="Movimientos de efectivo">
      <h3 class="section-title mb-2">Movimientos de efectivo</h3>
      {#if session.movements?.length}
        <ul class="divide-y divide-app-ink/10 rounded-2xl ring-1 ring-inset ring-app-ink/10">
          {#each session.movements as m (m.id)}
            <li class="flex items-center justify-between gap-3 px-4 py-3 text-sm">
              <div class="min-w-0">
                <p class="truncate font-medium">{m.concept}</p>
                <p class="text-xs text-app-muted">{time(m.created_at)}{m.by ? ` · ${m.by}` : ''}</p>
              </div>
              <span class="font-medium tabular-nums {m.kind === 'in' ? 'text-app-accent' : 'text-app-danger'}">{m.kind === 'in' ? '+' : '−'} {moneyCents(m.amount_cents)}</span>
            </li>
          {/each}
        </ul>
      {:else}
        <p class="rounded-2xl bg-app-elevated px-4 py-6 text-center text-sm text-app-muted">Sin entradas ni salidas de efectivo.</p>
      {/if}
    </section>
  </div>
</div>
