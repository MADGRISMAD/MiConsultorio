<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import { portalApi } from '$lib/api/portal';
  import { Op } from '$lib/op.svelte';
  import type { PortalInfo } from '$lib/types/portal';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { t } from '$lib/i18n/index.svelte';

  let { slug, info, onsignedin }: { slug: string; info: PortalInfo; onsignedin: () => void } = $props();

  let step = $state<'email' | 'code'>('email');
  let email = $state('');
  let code = $state('');
  const sendOp = new Op();
  const loginOp = new Op();

  async function send(e: SubmitEvent) {
    e.preventDefault();
    if (await sendOp.run(() => portalApi.requestCode(slug, email.trim()))) {
      code = '';
      step = 'code';
    }
  }

  async function enter(e: SubmitEvent) {
    e.preventDefault();
    if (await loginOp.run(() => portalApi.login(slug, email.trim(), code.replace(/\s/g, '')))) onsignedin();
  }

  async function resend() {
    await sendOp.run(() => portalApi.requestCode(slug, email.trim()), t('portal.login.newCode'));
  }
</script>

<section class="card px-6 py-8 sm:px-8" aria-labelledby="pl-title">
  <div class="grid h-12 w-12 place-items-center rounded-full bg-app-primary/10 text-app-primary"><Icon name="user" size={24} /></div>
  <p class="section-title mt-5">{info.name}</p>
  <h1 id="pl-title" class="display mt-1 text-3xl leading-tight">{t('portal.login.title')}</h1>
  {#if info.welcome}<p class="mt-3 whitespace-pre-line text-app-muted">{info.welcome}</p>{/if}

  {#if step === 'email'}
    <form onsubmit={send} class="mt-6 grid gap-4">
      <div>
        <label class="label" for="pl-email">{t('portal.login.email')}</label>
        <input id="pl-email" class="field" type="email" inputmode="email" autocomplete="email" required maxlength="200" bind:value={email} aria-describedby="pl-email-hint" />
        <p id="pl-email-hint" class="hint">{t('portal.login.emailHint')}</p>
      </div>
      <OpError op={sendOp} />
      <button class="btn-primary btn-lg" disabled={sendOp.phase === 'loading'}>
        {sendOp.phase === 'loading' ? t('portal.login.sending') : t('portal.login.sendCode')}
      </button>
    </form>
  {:else}
    <form onsubmit={enter} class="mt-6 grid gap-4">
      <p class="rounded-xl bg-app-primary/8 px-3.5 py-3 text-sm" role="status">
        {t('portal.login.sentBefore')}<strong class="break-all">{email}</strong>{t('portal.login.sentAfter')}
      </p>
      <div>
        <label class="label" for="pl-code">{t('portal.login.code')}</label>
        <input id="pl-code" class="field text-center font-mono text-2xl tracking-[0.4em]" inputmode="numeric" autocomplete="one-time-code" pattern="[0-9 ]*" maxlength="7" required bind:value={code} />
      </div>
      <OpError op={loginOp} />
      {#if sendOp.phase === 'success'}<p class="text-sm text-app-accent" role="status">{sendOp.message}</p>{/if}
      <button class="btn-primary btn-lg" disabled={loginOp.phase === 'loading' || code.replace(/\s/g, '').length !== 6}>
        {loginOp.phase === 'loading' ? t('portal.login.entering') : t('portal.login.enter')}
      </button>
      <div class="flex flex-wrap justify-between gap-2 text-sm">
        <button type="button" class="btn-ghost" onclick={resend} disabled={sendOp.phase === 'loading'}>{t('portal.login.resend')}</button>
        <button type="button" class="btn-ghost" onclick={() => { step = 'email'; loginOp.reset(); sendOp.reset(); }}>{t('portal.login.changeEmail')}</button>
      </div>
    </form>
  {/if}
  <p class="mt-6 text-xs text-app-muted">
    {t('portal.login.privacy')}
  </p>
</section>
