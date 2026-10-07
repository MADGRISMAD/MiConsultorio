<script lang="ts">
  import type { Patient, PatientSchema } from '$lib/types';

  let { patient, schema, canWrite, isAdmin }: { patient: Patient; schema: PatientSchema | null; canWrite: boolean; isAdmin: boolean } = $props();
</script>

<p class="text-sm text-app-muted">Próximamente.</p>
