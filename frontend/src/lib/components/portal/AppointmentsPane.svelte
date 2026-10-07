<script lang="ts">
  import { onMount } from 'svelte';
  import { portalApi } from '$lib/api/portal';
  import { Op } from '$lib/op.svelte';
  import type { PortalAppointment } from '$lib/types/portal';
  import ConfirmModal from '$lib/components/ConfirmModal.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';

  let { slug, patientId, multi }: { slug: string; patientId: string; multi: boolean } = $props();

  let upcoming = $state<PortalAppointment[]>([]);
  let history = $state<PortalAppointment[]>([]);
  let minHours = $state(0);
  let loaded = $state(false);
  const loadOp = new Op();
  const cancelOp = new Op();
  let target = $state<PortalAppointment | null>(null);
  let reason = $state('');
  let notice = $state('');

  async function load() {
    await loadOp.run(async () => {
      const r = await portalApi.appointments();
      upcoming = r.upcoming;
      history = r.history;
      minHours = r.cancel_min_hours;
      loaded = true;
    });
  }
  onMount(load);

  const mine = (list: PortalAppointment[]) => list.filter((a) => !patientId || a.patient_id === patientId);
  const up = $derived(mine(upcoming));
  const past = $derived(mine(history));

  const STATUS: Record<string, [string, 'info' | 'ok' | 'warn' | 'bad' | 'muted']> = {
    scheduled: ['Programada', 'info'],
    confirmed: ['Confirmada', 'ok'],
    arrived: ['En consultorio', 'ok'],
    in_progress: ['En consulta', 'ok'],
    completed: ['Atendida', 'muted'],
    no_show: ['No asististe', 'warn'],
    cancelled: ['Cancelada', 'bad']
  };
  const tone = (s: string) => `pill pill-${STATUS[s]?.[1] ?? 'info'}`.replace('pill-muted', '');

  const dateLong = (d: string) => {
    const [y, m, day] = d.split('-').map(Number);
    return new Date(y, m - 1, day).toLocaleDateString('es-MX', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' });
  };

  async function confirmCancel() {
    if (!target) return;
    const id = target.id;
    if (
      await cancelOp.run(async () => {
        await portalApi.cancel(id, reason.trim());
        await load();
      })
    ) {
      notice = 'Tu cita fue cancelada.';
      target = null;
      reason = '';
    }
  }
</script>

{#snippet card(a: PortalAppointment, withCancel: boolean)}
  <li class="card flex flex-wrap items-start justify-between gap-3 px-4 py-4 sm:px-5">
    <div class="min-w-0">
      <p class="font-medium first-letter:uppercase">{dateLong(a.date)}</p>
      <p class="mt-0.5 flex items-center gap-1.5 text-sm text-app-muted"><Icon name="clock" size={15} />{a.start_hour} a {a.end_hour} h</p>
      {#if multi && a.patient_name}<p class="mt-1 text-sm">Paciente: <strong>{a.patient_name}</strong></p>{/if}
      {#if a.service}<p class="text-sm text-app-muted">{a.service}</p>{/if}
      {#if a.professional}<p class="text-sm text-app-muted">Con {a.professional}</p>{/if}
    </div>
    <div class="flex flex-col items-end gap-2">
      <span class={tone(a.status)}>{STATUS[a.status]?.[0] ?? a.status}</span>
      {#if withCancel}
        {#if a.can_cancel}
          <button class="btn-secondary !min-h-9" onclick={() => { target = a; reason = ''; cancelOp.reset(); }}>Cancelar cita</button>
        {:else}
          <span class="max-w-[14rem] text-right text-xs text-app-muted">Para cancelar con menos de {minHours} h de anticipación, comunícate con el consultorio.</span>
        {/if}
      {/if}
    </div>
  </li>
{/snippet}

{#if !loaded && loadOp.phase !== 'error'}
  <LoadingRows />
{:else if loadOp.phase === 'error' && !loaded}
  <p class="alert" role="alert"><Icon name="alert" size={18} />{loadOp.message}</p>
{:else}
  <div class="grid gap-8">
    {#if notice}<p class="rounded-xl bg-app-accent/12 px-3.5 py-3 text-sm font-medium text-app-accent" role="status">{notice}</p>{/if}
    <section aria-labelledby="ap-next">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-3">
        <h2 id="ap-next" class="section-title">Próximas citas</h2>
        <a class="btn-primary !min-h-9" href="/reservar/{encodeURIComponent(slug)}"><Icon name="plus" size={16} />Agendar nueva cita</a>
      </div>
      {#if up.length === 0}
        <div class="card px-5 py-8 text-center text-sm text-app-muted">No tienes citas próximas.</div>
      {:else}
        <ul class="grid gap-3">{#each up as a (a.id)}{@render card(a, true)}{/each}</ul>
      {/if}
    </section>

    <section aria-labelledby="ap-hist">
      <h2 id="ap-hist" class="section-title mb-3">Historial</h2>
      {#if past.length === 0}
        <div class="card px-5 py-8 text-center text-sm text-app-muted">Aún no hay citas pasadas.</div>
      {:else}
        <ul class="grid gap-3">{#each past as a (a.id)}{@render card(a, false)}{/each}</ul>
      {/if}
    </section>
  </div>
{/if}

<ConfirmModal open={target !== null} title="Cancelar cita" op={cancelOp} onconfirm={confirmCancel} onclose={() => (target = null)} confirmLabel="Sí, cancelar mi cita">
  {#if target}
    <p>Vas a cancelar la cita del <strong>{dateLong(target.date)}</strong> a las {target.start_hour} h.</p>
    <label class="label mt-4" for="ap-reason">Motivo (opcional)</label>
    <input id="ap-reason" class="field" maxlength="300" bind:value={reason} />
  {/if}
</ConfirmModal>
