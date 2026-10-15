<script lang="ts">
  import { Loader } from '$lib/loader.svelte';
  import { onDestroy, onMount } from 'svelte';
  import { consult } from '$lib/api/consult';
  import { ago, moneyCents } from '$lib/format';
  import type { Charge } from '$lib/types/consult';
  import Icon from '$lib/components/ui/Icon.svelte';

  interface Props {
    /** id of the pre-account being charged right now, if any */
    activeId?: string;
    /** bump to reload right away (after a sale) */
    refreshKey?: number;
    onopen: (c: Charge) => void;
  }
  let { activeId = '', refreshKey = 0, onopen }: Props = $props();

  const EVERY_MS = 20_000;
  let charges = $state<Charge[]>([]);
  const ld = new Loader('No se pudieron cargar las pre-cuentas.');
  let updatedAt = $state<Date | null>(null);
  let showAll = $state(false);
  let timer: ReturnType<typeof setInterval> | undefined;

  async function load() {
    await ld.run(async (current) => {
      const r = await consult.list({ status: 'sent' });
      if (!current()) return;
      charges = r;
      updatedAt = new Date();
    });
  }

  const visible = () => typeof document === 'undefined' || document.visibilityState === 'visible';
  function start() {
    stop();
    timer = setInterval(() => visible() && void load(), EVERY_MS);
  }
  function stop() {
    if (timer) clearInterval(timer);
    timer = undefined;
  }
  function onVisibility() {
    if (visible()) void load();
  }

  onMount(() => {
    void load();
    start();
    document.addEventListener('visibilitychange', onVisibility);
  });
  onDestroy(() => {
    stop();
    if (typeof document !== 'undefined') document.removeEventListener('visibilitychange', onVisibility);
  });
  $effect(() => {
    void refreshKey;
    if (!ld.loading) void load();
  });

  const hhmm = (d: Date) => d.toLocaleTimeString('es-MX', { hour: '2-digit', minute: '2-digit' });
</script>

{#if !ld.loading || charges.length}
  <section class="card mb-5 {charges.length ? 'p-4 sm:p-5' : 'px-4 py-2.5'}" aria-labelledby="precuentas-title" data-testid="consult-charges-panel">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <h2 id="precuentas-title" class="section-title flex items-center gap-2">
        <Icon name="receipt" size={14} />{charges.length ? 'Por cobrar de consulta' : 'Sin pre-cuentas pendientes'}
        {#if charges.length}<span class="pill pill-info">{charges.length}</span>{/if}
      </h2>
      <p class="flex items-center gap-2 text-xs text-app-muted">
        {#if updatedAt && charges.length}Actualizado {hhmm(updatedAt)}{/if}
        <button type="button" class="btn-ghost min-h-9 px-2 text-xs" onclick={load} aria-label="Actualizar pre-cuentas"><Icon name="refresh" size={14} />Actualizar</button>
      </p>
    </div>
    {#if ld.error}
      <p class="mt-2 text-sm text-app-warning" role="alert">No pudimos actualizar las pre-cuentas{charges.length ? '; lo que ves puede estar desactualizado' : ''}. Usa «Actualizar» para reintentar.</p>
    {/if}
    <div aria-live="polite">
      {#if charges.length}
        <ul class="mt-3 grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
          {#each charges.slice(0, showAll ? charges.length : 3) as c (c.id)}
            <li class="flex items-center justify-between gap-3 rounded-xl border border-app-ink/10 px-3 py-2.5 {activeId === c.id ? 'bg-app-primary/10' : ''}">
              <div class="min-w-0">
                <p class="truncate text-sm font-medium">{c.patient_name}</p>
                <p class="truncate text-xs text-app-muted">{c.professional_name || c.created_by} · {c.item_count} {c.item_count === 1 ? 'concepto' : 'conceptos'} · {ago(c.sent_at)}</p>
              </div>
              <div class="flex shrink-0 items-center gap-3">
                <span class="text-sm font-semibold tabular-nums">{moneyCents(c.total_cents)}</span>
                <button type="button" class="btn-primary min-h-9 px-3.5 text-[13px]" disabled={activeId === c.id} onclick={() => onopen(c)} aria-label="Cobrar la pre-cuenta de {c.patient_name}">Cobrar</button>
              </div>
            </li>
          {/each}
        </ul>
        {#if charges.length > 3}
          <button type="button" class="btn-ghost mt-2 min-h-11 px-3 text-sm" onclick={() => (showAll = !showAll)}>{showAll ? 'Ver menos' : `Ver todas (${charges.length})`}</button>
        {/if}
      {/if}
    </div>
  </section>
{/if}
