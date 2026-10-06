<script lang="ts">
  import type { Snippet } from 'svelte';
  import { goto } from '$app/navigation';
  import { session } from '$lib/session.svelte';
  import Navbar from './Navbar.svelte';
  import Spinner from './Spinner.svelte';

  interface Props {
    /** Any one of these permissions grants access; omit to only require a session. */
    permissions?: string[];
    children: Snippet;
  }
  let { permissions, children }: Props = $props();

  const allowed = $derived(!permissions || permissions.some((p) => session.has(p)));

  $effect(() => {
    if (session.status === 'anonymous') goto('/login', { replaceState: true });
  });
</script>

{#if session.status === 'authenticated'}
  <Navbar />
  <main>
    {#if allowed}
      {@render children()}
    {:else}
      <div class="mx-auto mt-24 max-w-md rounded-2xl bg-white p-8 text-center shadow">
        <h1 class="font-display text-3xl">Sin acceso</h1>
        <p class="mt-3 text-ink-soft">Tu usuario no tiene permiso para ver esta página.</p>
        <a href="/" class="btn-primary mt-6">Volver al inicio</a>
      </div>
    {/if}
  </main>
{:else}
  <Spinner />
{/if}
