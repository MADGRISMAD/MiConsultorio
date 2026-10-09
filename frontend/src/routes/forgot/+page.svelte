<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import { api, ApiError } from '$lib/api';
  import { Op } from '$lib/op.svelte';
  import { theme } from '$lib/theme.svelte';
  import Brand from '$lib/components/ui/Brand.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';

  $effect(() => theme.init());

  let identifier = $state('');
  let sent = $state(false);
  let noMail = $state(false);
  const op = new Op();

  // Shown when the server has no e-mail configured.
  const steps = [
    'Pídele a un administrador de tu consultorio que entre a Equipo.',
    'Que abra el menú de tu cuenta y elija «Cambiar contraseña» para escribir una nueva.',
    'Entra con la contraseña nueva y cámbiala cuando quieras.'
  ];

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!identifier.trim()) return op.fail('Escribe tu correo o tu usuario.');
    try {
      await op.run(async () => {
        try {
          await api.forgotPassword(identifier.trim());
          sent = true;
        } catch (err) {
          if (err instanceof ApiError && err.code === 'NOT_CONFIGURED') {
            noMail = true;
            return;
          }
          throw err;
        }
      });
    } catch {
      /* Op shows the error */
    }
  }
</script>

<svelte:head><title>Recuperar contraseña · Caresia</title></svelte:head>

<div
  class="app fixed inset-0 grid place-items-center overflow-y-auto px-4 py-8"
  data-theme={theme.mode}
  style="background-image: radial-gradient(ellipse 60% 420px at 50% 0%, rgb(var(--app-primary) / 0.1), transparent 75%); background-repeat: no-repeat"
>
  <div class="card page-in w-full max-w-md px-6 py-9 sm:px-9">
    <a href="/" class="inline-flex" aria-label="Caresia, ir al inicio"><Brand size={32} class="text-[1.05rem]" /></a>

    <h1 class="display mt-7 text-[2.2rem] leading-none">Recuperar <em class="italic text-app-primary">contraseña</em></h1>

    {#if sent}
      <div class="mt-5 rounded-xl bg-app-accent/10 p-4 text-sm text-app-accent" role="status">
        <p class="flex items-start gap-2 font-medium"><Icon name="mail" size={18} class="mt-px flex-none" />Revisa tu correo</p>
        <p class="mt-1.5 text-app-ink">Si esa cuenta existe y tiene un correo registrado, te enviamos un enlace para elegir una contraseña nueva. Vale 1 hora. Si no llega, revisa la carpeta de spam.</p>
      </div>
      <a href="/login" class="btn-primary btn-lg mt-7">Volver al login</a>
    {:else if noMail}
      <p class="mt-3 text-[15px] text-app-muted">El envío de enlaces por correo no está disponible en este momento. Mientras tanto, restablecerla es muy sencillo:</p>
      <ol class="mt-6 space-y-2.5 text-sm">
        {#each steps as step, i}
          <li class="flex items-start gap-3 rounded-xl bg-app-elevated p-3.5">
            <span class="grid h-6 w-6 flex-none place-items-center rounded-full bg-app-ink font-mono text-xs text-app-surface">{i + 1}</span>
            <span class="pt-0.5">{step}</span>
          </li>
        {/each}
      </ol>
      <p class="mt-5 flex items-start gap-2 text-xs text-app-muted"><Icon name="info" size={16} class="mt-px flex-none" />¿Eres el único administrador? Contacta al soporte de Caresia para recuperar el acceso.</p>
      <a href="/login" class="btn-primary btn-lg mt-7">Volver al login</a>
    {:else}
      <p class="mt-3 text-[15px] text-app-muted">Escribe tu correo o tu usuario y te enviamos un enlace para elegir una contraseña nueva.</p>
      <form class="mt-6 grid gap-4" novalidate onsubmit={submit}>
        <OpError op={op} />
        <div>
          <label class="label" for="fg-id">Correo o usuario</label>
          <input id="fg-id" class="field" type="text" bind:value={identifier} placeholder="tu@correo.com o tu_usuario" autocomplete="username" autocapitalize="none" spellcheck="false" />
        </div>
        <button type="submit" class="btn-primary btn-lg" disabled={op.phase === 'loading'}>{#if op.phase === 'loading'}<span class="spin"></span>Enviando…{:else}Enviarme el enlace{/if}</button>
      </form>
      <a href="/login" class="mt-5 block text-center text-sm font-medium text-app-muted hover:text-app-ink">← Volver al login</a>
    {/if}
  </div>
</div>
