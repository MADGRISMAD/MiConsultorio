<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import { emptyAppointment, PERMISSIONS, type Appointment, type AppointmentInput, type Expedient } from '$lib/types';
  import AppointmentForm from './AppointmentForm.svelte';
  import ConfirmModal from './ConfirmModal.svelte';
  import ExpedientView from './ExpedientView.svelte';
  import Modal from './Modal.svelte';
  import Notice from './Notice.svelte';

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

  // ----- details modal -----
  let viewing = $state<Appointment | null>(null);
  let viewingExpedient = $state<Expedient | null>(null);
  const canSeeExpedients = $derived(session.has(PERMISSIONS.navHistorials) || session.has(PERMISSIONS.adminHistorials));

  async function openDetails(a: Appointment) {
    viewing = a;
    viewingExpedient = null;
    if (!canSeeExpedients) return;
    try {
      const e = await api.expedient(a.CURP);
      if (viewing?.id === a.id) viewingExpedient = e;
    } catch {
      /* no matching record: the modal says so */
    }
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
    const ok = await formOp.run(() => (id ? api.updateAppointment(id, form) : api.createAppointment(form)), id ? 'Cita actualizada exitosamente' : 'Cita creada exitosamente');
    if (!ok) return;
    await load();
    setTimeout(() => {
      formOpen = false;
      formOp.reset();
    }, 800);
  }

  // ----- delete -----
  let deleting = $state<Appointment | null>(null);
  const deleteOp = new Op();
  async function confirmDelete() {
    const target = deleting;
    if (!target) return;
    if (await deleteOp.run(() => api.deleteAppointment(target.id))) {
      deleting = null;
      deleteOp.reset();
      await load();
    }
  }
</script>

<div class="mx-auto max-w-6xl px-4 py-8">
  <div class="flex items-center justify-between gap-4">
    <h1 class="font-display text-4xl">{admin ? 'Administrar citas' : 'Citas'}</h1>
    {#if admin}<button type="button" class="btn-primary" onclick={openCreate}>Nueva cita</button>{/if}
  </div>

  <div class="mt-6 overflow-x-auto rounded-2xl bg-white shadow-sm ring-1 ring-ink/10">
    {#if loading}
      <p class="p-8 text-center text-ink-soft">Cargando...</p>
    {:else if loadError}
      <p class="p-8 text-center text-red-600" role="alert">{loadError}</p>
    {:else if items.length === 0}
      <p class="p-8 text-center text-ink-soft">No hay citas registradas.</p>
    {:else}
      <table class="w-full min-w-max">
        <thead class="border-b border-ink/10 bg-paper">
          <tr>
            <th class="th">Nombre completo</th>
            <th class="th">Fecha</th>
            <th class="th">Hora de inicio</th>
            <th class="th">Hora de finalización</th>
            <th class="th"><span class="sr-only">Acciones</span></th>
          </tr>
        </thead>
        <tbody class="divide-y divide-ink/5">
          {#each items as a (a.id)}
            <tr>
              <td class="td whitespace-nowrap">{a.names} {a.last_names}</td>
              <td class="td whitespace-nowrap">{a.date}</td>
              <td class="td whitespace-nowrap">{a.startHour}</td>
              <td class="td whitespace-nowrap">{a.endHour}</td>
              <td class="td">
                <div class="flex justify-end gap-2">
                  <button type="button" class="icon-btn" title="Ver detalles" aria-label="Ver detalles de la cita de {a.names} {a.last_names}" onclick={() => openDetails(a)}><img src="/watch.png" alt="" class="h-5 w-5" /></button>
                  {#if admin}
                    <button type="button" class="icon-btn" title="Eliminar" aria-label="Eliminar la cita de {a.names} {a.last_names}" onclick={() => { deleteOp.reset(); deleting = a; }}><img src="/delete.png" alt="" class="h-5 w-5" /></button>
                    <button type="button" class="icon-btn" title="Editar" aria-label="Editar la cita de {a.names} {a.last_names}" onclick={() => openEdit(a)}><img src="/edit.png" alt="" class="h-5 w-5" /></button>
                  {/if}
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>
</div>

<Modal open={viewing !== null} title="Detalles de cita" wide onclose={() => (viewing = null)}>
  {#if viewing}
    <dl class="grid gap-4 rounded-xl bg-paper p-4 text-left sm:grid-cols-3">
      <div><dt class="text-xs font-semibold uppercase text-ink-faint">Paciente</dt><dd>{viewing.names} {viewing.last_names}</dd></div>
      <div><dt class="text-xs font-semibold uppercase text-ink-faint">CURP</dt><dd>{viewing.CURP}</dd></div>
      <div><dt class="text-xs font-semibold uppercase text-ink-faint">Fecha</dt><dd>{viewing.date}</dd></div>
      <div><dt class="text-xs font-semibold uppercase text-ink-faint">Hora de inicio</dt><dd>{viewing.startHour}</dd></div>
      <div><dt class="text-xs font-semibold uppercase text-ink-faint">Hora de finalización</dt><dd>{viewing.endHour}</dd></div>
      <div class="sm:col-span-3"><dt class="text-xs font-semibold uppercase text-ink-faint">Detalles de la cita</dt><dd class="whitespace-pre-wrap">{viewing.details || '—'}</dd></div>
    </dl>
    {#if canSeeExpedients}
      <h3 class="py-4 text-center text-lg font-semibold">Historial asociado</h3>
      {#if viewingExpedient}
        <ExpedientView expedient={viewingExpedient} />
      {:else}
        <p class="rounded-lg bg-red-50 p-4 text-center text-red-700">No se encontró un historial asociado con la CURP {viewing.CURP}</p>
      {/if}
    {/if}
  {/if}
  {#snippet footer()}<button type="button" class="btn-secondary" onclick={() => (viewing = null)}>Cerrar</button>{/snippet}
</Modal>

<Modal open={formOpen} title={editingId ? 'Editar cita' : 'Nueva cita'} onclose={() => (formOpen = false)}>
  <form id="appointment-form" onsubmit={submitForm}>
    <AppointmentForm bind:data={form} />
  </form>
  {#if formOp.phase !== 'idle'}
    <div class="mt-4"><Notice kind={formOp.phase} message={formOp.phase === 'loading' ? 'Cargando...' : formOp.message} /></div>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={() => (formOpen = false)}>Cancelar</button>
    <button type="button" class="btn-secondary" onclick={() => (form = emptyAppointment())}>Limpiar</button>
    <button type="submit" form="appointment-form" class="btn-primary" disabled={formOp.phase === 'loading'}>Aceptar</button>
  {/snippet}
</Modal>

<ConfirmModal
  open={deleting !== null}
  title="¿Seguro que quieres eliminar esta cita?"
  op={deleteOp}
  onconfirm={confirmDelete}
  onclose={() => (deleting = null)}
>
  <p class="text-ink-soft">Esto eliminará la cita de {deleting?.names} {deleting?.last_names} del {deleting?.date}.</p>
</ConfirmModal>
