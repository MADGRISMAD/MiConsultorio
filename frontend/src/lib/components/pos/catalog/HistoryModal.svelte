<script lang="ts">
  import { api } from '$lib/api';
  import type { CatalogItem, StockMovement } from '$lib/types';
  import Modal from '$lib/components/Modal.svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import { REASON_LABELS, qty } from './helpers';

  interface Props {
    open: boolean;
    item: CatalogItem | null;
    onclose: () => void;
  }
  let { open, item, onclose }: Props = $props();

  const when = (iso: string) => new Date(iso).toLocaleString('es-MX', { day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' });

  let moves = $state<StockMovement[]>([]);
  let loading = $state(false);
  let error = $state('');

  $effect(() => {
    if (!open || !item) return;
    const id = item.id;
    loading = true;
    error = '';
    moves = [];
    api.pos
      .stockMovements(id)
      .then((m) => (moves = m ?? []))
      .catch((e) => (error = e instanceof Error ? e.message : 'No se pudo cargar el historial.'))
      .finally(() => (loading = false));
  });
</script>

<Modal {open} title="Historial de existencias" {onclose} wide>
  {#if item}<p class="mb-3 text-sm"><strong>{item.name}</strong> <span class="text-app-muted">· existencia actual {qty(item.stock)} {item.unit}</span></p>{/if}
  {#if loading}
    <LoadingRows />
  {:else if error}
    <p class="alert" role="alert"><Icon name="alert" size={18} />{error}</p>
  {:else if !moves.length}
    <EmptyState icon="clock" title="Sin movimientos" text="Aquí verás cada entrada, venta y ajuste de este producto." />
  {:else}
    <div class="overflow-x-auto rounded-2xl border border-app-ink/10">
      <table class="w-full min-w-[34rem]">
        <thead class="border-b border-app-ink/10 bg-app-elevated">
          <tr><th class="th">Fecha</th><th class="th">Motivo</th><th class="th text-right">Cambio</th><th class="th text-right">Saldo</th><th class="th">Quién</th><th class="th">Nota</th></tr>
        </thead>
        <tbody class="divide-y divide-app-ink/8">
          {#each moves as m (m.id)}
            <tr>
              <td class="td whitespace-nowrap text-app-muted">{when(m.created_at)}</td>
              <td class="td">{REASON_LABELS[m.reason] ?? m.reason}</td>
              <td class="td text-right font-medium tabular-nums {m.delta > 0 ? 'text-app-accent' : m.delta < 0 ? 'text-app-danger' : 'text-app-muted'}">{m.delta > 0 ? '+' : ''}{qty(m.delta)}</td>
              <td class="td text-right tabular-nums">{qty(m.balance)}</td>
              <td class="td text-app-muted">{m.by || '—'}</td>
              <td class="td text-app-muted">{m.note || '—'}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</Modal>
