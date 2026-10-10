<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import AddToCalendar from '$lib/components/ui/AddToCalendar.svelte';
  import { page } from '$app/state';
  import { bookingApi } from '$lib/api/booking';
  import { Op } from '$lib/op.svelte';
  import type { PublicAppointment } from '$lib/types/booking';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { fmtDay, t } from '$lib/i18n/index.svelte';

  let { token }: { token: string } = $props();

  let appt = $state<PublicAppointment | null>(null);
  let missing = $state(false);
  let notice = $state('');
  let reason = $state('');
  let askCancel = $state(false);
  let askOptout = $state(false);
  const load = new Op();
  const act = new Op();

  const STATUS: Record<string, string> = {
    scheduled: 'pill-info',
    confirmed: 'pill-ok',
    arrived: 'pill-ok',
    in_progress: 'pill-ok',
    completed: '',
    no_show: 'pill-warn',
    cancelled: 'pill-bad'
  };
  const statusLabel = (s: string) => (s in STATUS ? t(`appt.status.${s}`) : s);

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

  const confirm = () => run(() => bookingApi.confirm(token), t('appt.msgConfirmed'));
  const cancel = async () => {
    await run(() => bookingApi.cancel(token, reason), t('appt.msgCancelled'));
    if (act.phase !== 'error') askCancel = false;
  };
  const optout = async () => {
    await run(() => bookingApi.optout(token), t('appt.msgOptout'));
    if (act.phase !== 'error') askOptout = false;
  };
</script>

