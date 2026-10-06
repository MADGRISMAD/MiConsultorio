<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import ExpedientsPanel from '$lib/components/ExpedientsPanel.svelte';
</script>

<svelte:head><title>Administrar historiales · Caresia</title></svelte:head>

<Guard permissions={['adminHistorials']}>
  <ExpedientsPanel admin/>
</Guard>
