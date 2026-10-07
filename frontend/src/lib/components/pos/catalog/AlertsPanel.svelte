<script lang="ts">
  import { onMount } from 'svelte';
  import { pos2 } from '$lib/api/pos2';
  import { dayLabel } from './expiry';
  import Icon from '$lib/components/ui/Icon.svelte';
  import type { LotAlert, PosAlerts } from '$lib/types/pos2';
  import { qty } from './helpers';

  interface Props {
    /** Bumped by the parent after a stock change to reload. */
    refresh?: number;
    onitem?: (itemId: string) => void;
  }
  let { refresh = 0, onitem }: Props = $props();

  const uid = $props.id();
  let days = $state(60);
  let alerts = $state<PosAlerts | null>(null);
  let error = $state('');
  let seq = 0;

  async function load() {
    const my = ++seq;
    try {
      const a = await pos2.alerts(days);
      if (my === seq) (alerts = a), (error = '');
    } catch (e) {
      if (my === seq) error = e instanceof Error ? e.message : 'No se pudieron cargar las alertas.';
    }
  }
  onMount(load);
  $effect(() => {
    void refresh;
    void days;
    void load();
  });

  const total = $derived((alerts?.expired.length ?? 0) + (alerts?.expiring.length ?? 0) + (alerts?.low.length ?? 0));
  const dayText = (d: number) => (d < 0 ? `caducó hace ${-d} ${d === -1 ? 'día' : 'días'}` : d === 0 ? 'caduca hoy' : `caduca en ${d} ${d === 1 ? 'día' : 'días'}`);
</script>

{#snippet lots(list: LotAlert[], tone: string)}
  <ul class="divide-y divide-app-ink/10">
    {#each list as l (l.lot_id)}
      <li class="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 px-4 py-2.5 text-sm">
        <button type="button" class="min-h-9 min-w-0 flex-1 truncate text-left font-medium hover:underline" onclick={() => onitem?.(l.item_id)}>{l.name}</button>
        <span class="text-xs text-app-muted">Lote {l.lot_code} · {qty(l.qty)} {l.unit}</span>
        <span class="pill {tone}">{dayText(l.days_left)} · {dayLabel(l.expires_on)}</span>
      </li>
    {/each}
  </ul>
{/snippet}

<section class="card mb-5 overflow-hidden" aria-labelledby="{uid}-t">
  <div class="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
    <h2 id="{uid}-t" class="flex items-center gap-2 font-medium">
      <Icon name="alert" size={18} class={total ? 'text-app-warning' : 'text-app-accent'} />Alertas de inventario
      {#if alerts}<span class="pill {total ? 'pill-warn' : 'pill-ok'}">{total || 'Todo en orden'}</span>{/if}
    </h2>
    <div class="flex items-center gap-2 text-sm">
      <label for="{uid}-d" class="text-app-muted">Caducan en</label>
      <select id="{uid}-d" class="field !min-h-9 !w-auto !py-1" bind:value={days}>
        {#each [30, 60, 90, 180] as d}<option value={d}>{d} días</option>{/each}
      </select>
    </div>
  </div>
  {#if error}
    <p class="alert m-4" role="alert"><Icon name="alert" size={18} />{error}</p>
  {:else if alerts && total > 0}
    <div class="border-t border-app-ink/10">
      {#if alerts.expired.length}
        <h3 class="section-title bg-app-danger/10 px-4 py-1.5 text-app-danger">Ya caducados · no se pueden vender</h3>
        {@render lots(alerts.expired, 'pill-bad')}
      {/if}
      {#if alerts.expiring.length}
        <h3 class="section-title bg-app-warning/10 px-4 py-1.5 text-app-warning">Caducan pronto</h3>
        {@render lots(alerts.expiring, 'pill-warn')}
      {/if}
      {#if alerts.low.length}
        <h3 class="section-title bg-app-ink/5 px-4 py-1.5">Bajo mínimo</h3>
        <ul class="divide-y divide-app-ink/10">
          {#each alerts.low as l (l.item_id)}
            <li class="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 px-4 py-2.5 text-sm">
              <button type="button" class="min-h-9 min-w-0 flex-1 truncate text-left font-medium hover:underline" onclick={() => onitem?.(l.item_id)}>{l.name}</button>
              <span class="pill pill-warn">Quedan {qty(l.stock)} {l.unit} · mínimo {qty(l.min_stock)}</span>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  {/if}
</section>
