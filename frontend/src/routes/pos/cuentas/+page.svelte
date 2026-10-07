<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import ReceivablesPanel from '$lib/components/pos/credit/ReceivablesPanel.svelte';
</script>

<svelte:head><title>Cuentas por cobrar · Caresia</title></svelte:head>

<Guard title="Cuentas por cobrar" permissions={['pos']} cobros>
  <ReceivablesPanel />
</Guard>
