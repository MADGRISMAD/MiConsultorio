<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import InventoryPanel from '$lib/components/pos/catalog/InventoryPanel.svelte';
</script>

<svelte:head><title>Inventario · Caresia</title></svelte:head>

<Guard title="Inventario" permissions={['pos']} cobros>
  <InventoryPanel />
</Guard>
