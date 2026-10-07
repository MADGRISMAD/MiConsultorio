<script lang="ts">
  import { page } from '$app/state';
  import { api } from '$lib/api';
  import { Op } from '$lib/op.svelte';
  import { theme } from '$lib/theme.svelte';
  import Brand from '$lib/components/ui/Brand.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';

  $effect(() => theme.init());

  const token = $derived(page.url.searchParams.get('token') ?? '');
  let password = $state('');
  let confirm = $state('');
  let show = $state(false);
  let done = $state(false);
  const op = new Op();

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (password.length < 8) return op.fail('La contraseña debe tener al menos 8 caracteres.');
    if (password !== confirm) return op.fail('Las contraseñas no coinciden.');
    const ok = await op.run(() => api.resetPassword(token, password));
    if (ok) done = true;
  }
</script>

<svelte:head><title>Nueva contraseña · Caresia</title></svelte:head>

<div
  class="app fixed inset-0 grid place-items-center overflow-y-auto px-4 py-8"
  data-theme={theme.mode}
  style="background-image: radial-gradient(ellipse 60% 420px at 50% 0%, rgb(var(--app-primary) / 0.1), transparent 75%); background-repeat: no-repeat"
>
  <div class="card page-in w-full max-w-md px-6 py-9 sm:px-9">
    <a href="/" class="inline-flex" aria-label="Caresia, ir al inicio"><Brand size={32} class="text-[1.05rem]" /></a>
    <h1 class="display mt-7 text-[2.2rem] leading-none">Nueva <em class="italic text-app-primary">contraseña</em></h1>

    {#if done}
      <div class="mt-5 rounded-xl bg-app-accent/10 p-4 text-sm text-app-ink" role="status">
        <p class="flex items-start gap-2 font-medium text-app-accent"><Icon name="check" size={18} class="mt-px flex-none" />Contraseña actualizada</p>
        <p class="mt-1.5">Ya puedes entrar con tu contraseña nueva. Cerramos tus sesiones abiertas por seguridad.</p>
      </div>
      <a href="/login" class="btn-primary btn-lg mt-7">Ir al login</a>
    {:else if !token}
      <p class="alert mt-5" role="alert"><Icon name="alert" size={18} />Este enlace está incompleto. Pide uno nuevo.</p>
      <a href="/forgot" class="btn-primary btn-lg mt-7">Pedir otro enlace</a>
    {:else}
      <p class="mt-3 text-[15px] text-app-muted">Elige una contraseña de al menos 8 caracteres.</p>
      <form class="mt-6 grid gap-4" novalidate onsubmit={submit}>
        {#if op.phase === 'error'}
          <p class="alert" role="alert"><Icon name="alert" size={18} />{op.message}</p>
        {/if}
        <div>
          <label class="label" for="rp-pass">Contraseña nueva</label>
          <div class="relative">
            <input id="rp-pass" class="field pr-12" type={show ? 'text' : 'password'} bind:value={password} autocomplete="new-password" />
            <button type="button" class="icon-btn absolute right-1 top-1/2 -translate-y-1/2" aria-label={show ? 'Ocultar contraseña' : 'Mostrar contraseña'} aria-pressed={show} onclick={() => (show = !show)}>
              <Icon name={show ? 'eye-off' : 'eye'} size={20} />
            </button>
          </div>
        </div>
        <div>
          <label class="label" for="rp-confirm">Repite la contraseña</label>
          <input id="rp-confirm" class="field" type={show ? 'text' : 'password'} bind:value={confirm} autocomplete="new-password" />
        </div>
        <button type="submit" class="btn-primary btn-lg" disabled={op.phase === 'loading'}>{#if op.phase === 'loading'}<span class="spin"></span>Guardando…{:else}Guardar contraseña{/if}</button>
      </form>
      {#if op.phase === 'error' && /venci|válido/.test(op.message)}
        <a href="/forgot" class="mt-4 block text-center text-sm font-medium text-app-primary hover:underline">Pedir un enlace nuevo</a>
      {/if}
    {/if}
  </div>
</div>
