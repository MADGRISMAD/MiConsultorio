<script lang="ts">
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { ago } from '$lib/format';
  import type { ActivityItem } from '$lib/types';
  import EmptyState from '../ui/EmptyState.svelte';
  import Icon, { type IconName } from '../ui/Icon.svelte';
  import LoadingRows from '../ui/LoadingRows.svelte';
  import PageHeader from '../ui/PageHeader.svelte';

  let items = $state<ActivityItem[] | null>(null);
  let error = $state('');

  async function load() {
    try {
      items = await api.platform.activity();
      error = '';
    } catch (e) {
      error = e instanceof Error ? e.message : 'No se pudo cargar la actividad.';
    }
  }
  onMount(load);

  const iconFor = (type: string): IconName =>
    type.startsWith('payment') ? 'cash' : type.startsWith('clinic') ? 'building' : type.startsWith('staff') || type.startsWith('user') ? 'users' : type.includes('password') ? 'key' : 'activity';
</script>

<PageHeader title="Actividad" subtitle="Lo que se ha hecho en la plataforma y en los negocios, de lo más reciente a lo más antiguo.">
  {#snippet actions()}<button type="button" class="btn-secondary" onclick={load}><Icon name="refresh" size={17} />Actualizar</button>{/snippet}
</PageHeader>

<section class="card overflow-hidden">
  {#if error}
    <Alert class="m-5">{error}</Alert>
  {:else if !items}
    <LoadingRows />
  {:else if items.length === 0}
    <EmptyState icon="activity" title="Sin actividad todavía" text="Aquí aparecerán los cambios de planes, pagos, cuentas y equipo." />
  {:else}
    <ul class="divide-y divide-app-ink/8">
      {#each items as a (a.id)}
        <li class="flex items-start gap-3 px-5 py-3.5">
          <span class="mt-0.5 grid h-9 w-9 flex-none place-items-center rounded-full bg-app-primary/10 text-app-primary"><Icon name={iconFor(a.type)} size={17} /></span>
          <div class="min-w-0 flex-1">
            <p class="text-sm">{a.message}</p>
            <p class="mt-0.5 text-xs text-app-muted">
              {a.actor_name || 'Sistema'} ·
              {#if a.clinic_id}<a class="text-app-primary hover:underline" href="/plataforma/negocios/{a.clinic_id}">{a.clinic_name}</a>{:else}plataforma{/if}
            </p>
          </div>
          <time class="flex-none text-xs text-app-muted" datetime={a.created_at} title={new Date(a.created_at).toLocaleString('es-MX')}>{ago(a.created_at)}</time>
        </li>
      {/each}
    </ul>
  {/if}
</section>
