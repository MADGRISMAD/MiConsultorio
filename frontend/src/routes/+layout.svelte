<script lang="ts">
  import '../app.css';
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
