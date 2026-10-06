<script lang="ts">
  import { goto } from '$app/navigation';
  import { session } from '$lib/session.svelte';
  import { Op } from '$lib/op.svelte';
  import AuthLayout from '$lib/components/AuthLayout.svelte';
  import Spinner from '$lib/components/Spinner.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';

  let identifier = $state('');
  let password = $state('');
  let showPass = $state(false);
  let capsOn = $state(false);
  let missing = $state({ identifier: false, password: false });
  let idEl = $state<HTMLInputElement>();
  const op = new Op();

  $effect(() => {
    if (session.status === 'authenticated') goto(session.home, { replaceState: true });
  });
  $effect(() => idEl?.focus());

  const caps = (e: KeyboardEvent) => (capsOn = e.getModifierState?.('CapsLock') ?? false);

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    missing = { identifier: !identifier.trim(), password: !password };
    if (missing.identifier || missing.password) {
      op.fail('Escribe tu correo o usuario y tu contraseña.');
      return;
    }
    const ok = await op.run(() => session.login(identifier.trim(), password));
    if (!ok) password = '';
  }
</script>

<svelte:head><title>Iniciar sesión · Caresia</title></svelte:head>

{#if session.status === 'loading'}
  <Spinner />
{:else}
  <AuthLayout>
    <header class="mb-6">
      <h1 class="display text-[2.4rem] leading-none">Bienvenido <em class="italic text-app-primary">de vuelta</em></h1>
      <p class="mt-3 text-[15px] text-app-muted">Entra para ver tu agenda, tus pacientes y tu consultorio.</p>
    </header>

    <form class="grid gap-4" novalidate onsubmit={submit}>
      {#if op.phase === 'error'}
        <p class="alert" role="alert"><Icon name="alert" size={18} />{op.message}</p>
      {/if}

      <div>
        <label class="label" for="login-id">Correo o usuario</label>
        <input id="login-id" bind:this={idEl} class="field" type="text" bind:value={identifier} placeholder="tu@correo.com o tu_usuario" autocomplete="username" autocapitalize="none" autocorrect="off" spellcheck="false" aria-invalid={missing.identifier} oninput={() => (missing.identifier = false)} />
      </div>

      <div>
        <div class="flex items-baseline justify-between gap-2">
          <label class="label" for="login-pass">Contraseña</label>
          <a href="/forgot" class="text-[13px] font-medium text-app-primary hover:underline">¿La olvidaste?</a>
        </div>
        <div class="relative">
          <input id="login-pass" class="field pr-12" type={showPass ? 'text' : 'password'} bind:value={password} placeholder="Tu contraseña" autocomplete="current-password" aria-invalid={missing.password} oninput={() => (missing.password = false)} onkeydown={caps} onkeyup={caps} />
          <button type="button" class="icon-btn absolute right-1 top-1/2 -translate-y-1/2" aria-label={showPass ? 'Ocultar contraseña' : 'Mostrar contraseña'} aria-pressed={showPass} onclick={() => (showPass = !showPass)}>
            <Icon name={showPass ? 'eye-off' : 'eye'} size={20} />
          </button>
        </div>
        {#if capsOn}<p class="mt-1.5 text-[13px] font-medium text-app-warning">Bloq Mayús está activado.</p>{/if}
      </div>

      <button type="submit" class="btn-primary btn-lg" disabled={op.phase === 'loading'}>
        {#if op.phase === 'loading'}<span class="spin"></span>Entrando…{:else}Entrar{/if}
      </button>
    </form>

    <div class="mt-6 grid gap-2.5 border-t border-app-ink/10 pt-5 text-center text-sm text-app-muted">
      <span>¿Aún no tienes cuenta?</span>
      <a href="/register" class="btn-secondary btn-lg">Crear mi consultorio</a>
    </div>
    <a href="/" class="mt-4 block text-center text-sm font-medium text-app-muted hover:text-app-ink">← Volver al inicio</a>
  </AuthLayout>
{/if}
