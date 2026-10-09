<script lang="ts">
  import { page } from '$app/state';
  import { onMount } from 'svelte';
  import { bookingApi } from '$lib/api/booking';
  import PublicShell from '$lib/components/booking/PublicShell.svelte';
  import { Op } from '$lib/op.svelte';

  const token = $derived(page.params.token ?? '');
  let clinic = $state('');
  let subscribed = $state(true);
  let view = $state<'loading' | 'ready' | 'done' | 'invalid'>('loading');
  const op = new Op();

  onMount(async () => {
    try {
      const r = await bookingApi.unsubscribeInfo(token);
      clinic = r.clinic;
      subscribed = r.subscribed;
      view = subscribed ? 'ready' : 'done';
    } catch {
      view = 'invalid';
    }
  });

  async function leave() {
    if (await op.run(() => bookingApi.unsubscribe(token))) view = 'done';
  }
</script>

<svelte:head>
  <title>Darme de baja de los correos</title>
  <meta name="robots" content="noindex, nofollow" />
  <meta name="referrer" content="no-referrer" />
</svelte:head>

<PublicShell>
  <div class="card p-6 text-center">
    {#if view === 'loading'}
      <div class="mx-auto h-24 animate-pulse rounded-xl bg-app-ink/5"></div>
    {:else if view === 'invalid'}
      <h1 class="display text-2xl">Enlace no válido</h1>
      <p class="mt-2 text-sm text-app-muted">Este enlace no existe o ya no es válido. Si quieres dejar de recibir correos, avísale al consultorio.</p>
    {:else if view === 'done'}
      <h1 class="display text-2xl">Listo, ya no recibirás correos</h1>
      <p class="mt-2 text-sm text-app-muted">{clinic} no volverá a enviarte felicitaciones ni recordatorios por correo. Si cambias de opinión, pídeselo en tu próxima visita.</p>
    {:else}
      <h1 class="display text-2xl">¿Dejar de recibir correos?</h1>
      <p class="mt-2 text-sm text-app-muted">Dejarás de recibir de {clinic} las felicitaciones de cumpleaños y los recordatorios de citas por correo.</p>
      {#if op.phase === 'error'}<p class="alert mt-3" role="alert">{op.message}</p>{/if}
      <button type="button" class="btn-primary mt-5" disabled={op.phase === 'loading'} onclick={leave}>{#if op.phase === 'loading'}<span class="spin"></span>{/if}Sí, darme de baja</button>
    {/if}
  </div>
</PublicShell>
