<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import PageHeader from '$lib/components/ui/PageHeader.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import CommissionRulesSection from '$lib/components/pos/settings/CommissionRulesSection.svelte';
</script>

<svelte:head><title>Comisiones · Caresia</title></svelte:head>

<Guard title="Comisiones" permissions={['posManage']} cobros>
  <PageHeader title="Reglas de comisión" subtitle="Cuánto gana cada profesional por lo que cobra.">
    {#snippet actions()}
      <a class="btn-secondary" href="/pos/reportes"><Icon name="chart" size={16} />Ver reporte</a>
    {/snippet}
  </PageHeader>
  <CommissionRulesSection />
</Guard>
