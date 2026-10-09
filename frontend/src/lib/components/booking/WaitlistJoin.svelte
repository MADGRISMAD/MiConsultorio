<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import { waitlistApi } from '$lib/api/waitlist';
  import { Op } from '$lib/op.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import PrivacyModal from './PrivacyModal.svelte';
  let privacyOpen = $state(false);
  import { t } from '$lib/i18n/index.svelte';

  interface Props {
    slug: string;
    clinicName: string;
    professionalId: string;
    serviceId: string;
  }
  let { slug, clinicName, professionalId, serviceId }: Props = $props();

  const uid = $props.id();
  const DAYS = [1, 2, 3, 4, 5, 6, 0];

  let open = $state(false);
  let done = $state(false);
  let names = $state('');
  let lastNames = $state('');
  let email = $state('');
  let phone = $state('');
  let days = $state<number[]>([]);
  let fromTime = $state('');
  let toTime = $state('');
  let acceptPrivacy = $state(false);
  let acceptNotices = $state(false);
  let website = $state('');
  const op = new Op();

  const toggle = (d: number) => (days = days.includes(d) ? days.filter((x) => x !== d) : [...days, d]);

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!names.trim() || !lastNames.trim()) return op.fail(t('waitlist.join.errName'));
    if (!email.trim()) return op.fail(t('waitlist.join.errEmail'));
    if (!!fromTime !== !!toTime) return op.fail(t('waitlist.join.errTimes'));
    if (fromTime && toTime <= fromTime) return op.fail(t('waitlist.join.errOrder'));
    if (!acceptPrivacy) return op.fail(t('waitlist.join.errPrivacy'));
    if (!acceptNotices) return op.fail(t('waitlist.join.errNotices'));
    const ok = await op.run(() =>
      waitlistApi.join(slug, {
        names: names.trim(), last_names: lastNames.trim(), email: email.trim(), phone: phone.trim(),
        professional_id: professionalId || undefined, service_id: serviceId || undefined,
        days, from_time: fromTime, to_time: toTime, notes: '', accept_privacy: acceptPrivacy, accept_notices: acceptNotices, website
      })
    );
    if (ok) done = true;
  }
</script>

<div class="mt-4 rounded-2xl border border-app-primary/25 bg-app-primary/6 p-4 sm:p-5" aria-live="polite">
  {#if done}
    <p class="flex items-start gap-2 text-sm"><Icon name="check" size={18} class="mt-0.5 flex-none text-app-accent" /><span><strong>{t('waitlist.join.done')}</strong>{t('waitlist.join.doneText')}</span></p>
  {:else if !open}
    <p class="text-sm font-medium">{t('waitlist.join.none')}</p>
    <p class="mt-1 text-sm text-app-muted">{t('waitlist.join.noneText')}</p>
    <button type="button" class="btn-secondary mt-3" onclick={() => (open = true)}><Icon name="clock-plus" size={18} />{t('waitlist.join.open')}</button>
  {:else}
    <form class="grid gap-4" novalidate onsubmit={submit}>
      <p class="text-sm font-medium">{t('waitlist.join.heading', { clinic: clinicName })}</p>
      <OpError op={op} />
      <div class="grid gap-4 sm:grid-cols-2">
        <div><label class="label" for="{uid}-n">{t('common.firstNames')}</label><input id="{uid}-n" class="field" bind:value={names} autocomplete="given-name" maxlength="100" required /></div>
        <div><label class="label" for="{uid}-l">{t('common.lastNames')}</label><input id="{uid}-l" class="field" bind:value={lastNames} autocomplete="family-name" maxlength="100" required /></div>
        <div><label class="label" for="{uid}-e">{t('common.email')}</label><input id="{uid}-e" class="field" type="email" bind:value={email} autocomplete="email" autocapitalize="none" required /></div>
        <div><label class="label" for="{uid}-p">{t('common.phone')} <span class="font-normal text-app-muted">{t('booking.optional')}</span></label><input id="{uid}-p" class="field" type="tel" inputmode="tel" bind:value={phone} autocomplete="tel" /></div>
      </div>
      <fieldset>
        <legend class="label">{t('waitlist.join.days')} <span class="font-normal text-app-muted">{t('waitlist.join.daysHint')}</span></legend>
        <div class="flex flex-wrap gap-2">
          {#each DAYS as d (d)}
            <label class="grid min-h-10 min-w-12 cursor-pointer place-items-center rounded-xl border border-app-ink/12 px-3 text-sm font-medium has-[:checked]:border-app-ink has-[:checked]:bg-app-ink has-[:checked]:text-app-surface has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-app-primary/60">
              <input type="checkbox" class="sr-only" checked={days.includes(d)} onchange={() => toggle(d)} />{t(`waitlist.day.${d}`)}
            </label>
          {/each}
        </div>
      </fieldset>
      <div class="grid grid-cols-2 gap-4">
        <div><label class="label" for="{uid}-f">{t('waitlist.join.from')} <span class="font-normal text-app-muted">{t('booking.optional')}</span></label><input id="{uid}-f" class="field" type="time" bind:value={fromTime} /></div>
        <div><label class="label" for="{uid}-t">{t('waitlist.join.to')}</label><input id="{uid}-t" class="field" type="time" bind:value={toTime} /></div>
      </div>
      <div class="absolute -left-[9999px]" aria-hidden="true">
        <label for="{uid}-w">{t('common.website')}</label><input id="{uid}-w" tabindex="-1" autocomplete="off" bind:value={website} />
      </div>
      <div class="grid gap-3 text-sm">
        <label class="flex cursor-pointer items-start gap-3">
          <input type="checkbox" class="mt-0.5 h-5 w-5 flex-none accent-[rgb(var(--app-primary))]" bind:checked={acceptPrivacy} required />
          <span>{t('booking.privacyBefore')}<button type="button" class="text-app-primary underline" onclick={(ev) => { ev.preventDefault(); ev.stopPropagation(); privacyOpen = true; }}>{t('common.privacyLink')}</button>{t('waitlist.join.privacyAfter')}</span>
        </label>
        <label class="flex cursor-pointer items-start gap-3">
          <input type="checkbox" class="mt-0.5 h-5 w-5 flex-none accent-[rgb(var(--app-primary))]" bind:checked={acceptNotices} required />
          <span>{t('waitlist.join.notices', { clinic: clinicName })}</span>
        </label>
      </div>
      <div class="flex flex-wrap gap-2">
        <button class="btn-primary" type="submit" disabled={op.phase === 'loading'}>{#if op.phase === 'loading'}<span class="spin"></span>{/if}{t('waitlist.join.submit')}</button>
        <button class="btn-secondary" type="button" onclick={() => (open = false)}>{t('common.cancel')}</button>
      </div>
    </form>
  {/if}
</div>

<PrivacyModal open={privacyOpen} onclose={() => (privacyOpen = false)} />
