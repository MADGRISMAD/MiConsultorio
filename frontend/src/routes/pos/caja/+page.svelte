<script lang="ts">
  import SectionTabs from '$lib/components/nav/SectionTabs.svelte';
  import { COBROS_TABS } from '$lib/components/nav/tabs';
  import Guard from '$lib/components/Guard.svelte';
  import PosCash from '$lib/components/pos/cash/PosCash.svelte';
</script>

<svelte:head><title>Caja · Caresia</title></svelte:head>

<Guard title="Caja" permissions={['pos']} cobros>
  <SectionTabs tabs={COBROS_TABS} label="Cobros" />
  <PosCash />
</Guard>
