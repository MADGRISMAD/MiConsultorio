<script lang="ts">
  import { goto } from '$app/navigation';
  import { agendaApi } from '$lib/api/agenda';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import { PERMISSIONS } from '$lib/types';
  import { STATUS_META, type Appt, type ApptStatus } from '$lib/types/agenda';
  import Modal from '../Modal.svelte';
  import Icon from '../ui/Icon.svelte';
  import Pill from '../ui/Pill.svelte';
  import { fmtLong, fullName } from './util';

  interface Props {
    appt: Appt | null;
    /** may edit, move and delete (administrators and reception) */
    canEdit: boolean;
    onclose: () => void;
    onchanged: (a: Appt) => void;
    onedit: (a: Appt) => void;
    ondelete: (a: Appt) => void;
  }
  let { appt, canEdit, onclose, onchanged, onedit, ondelete }: Props = $props();

  const op = new Op();
  let cancelling = $state(false);
  let reason = $state('');
  let noShowAsk = $state(false);

  $effect(() => {
    appt?.id;
    cancelling = false;
    noShowAsk = false;
    reason = '';
    op.reset();
  });

  const canStatus = $derived(!!appt && (session.has(PERMISSIONS.adminAppointments) || (session.user?.role === 'doctor' && appt.professional_id === session.user.userId)));
  const canExpedient = $derived(session.has(PERMISSIONS.navHistorials) || session.has(PERMISSIONS.adminHistorials));
  const canCharge = $derived(session.cobros && session.has(PERMISSIONS.pos));
  const open = $derived(!!appt && ['scheduled', 'confirmed', 'arrived', 'in_progress'].includes(appt.status));
  const editable = $derived(!!appt && ['scheduled', 'confirmed', 'arrived'].includes(appt.status));

  async function change(status: ApptStatus, why = '') {
    if (!appt) return;
    const id = appt.id;
    if (await op.run(async () => onchanged(await agendaApi.setStatus(id, status, why)))) {
      cancelling = false;
      noShowAsk = false;
      if (status === 'cancelled') toast.show('Cita cancelada');
      else if (status === 'confirmed') toast.show('Cita confirmada');
    }
  }

  const consultUrl = (a: Appt) => `/pacientes/${encodeURIComponent(a.patient_id ?? '')}?cita=${encodeURIComponent(a.id)}&motivo=${encodeURIComponent(a.details || a.service_name)}`;

  /** Marks the arrival when needed, then the consultation as started, and opens the patient's record. */
  async function startConsult() {
    if (!appt?.patient_id) return;
    const a = appt;
    const ok = await op.run(async () => {
      let cur = a;
      if (cur.status === 'scheduled' || cur.status === 'confirmed') cur = await agendaApi.setStatus(cur.id, 'arrived');
      if (cur.status === 'arrived') cur = await agendaApi.setStatus(cur.id, 'in_progress');
      onchanged(cur);
    });
    if (ok) goto(consultUrl(a));
  }
</script>

