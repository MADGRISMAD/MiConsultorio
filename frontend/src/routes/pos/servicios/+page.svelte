<script lang="ts">
  import SectionTabs from '$lib/components/nav/SectionTabs.svelte';
  import { COBROS_TABS } from '$lib/components/nav/tabs';
  import Guard from '$lib/components/Guard.svelte';
  import CatalogPanel from '$lib/components/pos/catalog/CatalogPanel.svelte';
</script>

<svelte:head><title>Servicios y precios · Caresia</title></svelte:head>

<Guard title="Servicios y precios" permissions={['pos']} cobros>
  <SectionTabs tabs={COBROS_TABS} label="Cobros" />
  <CatalogPanel />
</Guard>
