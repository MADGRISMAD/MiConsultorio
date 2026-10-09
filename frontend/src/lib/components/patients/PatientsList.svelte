<script lang="ts">
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { api } from '$lib/api';
  import { ownersApi } from '$lib/api/owners';
  import { ago } from '$lib/format';
  import { session } from '$lib/session.svelte';
  import { PERMISSIONS, type PatientRow } from '$lib/types';
  import type { OwnerGroup, OwnerRef } from '$lib/types/owners';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import ConfirmModal from '../ConfirmModal.svelte';
  import EmptyState from '../ui/EmptyState.svelte';
  import Icon from '../ui/Icon.svelte';
  import LoadingRows from '../ui/LoadingRows.svelte';
  import PageHeader from '../ui/PageHeader.svelte';
  import Pill from '../ui/Pill.svelte';
  import OwnerEditModal from './OwnerEditModal.svelte';
  import { ageText, fullName, subtitle } from './util';

  type Tab = 'activos' | 'pendientes' | 'archivados';
  const TABS: { id: Tab; label: string; param: string }[] = [
    { id: 'activos', label: 'Activos', param: '' },
    { id: 'pendientes', label: 'Pendientes', param: '?pendientes=1' },
    { id: 'archivados', label: 'Archivados', param: '?archivados=1' }
  ];

  const tab = $derived<Tab>(page.url.searchParams.get('pendientes') === '1' ? 'pendientes' : page.url.searchParams.get('archivados') === '1' ? 'archivados' : 'activos');
  const canCreate = $derived(session.has(PERMISSIONS.adminHistorials));

  const canEditOwner = $derived(session.has(PERMISSIONS.adminHistorials) || session.has(PERMISSIONS.adminAppointments));

  // Clinics with animals can read the list by owner (one card per owner with all of their pets) or by patient.
  type View = 'owner' | 'patient';
  let animals = $state(false);
  let view = $state<View>('owner');
  try {
    if (localStorage.getItem('caresia_patients_view') === 'patient') view = 'patient';
  } catch {
    /* no storage: the default view */
  }
  function setView(v: View) {
    view = v;
    try {
      localStorage.setItem('caresia_patients_view', v);
    } catch {
      /* ignore */
    }
  }
  onMount(() => {
    api.patients.schema().then((s) => (animals = s.subjects.includes('animal')), () => {});
  });
  const byOwner = $derived(animals && view === 'owner' && tab !== 'pendientes');
  let groups = $state<OwnerGroup[]>([]);
  let orphans = $state<PatientRow[]>([]);
  let editingOwner = $state<OwnerRef | null>(null);
  let reloadKey = $state(0);

  // archive / reactivate from the list (the expediente has the same buttons)
  const canArchive = $derived(session.has(PERMISSIONS.adminHistorials));
  let archiveFor = $state<{ id: string; name: string; restore: boolean } | null>(null);
  let archiveReason = $state('');
  const archiveOp = new Op();
  function askArchive(id: string, name: string) {
    archiveReason = '';
    archiveOp.reset();
    archiveFor = { id, name, restore: tab === 'archivados' };
  }
  async function confirmArchive() {
    const t = archiveFor;
    if (!t) return;
    if (!t.restore && !archiveReason.trim()) return archiveOp.fail('Escribe el motivo.');
    const ok = await archiveOp.run(async () => {
      if (t.restore) await api.patients.unarchive(t.id);
      else await api.patients.archive(t.id, archiveReason.trim());
    });
    if (!ok) return;
    toast.show(t.restore ? 'Expediente reactivado' : 'Expediente archivado');
    archiveFor = null;
    reloadKey++;
  }

  let search = $state('');
  let items = $state<PatientRow[]>([]);
  let loading = $state(true);
  let loadError = $state('');
  let seq = 0;

  // reload when the tab or the (debounced) search changes
  $effect(() => {
    const t = tab;
    const q = search.trim();
    const grouped = byOwner;
    reloadKey;
    const mine = ++seq;
    loading = true;
    const timer = setTimeout(
      async () => {
        try {
          const [r, g] = await Promise.all([
            api.patients.list({ q, archived: t === 'archivados', pending: t === 'pendientes' }),
            grouped ? ownersApi.grouped({ q, archived: t === 'archivados' }) : Promise.resolve(null)
          ]);
          if (mine !== seq) return;
          items = r;
          groups = g?.groups ?? [];
          orphans = g?.orphans ?? [];
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

  /** In the by-owner view the flat table only keeps the people (animals are in the owner cards). */
  const rows = $derived(byOwner ? items.filter((p) => p.subject !== 'animal') : items);
  const empty = $derived(byOwner ? groups.length === 0 && orphans.length === 0 && rows.length === 0 : items.length === 0);
  const petHref = (id: string) => `/pacientes/${encodeURIComponent(id)}`;

  const href = (p: PatientRow) => `/pacientes/${encodeURIComponent(p.id)}`;
  /** a click anywhere on the row (age, phone, last visit...) opens the record; the links inside keep their own behavior */
  function openRow(e: MouseEvent, p: PatientRow) {
    if ((e.target as HTMLElement).closest('a, button')) return;
    if (e.metaKey || e.ctrlKey) window.open(href(p), '_blank');
    else void goto(href(p));
  }
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
  {#if animals && tab !== 'pendientes'}
    <div class="flex gap-1 rounded-full bg-app-ink/6 p-1" role="group" aria-label="Cómo ver la lista">
      <button type="button" aria-pressed={view === 'owner'} class="min-h-9 rounded-full px-4 text-sm font-medium transition {view === 'owner' ? 'bg-app-panel text-app-ink shadow-app' : 'text-app-muted hover:text-app-ink'}" onclick={() => setView('owner')}>Por propietario</button>
      <button type="button" aria-pressed={view === 'patient'} class="min-h-9 rounded-full px-4 text-sm font-medium transition {view === 'patient' ? 'bg-app-panel text-app-ink shadow-app' : 'text-app-muted hover:text-app-ink'}" onclick={() => setView('patient')}>Por mascota</button>
    </div>
  {/if}
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
  {#if loading && items.length === 0 && groups.length === 0}
    <LoadingRows />
  {:else if loadError}
    <Alert class="m-5">{loadError}</Alert>
  {:else if empty}
    <EmptyState icon={search.trim() ? 'search' : 'folder'} title={search.trim() ? 'Sin resultados' : tab === 'pendientes' ? 'Nada pendiente' : tab === 'archivados' ? 'No hay expedientes archivados' : 'Aún no hay pacientes'} text={emptyText}>
      {#if canCreate && tab === 'activos' && !search.trim()}<a href="/pacientes/nuevo" class="btn-primary"><Icon name="plus" size={18} stroke={2.2} />Nuevo paciente</a>{/if}
    </EmptyState>
  {:else}
    {#if byOwner && (groups.length > 0 || orphans.length > 0)}
      <ul class="divide-y divide-app-ink/8 {loading ? 'opacity-60' : ''}" aria-label="Propietarios y sus mascotas">
        {#each groups as g (g.owner.id)}
          <li class="px-4 py-4">
            <div class="flex flex-wrap items-start justify-between gap-2">
              <div class="min-w-0">
                <p class="flex items-center gap-2 font-semibold"><Icon name="user" size={17} class="flex-none text-app-primary" /><span class="truncate">{g.owner.name}</span></p>
                <p class="mt-0.5 text-sm text-app-muted">{[g.owner.phone, g.owner.email].filter(Boolean).join(' · ') || 'Sin datos de contacto'} · {g.pets.length === 1 ? '1 mascota' : `${g.pets.length} mascotas`}</p>
              </div>
              {#if canEditOwner && tab === 'activos'}
                <div class="flex gap-1.5">
                  <button type="button" class="btn-ghost !min-h-9 px-3 text-sm" onclick={() => (editingOwner = g.owner)}><Icon name="edit" size={15} />Datos del propietario</button>
                  {#if canCreate}<a href="/pacientes/nuevo?propietario={encodeURIComponent(g.owner.id)}" class="btn-secondary !min-h-9 px-3 text-sm"><Icon name="plus" size={15} stroke={2.2} />Mascota</a>{/if}
                </div>
              {/if}
            </div>
            <ul class="mt-3 grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
              {#each g.pets as pet (pet.id)}
                <li class="relative">
                  <a href={petHref(pet.id)} class="flex items-center gap-3 rounded-xl border py-2.5 pl-3 {canArchive ? 'pr-11' : 'pr-3'} transition hover:bg-app-ink/[0.04] focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-app-primary/20 {search.trim() && !pet.matched ? 'border-app-ink/8 opacity-60' : 'border-app-ink/12'}">
                    <span class="grid h-9 w-9 flex-none place-items-center rounded-full bg-app-primary/12 text-app-primary"><Icon name="paw" size={17} /></span>
                    <span class="min-w-0">
                      <span class="flex items-center gap-2"><span class="truncate font-semibold">{pet.names}</span>{#if pet.species}<span class="pill pill-info flex-none">{pet.species}</span>{/if}</span>
                      <span class="block truncate text-xs text-app-muted"><span class="font-mono">#{pet.file_number}</span> · {pet.last_visit ? `visita ${ago(pet.last_visit)}` : 'Sin consultas'}</span>
                    </span>
                  </a>
                  {#if canArchive}<span class="absolute right-1.5 top-1/2 -translate-y-1/2">{@render archiveBtn(pet.id, pet.names)}</span>{/if}
                </li>
              {/each}
            </ul>
          </li>
        {/each}
        {#if orphans.length > 0}
          <li class="px-4 py-4">
            <p class="font-semibold">Mascotas sin propietario registrado</p>
            <ul class="mt-3 grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
              {#each orphans as pet (pet.id)}
                <li class="relative"><a href={petHref(pet.id)} class="flex items-center gap-3 rounded-xl border border-app-ink/12 py-2.5 pl-3 hover:bg-app-ink/[0.04] {canArchive ? 'pr-11' : 'pr-3'}"><Icon name="paw" size={17} class="text-app-primary" /><span class="font-semibold">{pet.names}</span> <span class="font-mono text-xs text-app-muted">#{pet.file_number}</span></a>{#if canArchive}<span class="absolute right-1.5 top-1/2 -translate-y-1/2">{@render archiveBtn(pet.id, pet.names)}</span>{/if}</li>
              {/each}
            </ul>
          </li>
        {/if}
      </ul>
    {/if}
    {#if rows.length > 0 && byOwner && (groups.length > 0 || orphans.length > 0)}<p class="section-title border-t border-app-ink/10 px-4 pb-1 pt-4">Personas</p>{/if}
    {#if rows.length > 0}
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
            {#if canArchive}<th class="th w-12"><span class="sr-only">Acciones</span></th>{/if}
          </tr>
        </thead>
        <tbody class="divide-y divide-app-ink/8">
          {#each rows as p (p.id)}
            <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_noninteractive_element_interactions -->
            <tr class="cursor-pointer transition hover:bg-app-ink/[0.03]" onclick={(e) => openRow(e, p)}>
              <td class="td font-mono text-xs text-app-muted">{p.file_number}</td>
              <td class="td">
                <a href={href(p)} class="flex items-center gap-3 rounded-lg outline-none focus-visible:ring-4 focus-visible:ring-app-primary/20">
                  <span class="grid h-9 w-9 flex-none place-items-center rounded-full bg-app-primary/12 text-app-primary"><Icon name={p.subject === 'animal' ? 'paw' : 'user'} size={17} /></span>
                  <span class="min-w-0">
                    <span class="flex items-center gap-2"><span class="font-semibold">{fullName(p)}</span>{#if p.subject === 'animal' && p.species}<span class="pill pill-info flex-none">{p.species}</span>{/if}</span>
                    {#if p.subject === 'animal'}<span class="block text-xs text-app-muted">{[p.guardian_name ? `dueño: ${p.guardian_name}` : '', p.guardian_phone].filter(Boolean).join(' · ')}</span>{/if}
                    {#if p.incomplete || p.no_privacy_notice || p.archived_at}<span class="mt-1 flex flex-wrap gap-1.5">{@render pills(p)}</span>{/if}
                  </span>
                </a>
              </td>
              <td class="td whitespace-nowrap">{ageText(p.age) || '—'}</td>
              <td class="td whitespace-nowrap">{p.phone || '—'}</td>
              <td class="td whitespace-nowrap text-app-muted">{lastVisit(p)}</td>
              {#if canArchive}<td class="td w-12 text-right">{@render archiveBtn(p.id, fullName(p))}</td>{/if}
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <!-- phones -->
    <ul class="divide-y divide-app-ink/8 md:hidden {loading ? 'opacity-60' : ''}">
      {#each rows as p (p.id)}
        <li class="flex items-center">
          <a href={href(p)} class="flex min-w-0 flex-1 items-start gap-3 px-4 py-3.5 transition hover:bg-app-ink/[0.03] focus-visible:bg-app-ink/[0.04] focus-visible:outline-none">
            <span class="grid h-10 w-10 flex-none place-items-center rounded-full bg-app-primary/12 text-app-primary"><Icon name={p.subject === 'animal' ? 'paw' : 'user'} size={18} /></span>
            <span class="min-w-0 flex-1">
              <span class="flex flex-wrap items-center gap-x-2 gap-y-1 font-semibold">{fullName(p)} <span class="font-mono text-xs font-normal text-app-muted">#{p.file_number}</span>{#if p.subject === 'animal' && p.species}<span class="pill pill-info">{p.species}</span>{/if}</span>
              <span class="block text-sm text-app-muted">{subtitle(p, false) || 'Sin edad registrada'}</span>
              <span class="mt-0.5 block text-xs text-app-muted">{p.phone || 'Sin teléfono'} · {lastVisit(p)}</span>
              {#if p.incomplete || p.no_privacy_notice || p.archived_at}<span class="mt-1.5 flex flex-wrap gap-1.5">{@render pills(p)}</span>{/if}
            </span>
            <Icon name="arrow-right" size={18} class="mt-2.5 flex-none text-app-muted" />
          </a>
          {#if canArchive}<span class="pr-3">{@render archiveBtn(p.id, fullName(p))}</span>{/if}
        </li>
      {/each}
    </ul>
    {/if}
  {/if}
</div>

{#snippet archiveBtn(id: string, name: string)}
  {#if canArchive}
    <button type="button" class="icon-btn flex-none" title={tab === 'archivados' ? 'Reactivar' : 'Archivar'} aria-label="{tab === 'archivados' ? 'Reactivar' : 'Archivar'} a {name}" onclick={() => askArchive(id, name)}>
      <Icon name={tab === 'archivados' ? 'refresh' : 'ban'} size={17} />
    </button>
  {/if}
{/snippet}

<ConfirmModal open={archiveFor !== null} title={archiveFor?.restore ? 'Reactivar expediente' : 'Archivar expediente'} op={archiveOp} onconfirm={confirmArchive} onclose={() => (archiveFor = null)} confirmLabel={archiveFor?.restore ? 'Reactivar' : 'Archivar'}>
  {#if archiveFor?.restore}
    <p><strong class="text-app-ink">{archiveFor.name}</strong> volverá a aparecer entre los pacientes activos.</p>
  {:else if archiveFor}
    <p><strong class="text-app-ink">{archiveFor.name}</strong> dejará de aparecer entre los pacientes activos, pero <strong class="text-app-ink">no se borra</strong>: por norma (NOM-004) se conserva al menos 5 años desde el último acto médico. Puedes reactivarlo cuando quieras en la pestaña «Archivados».</p>
    <label class="label mt-4" for="list-arch-reason">Motivo</label>
    <input id="list-arch-reason" class="field" bind:value={archiveReason} autocomplete="off" placeholder="Ej. Se mudó de ciudad" />
  {/if}
</ConfirmModal>

<OwnerEditModal owner={editingOwner} onclose={() => (editingOwner = null)} onchanged={() => reloadKey++} />

<p class="mt-4 text-xs text-app-muted">Los expedientes no se borran: se archivan y se conservan al menos 5 años (NOM-004-SSA3-2012).</p>
