<script lang="ts">
  import { portalApi } from '$lib/api/portal';
  import { Op } from '$lib/op.svelte';
  import type { PortalInfo } from '$lib/types/portal';
  import Icon from '$lib/components/ui/Icon.svelte';

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
    await sendOp.run(() => portalApi.requestCode(slug, email.trim()), 'Te enviamos un código nuevo.');
  }
</script>

<section class="card px-6 py-8 sm:px-8" aria-labelledby="pl-title">
  <div class="grid h-12 w-12 place-items-center rounded-full bg-app-primary/10 text-app-primary"><Icon name="user" size={24} /></div>
  <p class="section-title mt-5">{info.name}</p>
  <h1 id="pl-title" class="display mt-1 text-3xl leading-tight">Portal del paciente</h1>
  {#if info.welcome}<p class="mt-3 whitespace-pre-line text-app-muted">{info.welcome}</p>{/if}

  {#if step === 'email'}
    <form onsubmit={send} class="mt-6 grid gap-4">
      <div>
        <label class="label" for="pl-email">Tu correo electrónico</label>
        <input id="pl-email" class="field" type="email" inputmode="email" autocomplete="email" required maxlength="200" bind:value={email} aria-describedby="pl-email-hint" />
        <p id="pl-email-hint" class="hint">Usa el correo que registraste en el consultorio, como paciente o como tutor.</p>
      </div>
      {#if sendOp.phase === 'error'}<p class="alert" role="alert"><Icon name="alert" size={18} />{sendOp.message}</p>{/if}
      <button class="btn-primary btn-lg" disabled={sendOp.phase === 'loading'}>
        {sendOp.phase === 'loading' ? 'Enviando…' : 'Enviarme un código'}
      </button>
    </form>
  {:else}
    <form onsubmit={enter} class="mt-6 grid gap-4">
      <p class="rounded-xl bg-app-primary/8 px-3.5 py-3 text-sm" role="status">
        Si <strong class="break-all">{email}</strong> está registrado en el consultorio, te enviamos un código de 6 dígitos. Vale 10 minutos.
      </p>
      <div>
        <label class="label" for="pl-code">Código</label>
        <input id="pl-code" class="field text-center font-mono text-2xl tracking-[0.4em]" inputmode="numeric" autocomplete="one-time-code" pattern="[0-9 ]*" maxlength="7" required bind:value={code} />
      </div>
      {#if loginOp.phase === 'error'}<p class="alert" role="alert"><Icon name="alert" size={18} />{loginOp.message}</p>{/if}
      {#if sendOp.phase === 'success'}<p class="text-sm text-app-accent" role="status">{sendOp.message}</p>{/if}
      <button class="btn-primary btn-lg" disabled={loginOp.phase === 'loading' || code.replace(/\s/g, '').length !== 6}>
        {loginOp.phase === 'loading' ? 'Entrando…' : 'Entrar'}
      </button>
      <div class="flex flex-wrap justify-between gap-2 text-sm">
        <button type="button" class="btn-ghost" onclick={resend} disabled={sendOp.phase === 'loading'}>Enviar otro código</button>
        <button type="button" class="btn-ghost" onclick={() => { step = 'email'; loginOp.reset(); sendOp.reset(); }}>Cambiar correo</button>
      </div>
    </form>
  {/if}
  <p class="mt-6 text-xs text-app-muted">
    Tus datos de salud son personales y sensibles. Este portal solo muestra tus citas, recetas y carnet de vacunación, y la sesión se cierra sola a las 2 horas.
  </p>
</section>
