<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import UsersPanel from '$lib/components/UsersPanel.svelte';
</script>

<svelte:head><title>Usuarios y permisos · Caresia</title></svelte:head>

<Guard title="Usuarios y permisos" permissions={['adminUsers']}>
  <UsersPanel />
</Guard>
