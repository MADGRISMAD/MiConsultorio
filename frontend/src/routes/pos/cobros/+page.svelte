<script lang="ts">
  import SectionTabs from '$lib/components/nav/SectionTabs.svelte';
  import { COBROS_TABS } from '$lib/components/nav/tabs';
  import Guard from '$lib/components/Guard.svelte';
  import PosSale from '$lib/components/pos/sale/PosSale.svelte';
</script>

<svelte:head><title>Punto de venta · Caresia</title></svelte:head>

<Guard title="Punto de venta" permissions={['pos']} cobros>
  <SectionTabs tabs={COBROS_TABS} label="Cobros" />
  <PosSale />
</Guard>
