<script lang="ts">
  import { goto } from '$app/navigation';
  import { session, maybeSignedIn } from '$lib/session.svelte';
  import { PERMISSIONS } from '$lib/types';
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
  const displayName = $derived.by(() => {
    const name = session.user?.name?.trim() || session.user?.username || '';
    const title = session.user?.professional?.title?.trim();
    return title && !name.toLocaleLowerCase('es-MX').startsWith(title.toLocaleLowerCase('es-MX')) ? `${title} ${name}` : name;
  });
</script>

<svelte:head>
  <title>{session.status === 'authenticated' ? 'Inicio · Caresia' : 'Caresia · Software para clínicas dentales y consultorios médicos'}</title>
</svelte:head>

{#if session.status === 'loading' && maybeSignedIn()}
  <Spinner />
{:else if session.status === 'authenticated'}
  <Guard title="Inicio">
    <header class="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
      <h1 class="text-xl font-semibold sm:text-2xl">{greeting}, {displayName}</h1>
      {#if session.clinic}<p class="text-sm text-app-muted">{session.clinic.name}</p>{/if}
    </header>
    <div class="my-4 flex flex-wrap items-center gap-2">
      {#if canPatients}<div class="w-full min-w-0 sm:w-auto sm:flex-1"><PatientSearch /></div>{/if}
      {#if session.has(PERMISSIONS.adminAppointments)}<a href="/admin/admin-citas" class="btn-secondary min-h-11"><Icon name="plus" size={18} />Nueva cita</a>{/if}
      {#if session.has(PERMISSIONS.adminHistorials)}<a href="/pacientes" class="btn-secondary min-h-11"><Icon name="stethoscope" size={18} />Consulta sin cita</a>{/if}
    </div>
    {#if canAppointments}<TodayPanel />{:else}<AttentionStrip />{/if}
  </Guard>
{:else}
  <Landing />
{/if}
