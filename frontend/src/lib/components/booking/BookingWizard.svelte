<script lang="ts">
  import { emailOk } from '$lib/format';
  import { bookingApi } from '$lib/api/booking';
  import { bookingMonthApi } from '$lib/api/waitlist';
  import { Op } from '$lib/op.svelte';
  import { fmtDay, fmtMoneyCents, t } from '$lib/i18n/index.svelte';
  import { CLINIC_KINDS } from '$lib/types';
  import type { BookingInfo, BookingResult, BookingSlot } from '$lib/types/booking';
  import Icon from '$lib/components/ui/Icon.svelte';
  import PrivacyModal from './PrivacyModal.svelte';
  let privacyOpen = $state(false);
  import DatePicker from './DatePicker.svelte';
  import WaitlistJoin from './WaitlistJoin.svelte';

  let { slug, info }: { slug: string; info: BookingInfo } = $props();

  const kind = $derived(CLINIC_KINDS[info.clinic.kind as keyof typeof CLINIC_KINDS]);

  // who the visit is for: clinics that see both people and pets let the visitor choose
  const bothKinds = $derived(!!info.animals && !!info.people);
  let forPet = $state(!!info.animals && !info.people);
  let serviceId = $state('');
  let professionalId = $state('');
  /** the area the patient chose (only asked when the clinic has several) */
  let area = $state('');
  let date = $state('');
  /** the free times of each specialist on the picked date (null while loading) */
  let day = $state<{ id: string; name: string; slots: BookingSlot[] }[] | null>(null);
  let start = $state('');
  let registered = $state(false);
  let names = $state('');
  let phone = $state('');
  let email = $state('');
  let reason = $state('');
  let species = $state('');
  let petName = $state('');
  let pets = $state<{ id: string; name: string; professional_id: string }[] | null>(null);
  // a registered person: did the phone match a record, are there several, and who has been attending them
  let personFound = $state<boolean | null>(null);
  let personPro = $state('');
  let petId = $state('');
  let needName = $state(false);
  let acceptPrivacy = $state(false);
  let acceptReminders = $state(false);
  let website = $state(''); // honeypot
  let held = $state(false);
  let done = $state<BookingResult | null>(null);
  let monthKey = $state('');
  let available = $state<string[] | null>(null); // days of the shown month with a free slot; null while loading
  let monthSeq = 0;

  // the key under which the picked time is held for this visitor
  const holder = Array.from(crypto.getRandomValues(new Uint8Array(18)), (n) => n.toString(36).padStart(2, '0')).join('').slice(0, 32);

  const dayOp = new Op();
  const holdOp = new Op();
  const lookupOp = new Op();
  const op = new Op();
  let seq = 0;

  const areas = $derived(info.areas ?? []);
  /** the professionals of the chosen area (those without areas attend in all) */
  const pros = $derived(info.professionals.filter((p) => !area || !p.areas?.length || p.areas.includes(area)));
  const professional = $derived(info.professionals.find((p) => p.id === professionalId));
  /** a patient already in treatment sees only the agenda of the professional who has been attending them */
  const lockedPro = $derived(registered ? (forPet ? (pets?.find((x) => x.id === petId)?.professional_id ?? '') : personPro) : '');
  const lockedName = $derived(info.professionals.find((p) => p.id === lockedPro)?.name ?? '');
  const phoneDigits = $derived(phone.replace(/\D/g, ''));
  let lookedPhone = '';
  $effect(() => {
    // registered people: look the phone up once it is complete (pets use the button)
    if (!registered || forPet || phoneDigits.length < 10 || phoneDigits === lookedPhone) return;
    lookedPhone = phoneDigits;
    personFound = null;
    bookingApi
      .lookup(slug, phone.trim())
      .then((r) => {
        if (lookedPhone !== phoneDigits) return;
        personFound = r.person;
        personPro = r.professional_id;
        if (r.several) needName = true;
      })
      .catch(() => (personFound = null));
  });
  const kindLabel = (k: string) => (k in CLINIC_KINDS ? t(`kind.${k}`) : t('kind.fallback'));
  /** the visitor's details come first: the calendar opens once they are complete */
  const dataReady = $derived.by(() => {
    if (registered) return forPet ? !!petId : personFound === true && (!needName || !!names.trim());
    return !!names.trim() && !!phone.trim() && emailOk(email) && (!forPet || !!species);
  });
  function dataError(): string {
    if (!phone.trim()) return t('booking.errPhone');
    if (registered) return forPet && !petId ? t('booking.errPickPet') : personFound === false ? t('booking.noRecord') : t('booking.nameToo');
    if (!names.trim()) return t('booking.errName');
    if (!emailOk(email)) return t('booking.errEmail');
    return t('booking.errPet');
  }
  const anySlot = $derived(!!day && day.some((p) => p.slots.length > 0));

  /** every specialist's times for the date, in parallel: the ones with none show as not available */
  async function loadDay() {
    if (!date) return;
    const mine = ++seq;
    day = null;
    start = '';
    professionalId = '';
    held = false;
    await dayOp.run(async () => {
      const r = await Promise.all(
        pros.filter((p) => !lockedPro || p.id === lockedPro).map(async (p) => ({ id: p.id, name: p.name, slots: await bookingApi.slots(slug, p.id, date, serviceId, holder).catch(() => [] as BookingSlot[]) }))
      );
      if (mine === seq) day = r;
    });
  }

  // Days without room for anybody are disabled in the calendar: one request per month, not per day.
  $effect(() => {
    const ym = monthKey;
    const svc = serviceId;
    const only = lockedPro;
    const ar = area;
    if (!ym) {
      available = null;
      return;
    }
    const mine = ++monthSeq;
    available = null;
    bookingMonthApi
      .month(slug, ym, only, svc, ar)
      .then((days) => {
        if (mine === monthSeq) available = days;
      })
      .catch(() => {
        if (mine === monthSeq) available = null; // without it the picker simply allows every day
      });
  });

  function pickDate(d: string) {
    date = d;
    loadDay();
  }

  /** choosing a time holds it, so nobody else can fill in the form for the same one */
  async function pickSlot(proId: string, st: string) {
    professionalId = proId;
    start = st;
    held = false;
    holdOp.reset();
    try {
      await holdOp.run(() => bookingApi.hold(slug, { professional_id: proId, service_id: serviceId || undefined, date, start: st, holder }));
      held = true;
    } catch {
      /* Op shows the error */
    }
    if (holdOp.phase === 'error') {
      start = '';
      professionalId = '';
      loadDay();
    }
  }

  function clearPick() {
    start = '';
    professionalId = '';
    held = false;
  }

  async function findPets() {
    pets = null;
    petId = '';
    if (!phone.trim()) return lookupOp.fail(t('booking.errPhone'));
    try {
      await lookupOp.run(async () => {
        pets = (await bookingApi.lookup(slug, phone.trim())).pets;
        if (pets.length === 1) petId = pets[0].id;
      });
    } catch {
      /* Op shows the error */
    }
  }

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!dataReady) return op.fail(dataError());
    if (!professionalId || !date || !start) return op.fail(t('booking.errChoose'));
    if (!acceptPrivacy) return op.fail(t('booking.errPrivacy'));
    try {
      await op.run(async () => {
        done = await bookingApi.book(slug, {
          professional_id: professionalId,
          service_id: serviceId || undefined,
          date,
          start,
          names: names.trim(),
          last_names: '',
          phone: phone.trim(),
          email: email.trim(),
          reason: reason.trim(),
          accept_privacy: acceptPrivacy,
          accept_reminders: acceptReminders,
          website,
          holder,
          registered,
          animal: forPet,
          ...(registered && forPet ? { patient_id: petId } : {}),
          ...(!registered && forPet ? { species, pet_name: petName.trim() } : {})
        });
      });
    } catch {
      /* Op shows the error */
    }
    if (op.phase === 'error') {
      if (/varias personas/i.test(op.message)) needName = true;
      if (/horario/i.test(op.message)) loadDay(); // somebody else took it
    }
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
      <div><dt class="text-xs text-app-muted">{t('common.date')}</dt><dd class="font-semibold first-letter:uppercase">{fmtDay(done.date)}</dd></div>
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
    <section class="card px-5 py-5 sm:px-6" aria-labelledby="bk-s1">
      <h2 id="bk-s1" class="flex items-center gap-2.5 font-semibold"><span class={stepNo(1)}>1</span>{t('booking.step1')}</h2>
      {#if bothKinds}
        <fieldset class="mt-4">
          <legend class="label">{t('booking.whoFor')}</legend>
          <div class="grid gap-2 sm:grid-cols-2">
            {#each [[false, t('booking.forMe')], [true, t('booking.forPet')]] as [v, label]}
              <label class="flex min-h-11 cursor-pointer items-center gap-3 rounded-xl border border-app-ink/12 px-3.5 py-2 text-sm has-[:checked]:border-app-primary has-[:checked]:bg-app-primary/8 has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-app-primary/50">
                <input type="radio" class="sr-only" name="who" checked={forPet === v} onchange={() => { forPet = v as boolean; registered = false; pets = null; petId = ''; clearPick(); }} />
                <span>{label}</span>
              </label>
            {/each}
          </div>
        </fieldset>
      {/if}

      <div class="mt-4 grid grid-cols-2 gap-2">
        {#each [[false, forPet ? t('booking.newClient') : t('booking.newPatient')], [true, forPet ? t('booking.registeredClient') : t('booking.registered')]] as [v, label]}
          <label class="flex min-h-11 cursor-pointer items-center justify-center rounded-xl border border-app-ink/12 px-3 py-2 text-center text-sm font-medium has-[:checked]:border-app-primary has-[:checked]:bg-app-primary/8 has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-app-primary/50">
            <input type="radio" class="sr-only" name="reg" checked={registered === v} onchange={() => { registered = v as boolean; pets = null; petId = ''; needName = false; personFound = null; personPro = ''; lookedPhone = ''; clearPick(); day = null; op.reset(); }} />{label}
          </label>
        {/each}
      </div>

      {#if registered}
        <p class="hint mt-3">{forPet ? t('booking.registeredPetHint') : t('booking.registeredHint')}</p>
        <div class="mt-3 grid gap-4 sm:grid-cols-2">
          <div class={forPet ? '' : 'sm:col-span-2'}>
            <label class="label" for="bk-phone">{t('booking.phone')}</label>
            <input id="bk-phone" class="field" type="tel" inputmode="tel" bind:value={phone} autocomplete="tel" placeholder="55 1234 5678" oninput={() => { pets = null; petId = ''; personFound = null; personPro = ''; }} />
          </div>
          {#if forPet}
            <div class="flex items-end"><button type="button" class="btn-secondary w-full" disabled={lookupOp.phase === 'loading'} onclick={findPets}>{#if lookupOp.phase === 'loading'}<span class="spin"></span>{/if}{t('booking.findPets')}</button></div>
            {#if lookupOp.phase === 'error'}<p class="alert sm:col-span-2" role="alert"><Icon name="alert" size={18} />{lookupOp.message}</p>{/if}
            {#if pets && pets.length === 0}<p class="rounded-xl bg-app-elevated px-4 py-3 text-sm text-app-muted sm:col-span-2">{t('booking.noPets')}</p>{/if}
            {#if pets && pets.length > 0}
              <fieldset class="sm:col-span-2">
                <legend class="label">{t('booking.pickPet')}</legend>
                <div class="grid gap-2 sm:grid-cols-2">
                  {#each pets as pet (pet.id)}
                    <label class="flex min-h-11 cursor-pointer items-center gap-3 rounded-xl border border-app-ink/12 px-3.5 py-2 text-sm has-[:checked]:border-app-primary has-[:checked]:bg-app-primary/8 has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-app-primary/50">
                      <input type="radio" class="sr-only" name="pet" value={pet.id} bind:group={petId} onchange={() => { clearPick(); if (date) loadDay(); }} /><Icon name="paw" size={16} class="flex-none text-app-muted" /><span>{pet.name}</span>
                    </label>
                  {/each}
                </div>
              </fieldset>
            {/if}
          {/if}
          {#if !forPet && personFound === false}<p class="rounded-xl bg-app-elevated px-4 py-3 text-sm text-app-muted sm:col-span-2">{t('booking.noRecord')}</p>{/if}
          {#if needName}
            <p class="hint !mt-0 sm:col-span-2">{t('booking.nameToo')}</p>
            <div class="sm:col-span-2"><label class="label" for="bk-names">{t('booking.fullName')}</label><input id="bk-names" class="field" bind:value={names} autocomplete="name" maxlength="100" /></div>
          {/if}
        </div>
      {:else}
        <div class="mt-4 grid gap-4 sm:grid-cols-2">
          <div class="sm:col-span-2">
            <label class="label" for="bk-names">{forPet ? t('booking.ownerNames') : t('booking.fullName')}</label>
            <input id="bk-names" class="field" bind:value={names} autocomplete="name" required maxlength="100" />
          </div>
          <div>
            <label class="label" for="bk-phone">{t('booking.phone')}</label>
            <input id="bk-phone" class="field" type="tel" inputmode="tel" bind:value={phone} autocomplete="tel" placeholder="55 1234 5678" required />
          </div>
          <div>
            <label class="label" for="bk-email">{t('booking.email')}</label>
            <input id="bk-email" class="field" type="email" bind:value={email} autocomplete="email" autocapitalize="none" placeholder="tu@correo.com" required />
          </div>
          {#if forPet}
            <div>
              <label class="label" for="bk-species">{t('booking.petKind')}</label>
              <select id="bk-species" class="field" bind:value={species}>
                <option value="">{t('booking.petKindPick')}</option>
                {#each info.species ?? [] as sp}<option>{sp}</option>{/each}
              </select>
            </div>
            <div>
              <label class="label" for="bk-pet">{t('booking.petName')} <span class="font-normal text-app-muted">{t('booking.optional')}</span></label>
              <input id="bk-pet" class="field" bind:value={petName} maxlength="80" />
            </div>
          {/if}
        </div>
      {/if}
      <!-- honeypot: invisible to people -->
      <div class="absolute -left-[9999px]" aria-hidden="true">
        <label for="bk-web">{t('common.website')}</label>
        <input id="bk-web" tabindex="-1" autocomplete="off" bind:value={website} />
      </div>
    </section>

    <section class="card px-5 py-5 sm:px-6 {dataReady ? '' : 'opacity-60'}" aria-labelledby="bk-s2">
      <h2 id="bk-s2" class="flex items-center gap-2.5 font-semibold"><span class={stepNo(2)}>2</span>{t('booking.step2')}</h2>
      {#if !dataReady}
        <p class="mt-3 text-sm text-app-muted">{t('booking.fillFirst')}</p>
      {:else}
        {#if areas.length > 0}
          <fieldset class="mt-4">
            <legend class="label">{t('booking.area')}</legend>
            <div class="grid gap-2 sm:grid-cols-2">
              {#each areas as a (a.id)}
                <label class="flex min-h-11 cursor-pointer items-center gap-3 rounded-xl border border-app-ink/12 px-3.5 py-2 text-sm has-[:checked]:border-app-primary has-[:checked]:bg-app-primary/8 has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-app-primary/50">
                  <input type="radio" class="sr-only" name="area" value={a.id} bind:group={area} onchange={loadDay} />
                  <span>{a.label}</span>
                </label>
              {/each}
            </div>
          </fieldset>
        {/if}
        {#if areas.length > 0 && !area}
          <p class="mt-4 text-sm text-app-muted">{t('booking.pickArea')}</p>
        {:else}
        {#if info.services.length > 0}
          <fieldset class="mt-4">
            <legend class="label">{t('booking.service')} <span class="font-normal text-app-muted">{t('booking.optional')}</span></legend>
            <div class="grid gap-2">
              <label class="flex min-h-11 cursor-pointer items-center gap-3 rounded-xl border border-app-ink/12 px-3.5 py-2 text-sm has-[:checked]:border-app-primary has-[:checked]:bg-app-primary/8 has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-app-primary/50">
                <input type="radio" class="sr-only" name="svc" value="" bind:group={serviceId} onchange={loadDay} />
                <span>{t('booking.firstTime')}</span>
              </label>
              {#each info.services as s (s.id)}
                <label class="flex min-h-11 cursor-pointer items-center justify-between gap-3 rounded-xl border border-app-ink/12 px-3.5 py-2 text-sm has-[:checked]:border-app-primary has-[:checked]:bg-app-primary/8 has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-app-primary/50">
                  <input type="radio" class="sr-only" name="svc" value={s.id} bind:group={serviceId} onchange={loadDay} />
                  <span>{s.name}{#if s.duration_minutes} <span class="text-app-muted">· {t('booking.minutes', { n: s.duration_minutes })}</span>{/if}</span>
                  {#if s.price_cents !== undefined}<span class="font-mono text-xs text-app-muted">{fmtMoneyCents(s.price_cents)}</span>{/if}
                </label>
              {/each}
            </div>
          </fieldset>
        {/if}
        {#if lockedPro && lockedName}<p class="mt-4 rounded-xl bg-app-primary/8 px-4 py-3 text-sm">{t('booking.followUp', { name: lockedName })}</p>{/if}
        <div class="mt-4"><DatePicker min={info.today} horizon={info.horizon_days} value={date} onpick={pickDate} {available} onmonth={(m) => (monthKey = m)} /></div>
        {#if available && available.length === 0 && !date}
          <p class="mt-3 rounded-xl bg-app-elevated px-4 py-3 text-sm text-app-muted">{t('booking.noMonth')}</p>
        {:else if !date}
          <p class="mt-3 text-sm text-app-muted">{t('booking.pickDate')}</p>
        {/if}
        {#if date}
          <div class="mt-4" aria-live="polite">
            <p class="label first-letter:uppercase">{fmtDay(date)}</p>
            {#if dayOp.phase === 'loading' || (!day && dayOp.phase !== 'error')}
              <p class="text-sm text-app-muted">{t('booking.searching')}</p>
            {:else if dayOp.phase === 'error'}
              <p class="alert" role="alert"><Icon name="alert" size={18} />{dayOp.message}</p>
            {:else if day}
              {#if !anySlot}<p class="mb-3 rounded-xl bg-app-elevated px-4 py-3 text-sm text-app-muted">{t('booking.noneThatDay')}</p>{/if}
              <p class="mb-2 text-xs font-semibold uppercase tracking-wide text-app-muted">{t('booking.specialists')}</p>
              <ul class="grid gap-3">
                {#each day as p (p.id)}
                  <li class="rounded-xl border border-app-ink/10 p-3 {p.slots.length === 0 ? 'opacity-60' : ''}">
                    <p class="flex items-center gap-2 text-sm font-semibold"><Icon name="user" size={16} class="flex-none text-app-muted" />{p.name}
                      {#if p.slots.length === 0}<span class="ml-auto text-xs font-normal text-app-muted">{t('booking.unavailable')}</span>{/if}</p>
                    {#if p.slots.length > 0}
                      <div class="mt-2 grid grid-cols-3 gap-2 sm:grid-cols-4" role="radiogroup" aria-label="{t('booking.slotsLabel')} · {p.name}">
                        {#each p.slots as s (s.start)}
                          <label class="grid min-h-11 cursor-pointer place-items-center rounded-xl border border-app-ink/12 text-sm font-medium has-[:checked]:border-app-ink has-[:checked]:bg-app-ink has-[:checked]:text-app-surface has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-app-primary/60">
                            <input type="radio" class="sr-only" name="slot" checked={professionalId === p.id && start === s.start} onchange={() => pickSlot(p.id, s.start)} />{s.start}
                          </label>
                        {/each}
                      </div>
                    {/if}
                  </li>
                {/each}
              </ul>
              {#if holdOp.phase === 'error'}<p class="alert mt-3" role="alert"><Icon name="alert" size={18} />{holdOp.message}</p>{/if}
            {/if}
          </div>
        {/if}
        {#if pros.length === 1 && ((available && available.length === 0) || (day && !anySlot))}
          <WaitlistJoin {slug} clinicName={info.clinic.name} professionalId={pros[0].id} {serviceId} />
        {/if}
        {/if}
      {/if}
    </section>

    {#if start && dataReady}
      <section class="card page-in px-5 py-5 sm:px-6" aria-labelledby="bk-s3">
        <h2 id="bk-s3" class="flex items-center gap-2.5 font-semibold"><span class={stepNo(3)}>3</span>{t('booking.step3')}</h2>
        <p class="mt-2 text-sm text-app-muted">
          {professional?.name ?? ''} · <span class="inline-block first-letter:uppercase">{fmtDay(date)}</span> · {t('common.hourSuffix', { time: start })}. {t('booking.dataHint')}
        </p>
        {#if held}<p class="hint mt-1">{t('booking.held')}</p>{/if}
        {#if op.phase === 'error'}<p class="alert mt-4" role="alert"><Icon name="alert" size={18} />{op.message}</p>{/if}
        <div class="mt-4">
          <label class="label" for="bk-reason">{t('booking.reason')} <span class="font-normal text-app-muted">{t('booking.reasonHint')}</span></label>
          <input id="bk-reason" class="field" bind:value={reason} maxlength="300" placeholder={t('booking.reasonPlaceholder')} />
        </div>

        <div class="mt-5 grid gap-3 text-sm">
          <label class="flex cursor-pointer items-start gap-3">
            <input type="checkbox" class="mt-0.5 h-5 w-5 flex-none accent-[rgb(var(--app-primary))]" bind:checked={acceptPrivacy} required />
            <span>{t('booking.privacyBefore')}<button type="button" class="text-app-primary underline" onclick={(ev) => { ev.preventDefault(); ev.stopPropagation(); privacyOpen = true; }}>{t('common.privacyLink')}</button>{t('booking.privacyAfter', { clinic: info.clinic.name })}</span>
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

<PrivacyModal open={privacyOpen} onclose={() => (privacyOpen = false)} />
