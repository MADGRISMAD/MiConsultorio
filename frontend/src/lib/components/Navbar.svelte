<script lang="ts">
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { session } from '$lib/session.svelte';
  import { PERMISSIONS } from '$lib/types';
  import Wordmark from '$lib/landing/Wordmark.svelte';
  import Modal from './Modal.svelte';

  const links = [
    [PERMISSIONS.navAppointments, 'Navegar Citas', '/admin/navegar-citas'],
    [PERMISSIONS.navHistorials, 'Navegar Historiales', '/admin/navegar-historiales'],
    [PERMISSIONS.adminHistorials, 'Admin. Historiales', '/admin/admin-historiales'],
    [PERMISSIONS.adminAppointments, 'Admin. Citas', '/admin/admin-citas'],
    [PERMISSIONS.adminUsers, 'Admin. Usuarios', '/admin/admin-usuario']
  ] as const;

  let confirming = $state(false);
  let menuOpen = $state(false);

  async function signOut() {
    await session.logout();
    confirming = false;
    await goto('/');
  }
</script>

<header class="bg-ink text-paper">
  <div class="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-x-6 gap-y-2 px-4 py-3">
    <a href="/" class="flex items-center gap-2.5" aria-label="Caresia, inicio">
      <Wordmark light />
    </a>

    <button
      type="button"
      class="rounded-lg px-3 py-2 text-sm ring-1 ring-paper/30 md:hidden"
      aria-expanded={menuOpen}
      aria-controls="main-nav"
      onclick={() => (menuOpen = !menuOpen)}>Menú</button
    >

    <nav id="main-nav" aria-label="Principal" class="{menuOpen ? 'flex' : 'hidden'} w-full flex-col gap-1 md:flex md:w-auto md:flex-row md:items-center">
      {#each links as [perm, label, href]}
        {#if session.has(perm)}
          <a
            {href}
            aria-current={page.url.pathname.startsWith(href) ? 'page' : undefined}
            class="rounded-lg px-3 py-2 text-sm transition-colors hover:bg-paper/10 aria-[current=page]:bg-paper/15 aria-[current=page]:font-medium"
            onclick={() => (menuOpen = false)}>{label}</a
          >
        {/if}
      {/each}
    </nav>

    <button type="button" class="btn rounded-full bg-red-500 text-white hover:bg-red-600" onclick={() => (confirming = true)}>Cerrar sesión</button>
  </div>
</header>

<Modal open={confirming} title="¿Seguro que quieres cerrar sesión?" onclose={() => (confirming = false)}>
  <p class="text-ink-soft">Tendrás que volver a iniciar sesión para continuar.</p>
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={() => (confirming = false)}>No</button>
    <button type="button" class="btn-danger" onclick={signOut}>Sí</button>
  {/snippet}
</Modal>
