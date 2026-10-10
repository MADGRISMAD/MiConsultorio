<script lang="ts">
  import SectionTabs from '$lib/components/nav/SectionTabs.svelte';
  import { COBROS_TABS } from '$lib/components/nav/tabs';
  import Guard from '$lib/components/Guard.svelte';
  import InventoryPanel from '$lib/components/pos/catalog/InventoryPanel.svelte';
</script>

<svelte:head><title>Inventario · Caresia</title></svelte:head>

<Guard title="Inventario" permissions={['pos']} cobros>
  <SectionTabs tabs={COBROS_TABS} label="Cobros" />
  <InventoryPanel />
</Guard>
