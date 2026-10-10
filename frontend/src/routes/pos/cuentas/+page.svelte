<script lang="ts">
  import SectionTabs from '$lib/components/nav/SectionTabs.svelte';
  import { COBROS_TABS } from '$lib/components/nav/tabs';
  import Guard from '$lib/components/Guard.svelte';
  import ReceivablesPanel from '$lib/components/pos/credit/ReceivablesPanel.svelte';
</script>

<svelte:head><title>Cuentas por cobrar · Caresia</title></svelte:head>

<Guard title="Cuentas por cobrar" permissions={['pos']} cobros>
  <SectionTabs tabs={COBROS_TABS} label="Cobros" />
  <ReceivablesPanel />
</Guard>
