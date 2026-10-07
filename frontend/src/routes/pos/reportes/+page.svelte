<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import ReportsPanel from '$lib/components/pos/reports/ReportsPanel.svelte';
</script>

<svelte:head><title>Reportes · Caresia</title></svelte:head>

<Guard title="Reportes" permissions={['posReports']} cobros>
  <ReportsPanel />
</Guard>
