<script lang="ts">
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { api } from '$lib/api';
  import { ago } from '$lib/format';
  import { session } from '$lib/session.svelte';
  import { PERMISSIONS, type PatientRow } from '$lib/types';
  import EmptyState from '../ui/EmptyState.svelte';
  import Icon from '../ui/Icon.svelte';
  import LoadingRows from '../ui/LoadingRows.svelte';
  import PageHeader from '../ui/PageHeader.svelte';
  import Pill from '../ui/Pill.svelte';
  import { ageText, fullName, subtitle } from './util';

  type Tab = 'activos' | 'pendientes' | 'archivados';
  const TABS: { id: Tab; label: string; param: string }[] = [
    { id: 'activos', label: 'Activos', param: '' },
    { id: 'pendientes', label: 'Pendientes', param: '?pendientes=1' },
    { id: 'archivados', label: 'Archivados', param: '?archivados=1' }
  ];

  const tab = $derived<Tab>(page.url.searchParams.get('pendientes') === '1' ? 'pendientes' : page.url.searchParams.get('archivados') === '1' ? 'archivados' : 'activos');
  const canCreate = $derived(session.has(PERMISSIONS.adminHistorials));

  let search = $state('');
  let items = $state<PatientRow[]>([]);
  let loading = $state(true);
  let loadError = $state('');
  let seq = 0;

  // reload when the tab or the (debounced) search changes
  $effect(() => {
    const t = tab;
    const q = search.trim();
    const mine = ++seq;
    loading = true;
    const timer = setTimeout(
      async () => {
        try {
          const r = await api.patients.list({ q, archived: t === 'archivados', pending: t === 'pendientes' });
          if (mine !== seq) return;
          items = r;
          loadError = '';
        } catch (e) {
          if (mine !== seq) return;
          loadError = e instanceof Error ? e.message : 'No se pudieron cargar los pacientes.';
        } finally {
          if (mine === seq) loading = false;
        }
      },
      q ? 300 : 0
    );
    return () => clearTimeout(timer);
  });

  function setTab(p: string) {
    goto(`/pacientes${p}`, { replaceState: true, keepFocus: true, noScroll: true });
  }

  const href = (p: PatientRow) => `/pacientes/${encodeURIComponent(p.id)}`;
  const lastVisit = (p: PatientRow) => (p.last_encounter_at ? ago(p.last_encounter_at) : 'Sin consultas');
  const emptyText = $derived(
    search.trim()
      ? 'Prueba con otro nombre, teléfono, CURP o número de expediente.'
      : tab === 'pendientes'
        ? 'Todos los expedientes están completos y con aviso de privacidad.'
        : tab === 'archivados'
          ? 'Los expedientes que archives aparecerán aquí.'
          : canCreate
            ? 'Registra a tu primer paciente.'
            : 'Cuando se registren pacientes aparecerán aquí.'
  );
</script>

