<script lang="ts">
  import { page } from '$app/state';
  import { api, ApiError } from '$lib/api';
  import type { Expedient } from '$lib/types';
  import Guard from '$lib/components/Guard.svelte';
  import ExpedientView from '$lib/components/ExpedientView.svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import PageHeader from '$lib/components/ui/PageHeader.svelte';

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

<Guard title="Historial" permissions={['navHistorials', 'adminHistorials']}>
  <a href="/admin/navegar-historiales" class="mb-4 inline-flex items-center gap-1.5 text-sm font-semibold text-app-primary hover:underline"><Icon name="arrow-left" size={16} />Volver a historiales</a>
  {#if status === 'ok' && expedient}
    <PageHeader title="Historial clínico" subtitle={expedient.CURP} />
    <div class="card p-6"><ExpedientView {expedient} /></div>
  {:else if status === 'loading'}
    <div class="card"><LoadingRows /></div>
  {:else}
    <div class="card"><EmptyState icon="search" title={status === 'notFound' ? 'Historial no encontrado' : 'No se pudo cargar el historial'} text={curp} /></div>
  {/if}
</Guard>
