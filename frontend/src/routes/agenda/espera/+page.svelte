<script lang="ts">
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onMount } from 'svelte';
  import { agendaApi } from '$lib/api/agenda';
  import { agendaLoadApi, waitlistApi } from '$lib/api/waitlist';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { Professional } from '$lib/types/agenda';
  import type { ServiceDuration, WaitlistEntry, WaitlistInput, WaitlistStatus } from '$lib/types/waitlist';
  import ConfirmModal from '$lib/components/ConfirmModal.svelte';
  import Guard from '$lib/components/Guard.svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import PageHeader from '$lib/components/ui/PageHeader.svelte';
  import WaitlistForm from '$lib/components/waitlist/WaitlistForm.svelte';

  const DAY_LABEL = ['dom', 'lun', 'mar', 'mié', 'jue', 'vie', 'sáb'];
  const STATUS: Record<WaitlistStatus, { label: string; cls: string }> = {
    waiting: { label: 'En espera', cls: 'bg-app-ink/8 text-app-ink' },
    offered: { label: 'Lugar ofrecido', cls: 'bg-app-primary/12 text-app-primary' },
    booked: { label: 'Agendó', cls: 'bg-app-accent/14 text-app-accent' },
    expired: { label: 'Venció', cls: 'bg-app-ink/8 text-app-muted' },
    cancelled: { label: 'Quitado', cls: 'bg-app-ink/8 text-app-muted' }
  };

  let entries = $state<WaitlistEntry[]>([]);
  let pros = $state<Professional[]>([]);
  let services = $state<ServiceDuration[]>([]);
  let formOpen = $state(false);
  let editing = $state<WaitlistEntry | null>(null);
  let removing = $state<WaitlistEntry | null>(null);
  const load = new Op();
  const act = new Op();
  const delOp = new Op();

  const canWrite = $derived(session.has('adminAppointments'));

  async function refresh() {
    await load.run(async () => {
      const [e, p, s] = await Promise.all([waitlistApi.list(), agendaApi.professionals(), agendaLoadApi.services()]);
      entries = e;
      pros = p.filter((x) => x.consults);
      services = s;
    });
  }

  function when(e: WaitlistEntry): string {
    const d = e.days.length ? [...e.days].sort((a, b) => ((a + 6) % 7) - ((b + 6) % 7)).map((x) => DAY_LABEL[x]).join(', ') : 'cualquier día';
    return e.from_time ? `${d}, ${e.from_time}–${e.to_time}` : d;
  }

  const fmtDate = (d: string) => new Date(d + 'T12:00:00').toLocaleDateString('es-MX', { weekday: 'short', day: 'numeric', month: 'short' });
  const fmtClock = (iso: string) => new Date(iso).toLocaleTimeString('es-MX', { hour: '2-digit', minute: '2-digit', hour12: false });

  function openForm(e: WaitlistEntry | null) {
    editing = e;
    formOpen = true;
  }

  function saved(e: WaitlistEntry) {
    entries = editing ? entries.map((x) => (x.id === e.id ? e : x)) : [...entries, e];
    formOpen = false;
  }

  function asInput(e: WaitlistEntry, status: WaitlistInput['status']): WaitlistInput {
    return {
      patient_id: e.patient_id, name: e.name, phone: e.phone, email: e.email, professional_id: e.professional_id, service_id: e.service_id,
      days: e.days, from_time: e.from_time ?? '', to_time: e.to_time ?? '', notes: e.notes, consent: e.consent, status
    };
  }

  async function markBooked(e: WaitlistEntry) {
    if (await act.run(async () => void (await waitlistApi.update(e.id, asInput(e, 'booked'))))) {
      entries = entries.filter((x) => x.id !== e.id);
      toast.show('Marcado como agendado');
    } else toast.show(act.message, 'error');
  }

  async function remove() {
    const e = removing;
    if (!e) return;
    if (await delOp.run(() => waitlistApi.remove(e.id))) {
      entries = entries.filter((x) => x.id !== e.id);
      removing = null;
      toast.show('Quitado de la lista');
    }
  }

  async function scan() {
    if (await act.run(async () => {
      const n = await waitlistApi.scan();
      await refresh();
      toast.show(n ? `Se ofreció ${n === 1 ? 'un lugar' : `${n} lugares`}` : 'No hay lugares libres para quienes esperan');
    })) return;
    toast.show(act.message, 'error');
  }

  onMount(refresh);
</script>

<svelte:head><title>Lista de espera · Caresia</title></svelte:head>

