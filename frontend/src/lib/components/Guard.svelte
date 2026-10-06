<script lang="ts">
  import type { Snippet } from 'svelte';
  import { goto } from '$app/navigation';
  import { session } from '$lib/session.svelte';
  import AppShell from './AppShell.svelte';
  import Spinner from './Spinner.svelte';
  import EmptyState from './ui/EmptyState.svelte';

  interface Props {
    /** Any one of these permissions grants access; omit to only require a session. */
    permissions?: string[];
    title?: string;
    children: Snippet;
  }
  let { permissions, title, children }: Props = $props();

  const allowed = $derived(!permissions || permissions.some((p) => session.has(p)));

  $effect(() => {
    if (session.status === 'anonymous') goto('/login', { replaceState: true });
  });
</script>

{#if session.status === 'authenticated'}
  <AppShell {title}>
    {#if allowed}
      {@render children()}
    {:else}
      <div class="card mx-auto mt-10 max-w-md">
        <EmptyState icon="lock" title="Sin acceso" text="Tu usuario no tiene permiso para ver esta página. Pídele a un administrador que te lo asigne.">
          <a href="/" class="btn-primary">Volver al inicio</a>
        </EmptyState>
      </div>
    {/if}
  </AppShell>
{:else}
  <Spinner />
{/if}
