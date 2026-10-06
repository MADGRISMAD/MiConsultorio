<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import AppointmentsPanel from '$lib/components/AppointmentsPanel.svelte';
</script>

<svelte:head><title>Administrar citas · Caresia</title></svelte:head>

<Guard title="Administrar citas" permissions={['adminAppointments']}>
  <AppointmentsPanel admin/>
</Guard>