{#snippet pills(p: PatientRow)}
  {#if p.incomplete}<Pill tone="warn">Alta rápida</Pill>{/if}
  {#if p.no_privacy_notice}<Pill tone="bad">Sin aviso de privacidad</Pill>{/if}
  {#if p.archived_at}<Pill>Archivado</Pill>{/if}
{/snippet}

<PageHeader title="Pacientes" subtitle="Expedientes de personas y animales de tu consultorio.">
  {#snippet actions()}
    {#if canCreate}<a href="/pacientes/nuevo" class="btn-primary"><Icon name="plus" size={18} stroke={2.2} />Nuevo paciente</a>{/if}
  {/snippet}
</PageHeader>

<div class="mb-4 flex flex-wrap items-center gap-3">
  <div class="relative w-full max-w-md" role="search">
    <Icon name="search" size={18} class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-app-muted" />
    <input type="search" class="field pl-10" placeholder="Buscar por nombre, CURP, teléfono o número…" aria-label="Buscar pacientes" autocomplete="off" bind:value={search} />
  </div>
  <div class="flex gap-1 rounded-full bg-app-ink/6 p-1" role="tablist" aria-label="Filtrar expedientes">
    {#each TABS as t (t.id)}
      <button
        type="button"
        role="tab"
        aria-selected={tab === t.id}
        class="min-h-9 rounded-full px-4 text-sm font-medium transition {tab === t.id ? 'bg-app-panel text-app-ink shadow-app' : 'text-app-muted hover:text-app-ink'}"
        onclick={() => setTab(t.param)}>{t.label}</button>
    {/each}
  </div>
</div>

<div class="card overflow-hidden" aria-busy={loading}>
  {#if loading && items.length === 0}
    <LoadingRows />
  {:else if loadError}
    <p class="alert m-5" role="alert"><Icon name="alert" size={18} />{loadError}</p>
  {:else if items.length === 0}
    <EmptyState icon={search.trim() ? 'search' : 'folder'} title={search.trim() ? 'Sin resultados' : tab === 'pendientes' ? 'Nada pendiente' : tab === 'archivados' ? 'No hay expedientes archivados' : 'Aún no hay pacientes'} text={emptyText}>
      {#if canCreate && tab === 'activos' && !search.trim()}<a href="/pacientes/nuevo" class="btn-primary"><Icon name="plus" size={18} stroke={2.2} />Nuevo paciente</a>{/if}
    </EmptyState>
  {:else}
    <!-- tablet and desktop -->
    <div class="hidden overflow-x-auto md:block {loading ? 'opacity-60' : ''}">
      <table class="w-full min-w-[44rem]">
        <caption class="sr-only">Lista de pacientes</caption>
        <thead class="border-b border-app-ink/10 bg-app-elevated">
          <tr>
            <th class="th w-16">#</th>
            <th class="th">Paciente</th>
            <th class="th">Edad</th>
            <th class="th">Teléfono</th>
            <th class="th">Última visita</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-app-ink/8">
          {#each items as p (p.id)}
            <tr class="transition hover:bg-app-ink/[0.03]">
              <td class="td font-mono text-xs text-app-muted">{p.file_number}</td>
              <td class="td">
                <a href={href(p)} class="flex items-center gap-3 rounded-lg outline-none focus-visible:ring-4 focus-visible:ring-app-primary/20">
                  <span class="grid h-9 w-9 flex-none place-items-center rounded-full bg-app-primary/12 text-app-primary"><Icon name={p.subject === 'animal' ? 'paw' : 'user'} size={17} /></span>
                  <span class="min-w-0">
                    <span class="block font-semibold">{fullName(p)}</span>
                    {#if p.subject === 'animal'}<span class="block text-xs text-app-muted">{[p.species, p.guardian_name ? `de ${p.guardian_name}` : ''].filter(Boolean).join(' · ')}</span>{/if}
                    {#if p.incomplete || p.no_privacy_notice || p.archived_at}<span class="mt-1 flex flex-wrap gap-1.5">{@render pills(p)}</span>{/if}
                  </span>
                </a>
              </td>
              <td class="td whitespace-nowrap">{ageText(p.age) || '—'}</td>
              <td class="td whitespace-nowrap">{p.phone || '—'}</td>
              <td class="td whitespace-nowrap text-app-muted">{lastVisit(p)}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <!-- phones -->
    <ul class="divide-y divide-app-ink/8 md:hidden {loading ? 'opacity-60' : ''}">
      {#each items as p (p.id)}
        <li>
          <a href={href(p)} class="flex items-start gap-3 px-4 py-3.5 transition hover:bg-app-ink/[0.03] focus-visible:bg-app-ink/[0.04] focus-visible:outline-none">
            <span class="grid h-10 w-10 flex-none place-items-center rounded-full bg-app-primary/12 text-app-primary"><Icon name={p.subject === 'animal' ? 'paw' : 'user'} size={18} /></span>
            <span class="min-w-0 flex-1">
              <span class="block font-semibold">{fullName(p)} <span class="font-mono text-xs font-normal text-app-muted">#{p.file_number}</span></span>
              <span class="block text-sm text-app-muted">{subtitle(p) || 'Sin edad registrada'}</span>
              <span class="mt-0.5 block text-xs text-app-muted">{p.phone || 'Sin teléfono'} · {lastVisit(p)}</span>
              {#if p.incomplete || p.no_privacy_notice || p.archived_at}<span class="mt-1.5 flex flex-wrap gap-1.5">{@render pills(p)}</span>{/if}
            </span>
            <Icon name="arrow-right" size={18} class="mt-2.5 flex-none text-app-muted" />
          </a>
        </li>
      {/each}
    </ul>
  {/if}
</div>

<p class="mt-4 text-xs text-app-muted">Los expedientes no se borran: se archivan y se conservan al menos 5 años (NOM-004-SSA3-2012).</p>