<Guard title="Lista de espera" permissions={['navAppointments', 'adminAppointments']}>
  <PageHeader title="Lista de espera" subtitle="Quienes esperan un lugar. Cuando se libera uno que les acomoda, el primero de la fila recibe la oferta por correo.">
    {#snippet actions()}
      <a href="/admin/navegar-citas" class="btn-secondary"><Icon name="calendar" size={18} />Agenda</a>
      {#if canWrite}
        <button type="button" class="btn-secondary" onclick={scan} disabled={act.phase === 'loading'}><Icon name="refresh" size={18} />Buscar lugares ahora</button>
        <button type="button" class="btn-primary" onclick={() => openForm(null)}><Icon name="plus" size={18} />Agregar</button>
      {/if}
    {/snippet}
  </PageHeader>

  {#if load.phase === 'loading' && entries.length === 0}
    <div class="card"><LoadingRows /></div>
  {:else if load.phase === 'error'}
    <Alert>{load.message}</Alert>
  {:else if entries.length === 0}
    <div class="card">
      <EmptyState icon="clock-plus" title="Nadie en espera" text="Aquí aparecen quienes se anotan desde la reserva en línea cuando no hay lugar, y a quienes agregues tú.">
        {#if canWrite}<button type="button" class="btn-primary" onclick={() => openForm(null)}><Icon name="plus" size={18} />Agregar a alguien</button>{/if}
      </EmptyState>
    </div>
  {:else}
    <ul class="grid gap-3">
      {#each entries as e, i (e.id)}
        <li class="card px-4 py-4 sm:px-5">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div class="min-w-0">
              <p class="flex flex-wrap items-center gap-2">
                <span class="font-mono text-xs text-app-muted" title="Lugar en la fila">#{i + 1}</span>
                <span class="font-semibold [overflow-wrap:anywhere]">{e.name}</span>
                <span class="rounded-full px-2.5 py-0.5 text-xs font-medium {STATUS[e.status].cls}">{STATUS[e.status].label}</span>
                {#if e.created_via === 'online'}<span class="rounded-full bg-app-ink/6 px-2.5 py-0.5 text-xs text-app-muted">Reserva en línea</span>{/if}
              </p>
              <p class="mt-1 text-sm text-app-muted">
                {e.professional_name || 'Cualquier profesional'}{e.service_name ? ` · ${e.service_name}` : ''} · {when(e)}
              </p>
              <p class="mt-1 flex flex-wrap gap-x-4 gap-y-1 text-sm">
                {#if e.phone}<a class="inline-flex items-center gap-1.5 text-app-primary" href="tel:{e.phone}"><Icon name="phone" size={14} />{e.phone}</a>{/if}
                {#if e.email}<span class="inline-flex items-center gap-1.5 [overflow-wrap:anywhere]"><Icon name="mail" size={14} />{e.email}</span>{/if}
                {#if !(e.consent && e.email)}<span class="text-app-muted">Sin aviso automático: llámale</span>{/if}
              </p>
              {#if e.notes}<p class="mt-1 text-sm text-app-muted [overflow-wrap:anywhere]">{e.notes}</p>{/if}
              {#if e.offer}
                <p class="mt-2 rounded-xl bg-app-primary/8 px-3 py-2 text-sm">
                  Se le ofreció <strong>{fmtDate(e.offer.date)} {e.offer.start} h</strong> con {e.offer.professional}; vence a las {fmtClock(e.offer.expires_at)}.
                </p>
              {/if}
            </div>
            {#if canWrite}
              <div class="flex flex-wrap gap-2">
                <button type="button" class="btn-secondary" onclick={() => openForm(e)} aria-label="Editar a {e.name}"><Icon name="edit" size={16} />Editar</button>
                <button type="button" class="btn-secondary" onclick={() => markBooked(e)} disabled={act.phase === 'loading'} aria-label="Marcar como agendado a {e.name}"><Icon name="check" size={16} />Ya agendó</button>
                <button type="button" class="btn-secondary" onclick={() => (removing = e)} aria-label="Quitar a {e.name} de la lista"><Icon name="trash" size={16} />Quitar</button>
              </div>
            {/if}
          </div>
        </li>
      {/each}
    </ul>
  {/if}

  <WaitlistForm open={formOpen} entry={editing} {pros} {services} onsaved={saved} onclose={() => (formOpen = false)} />
  <ConfirmModal open={!!removing} title="Quitar de la lista" op={delOp} confirmLabel="Quitar" onconfirm={remove} onclose={() => (removing = null)}>
    <p>¿Quitar a <strong>{removing?.name}</strong> de la lista de espera? Si tenía un lugar ofrecido, se retira.</p>
  </ConfirmModal>
</Guard>
