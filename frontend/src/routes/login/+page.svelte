<script lang="ts">
  import { goto } from '$app/navigation';
  import { session } from '$lib/session.svelte';
  import { Op } from '$lib/op.svelte';
  import Spinner from '$lib/components/Spinner.svelte';

  let email = $state('');
  let username = $state('');
  let password = $state('');
  const op = new Op();

  $effect(() => {
    if (session.status === 'authenticated') goto('/', { replaceState: true });
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    await op.run(() => session.login(email.trim(), username.trim(), password));
  }
</script>

<svelte:head><title>Iniciar sesión · Caresia</title></svelte:head>

{#if session.status === 'loading'}
  <Spinner />
{:else}
  <main class="grid min-h-[100svh] place-items-center bg-paper px-4">
    <form class="w-full max-w-sm rounded-2xl bg-white p-8 shadow-xl ring-1 ring-ink/10" onsubmit={submit}>
      <a href="/" class="font-display text-3xl text-ink">Caresia</a>
      <h1 class="mt-6 text-lg font-semibold">Iniciar sesión</h1>

      <label class="mt-5 block">
        <span class="mb-1 block text-xs font-semibold text-ink-soft">Correo de la clínica</span>
        <input class="field" type="email" bind:value={email} required autocomplete="email" placeholder="clinica@ejemplo.com" />
      </label>
      <label class="mt-4 block">
        <span class="mb-1 block text-xs font-semibold text-ink-soft">Usuario</span>
        <input class="field" type="text" bind:value={username} required autocomplete="username" placeholder="usuario_123" />
      </label>
      <label class="mt-4 block">
        <span class="mb-1 block text-xs font-semibold text-ink-soft">Contraseña</span>
        <input class="field" type="password" bind:value={password} required autocomplete="current-password" placeholder="••••••••" />
      </label>

      {#if op.phase === 'error'}
        <p class="mt-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700" role="alert">{op.message}</p>
      {/if}

      <button type="submit" class="btn-primary mt-6 w-full" disabled={op.phase === 'loading'}>
        {op.phase === 'loading' ? 'Entrando…' : 'Entrar'}
      </button>
    </form>
  </main>
{/if}
