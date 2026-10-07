<script lang="ts">
  import { page } from '$app/state';
  import { bookingApi } from '$lib/api/booking';
  import { Op } from '$lib/op.svelte';
  import type { PublicAppointment } from '$lib/types/booking';
  import Icon from '$lib/components/ui/Icon.svelte';

  let { token }: { token: string } = $props();

  let appt = $state<PublicAppointment | null>(null);
  let missing = $state(false);
  let notice = $state('');
  let reason = $state('');
  let askCancel = $state(false);
  let askOptout = $state(false);
  const load = new Op();
  const act = new Op();

  const STATUS: Record<string, { label: string; tone: string }> = {
    scheduled: { label: 'Agendada', tone: 'pill-info' },
    confirmed: { label: 'Confirmada', tone: 'pill-ok' },
    arrived: { label: 'En consultorio', tone: 'pill-ok' },
    in_progress: { label: 'En consulta', tone: 'pill-ok' },
    completed: { label: 'Realizada', tone: '' },
    no_show: { label: 'No asististe', tone: 'pill-warn' },
    cancelled: { label: 'Cancelada', tone: 'pill-bad' }
  };

  const dateLong = (d: string) => {
    const [y, m, day] = d.split('-').map(Number);
    return new Date(y, m - 1, day).toLocaleDateString('es-MX', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' });
  };

  $effect(() => {
    load
      .run(async () => {
        try {
          appt = await bookingApi.appointment(token);
        } catch (e) {
          if (e && typeof e === 'object' && 'status' in e && e.status === 404) {
            missing = true;
            return;
          }
          throw e;
        }
      })
      .then(() => {
        const q = page.url.searchParams;
        if (appt && q.get('baja') === '1' && appt.reminders) askOptout = true;
        if (appt && q.get('accion') === 'cancelar' && appt.can_cancel) askCancel = true;
        if (appt && q.get('accion') === 'confirmar' && appt.can_confirm) confirm();
      });
  });

  async function run(fn: () => Promise<PublicAppointment>, message: string) {
    notice = '';
    try {
      await act.run(async () => {
        appt = await fn();
        notice = message;
      });
    } catch {
      /* Op shows the error */
    }
  }

  const confirm = () => run(() => bookingApi.confirm(token), 'Gracias, tu asistencia quedó confirmada.');
  const cancel = async () => {
    await run(() => bookingApi.cancel(token, reason), 'Tu cita fue cancelada.');
    if (act.phase !== 'error') askCancel = false;
  };
  const optout = async () => {
    await run(() => bookingApi.optout(token), 'Listo, ya no recibirás recordatorios.');
    if (act.phase !== 'error') askOptout = false;
  };
</script>

{#if load.phase === 'loading' || (load.phase === 'idle' && !appt && !missing)}
  <div class="card px-6 py-10 text-center text-sm text-app-muted" role="status">Cargando tu cita…</div>
{:else if missing}
  <section class="card px-6 py-9">
    <div class="grid h-12 w-12 place-items-center rounded-full bg-app-ink/8 text-app-muted"><Icon name="calendar" size={24} /></div>
    <h1 class="display mt-4 text-3xl leading-tight">No encontramos esa cita</h1>
    <p class="mt-3 text-app-muted">El enlace puede estar incompleto o ya no ser válido. Revisa el último correo o mensaje que recibiste, o comunícate con tu consultorio.</p>
  </section>
{:else if load.phase === 'error' || !appt}
  <section class="card px-6 py-9" role="alert">
    <p class="alert"><Icon name="alert" size={18} />{load.message || 'No se pudo cargar la cita.'}</p>
    <button class="btn-secondary mt-5" onclick={() => location.reload()}>Reintentar</button>
  </section>
{:else}
  {@const st = STATUS[appt.status] ?? { label: appt.status, tone: '' }}
  <section class="card page-in px-5 py-7 sm:px-8" aria-labelledby="ap-title">
    <p class="section-title">{appt.clinic.name}</p>
    <div class="mt-2 flex flex-wrap items-center gap-3">
      <h1 id="ap-title" class="display text-[2rem] leading-tight">Tu cita</h1>
      <span class="pill {st.tone}">{st.label}</span>
    </div>

    <div class="mt-4 space-y-3" aria-live="polite">
      {#if notice}<p class="flex items-start gap-2 rounded-xl bg-app-accent/10 px-3.5 py-3 text-sm font-medium text-app-accent" role="status"><Icon name="check" size={18} class="mt-px flex-none" />{notice}</p>{/if}
      {#if act.phase === 'error'}<p class="alert" role="alert"><Icon name="alert" size={18} />{act.message}</p>{/if}
    </div>

    <dl class="mt-4 grid gap-3 rounded-xl bg-app-elevated p-4 text-sm">
      <div><dt class="text-xs text-app-muted">Fecha</dt><dd class="font-semibold first-letter:uppercase">{dateLong(appt.date)}</dd></div>
      <div><dt class="text-xs text-app-muted">Hora</dt><dd class="font-semibold">{appt.start} h</dd></div>
      {#if appt.professional}<div><dt class="text-xs text-app-muted">Atiende</dt><dd class="font-semibold">{appt.professional}</dd></div>{/if}
      {#if appt.service}<div><dt class="text-xs text-app-muted">Servicio</dt><dd class="font-semibold">{appt.service}</dd></div>{/if}
      {#if appt.clinic.address}<div><dt class="text-xs text-app-muted">Dirección</dt><dd class="font-semibold">{appt.clinic.address}</dd></div>{/if}
      {#if appt.clinic.phone}<div><dt class="text-xs text-app-muted">Teléfono</dt><dd class="font-semibold"><a class="text-app-primary underline" href="tel:{appt.clinic.phone}">{appt.clinic.phone}</a></dd></div>{/if}
    </dl>

    {#if appt.past && appt.status !== 'cancelled'}
      <p class="mt-5 text-sm text-app-muted">Esta cita ya pasó.</p>
    {/if}

    <div class="mt-6 grid gap-3">
      {#if appt.can_confirm}
        <button class="btn-primary btn-lg" onclick={confirm} disabled={act.phase === 'loading'}><Icon name="check" size={18} />Confirmar asistencia</button>
      {/if}
      {#if appt.rebook_slug && !appt.past && (appt.status === 'scheduled' || appt.status === 'confirmed' || appt.status === 'cancelled')}
        <a class="btn-secondary btn-lg" href="/reservar/{appt.rebook_slug}"><Icon name="calendar" size={18} />{appt.status === 'cancelled' ? 'Agendar una nueva cita' : 'Reagendar'}</a>
        {#if appt.status !== 'cancelled'}<p class="hint !mt-0 text-center">Elige otro horario y después cancela esta cita.</p>{/if}
      {/if}
      {#if appt.can_cancel && !askCancel}
        <button class="btn-ghost btn-lg !text-app-danger" onclick={() => (askCancel = true)}>Cancelar cita</button>
      {:else if !appt.can_cancel && (appt.status === 'scheduled' || appt.status === 'confirmed') && !appt.past}
        <p class="rounded-xl bg-app-warning/12 px-3.5 py-3 text-sm">Faltan menos de {appt.cancel_min_hours} horas para tu cita. Para cancelar, comunícate con el consultorio{appt.clinic.phone ? ` al ${appt.clinic.phone}` : ''}.</p>
      {/if}
    </div>

    {#if askCancel && appt.can_cancel}
      <form class="mt-5 grid gap-3 rounded-xl border border-app-danger/30 p-4" onsubmit={(e) => { e.preventDefault(); cancel(); }}>
        <label class="label" for="ap-reason">¿Quieres decirnos el motivo? <span class="font-normal text-app-muted">(opcional)</span></label>
        <input id="ap-reason" class="field" bind:value={reason} maxlength="300" />
        <div class="flex flex-wrap gap-2">
          <button class="btn-danger" type="submit" disabled={act.phase === 'loading'}>Sí, cancelar mi cita</button>
          <button class="btn-ghost" type="button" onclick={() => (askCancel = false)}>No, conservarla</button>
        </div>
      </form>
    {/if}

    {#if appt.reminders}
      <div class="mt-7 border-t border-app-ink/10 pt-5">
        {#if !askOptout}
          <button class="text-sm text-app-muted underline hover:text-app-ink" onclick={() => (askOptout = true)}>Dejar de recibir recordatorios</button>
        {:else}
          <p class="text-sm">¿Dejar de recibir recordatorios de citas por correo y WhatsApp?</p>
          <div class="mt-3 flex flex-wrap gap-2">
            <button class="btn-secondary" onclick={optout} disabled={act.phase === 'loading'}>Sí, dejar de recibirlos</button>
            <button class="btn-ghost" onclick={() => (askOptout = false)}>Cancelar</button>
          </div>
        {/if}
      </div>
    {/if}
  </section>
{/if}
