<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import PlatformStaff from '$lib/components/platform/PlatformStaff.svelte';
</script>

<svelte:head><title>Equipo de plataforma · Caresia</title></svelte:head>

<Guard title="Equipo de plataforma" roles={['platform_admin']}>
  <PlatformStaff />
</Guard>
