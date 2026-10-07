<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import SettingsHub from '$lib/components/settings/SettingsHub.svelte';
</script>

<svelte:head><title>Ajustes · Caresia</title></svelte:head>

<!-- allowLocked: the person must still reach Suscripción, tema and cuenta when the plan lapsed -->
<Guard title="Ajustes" allowLocked>
  <SettingsHub />
</Guard>
