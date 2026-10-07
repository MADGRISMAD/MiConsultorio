<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import CatalogPanel from '$lib/components/pos/catalog/CatalogPanel.svelte';
</script>

<svelte:head><title>Servicios y precios · Caresia</title></svelte:head>

<Guard title="Servicios y precios" permissions={['pos']} cobros>
  <CatalogPanel />
</Guard>
