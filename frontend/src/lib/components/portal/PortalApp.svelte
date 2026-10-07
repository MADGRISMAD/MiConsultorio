<script lang="ts">
  import { onMount } from 'svelte';
  import { ApiError } from '$lib/api';
  import { portalApi } from '$lib/api/portal';
  import { Op } from '$lib/op.svelte';
  import type { PortalMe, PortalVaccination } from '$lib/types/portal';
  import Icon from '$lib/components/ui/Icon.svelte';
  import AppointmentsPane from './AppointmentsPane.svelte';
  import RxPane from './RxPane.svelte';
  import VaccinesPane from './VaccinesPane.svelte';

  let { slug, onexit }: { slug: string; onexit: (expired: boolean) => void } = $props();

  let me = $state<PortalMe | null>(null);
  let patientId = $state('');
  let tab = $state<'citas' | 'recetas' | 'vacunas'>('citas');
  let vaccines = $state<PortalVaccination[]>([]);
  let vaccinesLoaded = $state(false);
  let vaccinesError = $state('');
  const loadOp = new Op();

  onMount(async () => {
    try {
      me = await portalApi.me();
    } catch (e) {
      if (e instanceof ApiError && (e.status === 401 || e.status === 404)) onexit(true);
      else loadOp.fail(e instanceof Error ? e.message : 'No se pudo cargar tu información.');
      return;
    }
    try {
      vaccines = (await portalApi.vaccinations()).vaccinations;
    } catch (e) {
      vaccinesError = e instanceof Error ? e.message : 'No se pudo cargar el carnet.';
    }
    vaccinesLoaded = true;
  });

  const hasVaccines = $derived(vaccines.length > 0);
  const multi = $derived((me?.patients.length ?? 0) > 1);
  const tabs = $derived([
    { id: 'citas' as const, label: 'Mis citas', icon: 'calendar' as const },
    { id: 'recetas' as const, label: 'Recetas', icon: 'receipt' as const },
    ...(hasVaccines ? [{ id: 'vacunas' as const, label: 'Carnet de vacunación', icon: 'shield' as const }] : [])
  ]);

  function onKey(e: KeyboardEvent) {
    const i = tabs.findIndex((t) => t.id === tab);
    let n = i;
    if (e.key === 'ArrowRight') n = (i + 1) % tabs.length;
    else if (e.key === 'ArrowLeft') n = (i - 1 + tabs.length) % tabs.length;
    else return;
    e.preventDefault();
    tab = tabs[n].id;
    document.getElementById(`pt-${tab}`)?.focus();
  }

  async function logout() {
    try {
      await portalApi.logout();
    } finally {
      onexit(false);
    }
  }
</script>

{#if loadOp.phase === 'error' && !me}
  <section class="card px-6 py-9" role="alert">
    <p class="alert"><Icon name="alert" size={18} />{loadOp.message}</p>
    <button class="btn-secondary mt-5" onclick={() => location.reload()}>Reintentar</button>
  </section>
{:else if me}
  <header class="mb-5 flex flex-wrap items-start justify-between gap-3">
    <div class="min-w-0">
      <p class="section-title">{me.clinic.name}</p>
      <h1 class="display text-3xl leading-tight">Hola</h1>
      <p class="break-all text-sm text-app-muted">{me.email}</p>
    </div>
    <button class="btn-ghost" onclick={logout}><Icon name="logout" size={16} />Salir</button>
  </header>

  {#if me.patients.length === 0}
    <section class="card px-6 py-9 text-sm text-app-muted">No encontramos pacientes ligados a este correo.</section>
  {:else}
    {#if multi}
      <div class="mb-4">
        <label class="label" for="pt-patient">Ver información de</label>
        <select id="pt-patient" class="field" bind:value={patientId}>
          <option value="">Todos ({me.patients.length})</option>
          {#each me.patients as p (p.id)}
            <option value={p.id}>{p.name}{p.species ? ` (${p.species})` : ''}</option>
          {/each}
        </select>
      </div>
    {/if}

    <div role="tablist" aria-label="Secciones del portal" class="mb-5 flex gap-1 overflow-x-auto rounded-full bg-app-ink/6 p-1" tabindex="-1">
      {#each tabs as t (t.id)}
        <button
          role="tab"
          id="pt-{t.id}"
          aria-selected={tab === t.id}
          aria-controls="pp-{t.id}"
          tabindex={tab === t.id ? 0 : -1}
          class="inline-flex min-h-10 flex-1 items-center justify-center gap-2 whitespace-nowrap rounded-full px-4 text-sm font-medium transition {tab === t.id ? 'bg-app-panel text-app-ink shadow-app' : 'text-app-muted hover:text-app-ink'}"
          onclick={() => (tab = t.id)}
          onkeydown={onKey}
        >
          <Icon name={t.icon} size={16} />{t.label}
        </button>
      {/each}
    </div>

    <div role="tabpanel" id="pp-{tab}" aria-labelledby="pt-{tab}">
      {#if tab === 'citas'}
        <AppointmentsPane {slug} {patientId} {multi} />
      {:else if tab === 'recetas'}
        <RxPane patients={me.patients} {patientId} {multi} />
      {:else}
        <VaccinesPane patients={me.patients} {patientId} clinicName={me.clinic.name} list={vaccines} loaded={vaccinesLoaded} error={vaccinesError} />
      {/if}
    </div>
  {/if}

  <footer class="mt-8 text-xs text-app-muted">
    {me.clinic.name}{me.clinic.address ? ` · ${me.clinic.address}` : ''}{me.clinic.phone ? ` · Tel. ${me.clinic.phone}` : ''}
  </footer>
{:else}
  <div class="card px-6 py-10 text-center text-sm text-app-muted" role="status">Cargando…</div>
{/if}
