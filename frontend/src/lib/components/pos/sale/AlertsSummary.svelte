<script lang="ts">
  import { onMount } from 'svelte';
  import { pos2 } from '$lib/api/pos2';
  import Icon from '$lib/components/ui/Icon.svelte';
  import type { PosAlerts } from '$lib/types/pos2';

  let alerts = $state<PosAlerts | null>(null);

  onMount(() => {
    pos2
      .alerts(30)
      .then((a) => (alerts = a))
      .catch(() => {
        /* the summary simply does not show */
      });
  });

  const expired = $derived(alerts?.expired.length ?? 0);
  const soon = $derived(alerts?.expiring.length ?? 0);
  const low = $derived(alerts?.low.length ?? 0);
</script>

{#if alerts && expired + soon + low > 0}
  <a
    href="/pos/inventario"
    class="card mb-5 flex flex-wrap items-center gap-x-5 gap-y-2 px-4 py-3 text-sm transition hover:bg-app-elevated/60 {expired ? 'border-app-danger/40' : 'border-app-warning/40'}"
    aria-label="Ver alertas de inventario"
    data-testid="stock-alerts"
  >
    <span class="flex items-center gap-2 font-medium {expired ? 'text-app-danger' : 'text-app-warning'}"><Icon name="alert" size={18} />Inventario</span>
    {#if expired}<span class="pill pill-bad">{expired} {expired === 1 ? 'lote caducado' : 'lotes caducados'}</span>{/if}
    {#if soon}<span class="pill pill-warn">{soon} {soon === 1 ? 'lote caduca' : 'lotes caducan'} en 30 días</span>{/if}
    {#if low}<span class="pill pill-warn">{low} {low === 1 ? 'producto' : 'productos'} bajo mínimo</span>{/if}
    <span class="ml-auto flex items-center gap-1 text-app-muted">Ver detalle<Icon name="arrow-right" size={14} /></span>
  </a>
{/if}
