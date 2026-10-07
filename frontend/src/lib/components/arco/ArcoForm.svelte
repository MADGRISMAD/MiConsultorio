
<script lang="ts">
  import { arcoApi } from '$lib/api/arco';
  import { Op } from '$lib/op.svelte';
  import type { ArcoPublicInfo, ArcoPublicInput } from '$lib/types/arco';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { KINDS } from './labels';
  import { t } from '$lib/i18n/index.svelte';

  let { slug, info }: { slug: string; info: ArcoPublicInfo } = $props();

  let form = $state<ArcoPublicInput>({ kind: '', requester_name: '', requester_email: '', requester_phone: '', description: '', acknowledged: false, website: '' });
  let sent = $state<{ folio: string; path: string } | null>(null);
  let errors = $state<Record<string, string>>({});
  const op = new Op();

  const contact = $derived([info.clinic.privacy_contact, info.clinic.privacy_email, info.clinic.privacy_phone].filter(Boolean).join(' · '));

  function validate() {
    const e: Record<string, string> = {};
    if (!form.kind) e.kind = t('arco.form.errKind');
    if (form.requester_name.trim().length < 3) e.requester_name = t('arco.form.errName');
    if (!/^[^\s@<>]+@[^\s@<>]+\.[^\s@<>]+$/.test(form.requester_email.trim())) e.requester_email = t('arco.form.errEmail');
    if (form.description.trim().length < 10) e.description = t('arco.form.errDescription');
    if (!form.acknowledged) e.acknowledged = t('arco.form.errAck');
    errors = e;
    return Object.keys(e).length === 0;
  }

  async function submit(ev: SubmitEvent) {
    ev.preventDefault();
    if (!validate()) {
      document.getElementById(`arco-${Object.keys(errors)[0]}`)?.focus();
      return;
    }
    await op.run(async () => {
      const r = await arcoApi.publicSubmit(slug, $state.snapshot(form) as ArcoPublicInput);
      sent = { folio: r.folio, path: r.status_path };
    });
  }
</script>

