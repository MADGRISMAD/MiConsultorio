<script lang="ts">
  import { api } from '$lib/api';
  import { goto } from '$app/navigation';
  import { session } from '$lib/session.svelte';
  import { POS_WINDOWS } from '$lib/pos';
  import { CLINIC_KINDS, PERMISSIONS, type Appointment } from '$lib/types';
  import Guard from '$lib/components/Guard.svelte';
  import Spinner from '$lib/components/Spinner.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import Landing from '$lib/landing/Landing.svelte';

  let appointments = $state<Appointment[]>([]);
  let patientCount = $state<number | null>(null);

  const canAppointments = $derived(session.has(PERMISSIONS.navAppointments) || session.has(PERMISSIONS.adminAppointments));
  const canPatients = $derived(session.has(PERMISSIONS.navHistorials) || session.has(PERMISSIONS.adminHistorials));

  $effect(() => {
    if (session.status === 'authenticated') {
      if (session.isPlatform) goto('/plataforma', { replaceState: true });
      else if (session.user?.setupPending) goto('/bienvenida', { replaceState: true });
    }
  });

  $effect(() => {
    if (session.status !== 'authenticated' || session.isPlatform || session.locked) return;
    if (canAppointments) api.appointments().then((a) => (appointments = a), () => {});
    if (canPatients) api.expedients().then((e) => (patientCount = e.length), () => {});
  });

  const today = new Date().toISOString().slice(0, 10);
  const upcoming = $derived(appointments.filter((a) => a.date >= today).slice(0, 5));
  const todayCount = $derived(appointments.filter((a) => a.date === today).length);
  const greeting = (() => {
    const h = new Date().getHours();
    return h < 12 ? 'Buenos días' : h < 19 ? 'Buenas tardes' : 'Buenas noches';
  })();
  const longDate = (d: string) => new Date(d + 'T12:00:00').toLocaleDateString('es-MX', { weekday: 'short', day: 'numeric', month: 'short' });

  const quick = $derived(
    [
      { label: 'Ver citas', href: '/admin/navegar-citas', icon: 'calendar', show: session.has(PERMISSIONS.navAppointments) },
      { label: 'Nueva cita', href: '/admin/admin-citas', icon: 'plus', show: session.has(PERMISSIONS.adminAppointments) },
      { label: 'Ver historiales', href: '/admin/navegar-historiales', icon: 'folder', show: session.has(PERMISSIONS.navHistorials) },
      { label: 'Nuevo expediente', href: '/admin/admin-historiales', icon: 'plus', show: session.has(PERMISSIONS.adminHistorials) },
      { label: 'Equipo', href: '/equipo', icon: 'users', show: session.has(PERMISSIONS.adminUsers) }
    ].filter((q) => q.show) as { label: string; href: string; icon: 'calendar' | 'plus' | 'folder' | 'users'; show: boolean }[]
  );
</script>

<svelte:head>
  <title>{session.status === 'authenticated' ? 'Inicio · Caresia' : 'Caresia · Software para clínicas dentales y consultorios médicos'}</title>
</svelte:head>

{#if session.status === 'loading'}
  <Spinner />
{:else if session.status === 'authenticated'}
  <Guard title="Inicio">
    <section class="relative">
      <p class="section-title">{greeting}</p>
      <h1 class="display mt-3 text-[2.75rem] leading-[0.95] sm:text-6xl">Tu consultorio, <em class="italic text-app-primary">{session.user?.name?.split(' ')[0] || session.user?.username}.</em></h1>
      {#if session.clinic}
        <p class="mt-4 flex flex-wrap items-center gap-x-3 gap-y-1 text-[15px] text-app-muted">
          <strong class="font-medium text-app-ink">{session.clinic.name}</strong>
          <span class="badge">{CLINIC_KINDS[session.clinic.kind]?.label}</span>
          {#if session.clinic.phone_number}<span>{session.clinic.phone_number}</span>{/if}
          {#if session.clinic.address}<span>{session.clinic.address}</span>{/if}
        </p>
      {/if}
      {#if quick.length}
        <div class="mt-6 flex flex-wrap gap-2">
          {#each quick as q}
            <a href={q.href} class="btn-secondary"><Icon name={q.icon} size={18} />{q.label}</a>
          {/each}
        </div>
      {/if}
    </section>

    <div class="mt-9 grid gap-4 sm:grid-cols-3">
      {#if canAppointments}
        <div class="card p-6">
          <p class="section-title">Citas de hoy</p>
          <p class="display mt-3 text-6xl leading-none tabular-nums">{todayCount}</p>
        </div>
        <div class="card p-6">
          <p class="section-title">Próximas citas</p>
          <p class="display mt-3 text-6xl leading-none tabular-nums">{appointments.filter((a) => a.date >= today).length}</p>
        </div>
      {/if}
      {#if canPatients}
        <div class="card p-6">
          <p class="section-title">Pacientes</p>
          <p class="display mt-3 text-6xl leading-none tabular-nums">{patientCount ?? '—'}</p>
        </div>
      {/if}
    </div>

    <div class="mt-4 grid gap-4 lg:grid-cols-[1.4fr_1fr]">
      {#if canAppointments}
        <section class="card p-5 sm:p-6">
          <div class="mb-4 flex items-center justify-between">
            <h2 class="display text-3xl">Próximas citas</h2>
            <a href="/admin/navegar-citas" class="text-sm font-semibold text-app-primary hover:underline">Ver todas</a>
          </div>
          {#if upcoming.length === 0}
            <p class="rounded-xl bg-app-ink/5 px-4 py-8 text-center text-sm text-app-muted">No hay citas próximas.</p>
          {:else}
            <ul class="divide-y divide-app-ink/8">
              {#each upcoming as a (a.id)}
                <li class="flex items-center gap-4 py-3">
                  <span class="grid w-14 flex-none place-items-center rounded-xl bg-app-primary/12 py-1.5 text-center text-app-primary">
                    <span class="text-[11px] font-semibold uppercase leading-tight">{longDate(a.date).split(' ')[0]}</span>
                    <span class="text-lg font-semibold leading-tight">{a.date.slice(8)}</span>
                  </span>
                  <span class="min-w-0 flex-1">
                    <span class="block truncate font-semibold">{a.names} {a.last_names}</span>
                    <span class="block truncate text-sm text-app-muted">{a.details || 'Sin detalles'}</span>
                  </span>
                  <span class="flex items-center gap-1.5 text-sm font-semibold tabular-nums"><Icon name="clock" size={16} class="text-app-muted" />{a.startHour}</span>
                </li>
              {/each}
            </ul>
          {/if}
        </section>
      {/if}

      {#if session.cobros}
      <section class="card p-5 sm:p-6 {canAppointments ? '' : 'lg:col-span-2'}">
        <div class="mb-4 flex items-center gap-2">
          <h2 class="display text-3xl">Cobros</h2>
          <span class="badge-soon">Próximamente</span>
        </div>
        <ul class="grid gap-1.5 {canAppointments ? '' : 'sm:grid-cols-2 lg:grid-cols-3'}">
          {#each POS_WINDOWS as w}
            <li>
              <a href="/pos/{w.slug}" class="flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-semibold text-app-muted transition hover:bg-app-ink/5 hover:text-app-ink">
                <Icon name={w.icon} size={19} />{w.title}
              </a>
            </li>
          {/each}
        </ul>
      </section>
      {/if}
    </div>
  </Guard>
{:else}
  <Landing />
{/if}
