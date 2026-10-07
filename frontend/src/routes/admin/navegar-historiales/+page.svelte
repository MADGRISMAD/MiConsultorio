<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';

  // Los historiales ahora viven en el módulo Pacientes.
  onMount(() => goto('/pacientes', { replaceState: true }));
</script>

<svelte:head><title>Pacientes · Caresia</title></svelte:head>
