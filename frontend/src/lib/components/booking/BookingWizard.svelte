<script lang="ts">
  import { bookingApi } from '$lib/api/booking';
  import { bookingMonthApi } from '$lib/api/waitlist';
  import { Op } from '$lib/op.svelte';
  import { fmtDay, fmtMoneyCents, t } from '$lib/i18n/index.svelte';
  import { CLINIC_KINDS } from '$lib/types';
  import type { BookingInfo, BookingResult, BookingSlot } from '$lib/types/booking';
  import Icon from '$lib/components/ui/Icon.svelte';
  import DatePicker from './DatePicker.svelte';
  import WaitlistJoin from './WaitlistJoin.svelte';

  let { slug, info }: { slug: string; info: BookingInfo } = $props();

  const kind = $derived(CLINIC_KINDS[info.clinic.kind as keyof typeof CLINIC_KINDS]);

  let serviceId = $state('');
  let professionalId = $state(info.professionals.length === 1 ? info.professionals[0].id : '');
  let date = $state('');
  let slots = $state<BookingSlot[] | null>(null);
  let start = $state('');
  let names = $state('');
  let lastNames = $state('');
  let phone = $state('');
  let email = $state('');
  let reason = $state('');
  let acceptPrivacy = $state(false);
  let acceptReminders = $state(false);
  let website = $state(''); // honeypot
  let done = $state<BookingResult | null>(null);
  let monthKey = $state('');
  let available = $state<string[] | null>(null); // days of the shown month with a free slot; null while loading
  let monthSeq = 0;

  const slotsOp = new Op();
  const op = new Op();
  let seq = 0;

  const professional = $derived(info.professionals.find((p) => p.id === professionalId));
  const dateLong = (d: string) => fmtDay(d);
  const kindLabel = (k: string) => (k in CLINIC_KINDS ? t(`kind.${k}`) : t('kind.fallback'));

  async function loadSlots() {
    if (!professionalId || !date) return;
    const mine = ++seq;
    slots = null;
    start = '';
    await slotsOp.run(async () => {
      const r = await bookingApi.slots(slug, professionalId, date, serviceId);
      if (mine === seq) slots = r;
    });
  }

  // Days without room are disabled in the calendar: one request per month, not per day.
  $effect(() => {
    const ym = monthKey;
    const pro = professionalId;
    const svc = serviceId;
    if (!ym || !pro) {
      available = null;
      return;
    }
    const mine = ++monthSeq;
    available = null;
    bookingMonthApi
      .month(slug, ym, pro, svc)
      .then((days) => {
        if (mine === monthSeq) available = days;
      })
      .catch(() => {
        if (mine === monthSeq) available = null; // without it the picker simply allows every day
      });
  });

  function pickDate(d: string) {
    date = d;
    loadSlots();
  }

  function pickProfessional(id: string) {
    professionalId = id;
    start = '';
    loadSlots();
  }

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!professionalId || !date || !start) return op.fail(t('booking.errChoose'));
    if (!names.trim() || !lastNames.trim()) return op.fail(t('booking.errName'));
    if (!phone.trim() && !email.trim()) return op.fail(t('booking.errContact'));
    if (!acceptPrivacy) return op.fail(t('booking.errPrivacy'));
    try {
      await op.run(async () => {
        done = await bookingApi.book(slug, {
          professional_id: professionalId,
          service_id: serviceId || undefined,
          date,
          start,
          names: names.trim(),
          last_names: lastNames.trim(),
          phone: phone.trim(),
          email: email.trim(),
          reason: reason.trim(),
          accept_privacy: acceptPrivacy,
          accept_reminders: acceptReminders,
          website
        });
      });
    } catch {
      /* Op shows the error */
    }
    if (op.phase === 'error' && /horario/i.test(op.message)) loadSlots(); // somebody else took it
  }

  const stepNo = (n: number) =>
    'grid h-6 w-6 flex-none place-items-center rounded-full bg-app-ink font-mono text-xs text-app-surface' + (n ? '' : '');
