<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { agendaApi } from '$lib/api/agenda';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { Appt, ApptStatus } from '$lib/types/agenda';
  import Guard from '$lib/components/Guard.svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import PageHeader from '$lib/components/ui/PageHeader.svelte';

  let list = $state<Appt[]>([]);
  let loading = $state(true);
  let error = $state('');
  let now = $state(Date.now());
  let busy = $state('');
  const op = new Op();
  let timer: ReturnType<typeof setInterval> | undefined;
  let tick: ReturnType<typeof setInterval> | undefined;

  const ymd = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;

  async function load() {
    const d = new Date();
    const back = new Date(d.getTime() - 2 * 86400000);
    try {
      list = await agendaApi.list({ from: ymd(back), to: ymd(d) });
      error = '';
    } catch (e) {
      error = e instanceof Error ? e.message : 'No se pudo cargar el panel.';
    } finally {
      loading = false;
    }
  }
  onMount(() => {
    load();
    timer = setInterval(load, 30000);
    tick = setInterval(() => (now = Date.now()), 15000);
  });
  onDestroy(() => {
    clearInterval(timer);
    clearInterval(tick);
  });

  const waiting = $derived(list.filter((a) => a.status === 'arrived').sort((a, b) => (a.arrived_at ?? '').localeCompare(b.arrived_at ?? '')));
  const inRoom = $derived(list.filter((a) => a.status === 'in_progress').sort((a, b) => (a.started_at ?? '').localeCompare(b.started_at ?? '')));
  const today = $derived(ymd(new Date(now)));
  const next = $derived(
    list
      .filter((a) => (a.status === 'confirmed' || a.status === 'scheduled') && a.date === today && a.starts_at && Date.parse(a.starts_at) > now - 15 * 60000)
      .sort((a, b) => a.startHour.localeCompare(b.startHour))
      .slice(0, 6)
  );

  function since(iso: string | null): string {
    if (!iso) return '';
    const m = Math.max(0, Math.floor((now - Date.parse(iso)) / 60000));
    if (m < 1) return 'hace un momento';
    if (m < 60) return `${m} min`;
    return `${Math.floor(m / 60)} h ${m % 60} min`;
  }
  const name = (a: Appt) => `${a.names} ${a.last_names}`.trim();
  const canAct = $derived(session.has('navAppointments') || session.has('adminAppointments'));

  async function move(a: Appt, to: ApptStatus) {
    busy = a.id;
    const ok = await op.run(async () => {
      const r = await agendaApi.setStatus(a.id, to);
      if (r.charge) toast.show('Consulta enviada a caja');
    });
    busy = '';
    if (ok) await load();
    else toast.show(op.message, 'error');
  }
</script>

<svelte:head><title>En proceso · Caresia</title></svelte:head>

<Guard title="En proceso" permissions={['navAppointments', 'adminAppointments']}>
  <PageHeader title="En proceso" subtitle="Quién está en el consultorio ahora: en espera y en consulta. Se actualiza solo." />

  {#if error}<p class="alert mb-4" role="alert"><Icon name="alert" size={18} />{error}</p>{/if}

  {#if loading}
    <div class="card"><LoadingRows /></div>
  {:else}
    <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
      {#each [{ title: 'En espera', icon: 'clock', items: waiting, to: 'in_progress' as ApptStatus, action: 'Pasar a consulta', time: 'arrived_at', empty: 'Nadie esperando.' }, { title: 'En consulta', icon: 'stethoscope', items: inRoom, to: 'completed' as ApptStatus, action: 'Terminar consulta', time: 'started_at', empty: 'Nadie en consulta.' }] as col}
        <section class="card p-4 sm:p-5" aria-label={col.title}>
          <h2 class="display flex items-baseline justify-between text-2xl">{col.title}<span class="rounded-full bg-app-ink/8 px-2.5 py-0.5 text-sm font-medium" data-testid="count-{col.to}">{col.items.length}</span></h2>
          {#if col.items.length === 0}
            <p class="mt-3 text-sm text-app-muted">{col.empty}</p>
          {:else}
            <ul class="mt-3 grid gap-2.5">
              {#each col.items as a (a.id)}
                <li class="rounded-xl border border-app-ink/10 p-3">
                  <div class="flex flex-wrap items-start justify-between gap-2">
                    <div class="min-w-0">
                      <p class="truncate font-medium">{name(a)}</p>
                      <p class="text-xs text-app-muted">{a.startHour} · {a.service_name || a.details || 'Cita'}{a.professional_name ? ` · ${a.professional_name}` : ''}{a.room ? ` · ${a.room}` : ''}</p>
                    </div>
                    <span class="whitespace-nowrap rounded-full bg-app-warning/14 px-2.5 py-0.5 text-xs font-medium">{since(a[col.time as 'arrived_at' | 'started_at'])}</span>
                  </div>
                  <div class="mt-2.5 flex flex-wrap gap-2">
                    {#if a.patient_id}<a class="btn-secondary !min-h-9" href="/pacientes/{a.patient_id}">Expediente</a>{/if}
                    {#if canAct}<button type="button" class="btn-primary !min-h-9" disabled={busy === a.id} onclick={() => move(a, col.to)}>{col.action}</button>{/if}
                  </div>
                </li>
              {/each}
            </ul>
          {/if}
        </section>
      {/each}
    </div>

    <section class="card mt-4 p-4 sm:p-5" aria-label="Próximas citas de hoy">
      <h2 class="display text-2xl">Por llegar hoy</h2>
      {#if next.length === 0}
        <p class="mt-3 text-sm text-app-muted">No hay más citas pendientes hoy.</p>
      {:else}
        <ul class="mt-3 divide-y divide-app-ink/8">
          {#each next as a (a.id)}
            <li class="flex items-center justify-between gap-3 py-2 text-sm"><span class="min-w-0 truncate"><strong class="font-mono">{a.startHour}</strong> · {name(a)}</span>
              {#if canAct}<button type="button" class="btn-ghost !min-h-8 whitespace-nowrap" disabled={busy === a.id} onclick={() => move(a, 'arrived')}>Ya llegó</button>{/if}</li>
          {/each}
        </ul>
      {/if}
    </section>

    {#if waiting.length === 0 && inRoom.length === 0 && next.length === 0}
      <div class="card mt-4"><EmptyState icon="check" title="Consultorio tranquilo" text="Cuando alguien llegue a su cita aparecerá aquí." /></div>
    {/if}
  {/if}
</Guard>
