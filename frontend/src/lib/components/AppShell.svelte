<script lang="ts">
  import type { Snippet } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { session } from '$lib/session.svelte';
  import { theme } from '$lib/theme.svelte';
  import { CLINIC_KINDS, PERMISSIONS } from '$lib/types';
  import Modal from './Modal.svelte';
  import BrandMark from './ui/BrandMark.svelte';
  import BrandName from './ui/BrandName.svelte';
  import Icon, { type IconName } from './ui/Icon.svelte';
  import Toasts from './ui/Toasts.svelte';

  interface NavItem {
    label: string;
    href: string;
    icon: IconName;
    /** any one of these grants access; omitted = everyone signed in */
    perms?: string[];
    soon?: boolean;
  }
  interface NavGroup {
    title: string;
    items: NavItem[];
  }

  const groups: NavGroup[] = [
    {
      title: 'Consultorio',
      items: [
        { label: 'Inicio', href: '/', icon: 'home' },
        { label: 'Citas', href: '/admin/navegar-citas', icon: 'calendar', perms: [PERMISSIONS.navAppointments] },
        { label: 'Historiales', href: '/admin/navegar-historiales', icon: 'folder', perms: [PERMISSIONS.navHistorials] }
      ]
    },
    {
      title: 'Administración',
      items: [
        { label: 'Administrar citas', href: '/admin/admin-citas', icon: 'calendar', perms: [PERMISSIONS.adminAppointments] },
        { label: 'Administrar historiales', href: '/admin/admin-historiales', icon: 'folder', perms: [PERMISSIONS.adminHistorials] },
        { label: 'Usuarios y permisos', href: '/admin/admin-usuario', icon: 'users', perms: [PERMISSIONS.adminUsers] }
      ]
    },
    {
      title: 'Cobros',
      items: [
        { label: 'Punto de venta', href: '/pos/cobros', icon: 'cash', soon: true },
        { label: 'Caja', href: '/pos/caja', icon: 'wallet', soon: true },
        { label: 'Servicios y precios', href: '/pos/servicios', icon: 'tag', soon: true },
        { label: 'Inventario', href: '/pos/inventario', icon: 'box', soon: true },
        { label: 'Facturación', href: '/pos/facturacion', icon: 'receipt', soon: true },
        { label: 'Reportes', href: '/pos/reportes', icon: 'chart', soon: true }
      ]
    }
  ];

  let { title, children }: { title?: string; children: Snippet } = $props();

  const visible = $derived(
    groups
      .map((g) => ({ ...g, items: g.items.filter((i) => !i.perms || i.perms.some((p) => session.has(p))) }))
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

  const isActive = (href: string) => (href === '/' ? page.url.pathname === '/' : page.url.pathname.startsWith(href));
  const initial = $derived((session.user?.username ?? '?').slice(0, 1).toUpperCase());

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
    <a href="/" class="flex items-center gap-3 px-5 py-5">
      <BrandMark size={38} />
      <span class="text-xl"><BrandName /></span>
    </a>

    <nav class="flex-1 space-y-6 overflow-y-auto px-3 pb-4">
      {#each visible as g}
        <div>
          <p class="section-title px-3 pb-2">{g.title}</p>
          <ul class="space-y-0.5">
            {#each g.items as item}
              <li>
                <a
                  href={item.href}
                  aria-current={isActive(item.href) ? 'page' : undefined}
                  class="group flex items-center gap-3 rounded-xl px-3 py-2.5 text-[15px] font-semibold text-app-muted transition hover:bg-app-ink/5 hover:text-app-ink aria-[current=page]:bg-app-primary/12 aria-[current=page]:text-app-primary"
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
      <div class="flex items-center gap-3 rounded-xl p-2">
        <span class="grid h-10 w-10 flex-none place-items-center rounded-full bg-app-primary/15 text-base font-extrabold text-app-primary">{initial}</span>
        <span class="min-w-0 flex-1">
          <span class="block truncate text-sm font-bold">{session.user?.username}</span>
          <span class="block truncate text-xs text-app-muted">{session.clinic?.name ?? '…'}</span>
        </span>
        <button type="button" class="icon-btn danger" title="Cerrar sesión" aria-label="Cerrar sesión" onclick={() => (confirming = true)}>
          <Icon name="logout" size={19} />
        </button>
      </div>
    </div>
  </aside>

  <div class="lg:pl-72">
    <header class="sticky top-0 z-20 flex h-16 items-center gap-3 border-b border-app-ink/10 bg-app-surface/85 px-4 backdrop-blur-xl sm:px-6 lg:px-8">
      <button type="button" class="icon-btn lg:hidden" aria-label="Abrir menú" aria-expanded={drawer} onclick={() => (drawer = true)}>
        <Icon name="menu" size={22} />
      </button>
      <div class="min-w-0 flex-1">
        {#if title}<p class="truncate text-sm font-bold">{title}</p>{/if}
      </div>
      {#if session.clinic}
        <span class="badge hidden sm:inline-flex">{CLINIC_KINDS[session.clinic.kind]?.label ?? session.clinic.kind}</span>
      {/if}
      <button
        type="button"
        class="icon-btn"
        title={theme.mode === 'dark' ? 'Tema claro' : 'Tema oscuro'}
        aria-label={theme.mode === 'dark' ? 'Cambiar a tema claro' : 'Cambiar a tema oscuro'}
        onclick={() => theme.toggle()}
      >
        <Icon name={theme.mode === 'dark' ? 'sun' : 'moon'} size={20} />
      </button>
    </header>

    <main class="page-fade mx-auto w-full max-w-7xl px-4 py-6 sm:px-6 lg:px-8 lg:py-8">
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
