<script lang="ts">
  import type { Snippet } from 'svelte';
  import { goto } from '$app/navigation';
  import { session } from '$lib/session.svelte';
  import type { Role } from '$lib/types';
  import AppShell from './AppShell.svelte';
  import LockedNotice from './LockedNotice.svelte';
  import Spinner from './Spinner.svelte';
  import EmptyState from './ui/EmptyState.svelte';

  interface Props {
    /** Clinic capabilities: any one grants access. */
    permissions?: string[];
    /** Roles allowed (use for platform pages). */
    roles?: Role[];
    title?: string;
    /** Show the page even when the subscription is not active (e.g. "Mi cuenta"). */
    allowLocked?: boolean;
    children: Snippet;
  }
  let { permissions, roles, title, allowLocked = false, children }: Props = $props();

  const allowed = $derived(
    roles ? (session.user ? roles.includes(session.user.role) : false) : permissions ? permissions.some((p) => session.has(p)) : true
  );

  $effect(() => {
    if (session.status === 'anonymous') goto('/login', { replaceState: true });
  });
</script>

{#if session.status === 'authenticated'}
  <AppShell {title}>
    {#if !allowed}
      <div class="card mx-auto mt-10 max-w-md">
        <EmptyState icon="lock" title="Sin acceso" text="Tu rol no tiene permiso para ver esta página. Pídele a un administrador que te cambie el rol si lo necesitas.">
          <a href={session.home} class="btn-primary">Volver al inicio</a>
        </EmptyState>
      </div>
    {:else if session.locked && !allowLocked}
      <LockedNotice />
    {:else}
      {@render children()}
    {/if}
  </AppShell>
{:else}
  <Spinner />
{/if}
