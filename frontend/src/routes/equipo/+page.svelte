<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import TeamPanel from '$lib/components/TeamPanel.svelte';
</script>

<svelte:head><title>Equipo · Caresia</title></svelte:head>

<Guard title="Equipo" permissions={['adminUsers']}>
  <TeamPanel />
</Guard>
