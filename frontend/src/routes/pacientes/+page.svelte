<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import PatientsList from '$lib/components/patients/PatientsList.svelte';
</script>

<svelte:head><title>Pacientes · Caresia</title></svelte:head>

<Guard title="Pacientes" permissions={['navHistorials', 'adminHistorials']}>
  <PatientsList />
</Guard>
