<script lang="ts">
  import { theme } from '$lib/theme.svelte';
  import BrandMark from '$lib/components/ui/BrandMark.svelte';
  import BrandName from '$lib/components/ui/BrandName.svelte';

  $effect(() => theme.init());
</script>

<svelte:head><title>Términos · Caresia</title></svelte:head>

<div class="app fixed inset-0 overflow-y-auto px-4 py-10" data-theme={theme.mode}>
  <article class="card page-in mx-auto max-w-2xl px-6 py-8 sm:px-10">
    <a href="/" class="inline-flex items-center gap-3 text-xl" aria-label="Caresia, ir al inicio"><BrandMark size={36} /><BrandName /></a>
    <h1 class="mt-6 text-2xl font-extrabold tracking-tight">Términos</h1>
    <p class="mt-3 text-app-muted">Este documento está en preparación y se publicará aquí antes del lanzamiento comercial.</p>
    <a href="/" class="btn-secondary mt-6">← Volver al inicio</a>
  </article>
</div>
