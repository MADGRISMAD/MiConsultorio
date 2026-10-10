<script lang="ts">
  import SectionTabs from '$lib/components/nav/SectionTabs.svelte';
  import { COBROS_TABS } from '$lib/components/nav/tabs';
  import Guard from '$lib/components/Guard.svelte';
  import InvoicesPanel from '$lib/components/pos/billing/InvoicesPanel.svelte';
</script>

<svelte:head><title>Facturación · Caresia</title></svelte:head>

<Guard title="Facturación" permissions={['pos']} cobros>
  <SectionTabs tabs={COBROS_TABS} label="Cobros" />
  <InvoicesPanel />
</Guard>
