<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import PlatformClients from '$lib/components/platform/PlatformClients.svelte';
</script>

<svelte:head><title>Negocios · Caresia</title></svelte:head>

<Guard title="Negocios" roles={['platform_admin', 'platform_support']}>
  <PlatformClients />
</Guard>
