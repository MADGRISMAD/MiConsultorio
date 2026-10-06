<script lang="ts">
  import { theme } from '$lib/theme.svelte';
  import Brand from '$lib/components/ui/Brand.svelte';

  $effect(() => theme.init());
</script>

<svelte:head><title>Aviso de privacidad · Caresia</title></svelte:head>

<div class="app fixed inset-0 overflow-y-auto px-4 py-10" data-theme={theme.mode}>
  <article class="card page-in mx-auto max-w-2xl px-6 py-9 sm:px-10">
    <a href="/" class="inline-flex" aria-label="Caresia, ir al inicio"><Brand size={32} class="text-[1.05rem]" /></a>
    <h1 class="display mt-7 text-5xl leading-none">Aviso de privacidad</h1>
    <p class="mt-4 text-app-muted">Este documento está en preparación y se publicará aquí antes del lanzamiento comercial.</p>
    <a href="/" class="btn-secondary mt-7">← Volver al inicio</a>
  </article>
</div>
