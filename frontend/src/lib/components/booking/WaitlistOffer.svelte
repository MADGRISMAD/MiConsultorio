<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import { onMount } from 'svelte';
  import { ApiError } from '$lib/api';
  import { waitlistApi } from '$lib/api/waitlist';
  import { Op } from '$lib/op.svelte';
  import type { PublicWaitlist } from '$lib/types/waitlist';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { fmtDay, t } from '$lib/i18n/index.svelte';

  let { token }: { token: string } = $props();

  let data = $state<PublicWaitlist | null>(null);
  let missing = $state(false);
  let result = $state<'' | 'accepted' | 'declined' | 'left'>('');
  let now = $state(Date.now());
  const load = new Op();
  const act = new Op();


  const left = $derived.by(() => {
    if (!data?.offer) return 0;
    return Math.max(0, Math.floor((new Date(data.offer.expires_at).getTime() - now) / 1000));
  });
  const clock = $derived(`${Math.floor(left / 60)}:${String(left % 60).padStart(2, '0')}`);

  onMount(() => {
    const t = setInterval(() => (now = Date.now()), 1000);
    load
      .run(async () => {
        data = await waitlistApi.view(token);
      })
      .then((ok) => {
        if (!ok) missing = load.message.length > 0;
      });
    return () => clearInterval(t);
  });

  async function respond(action: 'accept' | 'decline' | 'leave') {
    const ok = await act.run(async () => {
      data = await waitlistApi.act(token, action);
    });
    if (ok) result = action === 'accept' ? 'accepted' : action === 'decline' ? 'declined' : 'left';
    else if (/disponible|terminó|ya no/i.test(act.message)) {
      try {
        data = await waitlistApi.view(token);
      } catch (e) {
        if (e instanceof ApiError && e.status === 404) missing = true;
      }
    }
  }
</script>

{#if load.phase === 'loading' || (load.phase === 'idle' && !data)}
  <div class="card-empty" role="status">{t('common.loading')}</div>
{:else if !data}
  <section class="card px-6 py-9" role="alert">
    <div class="grid h-12 w-12 place-items-center rounded-full bg-app-ink/8 text-app-muted"><Icon name="clock" size={24} /></div>
    <h1 class="display mt-4 text-3xl leading-tight">{t('waitlist.missingTitle')}</h1>
    <p class="mt-3 text-app-muted">{t('waitlist.missingText')}</p>
  </section>
{:else}
  <header class="mb-5 mt-2">
    <p class="section-title flex items-center gap-1.5"><Icon name="clock-plus" size={14} />{t('waitlist.heading')}</p>
    <h1 class="display mt-2 text-[2rem] leading-[1.05] sm:text-[2.4rem]">{data.clinic.name}</h1>
  </header>

  <section class="card page-in px-5 py-6 sm:px-8" aria-live="polite">
    {#if result === 'accepted' || data.status === 'booked'}
      <div class="grid h-12 w-12 place-items-center rounded-full bg-app-accent/14 text-app-accent"><Icon name="check" size={26} /></div>
      <h2 class="display mt-4 text-3xl leading-tight">{t('waitlist.bookedTitle')}</h2>
      <p class="mt-3 text-app-muted">{t('waitlist.bookedText')}</p>
    {:else if result === 'declined'}
      <h2 class="display text-3xl leading-tight">{t('waitlist.declinedTitle')}</h2>
      <p class="mt-3 text-app-muted">{t('waitlist.declinedText')}</p>
      <button type="button" class="btn-secondary mt-5" onclick={() => respond('leave')} disabled={act.phase === 'loading'}>{t('waitlist.leaveInstead')}</button>
    {:else if result === 'left' || data.status === 'cancelled'}
      <h2 class="display text-3xl leading-tight">{t('waitlist.leftTitle')}</h2>
      <p class="mt-3 text-app-muted">{t('waitlist.leftText')}</p>
    {:else if data.status === 'expired'}
      <h2 class="display text-3xl leading-tight">{t('waitlist.expiredTitle')}</h2>
      <p class="mt-3 text-app-muted">{t('waitlist.expiredText')}</p>
    {:else if data.offer}
      <h2 class="display text-3xl leading-tight">{t('waitlist.offerTitle')}</h2>
      <dl class="mt-5 grid gap-2 rounded-xl bg-app-elevated p-4 text-sm">
        <div><dt class="text-xs text-app-muted">{t('common.date')}</dt><dd class="font-semibold first-letter:uppercase">{fmtDay(data.offer.date)}</dd></div>
        <div><dt class="text-xs text-app-muted">{t('common.time')}</dt><dd class="font-semibold">{t('common.hourSuffix', { time: data.offer.start })}</dd></div>
        {#if data.offer.professional}<div><dt class="text-xs text-app-muted">{t('common.attends')}</dt><dd class="font-semibold">{data.offer.professional}</dd></div>{/if}
        {#if data.clinic.address}<div><dt class="text-xs text-app-muted">{t('common.address')}</dt><dd class="font-semibold">{data.clinic.address}</dd></div>{/if}
      </dl>
      <p class="mt-4 text-sm text-app-muted">{t('waitlist.holdBefore')}<span class="font-mono font-semibold text-app-ink" role="timer">{clock}</span>{t('waitlist.holdAfter')}</p>
      <OpError op={act} class="mt-4" />
      <div class="mt-5 flex flex-wrap gap-2">
        <button type="button" class="btn-primary btn-lg" onclick={() => respond('accept')} disabled={act.phase === 'loading' || left === 0}>
          {#if act.phase === 'loading'}<span class="spin"></span>{/if}{t('waitlist.accept')}
        </button>
        <button type="button" class="btn-secondary btn-lg" onclick={() => respond('decline')} disabled={act.phase === 'loading'}>{t('waitlist.decline')}</button>
      </div>
    {:else}
      <h2 class="display text-3xl leading-tight">{data.expired ? t('waitlist.timeUpTitle') : t('waitlist.stillTitle', { name: data.name ? ', ' + data.name : '' })}</h2>
      <p class="mt-3 text-app-muted">
        {data.expired ? t('waitlist.timeUpText') : t('waitlist.stillText')}
      </p>
      <OpError op={act} class="mt-4" />
      <button type="button" class="btn-secondary mt-5" onclick={() => respond('leave')} disabled={act.phase === 'loading'}>{t('waitlist.leave')}</button>
    {/if}
    {#if data.clinic.phone}<p class="mt-5 text-sm text-app-muted">{t('waitlist.questions')} <a class="text-app-primary underline" href="tel:{data.clinic.phone}">{data.clinic.phone}</a>.</p>{/if}
  </section>
{/if}
