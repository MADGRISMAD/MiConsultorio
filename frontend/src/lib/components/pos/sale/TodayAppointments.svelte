<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { agendaApi } from '$lib/api/agenda';
  import { consult } from '$lib/api/consult';
  import { ymd } from '$lib/dates';
  import { STATUS_META, type Appt } from '$lib/types/agenda';
  import Icon from '$lib/components/ui/Icon.svelte';
  import Pill from '$lib/components/ui/Pill.svelte';

  /**
   * Today's visits that can be charged, so the patient is linked from the agenda (name and contact only):
   * whoever charges never needs to browse expedients. Visits that already have a sale, a package session or a
   * pre-account waiting above are left out.
   */
  interface Props {
    activeId?: string;
    /** bump to reload right away (after a sale) */
    refreshKey?: number;
    onopen: (a: Appt) => void;
  }
  let { activeId = '', refreshKey = 0, onopen }: Props = $props();

  const EVERY_MS = 30_000;
  let list = $state<Appt[]>([]);
  let showAll = $state(false);
  let timer: ReturnType<typeof setInterval> | undefined;

  async function load() {
    try {
      const day = ymd(new Date());
      const [appts, pre] = await Promise.all([agendaApi.list({ from: day, to: day }), consult.list({ status: 'sent' }).catch(() => [])]);
      const inPreAccount = new Set(pre.map((c) => c.appointment_id).filter(Boolean));
      list = appts
        .filter((a) => ['arrived', 'in_progress', 'completed'].includes(a.status) && !a.sale_id && !a.package_id && !inPreAccount.has(a.id))
        .sort((a, b) => a.startHour.localeCompare(b.startHour));
    } catch {
      /* a convenience: without agenda access the panel simply stays hidden */
    }
  }

  onMount(() => {
    void load();
    timer = setInterval(() => document.visibilityState === 'visible' && void load(), EVERY_MS);
  });
  onDestroy(() => clearInterval(timer));
  $effect(() => {
    void refreshKey;
    void load();
  });

  const shown = $derived(showAll ? list : list.slice(0, 3));
</script>

{#if list.length}
  <section class="card mb-5 p-4 sm:p-5" aria-labelledby="citas-cobrar-title" data-testid="today-appointments-panel">
    <h2 id="citas-cobrar-title" class="section-title flex items-center gap-2">
      <Icon name="calendar" size={14} />Citas de hoy por cobrar <span class="pill pill-info">{list.length}</span>
    </h2>
    <ul class="mt-3 grid gap-2 sm:grid-cols-2 xl:grid-cols-3" aria-live="polite">
      {#each shown as a (a.id)}
        <li class="flex items-center justify-between gap-3 rounded-xl border border-app-ink/10 px-3 py-2.5 {activeId === a.id ? 'bg-app-primary/10' : ''}">
          <div class="min-w-0">
            <p class="truncate text-sm font-medium">{a.names} {a.last_names}</p>
            <p class="flex items-center gap-2 truncate text-xs text-app-muted">
              <span class="tabular-nums">{a.startHour}</span>·<span class="truncate">{a.service_name || a.details || 'Consulta'}</span>
              <Pill tone={STATUS_META[a.status].tone}>{STATUS_META[a.status].label}</Pill>
            </p>
          </div>
          <button type="button" class="btn-primary min-h-9 shrink-0 px-3.5 text-[13px]" disabled={activeId === a.id} onclick={() => onopen(a)} aria-label="Cobrar la cita de {a.names} {a.last_names}">Cobrar</button>
        </li>
      {/each}
    </ul>
    {#if list.length > 3}
      <button type="button" class="btn-ghost mt-2 min-h-11 px-3 text-sm" onclick={() => (showAll = !showAll)}>{showAll ? 'Ver menos' : `Ver todas (${list.length})`}</button>
    {/if}
  </section>
{/if}
