<script lang="ts">
  import '../app.css';
  // Fuentes propias (sin Google Fonts): se sirven desde el mismo dominio.
  import '@fontsource-variable/instrument-sans/wght.css';
  import '@fontsource-variable/jetbrains-mono/wght.css';
  import '@fontsource/instrument-serif/400.css';
  import '@fontsource/instrument-serif/400-italic.css';
  import '@fontsource/caveat/400.css';
  import { onMount } from 'svelte';
  import { session } from '$lib/session.svelte';
  import { pwa } from '$lib/pwa/pwa.svelte';

  let { children } = $props();

  onMount(() => {
    pwa.init();
    session.load();
  });
</script>

<svelte:head>
  <title>Caresia · Software para clínicas dentales y consultorios médicos</title>
</svelte:head>

{@render children()}
