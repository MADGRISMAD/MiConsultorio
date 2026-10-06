<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import ExpedientsPanel from '$lib/components/ExpedientsPanel.svelte';
</script>

<svelte:head><title>Historiales · Caresia</title></svelte:head>

<Guard title="Historiales" permissions={['navHistorials']}>
  <ExpedientsPanel />
</Guard>
