<script lang="ts">
  import { session } from '$lib/session.svelte';
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import type { Clinic } from '$lib/types';
  import Navbar from '$lib/components/Navbar.svelte';
  import Spinner from '$lib/components/Spinner.svelte';
  import Landing from '$lib/landing/Landing.svelte';

  let clinic = $state<Clinic | null>(null);
  let error = $state('');

  $effect(() => {
    if (session.status !== 'authenticated' || clinic) return;
    api.clinic().then(
      (c) => (clinic = c),
      (e) => (error = e.message)
    );
  });
</script>

{#if session.status === 'loading'}
  <Spinner />
{:else if session.status === 'authenticated'}
  <Navbar />
  <main class="mx-auto mt-16 max-w-4xl rounded-2xl bg-white p-8 shadow-lg ring-1 ring-ink/10">
    <h1 class="text-center font-display text-4xl">Bienvenido {session.user?.username} a:</h1>
    {#if clinic}
      <h2 class="mt-2 text-center font-display text-3xl">Página administrativa de {clinic.name}</h2>
      {#if clinic.image_url}
        <img src={clinic.image_url} alt="Imagen de la clínica" class="mx-auto mt-6 max-h-64 rounded-xl object-cover" />
      {/if}
      <p class="mt-6 text-center text-lg text-ink-soft">{[clinic.phone_number, clinic.address].filter(Boolean).join(' • ')}</p>
    {:else if error}
      <p class="mt-6 text-center text-red-600" role="alert">{error}</p>
    {:else}
      <p class="mt-6 text-center text-ink-soft">Cargando...</p>
    {/if}
  </main>
{:else}
  <Landing />
{/if}
