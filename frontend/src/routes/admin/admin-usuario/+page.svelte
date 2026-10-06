<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import UsersPanel from '$lib/components/UsersPanel.svelte';
</script>

<svelte:head><title>Administrar usuarios · Caresia</title></svelte:head>

<Guard permissions={['adminUsers']}>
  <UsersPanel />
</Guard>
