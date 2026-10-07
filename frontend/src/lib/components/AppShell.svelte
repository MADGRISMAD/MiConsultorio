<script lang="ts">
  import type { Snippet } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { session } from '$lib/session.svelte';
  import { theme } from '$lib/theme.svelte';
  import NotificationBell from './notifications/NotificationBell.svelte';
  import { CLINIC_KINDS, PERMISSIONS, ROLES, type Role } from '$lib/types';
  import Modal from './Modal.svelte';
  import Avatar from './ui/Avatar.svelte';
  import Brand from './ui/Brand.svelte';
  import Icon, { type IconName } from './ui/Icon.svelte';
  import Toasts from './ui/Toasts.svelte';

  interface NavItem {
    label: string;
    href: string;
    /** path prefix that marks the item as current when it differs from href */
    match?: string;
    icon: IconName;
    /** any one of these capabilities grants access; omitted = everyone signed in */
    perms?: string[];
    /** restrict to these roles */
    roles?: Role[];
    soon?: boolean;
    /** only on plans that include cobros */
    cobros?: boolean;
  }
  interface NavGroup {
    title: string;
    items: NavItem[];
  }

  const clinicGroups: NavGroup[] = [
    {
      title: 'Consultorio',
      items: [
        { label: 'Inicio', href: '/', icon: 'home' },
        { label: 'Citas', href: '/admin/navegar-citas', icon: 'calendar', perms: [PERMISSIONS.navAppointments] },
        { label: 'Lista de espera', href: '/agenda/espera', icon: 'clock-plus', perms: [PERMISSIONS.navAppointments] },
        { label: 'Avisos', href: '/avisos', icon: 'bell', perms: [PERMISSIONS.navAppointments, PERMISSIONS.pos, PERMISSIONS.posManage] },
        { label: 'Pacientes', href: '/pacientes', icon: 'folder', perms: [PERMISSIONS.navHistorials, PERMISSIONS.adminHistorials] },
        { label: 'Reportes clínicos', href: '/reportes', icon: 'chart', perms: [PERMISSIONS.adminUsers, PERMISSIONS.navHistorials] }
      ]
    },
    {
      title: 'Administración',
      items: [
        { label: 'Administrar citas', href: '/admin/admin-citas', icon: 'calendar', perms: [PERMISSIONS.adminAppointments] },
        { label: 'Equipo', href: '/equipo', icon: 'users', perms: [PERMISSIONS.adminUsers] },
        { label: 'Solicitudes ARCO', href: '/arco-solicitudes', icon: 'shield', perms: [PERMISSIONS.adminUsers] },
        { label: 'Suscripción y plan', href: '/suscripcion', icon: 'sparkles', perms: [PERMISSIONS.adminUsers] }
      ]
    },
    {
      title: 'Cobros',
      items: [
        { label: 'Punto de venta', href: '/pos/cobros', icon: 'cash', perms: [PERMISSIONS.pos], cobros: true },
        { label: 'Caja', href: '/pos/caja', icon: 'wallet', perms: [PERMISSIONS.pos], cobros: true },
        { label: 'Servicios y precios', href: '/pos/servicios', icon: 'tag', perms: [PERMISSIONS.pos], cobros: true },
        { label: 'Inventario', href: '/pos/inventario', icon: 'box', perms: [PERMISSIONS.pos], cobros: true },
        { label: 'Facturación', href: '/pos/facturacion', icon: 'receipt', perms: [PERMISSIONS.pos], cobros: true },
        { label: 'Cuentas por cobrar', href: '/pos/cuentas', icon: 'wallet', perms: [PERMISSIONS.pos], cobros: true },
        { label: 'Comisiones', href: '/pos/comisiones', icon: 'users', perms: [PERMISSIONS.posManage], cobros: true },
        { label: 'Reportes de ventas', href: '/pos/reportes', icon: 'chart', perms: [PERMISSIONS.posReports], cobros: true }
      ]
    }
  ];

  const platformGroups: NavGroup[] = [
    {
      title: 'Plataforma',
      items: [
        { label: 'Resumen', href: '/plataforma', icon: 'home' },
        { label: 'Negocios', href: '/plataforma/negocios', icon: 'building' }
      ]
    },
    {
      title: 'Administración',
      items: [
        { label: 'Equipo de plataforma', href: '/plataforma/equipo', icon: 'users', roles: ['platform_admin'] },
        { label: 'Actividad', href: '/plataforma/actividad', icon: 'activity', roles: ['platform_admin'] }
      ]
    }
  ];

  let { title, children }: { title?: string; children: Snippet } = $props();

  const allowed = (i: NavItem) =>
    (!i.perms || i.perms.some((p) => session.has(p))) && (!i.roles || (session.user ? i.roles.includes(session.user.role) : false)) &&
    (!i.cobros || session.cobros);

  const visible = $derived(
    (session.isPlatform ? platformGroups : clinicGroups)
      .map((g) => ({ ...g, items: g.items.filter(allowed) }))
      .filter((g) => g.items.length > 0)
  );

  let drawer = $state(false);
  let confirming = $state(false);

  $effect(() => {
    theme.init();
    void session.loadClinic();
  });
  // close the drawer after navigating
  $effect(() => {
    page.url.pathname;
    drawer = false;
  });

  const isActive = (href: string) => (href === '/' || href === '/plataforma' ? page.url.pathname === href : page.url.pathname.startsWith(href));
  const billing = $derived(session.user?.billing);

  async function signOut() {
    await session.logout();
    confirming = false;
    await goto('/');
  }
