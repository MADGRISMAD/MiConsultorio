<script lang="ts">
  import SectionTabs from '$lib/components/nav/SectionTabs.svelte';
  import { COBROS_TABS } from '$lib/components/nav/tabs';
  import Guard from '$lib/components/Guard.svelte';
  import ReportsPanel from '$lib/components/pos/reports/ReportsPanel.svelte';
</script>

<svelte:head><title>Reportes · Caresia</title></svelte:head>

<Guard title="Reportes" permissions={['posReports']} cobros>
  <SectionTabs tabs={COBROS_TABS} label="Cobros" />
  <ReportsPanel />
</Guard>
