<script lang="ts">
  import { goto } from '$app/navigation';
  import { session, maybeSignedIn } from '$lib/session.svelte';
  import { CLINIC_KINDS, PERMISSIONS } from '$lib/types';
  import Guard from '$lib/components/Guard.svelte';
  import Spinner from '$lib/components/Spinner.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import Landing from '$lib/landing/Landing.svelte';
  import PatientSearch from '$lib/components/home/PatientSearch.svelte';
  import AttentionStrip from '$lib/components/home/AttentionStrip.svelte';
  import TodayPanel from '$lib/components/home/TodayPanel.svelte';

  const canAppointments = $derived(session.has(PERMISSIONS.navAppointments) || session.has(PERMISSIONS.adminAppointments));
  const canPatients = $derived(session.has(PERMISSIONS.navHistorials) || session.has(PERMISSIONS.adminHistorials));
  $effect(() => {
    if (session.status === 'authenticated') {
      if (session.isPlatform) goto('/plataforma', { replaceState: true });
      else if (session.user?.setupPending) goto('/bienvenida', { replaceState: true });
    }
  });
  const greeting = (() => {
    const h = new Date().getHours();
    return h < 12 ? 'Buenos días' : h < 19 ? 'Buenas tardes' : 'Buenas noches';
  })();
  // «Dra. Ana» / «Lic. Luis»: the first name with its honorific; never the username when there is a name
  const HONORIFIC = /^(dr|dra|lic|ing|mtro|mtra|c|q\.?f\.?b|nut|psic|fis|odont)\.?$/i;
  const displayName = $derived.by(() => {
    const words = (session.user?.name?.trim() ?? '').split(/\s+/).filter(Boolean);
    if (!words.length) return (session.user?.username ?? '').replace(/\.$/, '');
    if (HONORIFIC.test(words[0]) && words[1]) return `${words[0].replace(/\.?$/, '.')} ${words[1]}`;
    const title = session.user?.professional?.title?.trim();
    const first = words[0].replace(/\.$/, '');
    return title && title.length <= 6 && !first.toLocaleLowerCase('es-MX').startsWith(title.toLocaleLowerCase('es-MX')) ? `${title} ${first}` : first;
  });
</script>

<svelte:head>
  <title>{session.status === 'authenticated' ? 'Inicio · Caresia' : 'Caresia · Software para clínicas dentales y consultorios médicos'}</title>
</svelte:head>

{#if session.status === 'loading' && maybeSignedIn()}
  <Spinner />
{:else if session.status === 'authenticated'}
  <Guard title="Inicio">
    <header>
      <p class="section-title">{greeting}</p>
      <h1 class="display mt-2 text-[2.25rem] leading-[1] sm:text-5xl">Tu consultorio, <em class="italic text-app-primary">{displayName}.</em></h1>
      {#if session.clinic}
        <p class="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1 text-[15px] text-app-muted">
          <strong class="font-medium text-app-ink">{session.clinic.name}</strong>
          <span class="badge">{CLINIC_KINDS[session.clinic.kind]?.label}</span>
        </p>
      {/if}
    </header>
    <div class="mb-4 mt-6 flex flex-wrap items-center gap-2">
      {#if canPatients}<div class="w-full min-w-0 sm:w-auto sm:flex-1"><PatientSearch /></div>{/if}
      {#if session.has(PERMISSIONS.adminAppointments)}<a href="/admin/admin-citas" class="btn-secondary min-h-11"><Icon name="plus" size={18} />Nueva cita</a>{/if}
      {#if session.has(PERMISSIONS.adminHistorials)}<a href="/pacientes" class="btn-secondary min-h-11"><Icon name="stethoscope" size={18} />Consulta sin cita</a>{/if}
    </div>
    {#if canAppointments}<TodayPanel />{:else}<AttentionStrip />{/if}
  </Guard>
{:else}
  <Landing />
{/if}