</script>

<div class="app" data-theme={theme.mode}>
  <!-- Sidebar: fixed on desktop, drawer on small screens -->
  {#if drawer}
    <button type="button" aria-label="Cerrar menú" class="fixed inset-0 z-30 bg-black/50 backdrop-blur-sm lg:hidden" onclick={() => (drawer = false)}></button>
  {/if}
  <aside
    class="fixed inset-y-0 left-0 z-40 flex w-72 flex-col border-r border-app-ink/10 bg-app-panel transition-transform duration-300 lg:translate-x-0 {drawer ? 'translate-x-0' : '-translate-x-full'}"
    aria-label="Navegación principal"
  >
    <a href={session.home} class="px-5 py-5" aria-label="Caresia, inicio"><Brand size={32} class="text-[1.05rem]" /></a>

    <nav class="flex-1 space-y-6 overflow-y-auto px-3 pb-4">
      {#each visible as g}
        <div>
          <p class="section-title px-3 pb-2">{g.title}</p>
          <ul class="space-y-0.5">
            {#each g.items as item}
              <li>
                <a
                  href={item.href}
                  aria-current={isActive(item.match ?? item.href.split('?')[0]) ? 'page' : undefined}
                  class="group flex items-center gap-3 rounded-xl px-3 py-2.5 text-[15px] font-medium text-app-muted transition hover:bg-app-ink/5 hover:text-app-ink aria-[current=page]:bg-app-primary/10 aria-[current=page]:text-app-primary"
                >
                  <Icon name={item.icon} size={20} />
                  <span class="flex-1">{item.label}</span>
                  {#if item.soon}<span class="badge-soon">Pronto</span>{/if}
                </a>
              </li>
            {/each}
          </ul>
        </div>
      {/each}
    </nav>

    <div class="border-t border-app-ink/10 p-3">
      <div class="flex items-center gap-1 rounded-xl p-1">
        <a href="/cuenta" class="flex min-w-0 flex-1 items-center gap-3 rounded-xl p-1.5 transition hover:bg-app-ink/5" title="Mi cuenta" aria-label="Mi cuenta" aria-current={page.url.pathname === '/cuenta' ? 'page' : undefined}>
          <Avatar name={session.user?.name || session.user?.username || '?'} size={40} />
          <span class="min-w-0 flex-1">
            <span class="block truncate text-sm font-semibold">{session.user?.name || session.user?.username}</span>
            <span class="block truncate text-xs text-app-muted">{session.user ? ROLES[session.user.role]?.label : ''}{session.clinic ? ` · ${session.clinic.name}` : ''}</span>
          </span>
        </a>
        <button type="button" class="icon-btn danger" title="Cerrar sesión" aria-label="Cerrar sesión" onclick={() => (confirming = true)}>
          <Icon name="logout" size={19} />
        </button>
      </div>
    </div>
  </aside>

  <div class="lg:pl-72" style="background-image: radial-gradient(ellipse 60% 360px at 50% 0%, rgb(var(--app-primary) / 0.08), transparent 75%); background-repeat: no-repeat">
    <header class="sticky top-0 z-20 px-3 pt-3 sm:px-5 lg:px-8">
      <div class="mx-auto flex h-14 max-w-7xl items-center gap-2 rounded-full bg-app-surface/75 pl-2 pr-2.5 shadow-[0_1px_0_rgba(11,37,64,0.05),0_12px_32px_-14px_rgba(11,37,64,0.2)] ring-1 ring-app-ink/5 backdrop-blur-xl sm:pl-4">
        <button type="button" class="icon-btn lg:hidden" aria-label="Abrir menú" aria-expanded={drawer} onclick={() => (drawer = true)}>
          <Icon name="menu" size={22} />
        </button>
        <div class="min-w-0 flex-1 px-1">
          {#if title}<p class="truncate font-mono text-[11px] uppercase tracking-[0.14em] text-app-muted">{title}</p>{/if}
        </div>
        {#if session.isPlatform}
          <span class="badge hidden sm:inline-flex">Plataforma</span>
        {:else if session.clinic}
          <span class="badge hidden sm:inline-flex">{CLINIC_KINDS[session.clinic.kind]?.label ?? session.clinic.kind}</span>
        {/if}
        {#if !session.isPlatform}<NotificationBell />{/if}
        <a
          href="/ajustes"
          class="icon-btn {page.url.pathname.startsWith('/ajustes') ? 'bg-app-primary/12 text-app-primary' : ''}"
          title="Ajustes"
          aria-label="Ajustes"
          aria-current={page.url.pathname.startsWith('/ajustes') ? 'page' : undefined}
        >
          <Icon name="settings" size={20} />
        </a>
        <button
          type="button"
          class="icon-btn"
          title={theme.mode === 'dark' ? 'Tema claro' : 'Tema oscuro'}
          aria-label={theme.mode === 'dark' ? 'Cambiar a tema claro' : 'Cambiar a tema oscuro'}
          onclick={() => theme.toggle()}
        >
          <Icon name={theme.mode === 'dark' ? 'sun' : 'moon'} size={20} />
        </button>
      </div>
    </header>

    {#if billing?.state === 'trialing' && billing.trial_days_left !== null}
      <div class="mx-auto mt-3 max-w-7xl px-3 sm:px-5 lg:px-8" role="status">
        <p class="flex items-center gap-2 rounded-2xl bg-app-primary/10 px-4 py-2.5 text-sm text-app-primary">
          <Icon name="sparkles" size={17} class="flex-none" />
          <span>Estás en tu prueba gratuita: {billing.trial_days_left === 0 ? 'termina hoy' : billing.trial_days_left === 1 ? 'te queda 1 día' : `te quedan ${billing.trial_days_left} días`}.</span>
        </p>
      </div>
    {/if}

    {#if session.user?.mustSetup2fa}
      <div class="border-b border-app-warning/30 bg-app-warning/10 px-4 py-2.5 text-center text-sm" role="alert">
        Tu consultorio exige verificación en dos pasos para ver datos clínicos. <a href="/cuenta" class="font-semibold underline">Actívala en Mi cuenta</a>.
      </div>
    {/if}

    <main class="page-fade mx-auto w-full max-w-7xl px-4 pb-10 pt-8 sm:px-6 lg:px-8">
      {@render children()}
    </main>
  </div>

  <Modal open={confirming} title="¿Cerrar sesión?" onclose={() => (confirming = false)}>
    <p class="text-app-muted">Tendrás que volver a iniciar sesión para continuar.</p>
    {#snippet footer()}
      <button type="button" class="btn-secondary" onclick={() => (confirming = false)}>Cancelar</button>
      <button type="button" class="btn-danger" onclick={signOut}>Cerrar sesión</button>
    {/snippet}
  </Modal>

  <Toasts />
</div>
