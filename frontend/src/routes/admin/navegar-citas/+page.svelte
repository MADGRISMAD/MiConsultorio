<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import AppointmentsPanel from '$lib/components/AppointmentsPanel.svelte';
</script>

<svelte:head><title>Citas · Caresia</title></svelte:head>

<Guard permissions={['navAppointments']}>
  <AppointmentsPanel />
</Guard>
