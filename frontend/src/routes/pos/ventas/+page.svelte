<script lang="ts">
  import SectionTabs from '$lib/components/nav/SectionTabs.svelte';
  import { COBROS_TABS } from '$lib/components/nav/tabs';
  import Guard from '$lib/components/Guard.svelte';
  import SalesPanel from '$lib/components/pos/reports/SalesPanel.svelte';
</script>

<svelte:head><title>Ventas · Caresia</title></svelte:head>

<Guard title="Ventas" permissions={['pos']} cobros>
  <SectionTabs tabs={COBROS_TABS} label="Cobros" />
  <SalesPanel />
</Guard>
