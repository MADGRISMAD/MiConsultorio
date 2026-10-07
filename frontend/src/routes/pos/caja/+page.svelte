<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import PosCash from '$lib/components/pos/cash/PosCash.svelte';
</script>

<svelte:head><title>Caja · Caresia</title></svelte:head>

<Guard title="Caja" permissions={['pos']} cobros>
  <PosCash />
</Guard>