{#if load.phase === 'loading' || (load.phase === 'idle' && !appt && !missing)}
  <div class="card-empty" role="status">{t('appt.loading')}</div>
{:else if missing}
  <section class="card px-6 py-9">
    <div class="grid h-12 w-12 place-items-center rounded-full bg-app-ink/8 text-app-muted"><Icon name="calendar" size={24} /></div>
    <h1 class="display mt-4 text-3xl leading-tight">{t('appt.missingTitle')}</h1>
    <p class="mt-3 text-app-muted">{t('appt.missingText')}</p>
  </section>
{:else if load.phase === 'error' || !appt}
  <section class="card px-6 py-9" role="alert">
    <p class="alert"><Icon name="alert" size={18} />{load.message || t('appt.loadError')}</p>
    <button class="btn-secondary mt-5" onclick={() => location.reload()}>{t('common.retry')}</button>
  </section>
{:else}
  {@const tone = STATUS[appt.status] ?? ''}
  <section class="card page-in px-5 py-7 sm:px-8" aria-labelledby="ap-title">
    <p class="section-title">{appt.clinic.name}</p>
    <div class="mt-2 flex flex-wrap items-center gap-3">
      <h1 id="ap-title" class="display text-[2rem] leading-tight">{t('appt.title')}</h1>
      <span class="pill {tone}">{statusLabel(appt.status)}</span>
    </div>

    <div class="mt-4 space-y-3" aria-live="polite">
      {#if notice}<p class="flex items-start gap-2 rounded-xl bg-app-accent/10 px-3.5 py-3 text-sm font-medium text-app-accent" role="status"><Icon name="check" size={18} class="mt-px flex-none" />{notice}</p>{/if}
      <OpError op={act} />
    </div>

    <dl class="mt-4 grid gap-3 rounded-xl bg-app-elevated p-4 text-sm">
      <div><dt class="text-xs text-app-muted">{t('common.date')}</dt><dd class="font-semibold first-letter:uppercase">{fmtDay(appt.date)}</dd></div>
      <div><dt class="text-xs text-app-muted">{t('common.time')}</dt><dd class="font-semibold">{t('common.hourSuffix', { time: appt.start })}</dd></div>
      {#if appt.professional}<div><dt class="text-xs text-app-muted">{t('common.attends')}</dt><dd class="font-semibold">{appt.professional}</dd></div>{/if}
      {#if appt.service}<div><dt class="text-xs text-app-muted">{t('common.service')}</dt><dd class="font-semibold">{appt.service}</dd></div>{/if}
      {#if appt.clinic.address}<div><dt class="text-xs text-app-muted">{t('common.address')}</dt><dd class="font-semibold">{appt.clinic.address}</dd></div>{/if}
      {#if appt.clinic.phone}<div><dt class="text-xs text-app-muted">{t('common.phone')}</dt><dd class="font-semibold"><a class="text-app-primary underline" href="tel:{appt.clinic.phone}">{appt.clinic.phone}</a></dd></div>{/if}
    </dl>

    {#if appt.past && appt.status !== 'cancelled'}
      <p class="mt-5 text-sm text-app-muted">{t('appt.past')}</p>
    {/if}

    <div class="mt-6 grid gap-3">
      {#if appt.can_confirm}
        <button class="btn-primary btn-lg" onclick={confirm} disabled={act.phase === 'loading'}><Icon name="check" size={18} />{t('appt.confirm')}</button>
      {/if}
      {#if !appt.past && (appt.status === 'scheduled' || appt.status === 'confirmed')}
        <AddToCalendar event={{ title: `Cita en ${appt.clinic.name}`, date: appt.date, start: appt.start, end: appt.end, location: appt.clinic.address, details: [appt.professional && `Atiende: ${appt.professional}`, appt.service && `Servicio: ${appt.service}`, appt.clinic.phone && `Teléfono: ${appt.clinic.phone}`].filter(Boolean).join('\n') }} />
      {/if}
      {#if appt.rebook_slug && !appt.past && (appt.status === 'scheduled' || appt.status === 'confirmed' || appt.status === 'cancelled')}
        <a class="btn-secondary btn-lg" href="/{appt.rebook_slug}/reservar"><Icon name="calendar" size={18} />{appt.status === 'cancelled' ? t('appt.rebookNew') : t('appt.rebook')}</a>
        {#if appt.status !== 'cancelled'}<p class="hint !mt-0 text-center">{t('appt.rebookHint')}</p>{/if}
      {/if}
      {#if appt.can_cancel && !askCancel}
        <button class="btn-ghost btn-lg !text-app-danger" onclick={() => (askCancel = true)}>{t('appt.cancel')}</button>
      {:else if !appt.can_cancel && (appt.status === 'scheduled' || appt.status === 'confirmed') && !appt.past}
        <p class="rounded-xl bg-app-warning/12 px-3.5 py-3 text-sm">{t('appt.tooLate', { hours: appt.cancel_min_hours, phone: appt.clinic.phone ? t('appt.tooLatePhone', { phone: appt.clinic.phone }) : '' })}</p>
      {/if}
    </div>

    {#if askCancel && appt.can_cancel}
      <form class="mt-5 grid gap-3 rounded-xl border border-app-danger/30 p-4" onsubmit={(e) => { e.preventDefault(); cancel(); }}>
        <p class="font-semibold">{t('appt.sure')}</p>
        <p class="text-sm text-app-muted">{t('appt.sureText')}</p>
        <label class="label" for="ap-reason">{t('appt.reasonLabel')} <span class="font-normal text-app-muted">{t('booking.optional')}</span></label>
        <input id="ap-reason" class="field" bind:value={reason} maxlength="300" />
        <div class="flex flex-wrap gap-2">
          <button class="btn-danger" type="submit" disabled={act.phase === 'loading'}>{t('appt.yesCancel')}</button>
          <button class="btn-ghost" type="button" onclick={() => (askCancel = false)}>{t('appt.keep')}</button>
        </div>
      </form>
    {/if}

    {#if appt.reminders}
      <div class="mt-7 border-t border-app-ink/10 pt-5">
        {#if !askOptout}
          <button class="text-sm text-app-muted underline hover:text-app-ink" onclick={() => (askOptout = true)}>{t('appt.optoutLink')}</button>
        {:else}
          <p class="text-sm">{t('appt.optoutAsk')}</p>
          <div class="mt-3 flex flex-wrap gap-2">
            <button class="btn-secondary" onclick={optout} disabled={act.phase === 'loading'}>{t('appt.optoutYes')}</button>
            <button class="btn-ghost" onclick={() => (askOptout = false)}>{t('common.cancel')}</button>
          </div>
        {/if}
      </div>
    {/if}
  </section>
{/if}
