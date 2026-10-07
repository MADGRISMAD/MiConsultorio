<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import { emptyAppointment, PERMISSIONS, type Appointment, type AppointmentInput } from '$lib/types';
  import AppointmentForm from './AppointmentForm.svelte';
  import ConfirmModal from './ConfirmModal.svelte';
  import Modal from './Modal.svelte';
  import EmptyState from './ui/EmptyState.svelte';
  import Icon from './ui/Icon.svelte';
  import LoadingRows from './ui/LoadingRows.svelte';
  import PageHeader from './ui/PageHeader.svelte';

  let { admin = false }: { admin?: boolean } = $props();

  let items = $state<Appointment[]>([]);
  let loading = $state(true);
  let loadError = $state('');

  async function load() {
    try {
      items = await api.appointments();
      loadError = '';
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'No se pudieron cargar las citas.';
    } finally {
      loading = false;
    }
  }
  onMount(load);

  const today = new Date().toISOString().slice(0, 10);
  const longDate = (d: string) => new Date(d + 'T12:00:00').toLocaleDateString('es-MX', { weekday: 'short', day: 'numeric', month: 'short', year: 'numeric' });

  // ----- details modal -----
  let viewing = $state<Appointment | null>(null);
  const canSeeExpedients = $derived(session.has(PERMISSIONS.navHistorials) || session.has(PERMISSIONS.adminHistorials));

  function openDetails(a: Appointment) {
    viewing = a;
  }

  // ----- create / edit -----
  let formOpen = $state(false);
  let editingId = $state<string | null>(null);
  let form = $state<AppointmentInput>(emptyAppointment());
  const formOp = new Op();

  function openCreate() {
    editingId = null;
    form = emptyAppointment();
    formOp.reset();
    formOpen = true;
  }
  function openEdit(a: Appointment) {
    editingId = a.id;
    const { id: _id, ...rest } = a;
    form = { ...rest };
    formOp.reset();
    formOpen = true;
  }
  async function submitForm(e: SubmitEvent) {
    e.preventDefault();
    const id = editingId;
    if (!(await formOp.run(() => (id ? api.updateAppointment(id, form) : api.createAppointment(form))))) return;
    formOpen = false;
    toast.show(id ? 'Cita actualizada' : 'Cita creada');
    await load();
  }

  // ----- delete -----
  let deleting = $state<Appointment | null>(null);
  const deleteOp = new Op();
  async function confirmDelete() {
    const target = deleting;
    if (!target) return;
    if (await deleteOp.run(() => api.deleteAppointment(target.id))) {
      deleting = null;
      toast.show('Cita eliminada');
      await load();
    }
  }
</script>

