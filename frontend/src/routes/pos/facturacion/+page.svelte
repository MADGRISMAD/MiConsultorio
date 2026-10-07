<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import InvoicesPanel from '$lib/components/pos/billing/InvoicesPanel.svelte';
</script>

<svelte:head><title>Facturación · Caresia</title></svelte:head>

<Guard title="Facturación" permissions={['pos']} cobros>
  <InvoicesPanel />
</Guard>
