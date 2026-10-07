<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import PosSale from '$lib/components/pos/sale/PosSale.svelte';
</script>

<svelte:head><title>Punto de venta · Caresia</title></svelte:head>

<Guard title="Punto de venta" permissions={['pos']} cobros>
  <PosSale />
</Guard>
