<script lang="ts">
  import AccountPanel from '$lib/components/AccountPanel.svelte';
  import Guard from '$lib/components/Guard.svelte';
</script>

<svelte:head><title>Mi cuenta · Caresia</title></svelte:head>

<Guard title="Mi cuenta" allowLocked>
  <AccountPanel />
</Guard>
