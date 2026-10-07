<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import PageHeader from '$lib/components/ui/PageHeader.svelte';
  import ArcoPanel from '$lib/components/arco/ArcoPanel.svelte';

  let newOpen = $state(false);
</script>

<svelte:head><title>Solicitudes ARCO · Caresia</title></svelte:head>

<Guard title="Solicitudes ARCO" permissions={['adminUsers']}>
  <PageHeader title="Solicitudes ARCO" subtitle="Acceso, rectificación, cancelación y oposición: lo que las personas piden sobre sus datos personales.">
    {#snippet actions()}
      <button type="button" class="btn-primary" onclick={() => (newOpen = true)}><Icon name="plus" size={18} />Registrar solicitud</button>
    {/snippet}
  </PageHeader>
  <ArcoPanel bind:newOpen />
</Guard>
