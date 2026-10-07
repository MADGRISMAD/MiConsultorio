<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import SettingsPanel from '$lib/components/pos/settings/SettingsPanel.svelte';
</script>

<svelte:head><title>Ajustes de cobros · Caresia</title></svelte:head>

<Guard title="Ajustes de cobros" permissions={['posManage']} cobros>
  <SettingsPanel />
</Guard>