</script>

<header class="mb-5 mt-2">
  <p class="section-title flex items-center gap-1.5">
    <Icon name={(kind?.icon ?? 'calendar') as 'calendar'} size={14} />{kind ? kindLabel(info.clinic.kind) : t('kind.fallback')}
  </p>
  <h1 class="display mt-2 text-[2.1rem] leading-[1.05] sm:text-[2.5rem]">{info.clinic.name}</h1>
  {#if info.clinic.address}<p class="mt-2 text-sm text-app-muted">{info.clinic.address}</p>{/if}
  {#if info.message}<p class="mt-3 rounded-xl bg-app-primary/8 px-4 py-3 text-sm">{info.message}</p>{/if}
</header>

{#if done}
  <section class="card page-in px-5 py-7 sm:px-8" aria-live="polite">
    <div class="grid h-12 w-12 place-items-center rounded-full bg-app-accent/14 text-app-accent"><Icon name="check" size={26} /></div>
    <h2 class="display mt-4 text-3xl leading-tight">{done.pending_confirmation ? t('booking.doneRequested') : t('booking.doneBooked')}</h2>
    <p class="mt-3 text-app-muted">
      {#if done.pending_confirmation}{t('booking.doneRequestedText')}{:else}{t('booking.doneBookedText')}{/if}
    </p>
    <dl class="mt-5 grid gap-2 rounded-xl bg-app-elevated p-4 text-sm">
      <div><dt class="text-xs text-app-muted">{t('common.date')}</dt><dd class="font-semibold first-letter:uppercase">{dateLong(done.date)}</dd></div>
      <div><dt class="text-xs text-app-muted">{t('common.time')}</dt><dd class="font-semibold">{t('common.hourSuffix', { time: done.start })}</dd></div>
      <div><dt class="text-xs text-app-muted">{t('common.attends')}</dt><dd class="font-semibold">{done.professional}</dd></div>
      <div><dt class="text-xs text-app-muted">{t('booking.clinic')}</dt><dd class="font-semibold">{done.clinic}</dd></div>
    </dl>
    <p class="mt-4 text-sm text-app-muted">{t('booking.saveLink')}{email ? t('booking.saveLinkEmail') : ''}.</p>
    <a href="/cita/{done.token}" class="btn-primary btn-lg mt-5">{t('booking.manage')}</a>
  </section>
{:else if info.professionals.length === 0}
  <section class="card px-5 py-8 text-center">
    <p class="font-medium">{t('booking.noSlots')}</p>
    {#if info.clinic.phone}<p class="mt-2 text-sm text-app-muted">{t('booking.callUs')} <a class="text-app-primary underline" href="tel:{info.clinic.phone}">{info.clinic.phone}</a>.</p>{/if}
  </section>
{:else}
  <form class="grid gap-5" novalidate onsubmit={submit}>
    {#if info.services.length > 0 || info.professionals.length > 1}
      <section class="card px-5 py-5 sm:px-6" aria-labelledby="bk-s1">
        <h2 id="bk-s1" class="flex items-center gap-2.5 font-semibold"><span class={stepNo(1)}>1</span>{t('booking.step1')}</h2>
        {#if info.services.length > 0}
          <fieldset class="mt-4">
            <legend class="label">{t('booking.service')} <span class="font-normal text-app-muted">{t('booking.optional')}</span></legend>
            <div class="grid gap-2">
              <label class="flex min-h-11 cursor-pointer items-center gap-3 rounded-xl border border-app-ink/12 px-3.5 py-2 text-sm has-[:checked]:border-app-primary has-[:checked]:bg-app-primary/8 has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-app-primary/50">
                <input type="radio" class="sr-only" name="svc" value="" bind:group={serviceId} onchange={loadSlots} />
                <span>{t('booking.firstTime')}</span>
              </label>
              {#each info.services as s (s.id)}
                <label class="flex min-h-11 cursor-pointer items-center justify-between gap-3 rounded-xl border border-app-ink/12 px-3.5 py-2 text-sm has-[:checked]:border-app-primary has-[:checked]:bg-app-primary/8 has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-app-primary/50">
                  <input type="radio" class="sr-only" name="svc" value={s.id} bind:group={serviceId} onchange={loadSlots} />
                  <span>{s.name}{#if s.duration_minutes} <span class="text-app-muted">· {t('booking.minutes', { n: s.duration_minutes })}</span>{/if}</span>
                  {#if s.price_cents !== undefined}<span class="font-mono text-xs text-app-muted">{fmtMoneyCents(s.price_cents)}</span>{/if}
                </label>
              {/each}
            </div>
          </fieldset>
        {/if}
        {#if info.professionals.length > 1}
          <fieldset class="mt-4">
            <legend class="label">{t('booking.professional')}</legend>
            <div class="grid gap-2 sm:grid-cols-2">
              {#each info.professionals as p (p.id)}
                <label class="flex min-h-11 cursor-pointer items-center gap-3 rounded-xl border border-app-ink/12 px-3.5 py-2 text-sm has-[:checked]:border-app-primary has-[:checked]:bg-app-primary/8 has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-app-primary/50">
                  <input type="radio" class="sr-only" name="pro" value={p.id} checked={professionalId === p.id} onchange={() => pickProfessional(p.id)} />
                  <Icon name="user" size={16} class="flex-none text-app-muted" /><span>{p.name}</span>
                </label>
              {/each}
            </div>
          </fieldset>
        {/if}
      </section>
    {/if}

    <section class="card px-5 py-5 sm:px-6" aria-labelledby="bk-s2">
      <h2 id="bk-s2" class="flex items-center gap-2.5 font-semibold"><span class={stepNo(2)}>2</span>{t('booking.step2')}</h2>
      {#if !professionalId}
        <p class="mt-3 text-sm text-app-muted">{t('booking.pickProfessional')}</p>
      {:else}
        <div class="mt-4"><DatePicker min={info.today} horizon={info.horizon_days} value={date} onpick={pickDate} {available} onmonth={(m) => (monthKey = m)} /></div>
        {#if available && available.length === 0 && !date}
          <p class="mt-3 rounded-xl bg-app-elevated px-4 py-3 text-sm text-app-muted">{t('booking.noMonth')}</p>
        {/if}
        {#if date}
          <div class="mt-4" aria-live="polite">
            <p class="label first-letter:uppercase">{dateLong(date)}</p>
            {#if slotsOp.phase === 'loading'}
              <p class="text-sm text-app-muted">{t('booking.searching')}</p>
            {:else if slotsOp.phase === 'error'}
              <p class="alert" role="alert"><Icon name="alert" size={18} />{slotsOp.message}</p>
            {:else if slots && slots.length === 0}
              <p class="rounded-xl bg-app-elevated px-4 py-3 text-sm text-app-muted">{t('booking.noDay')}</p>
            {:else if slots}
              <div class="grid grid-cols-3 gap-2 sm:grid-cols-4" role="radiogroup" aria-label={t('booking.slotsLabel')}>
                {#each slots as s (s.start)}
                  <label class="grid min-h-11 cursor-pointer place-items-center rounded-xl border border-app-ink/12 text-sm font-medium has-[:checked]:border-app-ink has-[:checked]:bg-app-ink has-[:checked]:text-app-surface has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-app-primary/60">
                    <input type="radio" class="sr-only" name="slot" value={s.start} bind:group={start} />{s.start}
                  </label>
                {/each}
              </div>
            {/if}
          </div>
        {/if}
        {#if (available && available.length === 0) || (slots && slots.length === 0)}
          <WaitlistJoin {slug} clinicName={info.clinic.name} {professionalId} {serviceId} />
        {/if}
      {/if}
    </section>

    {#if start}
      <section class="card page-in px-5 py-5 sm:px-6" aria-labelledby="bk-s3">
        <h2 id="bk-s3" class="flex items-center gap-2.5 font-semibold"><span class={stepNo(3)}>3</span>{t('booking.step3')}</h2>
        <p class="mt-2 text-sm text-app-muted">
          {professional?.name ?? ''} · <span class="inline-block first-letter:uppercase">{dateLong(date)}</span> · {t('common.hourSuffix', { time: start })}. {t('booking.dataHint')}
        </p>
        {#if op.phase === 'error'}<p class="alert mt-4" role="alert"><Icon name="alert" size={18} />{op.message}</p>{/if}
        <div class="mt-4 grid gap-4 sm:grid-cols-2">
          <div>
            <label class="label" for="bk-names">{t('common.firstNames')}</label>
            <input id="bk-names" class="field" bind:value={names} autocomplete="given-name" required maxlength="100" />
          </div>
          <div>
            <label class="label" for="bk-last">{t('common.lastNames')}</label>
            <input id="bk-last" class="field" bind:value={lastNames} autocomplete="family-name" required maxlength="100" />
          </div>
          <div>
            <label class="label" for="bk-phone">{t('booking.phone')}</label>
            <input id="bk-phone" class="field" type="tel" inputmode="tel" bind:value={phone} autocomplete="tel" placeholder="55 1234 5678" />
          </div>
          <div>
            <label class="label" for="bk-email">{t('booking.email')}</label>
            <input id="bk-email" class="field" type="email" bind:value={email} autocomplete="email" autocapitalize="none" placeholder="tu@correo.com" />
          </div>
          <p class="hint !mt-0 sm:col-span-2">{t('booking.contactHint')}</p>
          <div class="sm:col-span-2">
            <label class="label" for="bk-reason">{t('booking.reason')} <span class="font-normal text-app-muted">{t('booking.reasonHint')}</span></label>
            <input id="bk-reason" class="field" bind:value={reason} maxlength="300" placeholder={t('booking.reasonPlaceholder')} />
          </div>
          <!-- honeypot: invisible to people -->
          <div class="absolute -left-[9999px]" aria-hidden="true">
            <label for="bk-web">{t('common.website')}</label>
            <input id="bk-web" tabindex="-1" autocomplete="off" bind:value={website} />
          </div>
        </div>

        <div class="mt-5 grid gap-3 text-sm">
          <label class="flex cursor-pointer items-start gap-3">
            <input type="checkbox" class="mt-0.5 h-5 w-5 flex-none accent-[rgb(var(--app-primary))]" bind:checked={acceptPrivacy} required />
            <span>{t('booking.privacyBefore')}<a href="/privacidad" target="_blank" rel="noopener" class="text-app-primary underline">{t('common.privacyLink')}</a>{t('booking.privacyAfter', { clinic: info.clinic.name })}</span>
          </label>
          <label class="flex cursor-pointer items-start gap-3">
            <input type="checkbox" class="mt-0.5 h-5 w-5 flex-none accent-[rgb(var(--app-primary))]" bind:checked={acceptReminders} />
            <span>{t('booking.reminders')} <span class="text-app-muted">{t('booking.optional')}</span></span>
          </label>
        </div>

        <button class="btn-primary btn-lg mt-6" type="submit" disabled={op.phase === 'loading'}>
          {#if op.phase === 'loading'}<span class="spin"></span>{t('booking.booking')}{:else}{info.requires_confirmation ? t('booking.request') : t('booking.book')}{/if}
        </button>
        {#if info.requires_confirmation}<p class="hint mt-3 text-center">{t('booking.willConfirm')}</p>{/if}
      </section>
    {/if}
  </form>
{/if}
