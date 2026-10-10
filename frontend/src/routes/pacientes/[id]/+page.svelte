<script lang="ts">
  import { Loader } from '$lib/loader.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onMount } from 'svelte';
  import { rxApi } from '$lib/api/rx';
  import { certificatesApi } from '$lib/api/certificates';
  import { labApi } from '$lib/api/lab';
  import { filesApi } from '$lib/api/files';
  import { specialtyApi } from '$lib/api/specialty';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { api, ApiError } from '$lib/api';
  import ExportButton from '$lib/components/patients/ExportButton.svelte';
  import AccessTab from '$lib/components/patients/detail/AccessTab.svelte';
  import AddendumModal from '$lib/components/patients/detail/AddendumModal.svelte';
  import EncounterForm from '$lib/components/patients/detail/EncounterForm.svelte';
  import EncountersTab from '$lib/components/patients/detail/EncountersTab.svelte';
  import PrescriptionBuilder from '$lib/components/patients/detail/PrescriptionBuilder.svelte';
  import PrescriptionsTab from '$lib/components/patients/detail/PrescriptionsTab.svelte';
  import FilesTab from '$lib/components/patients/detail/FilesTab.svelte';
  import LabTab from '$lib/components/patients/detail/LabTab.svelte';
  import GrowthTab from '$lib/components/patients/detail/GrowthTab.svelte';
  import VaccinesTab from '$lib/components/patients/detail/VaccinesTab.svelte';
  import OdontogramTab from '$lib/components/patients/detail/OdontogramTab.svelte';
  import NutritionPlanTab from '$lib/components/patients/detail/NutritionPlanTab.svelte';
  import PsychologyTab from '$lib/components/patients/detail/PsychologyTab.svelte';
  import BodyMapTab from '$lib/components/patients/detail/BodyMapTab.svelte';
  import PlansTab from '$lib/components/patients/detail/PlansTab.svelte';
  import ArcoTab from '$lib/components/patients/detail/ArcoTab.svelte';
  import { arcoDone } from '$lib/components/arco/labels';
  import { ageFrom } from '$lib/components/patients/util';
  import { arcoApi } from '$lib/api/arco';
  import type { ArcoRequest } from '$lib/types/arco';
  import CertificatesTab from '$lib/components/patients/detail/CertificatesTab.svelte';
  import ChronicMedsTab from '$lib/components/patients/detail/ChronicMedsTab.svelte';
  import SummaryTab from '$lib/components/patients/detail/SummaryTab.svelte';
  import ConfirmModal from '$lib/components/ConfirmModal.svelte';
  import Guard from '$lib/components/Guard.svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import Pill from '$lib/components/ui/Pill.svelte';
  import { Op } from '$lib/op.svelte';
  import { printConsent, printExpediente, printPrivacyNotice } from '$lib/print';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { AccessEntry, Encounter, Patient, PatientSchema, Prescription } from '$lib/types';

  type Tab = 'resumen' | 'bitacora' | 'recetas' | 'cronicos' | 'certificados' | 'arco' | 'archivos' | 'laboratorio' | 'crecimiento' | 'vacunas' | 'odontograma' | 'esquema' | 'nutricion' | 'psico' | 'planes' | 'accesos';

  const id = $derived(page.params.id ?? '');
  let patient = $state<Patient | null>(null);
  let schema = $state<PatientSchema | null>(null);
  let encounters = $state<Encounter[]>([]);
  let prescriptions = $state<Prescription[]>([]);
  let access = $state<AccessEntry[]>([]);
  const ld = new Loader('No se pudo abrir el expediente.');
  let notFound = $state(false);
  let tab = $state<Tab>('resumen');

  const canWrite = $derived(session.has('adminHistorials'));
  const isAdmin = $derived(session.has('adminUsers'));

  let formOpen = $state(false);
  let prefill = $state<{ appointment_id?: string; reason?: string } | undefined>();
  let addendumFor = $state<Encounter | null>(null);
  let rxOpen = $state(false);
  let rxEncounter = $state<{ id?: string; diagnosis?: string; nextVisit?: string }>({});

  /** Giros that order or interpret lab studies. The others keep their studies in Archivos. */
  const LAB_KINDS = ['GENERAL_MEDICAL', 'INTERNAL_MEDICINE', 'PEDIATRICS', 'GYNECOLOGY', 'DERMATOLOGY', 'ORTHOPEDICS', 'VETERINARY'];
  let hasLabData = $state(false);
  /** Costed treatment plans make sense for these giros; nutrition and psychology work with a menu or sessions instead. */
  const PLAN_KINDS = ['DENTAL', 'CHIROPRACTIC', 'PHYSIOTHERAPY', 'ORTHOPEDICS', 'VETERINARY'];
  let hasPlanData = $state(false);

  async function load() {
    await ld.run(async () => {
      try {
        const [p, s, e, r] = await Promise.all([api.patients.get(id), api.patients.schema(), api.patients.encounters(id), api.patients.prescriptions(id)]);
        patient = p;
        schema = s;
        encounters = e;
        prescriptions = r;
        if (!(s.kinds ?? []).some((k) => LAB_KINDS.includes(k))) labApi.orders(id).then((o) => (hasLabData = o.length > 0)).catch(() => {});
        if (!(s.kinds ?? []).some((k) => PLAN_KINDS.includes(k))) specialtyApi.plans(id).then((l) => (hasPlanData = l.length > 0)).catch(() => {});
        if (isAdmin) api.patients.access(id).then((a) => (access = a)).catch(() => {});
      } catch (err) {
        if (err instanceof ApiError && err.status === 404) notFound = true;
        else throw err;
      }
    });
  }
  onMount(async () => {
    const q = page.url.searchParams;
    const cita = q.get('cita');
    await load();
    if (cita && patient && canWrite) {
      prefill = { appointment_id: cita, reason: q.get('motivo') ?? '' };
      tab = 'bitacora';
      formOpen = true;
      goto(page.url.pathname, { replaceState: true, noScroll: true });
    }
  });

  // How many records each tab holds (the first two come with the page). Refreshed whenever a tab is opened,
  // so what was just saved in one shows in its counter.
  let counts = $state<Record<string, number>>({});
  /** the patient's ARCO requests (administrators only): the tab and the "realizada" mark */
  let arco = $state<ArcoRequest[]>([]);
  async function loadCounts() {
    const set = (key: string, p: Promise<number>) => p.then((n) => (counts[key] = n)).catch(() => {});
    const chartCount = (kind: Parameters<typeof specialtyApi.charts>[1]) => specialtyApi.charts(id, kind).then((r) => r.charts.length);
    const k = schema?.kinds ?? [];
    await Promise.all([
      ...(isPerson ? [set('cronicos', rxApi.chronic.list(id).then((l) => l.filter((m) => m.active).length))] : []),
      set('certificados', certificatesApi.list(id).then((r) => r.certificates.filter((c) => !c.voided_at).length)),
      ...(isAdmin ? [arcoApi.list({ patient: id, status: '' }).then((r) => (arco = r.requests)).catch(() => {})] : []),
      set('archivos', filesApi.list(id).then((r) => r.files.length)),
      set('vacunas', specialtyApi.vaccinations(id).then((r) => r.vaccinations.filter((v) => !v.voided_at).length)),
      set('laboratorio', labApi.orders(id).then((o) => o.length)),
      set('planes', specialtyApi.plans(id).then((l) => l.length)),
      ...(k.includes('DENTAL') ? [set('odontograma', chartCount('odontogram'))] : []),
      ...(k.includes('NUTRITION') ? [set('nutricion', chartCount('nutrition_plan'))] : []),
      ...(k.includes('PSYCHOLOGY') ? [set('psico', chartCount('scale'))] : []),
      ...(k.some((x) => ['CHIROPRACTIC', 'PHYSIOTHERAPY', 'ORTHOPEDICS'].includes(x)) ? [set('esquema', chartCount('bodymap'))] : [])
    ]);
  }
  $effect(() => {
    tab; // opening a tab refreshes the counters
    if (schema) loadCounts();
  });

  const refreshEncounters = async () => (encounters = await api.patients.encounters(id));
  const refreshRx = async () => (prescriptions = await api.patients.prescriptions(id));

  function openForm() {
    prefill = undefined;
    formOpen = true;
  }
  async function saved(e: Encounter, thenRx: boolean) {
    formOpen = false;
    await refreshEncounters();
    if (thenRx) {
      rxEncounter = { id: e.id, diagnosis: e.assessment, nextVisit: e.next_visit ?? '' };
      rxOpen = true;
      tab = 'recetas';
    }
  }
  function newRx() {
    rxEncounter = {};
    rxOpen = true;
  }

  // ----- header actions -----
  const fullName = $derived(patient ? `${patient.names} ${patient.last_names}`.trim() : '');
  const animal = $derived(patient?.subject === 'animal');
  const species = $derived(animal && patient ? [patient.profile?.species ?? patient.profile?.especie, patient.profile?.breed ?? patient.profile?.raza].filter(Boolean).join(' · ') : '');
  const ageText = $derived(patient?.age == null ? '' : `${patient.age} ${patient.age === 1 ? 'año' : 'años'}`);

  let docsOpen = $state(false);
  let busy = $state('');
  async function run(key: string, fn: () => Promise<unknown>) {
    docsOpen = false;
    busy = key;
    try {
      await fn();
    } catch (e) {
      toast.show(e instanceof Error ? e.message : 'No se pudo completar la acción.', 'error');
    } finally {
      busy = '';
    }
  }
  const prof = $derived(session.user?.professional);
  const professional = $derived(session.user ? { name: session.user.name, cedula: prof?.cedula ?? '' } : undefined);

  const privacyOp = new Op();
  async function recordPrivacy() {
    if (await privacyOp.run(async () => (patient = await api.patients.privacy(id)))) toast.show('Aviso de privacidad registrado');
    else toast.show(privacyOp.message, 'error');
  }

  let archiveOpen = $state(false);
  let archiveReason = $state('');
  const archiveOp = new Op();
  async function archive() {
    if (!archiveReason.trim()) return archiveOp.fail('Escribe el motivo.');
    if (await archiveOp.run(async () => (patient = await api.patients.archive(id, archiveReason.trim())))) {
      archiveOpen = false;
      toast.show('Expediente archivado');
    }
  }
  async function unarchive() {
    await run('unarchive', async () => {
      patient = await api.patients.unarchive(id);
      toast.show('Expediente reactivado');
    });
  }

  const isPerson = $derived(patient?.subject === 'person');
  const hasKind = (...k: string[]) => (schema?.kinds ?? []).some((x) => k.includes(x));
  // the specialty records of an area are for the people of that area (everyone else keeps the common record)
  const myAreas = $derived(session.user?.areas ?? []);
  const inMyAreas = (...k: string[]) => isAdmin || myAreas.length === 0 || k.some((x) => myAreas.includes(x));
  const labGiro = $derived(hasKind(...LAB_KINDS));
  const planGiro = $derived(hasKind(...PLAN_KINDS) || hasPlanData);
  // Crecimiento: consultorios con Pediatría, animales (en cualquier consultorio veterinario) y, en Nutrición, solo menores de 18 años
  const isMinor = $derived((ageFrom(patient?.birth_date?.slice(0, 10) ?? '') ?? 99) < 18);
  const showGrowth = $derived(hasKind('PEDIATRICS') || patient?.subject === 'animal' || (hasKind('NUTRITION') && isMinor));
  const tabs = $derived<{ key: Tab; label: string; count?: number }[]>([
    { key: 'resumen', label: 'Resumen' },
    { key: 'bitacora', label: 'Bitácora', count: encounters.filter((e) => !e.addendum_of).length },
    { key: 'recetas', label: schema?.rx_mode === 'instructions' ? 'Indicaciones' : 'Recetas', count: prescriptions.length },
    ...(isPerson ? [{ key: 'cronicos' as Tab, label: 'Medicación crónica', count: counts.cronicos }] : []),
    ...(patient?.subject === 'animal' ? [{ key: 'vacunas' as Tab, label: 'Vacunas y desparasitación', count: counts.vacunas }] : hasKind('PEDIATRICS') ? [{ key: 'vacunas' as Tab, label: 'Carnet de vacunación', count: counts.vacunas }] : []),
    ...(isPerson && hasKind('DENTAL') && inMyAreas('DENTAL') ? [{ key: 'odontograma' as Tab, label: 'Odontograma', count: counts.odontograma }] : []),
    ...(isPerson && hasKind('NUTRITION') && inMyAreas('NUTRITION') ? [{ key: 'nutricion' as Tab, label: 'Plan nutricional', count: counts.nutricion }] : []),
    ...(isPerson && hasKind('PSYCHOLOGY') && inMyAreas('PSYCHOLOGY') ? [{ key: 'psico' as Tab, label: 'Escalas y objetivos', count: counts.psico }] : []),
    ...(isPerson && hasKind('CHIROPRACTIC', 'PHYSIOTHERAPY', 'ORTHOPEDICS') && inMyAreas('CHIROPRACTIC', 'PHYSIOTHERAPY', 'ORTHOPEDICS') ? [{ key: 'esquema' as Tab, label: 'Esquema corporal', count: counts.esquema }] : []),
    ...(planGiro ? [{ key: 'planes' as Tab, label: 'Planes de tratamiento', count: counts.planes }] : hasKind('PSYCHOLOGY') ? [{ key: 'planes' as Tab, label: 'Consentimientos' }] : []),
    ...(labGiro || hasLabData ? [{ key: 'laboratorio' as Tab, label: 'Laboratorio', count: counts.laboratorio }] : []),
    ...(showGrowth ? [{ key: 'crecimiento' as Tab, label: 'Crecimiento', count: encounters.filter((e) => !e.hidden && ['weight_kg', 'height_cm'].some((k) => e.measures?.[k] != null && e.measures[k] !== '')).length }] : []),
    { key: 'certificados', label: 'Certificados', count: counts.certificados },
    ...(isAdmin && (arco.length > 0 || tab === 'arco') ? [{ key: 'arco' as Tab, label: 'Solicitudes ARCO', count: arco.length }] : []),
    { key: 'archivos', label: 'Archivos', count: counts.archivos },
    ...(isAdmin ? [{ key: 'accesos' as Tab, label: 'Accesos', count: access.length }] : [])
  ]);
  function tabKey(ev: KeyboardEvent) {
    const i = tabs.findIndex((t) => t.key === tab);
    const n = ev.key === 'ArrowRight' ? i + 1 : ev.key === 'ArrowLeft' ? i - 1 : -1;
    if (n < 0) return;
    ev.preventDefault();
    tab = tabs[(n + tabs.length) % tabs.length].key;
    document.getElementById(`tab-${tab}`)?.focus();
  }
