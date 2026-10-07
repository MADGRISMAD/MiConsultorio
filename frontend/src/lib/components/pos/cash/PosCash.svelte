<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { moneyCents } from '$lib/format';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import { printer, openDrawer } from '$lib/printer/connection.svelte';
  import { PERMISSIONS, type CashSession, type PosSettings } from '$lib/types';
  import Icon from '$lib/components/ui/Icon.svelte';
  import PageHeader from '$lib/components/ui/PageHeader.svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import MoneyInput from '../sale/MoneyInput.svelte';
  import CashBreakdown from './CashBreakdown.svelte';
  import CloseModal from './CloseModal.svelte';
  import MoveModal from './MoveModal.svelte';
  import SessionDetailModal from './SessionDetailModal.svelte';
  import { Op } from '$lib/op.svelte';

  let settings = $state<PosSettings | null>(null);
  let current = $state<CashSession | null>(null);
  let history = $state<CashSession[]>([]);
  let loading = $state(true);
  let loadError = $state('');

  let opening = $state<number | null>(null);
  const openOp = new Op();

  let moveKind = $state<'in' | 'out' | null>(null);
  let closeOpen = $state(false);
  let detailId = $state<string | null>(null);
  let justClosed = false;

  const canHistory = $derived(session.has(PERMISSIONS.posReports));
  const canDrawer = $derived(!!settings?.printer.open_drawer && printer.connected);
  const businessName = $derived(settings?.business_name ?? '');
  const paperWidth = $derived(settings?.printer.width ?? 80);

  async function loadCurrent() {
    current = await api.pos.cash();
  }
  async function loadHistory() {
    if (!canHistory) return;
    try {
      history = (await api.pos.cashSessions()).filter((s) => s.closed_at != null);
    } catch {
      /* history is secondary */
    }
  }
  async function load() {
    loading = true;
    loadError = '';
    try {
      const [s] = await Promise.all([api.pos.settings(), loadCurrent()]);
      settings = s.settings;
      await loadHistory();
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'No se pudo cargar la caja.';
    } finally {
      loading = false;
    }
  }
  onMount(load);

  async function open(e: SubmitEvent) {
    e.preventDefault();
    let s: CashSession | undefined;
    if (await openOp.run(async () => (s = await api.pos.openCash(opening ?? 0)))) {
      current = s!;
      opening = null;
      toast.show('Caja abierta');
    }
  }

  async function moved() {
    moveKind = null;
    try {
      await loadCurrent();
    } catch {
      /* stale numbers until the next refresh */
    }
  }

  async function drawer() {
    try {
      await openDrawer();
    } catch (e) {
      toast.show(`No se pudo abrir el cajón${e instanceof Error && e.message ? ': ' + e.message : ''}`, 'error');
    }
  }

  const stamp = (iso: string) => new Date(iso).toLocaleString('es-MX', { day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' });
</script>

<PageHeader title="Caja" subtitle="Abre el turno, registra entradas y salidas de efectivo y haz el corte al cerrar.">
  {#snippet actions()}
    {#if canDrawer}<button type="button" class="btn-secondary min-h-11" onclick={drawer}><Icon name="cash" size={16} />Abrir cajón</button>{/if}
    <a href="/pos/cobros" class="btn-secondary min-h-11"><Icon name="receipt" size={16} />Punto de venta</a>
  {/snippet}
</PageHeader>

{#if loadError}
  <div class="card p-6">
    <p class="alert" role="alert"><Icon name="alert" size={18} />{loadError}</p>
    <button type="button" class="btn-primary mt-4" onclick={load}><Icon name="refresh" size={16} />Reintentar</button>
  </div>
{:else if loading}
  <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4" aria-busy="true">
    {#each Array(4) as _, n (n)}<div class="h-28 animate-pulse rounded-2xl bg-app-ink/5"></div>{/each}
  </div>
{:else}
  {#if current}
    <section class="card p-5 sm:p-6" aria-label="Turno actual">
      <div class="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div>
          <span class="pill pill-ok">Caja abierta</span>
          <p class="mt-2 text-sm text-app-muted">Desde {stamp(current.opened_at)}{current.opened_by ? ` · ${current.opened_by}` : ''}</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button type="button" class="btn-secondary min-h-11" onclick={() => (moveKind = 'in')}><Icon name="plus" size={16} />Entrada de efectivo</button>
          <button type="button" class="btn-secondary min-h-11" onclick={() => (moveKind = 'out')}><span aria-hidden="true">−</span>Salida de efectivo</button>
          <button type="button" class="btn-primary min-h-11" onclick={() => (closeOpen = true)}><Icon name="lock" size={16} />Cerrar caja</button>
        </div>
      </div>
      <CashBreakdown session={current} />
    </section>
  {:else}
    <section class="card mx-auto max-w-lg p-6 sm:p-8" aria-label="Abrir caja">
      <span class="grid h-12 w-12 place-items-center rounded-2xl bg-app-primary/10 text-app-primary"><Icon name="cash" size={24} /></span>
      <h2 class="display mt-4 text-3xl">Abrir caja</h2>
      <p class="mt-1 text-sm text-app-muted">Cuenta el efectivo con el que empieza el turno (fondo para dar cambio). Si no hay fondo, déjalo vacío.</p>
      <form onsubmit={open} class="mt-5 space-y-4">
        <MoneyInput id="opening" label="Fondo inicial" bind:cents={opening} autofocus />
        {#if openOp.phase === 'error'}<p class="alert" role="alert"><Icon name="alert" size={18} />{openOp.message}</p>{/if}
        <button type="submit" class="btn-primary btn-lg min-h-12" disabled={openOp.phase === 'loading'}>
          {#if openOp.phase === 'loading'}<span class="spin"></span>{/if}Abrir caja
        </button>
      </form>
    </section>
  {/if}

  {#if canHistory}
    <section class="mt-8" aria-label="Historial de cortes">
      <h2 class="display mb-3 text-3xl">Historial de cortes</h2>
      <div class="card overflow-hidden">
        {#if history.length === 0}
          <EmptyState icon="receipt" title="Aún no hay cortes" text="Cuando cierres una caja, el corte aparecerá aquí." />
        {:else}
          <div class="overflow-x-auto">
            <table class="w-full min-w-[44rem]">
              <caption class="sr-only">Turnos de caja cerrados</caption>
              <thead class="border-b border-app-ink/10">
                <tr>
                  <th class="th" scope="col">Fecha</th>
                  <th class="th" scope="col">Abrió</th>
                  <th class="th" scope="col">Cerró</th>
                  <th class="th text-right" scope="col">Esperado</th>
                  <th class="th text-right" scope="col">Contado</th>
                  <th class="th text-right" scope="col">Diferencia</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-app-ink/10">
                {#each history as h (h.id)}
                  {@const d = h.diff_cents ?? 0}
                  <tr class="cursor-pointer hover:bg-app-elevated" onclick={() => (detailId = h.id)}>
                    <td class="td">
                      <button
                        type="button"
                        class="min-h-11 text-left font-medium underline-offset-2 hover:underline"
                        onclick={(e) => {
                          e.stopPropagation();
                          detailId = h.id;
                        }}>{stamp(h.opened_at)}</button
                      >
                    </td>
                    <td class="td text-app-muted">{h.opened_by || '—'}</td>
                    <td class="td text-app-muted">{h.closed_by || '—'}</td>
                    <td class="td text-right tabular-nums">{moneyCents(h.expected_cents)}</td>
                    <td class="td text-right tabular-nums">{h.counted_cents != null ? moneyCents(h.counted_cents) : '—'}</td>
                    <td class="td text-right">
                      <span class="pill {d === 0 ? 'pill-ok' : d > 0 ? 'pill-warn' : 'pill-bad'} tabular-nums">
                        {d === 0 ? 'Cuadra' : (d > 0 ? 'Sobra ' : 'Falta ') + moneyCents(Math.abs(d))}
                      </span>
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </div>
    </section>
  {/if}
{/if}

<MoveModal open={moveKind != null} kind={moveKind ?? 'in'} onclose={() => (moveKind = null)} ondone={moved} />
{#if current}
  <CloseModal
    open={closeOpen}
    session={current}
    {businessName}
    {paperWidth}
    onclose={() => {
      closeOpen = false;
      if (justClosed) {
        justClosed = false;
        current = null;
      }
    }}
    onclosed={() => {
      // The register is closed on the server; the corte stays on screen until dismissed
      justClosed = true;
      loadHistory();
    }}
  />
{/if}
<SessionDetailModal id={detailId} {businessName} {paperWidth} onclose={() => (detailId = null)} />
