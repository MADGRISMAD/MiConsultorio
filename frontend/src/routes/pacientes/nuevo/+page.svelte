<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import PatientForm from '$lib/components/patients/PatientForm.svelte';
  import PageHeader from '$lib/components/ui/PageHeader.svelte';
</script>

<svelte:head><title>Nuevo paciente · Caresia</title></svelte:head>

<Guard title="Pacientes" permissions={['adminHistorials']}>
  <a href="/pacientes" class="mb-3 inline-block text-sm text-app-muted hover:text-app-ink">← Pacientes</a>
  <PageHeader title="Nuevo paciente" subtitle="Registra al paciente y su expediente clínico." />
  <PatientForm />
</Guard>
