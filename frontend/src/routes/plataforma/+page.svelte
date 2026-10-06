<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import PlatformOverview from '$lib/components/platform/PlatformOverview.svelte';
</script>

<svelte:head><title>Resumen de plataforma · Caresia</title></svelte:head>

<Guard title="Resumen" roles={['platform_admin', 'platform_support']}>
  <PlatformOverview />
</Guard>
