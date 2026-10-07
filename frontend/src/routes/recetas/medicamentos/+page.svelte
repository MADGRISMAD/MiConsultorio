<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import ClinicMedicines from '$lib/components/rx/ClinicMedicines.svelte';
</script>

<svelte:head><title>Medicamentos de la clínica · Caresia</title></svelte:head>

<Guard title="Medicamentos de la clínica" permissions={['navHistorials', 'adminHistorials']}>
  <ClinicMedicines />
</Guard>