<PageHeader title={admin ? 'Administrar citas' : 'Citas'} subtitle={admin ? 'Crea, reprograma y cancela citas.' : 'Consulta la agenda del consultorio.'}>
  {#snippet actions()}
    {#if admin}<button type="button" class="btn-primary" onclick={openCreate}><Icon name="plus" size={18} stroke={2.2} />Nueva cita</button>{/if}
  {/snippet}
</PageHeader>

<div class="card overflow-hidden">
  {#if loading}
    <LoadingRows />
  {:else if loadError}
    <p class="alert m-5" role="alert"><Icon name="alert" size={18} />{loadError}</p>
  {:else if items.length === 0}
    <EmptyState icon="calendar" title="Aún no hay citas" text={admin ? 'Agenda la primera cita de tu consultorio.' : 'Cuando se agenden citas aparecerán aquí.'}>
      {#if admin}<button type="button" class="btn-primary" onclick={openCreate}><Icon name="plus" size={18} stroke={2.2} />Nueva cita</button>{/if}
    </EmptyState>
  {:else}
    <div class="overflow-x-auto">
      <table class="w-full min-w-[40rem]">
        <thead class="border-b border-app-ink/10 bg-app-elevated">
          <tr>
            <th class="th">Paciente</th>
            <th class="th">Fecha</th>
            <th class="th">Horario</th>
            <th class="th"><span class="sr-only">Acciones</span></th>
          </tr>
        </thead>
        <tbody class="divide-y divide-app-ink/8">
          {#each items as a (a.id)}
            <tr class="transition hover:bg-app-ink/[0.03]">
              <td class="td">
                <span class="block font-semibold">{a.names} {a.last_names}</span>
                {#if a.CURP}<span class="block font-mono text-xs text-app-muted">{a.CURP}</span>{/if}
              </td>
              <td class="td whitespace-nowrap">
                {longDate(a.date)}
                {#if a.date === today}<span class="badge ml-2">Hoy</span>{/if}
              </td>
              <td class="td whitespace-nowrap font-semibold tabular-nums">{a.startHour} – {a.endHour}</td>
              <td class="td">
                <div class="flex justify-end gap-1">
                  <button type="button" class="icon-btn" title="Ver detalles" aria-label="Ver detalles de la cita de {a.names} {a.last_names}" onclick={() => openDetails(a)}><Icon name="eye" size={19} /></button>
                  {#if admin}
                    <button type="button" class="icon-btn" title="Editar" aria-label="Editar la cita de {a.names} {a.last_names}" onclick={() => openEdit(a)}><Icon name="edit" size={19} /></button>
                    <button type="button" class="icon-btn danger" title="Eliminar" aria-label="Eliminar la cita de {a.names} {a.last_names}" onclick={() => { deleteOp.reset(); deleting = a; }}><Icon name="trash" size={19} /></button>
                  {/if}
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<Modal open={viewing !== null} title="Detalles de la cita" wide onclose={() => (viewing = null)}>
  {#if viewing}
    <dl class="grid gap-4 rounded-xl bg-app-elevated p-4 text-left sm:grid-cols-3">
      <div><dt class="section-title">Paciente</dt><dd class="mt-1 font-semibold">{viewing.names} {viewing.last_names}</dd></div>
      {#if viewing.CURP}<div><dt class="section-title">CURP</dt><dd class="mt-1 font-mono text-sm">{viewing.CURP}</dd></div>{/if}
      <div><dt class="section-title">Fecha</dt><dd class="mt-1 font-semibold">{longDate(viewing.date)}</dd></div>
      <div><dt class="section-title">Hora de inicio</dt><dd class="mt-1 font-semibold">{viewing.startHour}</dd></div>
      <div><dt class="section-title">Hora de finalización</dt><dd class="mt-1 font-semibold">{viewing.endHour}</dd></div>
      <div class="sm:col-span-3"><dt class="section-title">Detalles</dt><dd class="mt-1 whitespace-pre-wrap">{viewing.details || '—'}</dd></div>
    </dl>
    {#if viewing.patient_id && canSeeExpedients}
      <div class="mt-5 flex flex-wrap gap-2">
        <a class="btn-secondary" href="/pacientes/{encodeURIComponent(viewing.patient_id)}"><Icon name="folder" size={18} />Abrir expediente</a>
        <a class="btn-primary" href="/pacientes/{encodeURIComponent(viewing.patient_id)}?cita={encodeURIComponent(viewing.id)}&motivo={encodeURIComponent(viewing.details)}"><Icon name="stethoscope" size={18} />Registrar consulta</a>
      </div>
    {/if}
  {/if}
  {#snippet footer()}<button type="button" class="btn-secondary" onclick={() => (viewing = null)}>Cerrar</button>{/snippet}
</Modal>

<Modal open={formOpen} title={editingId ? 'Editar cita' : 'Nueva cita'} onclose={() => (formOpen = false)}>
  <form id="appointment-form" onsubmit={submitForm}>
    <AppointmentForm bind:data={form} />
  </form>
  {#if formOp.phase === 'error'}
    <p class="alert mt-4" role="alert"><Icon name="alert" size={18} />{formOp.message}</p>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-ghost mr-auto" onclick={() => (form = emptyAppointment())}>Limpiar</button>
    <button type="button" class="btn-secondary" onclick={() => (formOpen = false)}>Cancelar</button>
    <button type="submit" form="appointment-form" class="btn-primary" disabled={formOp.phase === 'loading'}>
      {#if formOp.phase === 'loading'}<span class="spin"></span>{/if}Guardar
    </button>
  {/snippet}
</Modal>

<ConfirmModal open={deleting !== null} title="¿Eliminar esta cita?" confirmLabel="Eliminar" op={deleteOp} onconfirm={confirmDelete} onclose={() => (deleting = null)}>
  <p>Se eliminará la cita de <strong class="text-app-ink">{deleting?.names} {deleting?.last_names}</strong> del {deleting ? longDate(deleting.date) : ''}.</p>
</ConfirmModal>
