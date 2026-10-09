<script lang="ts">
  import { Loader } from '$lib/loader.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { ago } from '$lib/format';
  import { session } from '$lib/session.svelte';
  import { CLINIC_KINDS, type ClinicRow, type Plan } from '$lib/types';
  import Avatar from '../ui/Avatar.svelte';
  import EmptyState from '../ui/EmptyState.svelte';
  import Icon from '../ui/Icon.svelte';
  import LoadingRows from '../ui/LoadingRows.svelte';
  import PageHeader from '../ui/PageHeader.svelte';
  import Pill from '../ui/Pill.svelte';
  import StatePill from '../ui/StatePill.svelte';
  import ClinicDetail from './ClinicDetail.svelte';

  let { id = '' }: { id?: string } = $props();

  const FILTERS: [string, string][] = [
    ['all', 'Todos'],
    ['trialing', 'En prueba'],
    ['active', 'Activos'],
    ['past_due', 'Pago atrasado'],
    ['trial_expired', 'Prueba vencida'],
    ['suspended', 'Suspendidos']
  ];

  let clinics = $state<ClinicRow[]>([]);
  let counts = $state<Record<string, number>>({});
  let plans = $state<Plan[]>([]);
  const ld = new Loader('No se pudieron cargar los negocios.');
  let query = $state('');
  let filter = $state(page.url.searchParams.get('state') ?? 'all');

  let timer: ReturnType<typeof setTimeout>;
  async function load() {
    await ld.run(async () => {
        const r = await api.platform.clinics(query, filter === 'all' ? '' : filter);
        clinics = r.clinics;
        counts = r.counts;
    });
  }
  onMount(() => {
    void load();
    api.platform.plans().then((p) => (plans = p), () => {});
  });
  // refresh when the filter or the (debounced) search changes
  $effect(() => {
    filter;
    query;
    clearTimeout(timer);
    timer = setTimeout(load, 200);
    return () => clearTimeout(timer);
  });

  const open = (cid: string) => goto(`/plataforma/negocios/${cid}${filter !== 'all' ? `?state=${filter}` : ''}`, { keepFocus: true });
  const kind = (c: ClinicRow) => CLINIC_KINDS[c.kind]?.label ?? c.kind;
</script>

<PageHeader title="Negocios" subtitle="Consultorios y clínicas que usan Caresia: plan, suscripción y acceso." />

<div class="grid grid-cols-[minmax(0,1fr)] gap-4 lg:grid-cols-[minmax(20rem,26rem)_minmax(0,1fr)]">
  <section class="card overflow-hidden {id ? 'hidden lg:block' : ''}" aria-label="Lista de negocios">
    <div class="space-y-3 border-b border-app-ink/10 p-4">
      <div class="relative">
        <Icon name="search" size={18} class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-app-muted" />
        <input class="field pl-10" type="search" placeholder="Buscar negocio, dueño o correo" aria-label="Buscar negocio" bind:value={query} />
      </div>
      <div class="flex flex-wrap gap-1.5" role="group" aria-label="Filtrar por estado">
        {#each FILTERS as [key, label]}
          <button type="button" class="inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-sm font-medium transition {filter === key ? 'bg-app-ink text-app-surface' : 'bg-app-ink/6 text-app-muted hover:bg-app-ink/10'}" aria-pressed={filter === key} onclick={() => (filter = key)}>
            {label}<em class="font-mono text-[11px] not-italic opacity-70">{counts[key] ?? 0}</em>
          </button>
        {/each}
      </div>
    </div>

    {#if ld.loading}
      <LoadingRows />
    {:else if ld.error}
      <Alert class="m-4">{ld.error}</Alert>
    {:else if clinics.length === 0}
      <EmptyState icon="building" title={counts.all ? 'Ningún negocio coincide' : 'Aún no hay negocios'} text={counts.all ? 'Cambia el filtro o la búsqueda.' : 'Cuando alguien cree su consultorio aparecerá aquí.'} />
    {:else}
      <ul class="max-h-[70vh] divide-y divide-app-ink/8 overflow-y-auto">
        {#each clinics as c (c.id)}
          <li>
            <button type="button" class="flex w-full items-center gap-3 px-4 py-3.5 text-left transition hover:bg-app-ink/[0.03] {id === c.id ? 'bg-app-primary/8' : ''}" aria-current={id === c.id ? 'true' : undefined} onclick={() => open(c.id)}>
              <Avatar name={c.name} size={42} />
              <span class="min-w-0 flex-1">
                <span class="block truncate font-semibold">{c.name}</span>
                <span class="block truncate text-sm text-app-muted">{c.owner_name || 'Sin dueño'}</span>
                <span class="mt-1.5 flex flex-wrap items-center gap-1.5">
                  <StatePill state={c.state} /><Pill>{c.plan_name}</Pill>{#if plans.find((x) => x.id === c.plan)?.cobros}<Pill tone="ok">Cobros</Pill>{/if}<Pill>{kind(c)}</Pill>
                  <span class="text-xs text-app-muted">{c.last_seen ? ago(c.last_seen) : 'sin entrar'}</span>
                </span>
              </span>
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </section>

  <div class="min-w-0 {id ? '' : 'hidden lg:block'}">
    {#if id}
      <a href="/plataforma/negocios{filter !== 'all' ? `?state=${filter}` : ''}" class="mb-3 inline-flex items-center gap-1.5 text-sm font-medium text-app-primary hover:underline lg:hidden"><Icon name="arrow-left" size={16} />Volver a negocios</a>
      <ClinicDetail {id} canEdit={session.isPlatformAdmin} {plans} onchanged={() => load()} />
    {:else}
      <div class="card"><EmptyState icon="building" title="Elige un negocio" text="Verás su plan, su suscripción, quién entra y la historia de lo que se ha hecho con su cuenta." /></div>
    {/if}
  </div>
</div>
