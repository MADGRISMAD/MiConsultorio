<script lang="ts">
  import { page } from '$app/state';
  import Guard from '$lib/components/Guard.svelte';
  import PlatformClients from '$lib/components/platform/PlatformClients.svelte';
</script>

<svelte:head><title>Negocio · Caresia</title></svelte:head>

<Guard title="Negocios" roles={['platform_admin', 'platform_support']}>
  <PlatformClients id={page.params.id ?? ''} />
</Guard>
