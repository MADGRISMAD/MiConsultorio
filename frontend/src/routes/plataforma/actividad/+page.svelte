<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import PlatformActivity from '$lib/components/platform/PlatformActivity.svelte';
</script>

<svelte:head><title>Actividad · Caresia</title></svelte:head>

<Guard title="Actividad" roles={['platform_admin']}>
  <PlatformActivity />
</Guard>