{#if sent}
  <section class="card px-6 py-9 sm:px-9" aria-live="polite">
    <span class="grid h-12 w-12 place-items-center rounded-full bg-app-accent/15 text-app-accent"><Icon name="check" size={24} stroke={2.2} /></span>
    <h1 class="display mt-4 text-4xl leading-tight">{t('arco.form.sentTitle')}</h1>
    <p class="mt-3 text-app-muted">{t('arco.form.folioIntro')}</p>
    <p class="mt-1 font-mono text-2xl font-semibold tracking-wide" data-testid="arco-folio">{sent.folio}</p>
    <p class="mt-4 text-app-muted">
      {t('arco.form.sentText', { clinic: info.clinic.name })}
    </p>
    <a href={sent.path} class="btn-primary mt-6">{t('arco.form.checkStatus')}</a>
    <p class="hint mt-3">{t('arco.form.keepFolio')}</p>
  </section>
{:else}
  <form class="card px-6 py-8 sm:px-9" onsubmit={submit} novalidate aria-labelledby="arco-title">
    <div class="flex items-center gap-3">
      <span class="grid h-11 w-11 flex-none place-items-center rounded-2xl bg-app-primary/10 text-app-primary"><Icon name="shield" size={22} /></span>
      <p class="section-title">{info.clinic.name}</p>
    </div>
    <h1 id="arco-title" class="display mt-4 text-4xl leading-tight sm:text-5xl">{t('arco.form.title')}</h1>
    <p class="mt-3 text-[15px] text-app-muted">
      {t('arco.form.introBefore')}<strong class="text-app-ink">{t('arco.form.introStrong')}</strong>{t('arco.form.introAfter')}
    </p>
    {#if contact}<p class="mt-2 text-sm text-app-muted">{t('arco.form.contact', { contact })}</p>{/if}

    <fieldset class="mt-6">
      <legend class="label">{t('arco.form.legend')}</legend>
      <div class="grid gap-2" role="radiogroup" aria-describedby={errors.kind ? 'arco-kind-err' : undefined}>
        {#each KINDS as k (k)}
          <label class="flex cursor-pointer items-start gap-3 rounded-xl border border-app-ink/15 p-3 transition hover:border-app-ink/30 has-[:checked]:border-app-primary has-[:checked]:bg-app-primary/5 has-[:focus-visible]:ring-4 has-[:focus-visible]:ring-app-primary/15">
            <input id={k === 'acceso' ? 'arco-kind' : undefined} type="radio" name="kind" value={k} bind:group={form.kind} class="mt-1 h-4 w-4 flex-none accent-[rgb(var(--app-primary))]" />
            <span><span class="block text-[15px] font-semibold">{t(`arco.kind.${k}`)}</span><span class="block text-sm text-app-muted">{t(`arco.kindHelp.${k}`)}</span></span>
          </label>
        {/each}
      </div>
      {#if errors.kind}<p id="arco-kind-err" class="mt-1.5 text-sm text-app-danger" role="alert">{errors.kind}</p>{/if}
    </fieldset>

    <div class="mt-6 grid gap-4 sm:grid-cols-2">
      <div class="sm:col-span-2">
        <label class="label" for="arco-requester_name">{t('arco.form.name')}</label>
        <input id="arco-requester_name" class="field" autocomplete="name" maxlength="120" bind:value={form.requester_name} aria-invalid={!!errors.requester_name} aria-describedby={errors.requester_name ? 'arco-name-err' : undefined} />
        {#if errors.requester_name}<p id="arco-name-err" class="mt-1.5 text-sm text-app-danger" role="alert">{errors.requester_name}</p>{/if}
      </div>
      <div>
        <label class="label" for="arco-requester_email">{t('arco.form.email')}</label>
        <input id="arco-requester_email" class="field" type="email" autocomplete="email" inputmode="email" maxlength="254" bind:value={form.requester_email} aria-invalid={!!errors.requester_email} aria-describedby={errors.requester_email ? 'arco-email-err' : undefined} />
        {#if errors.requester_email}<p id="arco-email-err" class="mt-1.5 text-sm text-app-danger" role="alert">{errors.requester_email}</p>{/if}
      </div>
      <div>
        <label class="label" for="arco-requester_phone">{t('arco.form.phone')} <span class="font-normal text-app-muted">{t('booking.optional')}</span></label>
        <input id="arco-requester_phone" class="field" type="tel" autocomplete="tel" inputmode="tel" maxlength="30" bind:value={form.requester_phone} />
      </div>
      <div class="sm:col-span-2">
        <label class="label" for="arco-description">{t('arco.form.description')}</label>
        <textarea id="arco-description" class="field" rows="5" maxlength="3000" bind:value={form.description} aria-invalid={!!errors.description} aria-describedby="arco-desc-hint{errors.description ? ' arco-desc-err' : ''}"></textarea>
        <p id="arco-desc-hint" class="hint">{t('arco.form.descriptionHint')}</p>
        {#if errors.description}<p id="arco-desc-err" class="mt-1.5 text-sm text-app-danger" role="alert">{errors.description}</p>{/if}
      </div>
    </div>

    <!-- Honeypot: hidden from people and from assistive technology -->
    <div class="absolute -left-[9999px] h-0 w-0 overflow-hidden" aria-hidden="true">
      <label for="arco-website">{t('arco.form.honeypot')}</label>
      <input id="arco-website" tabindex="-1" autocomplete="off" bind:value={form.website} />
    </div>

    <div class="mt-6">
      <label class="flex cursor-pointer items-start gap-3 text-sm">
        <input id="arco-acknowledged" type="checkbox" class="mt-0.5 h-5 w-5 flex-none accent-[rgb(var(--app-primary))]" bind:checked={form.acknowledged} aria-invalid={!!errors.acknowledged} />
        <span>
          {t('arco.form.ackBefore')}<a href="/privacidad" target="_blank" rel="noopener" class="underline">{t('common.privacyLink')}</a>.
        </span>
      </label>
      {#if errors.acknowledged}<p class="mt-1.5 text-sm text-app-danger" role="alert">{errors.acknowledged}</p>{/if}
    </div>

    {#if op.phase === 'error'}<p class="alert mt-5" role="alert"><Icon name="alert" size={18} />{op.message}</p>{/if}
    <button type="submit" class="btn-primary btn-lg mt-6" disabled={op.phase === 'loading'}>
      {#if op.phase === 'loading'}<span class="spin"></span>{/if}{t('arco.form.submit')}
    </button>
  </form>
{/if}