</script>

<svelte:head><title>{fullName ? `${fullName} · Expediente` : 'Expediente'} · Caresia</title></svelte:head>
<svelte:window onclick={() => (docsOpen = false)} />

<Guard permissions={['navHistorials', 'adminHistorials']} title="Expediente">
  <a href="/pacientes" class="btn-ghost -ml-3 mb-3"><Icon name="arrow-left" size={16} />Pacientes</a>

  {#if ld.loading}
    <div class="card"><LoadingRows /></div>
  {:else if notFound}
    <div class="card mx-auto mt-6 max-w-md"><EmptyState icon="search" title="No encontramos este paciente" text="Puede que el enlace sea incorrecto o que el expediente no pertenezca a tu consultorio."><a href="/pacientes" class="btn-primary">Ver pacientes</a></EmptyState></div>
  {:else if ld.error || !patient || !schema}
    <Alert>{ld.error || 'No se pudo abrir el expediente.'}</Alert>
  {:else}
    <header class="card mb-5 p-5 sm:p-7">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div class="min-w-0 flex-1 basis-72">
          <p class="section-title">Expediente No. {patient.file_number}</p>
          <h1 class="display mt-1 break-words text-[2.25rem] leading-[1.05] sm:text-5xl">{fullName}</h1>
          {#if animal && species}<span class="pill pill-info mt-2 inline-flex text-sm" aria-label="Especie: {species}"><Icon name="paw" size={14} />{species}</span>{/if}
          <p class="mt-2 text-[15px] text-app-muted">
            {ageText}{#if animal && patient.guardian_name}{ageText ? ' · ' : ''}Propietario: {patient.guardian_name}{/if}
          </p>
          {#if patient.phone || (animal && patient.guardian_phone) || patient.email}
            <p class="mt-1 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-app-muted">
              {#if patient.phone || (animal && patient.guardian_phone)}<span class="inline-flex items-center gap-1.5"><Icon name="phone" size={14} />{patient.phone || patient.guardian_phone}</span>{/if}
              {#if patient.email}<span class="inline-flex items-center gap-1.5 break-all"><Icon name="mail" size={14} />{patient.email}</span>{/if}
            </p>
          {/if}
          <div class="mt-3 flex flex-wrap gap-2">
            {#if patient.archived_at}<Pill tone="muted">Archivado</Pill>{/if}
            {#if arco.some(arcoDone)}<Pill tone="ok">Solicitud ARCO realizada</Pill>{:else if arco.some((r) => r.open || r.pending_execute)}<Pill tone="warn">Solicitud ARCO pendiente</Pill>{/if}
            {#if patient.incomplete}<Pill tone="warn">Alta rápida</Pill>{/if}
            {#if !patient.privacy_notice_at}<Pill tone="warn">Sin aviso de privacidad</Pill>{/if}
          </div>
        </div>

        <div class="flex flex-wrap items-center gap-2">
          {#if canWrite}<a href="/pacientes/{patient.id}/editar" class="btn-secondary"><Icon name="edit" size={16} />Editar datos</a>{/if}
          <button type="button" class="btn-secondary" disabled={busy === 'rec'} onclick={() => run('rec', () => printExpediente(patient!.id))}>
            {#if busy === 'rec'}<span class="spin"></span>{:else}<Icon name="folder" size={16} />{/if}Imprimir expediente
          </button>
          {#if isAdmin || canWrite}<ExportButton patientId={patient.id} fileNumber={patient.file_number} />{/if}
          <div class="relative">
            <button type="button" class="btn-secondary" aria-haspopup="menu" aria-expanded={docsOpen} onclick={(ev) => { ev.stopPropagation(); docsOpen = !docsOpen; }}>Documentos<Icon name="chevron-down" size={16} /></button>
            {#if docsOpen}
              <div role="menu" tabindex="-1" class="absolute right-0 z-20 mt-2 w-72 max-w-[calc(100vw-2rem)] rounded-2xl border border-app-ink/10 bg-app-panel p-2 shadow-app" onclick={(ev) => ev.stopPropagation()} onkeydown={(ev) => ev.key === 'Escape' && (docsOpen = false)}>
                <button type="button" role="menuitem" class="btn-ghost w-full justify-start" onclick={() => run('priv', () => printPrivacyNotice(patient))}>Aviso de privacidad</button>
                <button type="button" role="menuitem" class="btn-ghost w-full justify-start" onclick={() => run('cons', () => printConsent(patient, undefined, professional))}>Consentimiento informado</button>
                <p class="px-3 pb-2 pt-1 text-xs text-app-muted">Son formatos base: revísalos con tu asesor legal.</p>
              </div>
            {/if}
          </div>
          {#if canWrite}
            {#if patient.archived_at}
              <button type="button" class="btn-secondary" disabled={busy === 'unarchive'} onclick={unarchive}><Icon name="refresh" size={16} />Reactivar</button>
            {:else}
              <button type="button" class="btn-ghost" onclick={() => { archiveReason = ''; archiveOp.reset(); archiveOpen = true; }}><Icon name="ban" size={16} />Archivar</button>
            {/if}
          {/if}
        </div>
      </div>

      {#if !patient.privacy_notice_at && canWrite}
        <div class="mt-5 flex flex-wrap items-center justify-between gap-3 rounded-2xl bg-app-warning/10 px-4 py-3 text-sm">
          <span class="text-app-ink">Falta registrar que el paciente conoce el aviso de privacidad. Imprímelo desde Documentos y regístralo cuando lo firme.</span>
          <button type="button" class="btn-secondary" disabled={privacyOp.phase === 'loading'} onclick={recordPrivacy}><Icon name="shield" size={16} />Registrar aviso de privacidad</button>
        </div>
      {:else if patient.privacy_notice_at}
        <p class="mt-4 flex items-center gap-2 text-xs text-app-muted"><Icon name="shield" size={14} />Aviso de privacidad registrado el {new Date(patient.privacy_notice_at).toLocaleDateString('es-MX', { day: 'numeric', month: 'long', year: 'numeric' })}{patient.privacy_notice_by ? ` por ${patient.privacy_notice_by}` : ''}.</p>
      {/if}
      {#if patient.archived_at}
        <p class="mt-3 text-sm text-app-muted">Archivado el {new Date(patient.archived_at).toLocaleDateString('es-MX')}{patient.archive_reason ? `: ${patient.archive_reason}` : ''}. El expediente se conserva y no se borra.</p>
      {/if}
    </header>

    <div role="tablist" aria-label="Secciones del expediente" tabindex="-1" class="mb-5 flex gap-1 overflow-x-auto border-b border-app-ink/10" onkeydown={tabKey}>
      {#each tabs as t (t.key)}
        <button type="button" role="tab" id="tab-{t.key}" aria-selected={tab === t.key} aria-controls="panel-{t.key}" tabindex={tab === t.key ? 0 : -1}
          class="-mb-px min-h-11 shrink-0 whitespace-nowrap border-b-2 px-4 text-sm font-medium transition {tab === t.key ? 'border-app-primary text-app-primary' : 'border-transparent text-app-muted hover:text-app-ink'}"
          onclick={() => (tab = t.key)}>
          {t.label}{#if t.count}<span class="ml-1.5 rounded-full bg-app-ink/8 px-1.5 py-0.5 font-mono text-[11px]">{t.count}</span>{/if}
        </button>
      {/each}
    </div>

    <div role="tabpanel" id="panel-{tab}" aria-labelledby="tab-{tab}" tabindex="-1">
      {#if tab === 'resumen'}
        <SummaryTab {patient} {schema} {encounters} />
      {:else if tab === 'bitacora'}
        <EncountersTab {patient} {schema} {encounters} {canWrite} onnew={openForm} onaddendum={(e) => (addendumFor = e)} />
      {:else if tab === 'recetas'}
        <PrescriptionsTab {patient} {schema} {prescriptions} {canWrite} {isAdmin} userName={session.user?.name ?? ''} onnew={newRx} onchange={refreshRx} />
      {:else if patient && tab === 'arco' && isAdmin}
        <ArcoTab {patient} onchange={(r) => (arco = r)} />
      {:else if patient && tab === 'certificados'}
        <CertificatesTab {patient} {canWrite} />
      {:else if patient && tab === 'cronicos'}
        <ChronicMedsTab {patient} {canWrite} />
      {:else if patient && tab === 'archivos'}
        <FilesTab {patient} {schema} {canWrite} {isAdmin} />
      {:else if patient && tab === 'laboratorio'}
        <LabTab {patient} {schema} {canWrite} {isAdmin} />
      {:else if patient && tab === 'crecimiento'}
        <GrowthTab {patient} {schema} {canWrite} {isAdmin} />
      {:else if patient && tab === 'vacunas'}
        <VaccinesTab {patient} {schema} {canWrite} {isAdmin} />
      {:else if patient && tab === 'odontograma'}
        <OdontogramTab {patient} {schema} {canWrite} {isAdmin} />
      {:else if patient && tab === 'nutricion'}
        <NutritionPlanTab {patient} {canWrite} {encounters} onchange={async () => { await refreshEncounters(); }} />
      {:else if patient && tab === 'psico'}
        <PsychologyTab {patient} {canWrite} />
      {:else if patient && tab === 'esquema'}
        <BodyMapTab {patient} {schema} {canWrite} {isAdmin} />
      {:else if patient && tab === 'planes'}
        <PlansTab {patient} {schema} {canWrite} {isAdmin} plansOn={planGiro} />
      {:else if tab === 'accesos' && isAdmin}
        <AccessTab {access} />
      {/if}
    </div>

    <EncounterForm open={formOpen} {patient} {schema} {prefill} onclose={() => (formOpen = false)} onsaved={saved} />
    <AddendumModal target={addendumFor} onclose={() => (addendumFor = null)} onsaved={async () => { addendumFor = null; await refreshEncounters(); }} />
    <PrescriptionBuilder open={rxOpen} {patient} {schema} encounterId={rxEncounter.id} diagnosis={rxEncounter.diagnosis} nextVisit={rxEncounter.nextVisit} onclose={() => (rxOpen = false)} oncreated={refreshRx} />

    <ConfirmModal open={archiveOpen} title="Archivar expediente" op={archiveOp} onconfirm={archive} onclose={() => (archiveOpen = false)} confirmLabel="Archivar">
      <p>El expediente dejará de aparecer entre los pacientes activos, pero <strong class="text-app-ink">no se borra</strong>: por norma (NOM-004) se conserva al menos 5 años desde el último acto médico. Puedes reactivarlo cuando quieras.</p>
      <label class="label mt-4" for="arch-reason">Motivo</label>
      <input id="arch-reason" class="field" bind:value={archiveReason} autocomplete="off" placeholder="Ej. Se mudó de ciudad" />
    </ConfirmModal>
  {/if}
</Guard>
