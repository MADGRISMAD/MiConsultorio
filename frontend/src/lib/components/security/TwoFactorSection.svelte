<script lang="ts">
  import QRCode from 'qrcode';
  import { securityApi } from '$lib/api/security';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { TwoFactorSetup } from '$lib/types/security';
  import Icon from '$lib/components/ui/Icon.svelte';
  import RecoveryCodes from './RecoveryCodes.svelte';

  const enabled = $derived(!!session.user?.twoFactorEnabled);
  const required = $derived(!!session.user?.mustSetup2fa);

  type Mode = 'idle' | 'setup' | 'disable' | 'regenerate';
  let mode = $state<Mode>('idle');
  let setup = $state<TwoFactorSetup | null>(null);
  let qr = $state('');
  let code = $state('');
  let password = $state('');
  /** Se muestran una sola vez: el servidor solo guarda su huella. */
  let codes = $state<string[] | null>(null);
  const op = new Op();

  function close() {
    mode = 'idle';
    setup = null;
    qr = '';
    code = password = '';
    op.reset();
  }

  async function start() {
    if (await op.run(async () => (setup = await securityApi.setup()))) {
      mode = 'setup';
      qr = await QRCode.toDataURL(setup!.otpauth_uri, { margin: 1, width: 220 }).catch(() => '');
    }
  }

  async function enable(e: SubmitEvent) {
    e.preventDefault();
    let result: Awaited<ReturnType<typeof securityApi.enable>> | undefined;
    if (await op.run(async () => (result = await securityApi.enable(code.trim())))) {
      session.setUser(result!.session);
      codes = result!.recovery_codes;
      close();
      toast.show('Verificación en dos pasos activada');
    }
  }

  async function disable(e: SubmitEvent) {
    e.preventDefault();
    let result: Awaited<ReturnType<typeof securityApi.disable>> | undefined;
    if (await op.run(async () => (result = await securityApi.disable(password, code.trim())))) {
      session.setUser(result!.session);
      close();
      toast.show('Verificación en dos pasos desactivada');
    }
  }

  async function regenerate(e: SubmitEvent) {
    e.preventDefault();
    let result: Awaited<ReturnType<typeof securityApi.newRecoveryCodes>> | undefined;
    if (await op.run(async () => (result = await securityApi.newRecoveryCodes(password, code.trim())))) {
      session.setUser(result!.session);
      codes = result!.recovery_codes;
      close();
    }
  }
</script>

<section class="card p-6" aria-labelledby="tfa-title">
  <div class="flex flex-wrap items-center justify-between gap-2">
    <h2 id="tfa-title" class="display text-2xl">Verificación en dos pasos</h2>
    <span class="pill {enabled ? 'pill-ok' : 'pill-warn'}">{enabled ? 'Activada' : 'No activada'}</span>
  </div>
  <p class="mt-1 text-sm text-app-muted">
    Además de tu contraseña, se pide un código de 6 dígitos de una app de autenticación (Google Authenticator, Microsoft Authenticator, Authy, 1Password…). Protege tu cuenta aunque alguien conozca tu contraseña.
  </p>
  {#if required && !enabled}
    <p class="alert mt-4" role="alert"><Icon name="alert" size={18} />Tu consultorio exige verificación en dos pasos. Actívala para volver a ver expedientes, recetas y archivos.</p>
  {/if}

  {#if codes}
    <RecoveryCodes {codes} onclose={() => (codes = null)} />
  {:else if mode === 'idle'}
    {#if op.phase === 'error'}<p class="alert mt-4" role="alert"><Icon name="alert" size={18} />{op.message}</p>{/if}
    <div class="mt-4 flex flex-wrap gap-2">
      {#if enabled}
        <button type="button" class="btn-secondary" onclick={() => (mode = 'regenerate')}>Códigos de recuperación nuevos</button>
        <button type="button" class="btn-secondary" onclick={() => (mode = 'disable')}>Desactivar</button>
      {:else}
        <button type="button" class="btn-primary" disabled={op.phase === 'loading'} onclick={start}>
          {#if op.phase === 'loading'}<span class="spin"></span>{/if}Activar verificación en dos pasos
        </button>
      {/if}
    </div>
  {:else if mode === 'setup' && setup}
    <ol class="mt-5 grid gap-5 text-sm">
      <li>
        <p class="font-semibold">1. Escanea el código con tu app de autenticación</p>
        <div class="mt-3 flex flex-wrap items-center gap-4">
          {#if qr}
            <img src={qr} width="220" height="220" alt="Código QR para tu app de autenticación" class="rounded-lg border border-app-ink/10 bg-white p-1" />
          {/if}
          <div class="min-w-0 text-app-muted">
            <p>¿No puedes escanear? Escribe esta clave en la app (tipo de clave: por tiempo):</p>
            <p class="mt-2 break-all rounded-lg bg-app-ink/5 px-3 py-2 font-mono text-[15px] tracking-wider text-app-ink select-all">{setup.secret}</p>
          </div>
        </div>
      </li>
      <li>
        <form class="grid gap-3" onsubmit={enable}>
          <div>
            <label class="label" for="tfa-code">2. Escribe el código de 6 dígitos que muestra la app</label>
            <input id="tfa-code" class="field max-w-[12rem] text-center font-mono text-lg tracking-[0.3em]" inputmode="numeric" autocomplete="one-time-code" maxlength="7" bind:value={code} required />
          </div>
          {#if op.phase === 'error'}<p class="alert" role="alert"><Icon name="alert" size={18} />{op.message}</p>{/if}
          <div class="flex flex-wrap gap-2">
            <button type="submit" class="btn-primary" disabled={op.phase === 'loading'}>Confirmar y activar</button>
            <button type="button" class="btn-secondary" onclick={close}>Cancelar</button>
          </div>
        </form>
      </li>
    </ol>
  {:else if mode === 'disable' || mode === 'regenerate'}
    <form class="mt-5 grid max-w-md gap-3.5" onsubmit={mode === 'disable' ? disable : regenerate}>
      <p class="text-sm text-app-muted">
        {mode === 'disable' ? 'Para desactivarla, confirma con tu contraseña y un código actual.' : 'Los códigos anteriores dejarán de servir. Confirma con tu contraseña y un código actual.'}
      </p>
      <div>
        <label class="label" for="tfa-pw">Contraseña</label>
        <input id="tfa-pw" class="field" type="password" bind:value={password} required autocomplete="current-password" />
      </div>
      <div>
        <label class="label" for="tfa-code2">Código de la app <span class="font-normal text-app-muted">(o un código de recuperación)</span></label>
        <input id="tfa-code2" class="field font-mono" inputmode="text" autocomplete="one-time-code" bind:value={code} required />
      </div>
      {#if op.phase === 'error'}<p class="alert" role="alert"><Icon name="alert" size={18} />{op.message}</p>{/if}
      <div class="flex flex-wrap gap-2">
        <button type="submit" class={mode === 'disable' ? 'btn-danger' : 'btn-primary'} disabled={op.phase === 'loading'}>
          {mode === 'disable' ? 'Desactivar' : 'Generar códigos nuevos'}
        </button>
        <button type="button" class="btn-secondary" onclick={close}>Cancelar</button>
      </div>
    </form>
  {/if}
</section>
