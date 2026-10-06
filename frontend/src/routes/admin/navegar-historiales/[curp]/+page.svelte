<script lang="ts">
  import { page } from '$app/state';
  import { api, ApiError } from '$lib/api';
  import type { Expedient } from '$lib/types';
  import Guard from '$lib/components/Guard.svelte';
  import ExpedientView from '$lib/components/ExpedientView.svelte';

  let expedient = $state<Expedient | null>(null);
  let status = $state<'loading' | 'ok' | 'notFound' | 'error'>('loading');
  const curp = $derived(page.params.curp ?? '');

  $effect(() => {
    const c = curp;
    status = 'loading';
    api.expedient(c).then(
      (e) => {
        expedient = e;
        status = 'ok';
      },
      (e) => (status = e instanceof ApiError && e.status === 404 ? 'notFound' : 'error')
    );
  });
</script>

<svelte:head><title>Historial {curp} · Caresia</title></svelte:head>

<Guard permissions={['navHistorials', 'adminHistorials']}>
  <div class="mx-auto max-w-4xl px-4 py-8">
    <a href="/admin/navegar-historiales" class="text-sm text-signal hover:underline">← Volver a historiales</a>
    {#if status === 'ok' && expedient}
      <h1 class="mb-6 mt-2 font-display text-4xl">Historial {curp}</h1>
      <div class="rounded-2xl bg-white p-6 shadow-sm ring-1 ring-ink/10"><ExpedientView {expedient} /></div>
    {:else if status === 'loading'}
      <p class="mt-6 text-ink-soft">Cargando...</p>
    {:else}
      <h1 class="mt-2 font-display text-4xl">{status === 'notFound' ? 'Historial no encontrado' : 'No se pudo cargar el historial'}</h1>
    {/if}
  </div>
</Guard>