<Modal open={appt !== null} title="Detalles de la cita" wide {onclose}>
  {#if appt}
    <div class="mb-4 flex flex-wrap items-center gap-2">
      <Pill tone={STATUS_META[appt.status].tone}>{STATUS_META[appt.status].label}</Pill>
      {#if appt.source !== 'staff'}<span class="badge">Reservada en línea</span>{/if}
    </div>
    <dl class="grid gap-4 rounded-xl bg-app-elevated p-4 text-left sm:grid-cols-3">
      <div class="sm:col-span-2"><dt class="section-title">Paciente</dt><dd class="mt-1 font-semibold">{fullName(appt)}</dd></div>
      <div><dt class="section-title">Profesional</dt><dd class="mt-1 font-semibold">{appt.professional_name || 'Sin asignar'}</dd></div>
      <div class="sm:col-span-2"><dt class="section-title">Fecha</dt><dd class="mt-1 font-semibold">{fmtLong(appt.date)}</dd></div>
      <div><dt class="section-title">Horario</dt><dd class="mt-1 font-semibold tabular-nums">{appt.startHour} – {appt.endHour}</dd></div>
      {#if appt.service_name}<div><dt class="section-title">Servicio</dt><dd class="mt-1">{appt.service_name}</dd></div>{/if}
      {#if appt.room}<div><dt class="section-title">Sala</dt><dd class="mt-1">{appt.room}</dd></div>{/if}
      {#if appt.phone}<div><dt class="section-title">Teléfono</dt><dd class="mt-1"><a class="text-app-primary hover:underline" href="tel:{appt.phone}">{appt.phone}</a></dd></div>{/if}
      {#if appt.email}<div><dt class="section-title">Correo</dt><dd class="mt-1 break-all">{appt.email}</dd></div>{/if}
      {#if appt.CURP}<div><dt class="section-title">CURP</dt><dd class="mt-1 font-mono text-sm">{appt.CURP}</dd></div>{/if}
      <div class="sm:col-span-3"><dt class="section-title">Notas</dt><dd class="mt-1 whitespace-pre-wrap">{appt.details || '—'}</dd></div>
      {#if appt.cancel_reason}<div class="sm:col-span-3"><dt class="section-title">Motivo</dt><dd class="mt-1 whitespace-pre-wrap">{appt.cancel_reason}</dd></div>{/if}
    </dl>

    {#if op.phase === 'error'}
      <p class="alert mt-4" role="alert"><Icon name="alert" size={18} />{op.message}</p>
    {/if}

    {#if cancelling || noShowAsk}
      <form class="mt-5 grid gap-2" onsubmit={(e) => { e.preventDefault(); change(cancelling ? 'cancelled' : 'no_show', reason); }}>
        <label class="label" for="cancel-reason">{cancelling ? 'Motivo de la cancelación' : 'Nota (opcional)'}</label>
        <textarea id="cancel-reason" class="field min-h-20" bind:value={reason} maxlength="500" placeholder={cancelling ? 'Por ejemplo: el paciente avisó que no puede venir' : ''}></textarea>
        <div class="flex flex-wrap justify-end gap-2">
          <button type="button" class="btn-secondary" onclick={() => { cancelling = false; noShowAsk = false; }}>Volver</button>
          <button type="submit" class="btn-danger" disabled={op.phase === 'loading'}>{cancelling ? 'Cancelar la cita' : 'Marcar como no asistió'}</button>
        </div>
      </form>
    {:else}
      <div class="mt-5 flex flex-wrap gap-2">
        {#if canStatus}
          {#if appt.status === 'scheduled'}
            <button type="button" class="btn-secondary" disabled={op.phase === 'loading'} onclick={() => change('confirmed')}><Icon name="check" size={18} />Confirmar</button>
          {/if}
          {#if appt.status === 'scheduled' || appt.status === 'confirmed'}
            <button type="button" class="btn-secondary" disabled={op.phase === 'loading'} onclick={() => change('arrived')}><Icon name="user" size={18} />Llegó</button>
          {/if}
          {#if ['scheduled', 'confirmed', 'arrived'].includes(appt.status) && appt.patient_id && canExpedient}
            <button type="button" class="btn-primary" disabled={op.phase === 'loading'} onclick={startConsult}><Icon name="stethoscope" size={18} />Iniciar consulta</button>
          {/if}
          {#if appt.status === 'in_progress'}
            {#if appt.patient_id && canExpedient}
              <a class="btn-primary" href={consultUrl(appt)}><Icon name="stethoscope" size={18} />Continuar consulta</a>
            {/if}
            <button type="button" class="btn-secondary" disabled={op.phase === 'loading'} onclick={() => change('completed')}><Icon name="check" size={18} />Terminar</button>
          {/if}
        {/if}
        {#if appt.status === 'completed' && appt.patient_id && canExpedient}
          <a class="btn-secondary" href="/pacientes/{encodeURIComponent(appt.patient_id)}"><Icon name="folder" size={18} />Abrir expediente</a>
        {/if}
        {#if canCharge && ['arrived', 'in_progress', 'completed'].includes(appt.status) && !appt.sale_id}
          <a class="btn-secondary" href="/pos/cobros?cita={encodeURIComponent(appt.id)}"><Icon name="cash" size={18} />Cobrar</a>
        {/if}
        {#if canStatus && ['scheduled', 'confirmed', 'arrived'].includes(appt.status)}
          <button type="button" class="btn-ghost" onclick={() => (noShowAsk = true)}><Icon name="ban" size={18} />No asistió</button>
          <button type="button" class="btn-ghost text-app-danger" onclick={() => (cancelling = true)}><Icon name="x" size={18} />Cancelar</button>
        {/if}
      </div>
    {/if}
  {/if}
  {#snippet footer()}
    {#if appt && canEdit}
      <button type="button" class="btn-ghost mr-auto text-app-danger" onclick={() => appt && ondelete(appt)}><Icon name="trash" size={18} />Eliminar</button>
      {#if editable}<button type="button" class="btn-secondary" onclick={() => appt && onedit(appt)}><Icon name="edit" size={18} />Editar o reprogramar</button>{/if}
    {/if}
    <button type="button" class="btn-secondary" onclick={onclose}>Cerrar</button>
  {/snippet}
</Modal>
