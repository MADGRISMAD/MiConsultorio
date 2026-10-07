<script lang="ts">
  import { page } from '$app/state';
  import { api, ApiError } from '$lib/api';
  import Guard from '$lib/components/Guard.svelte';
  import PatientForm from '$lib/components/patients/PatientForm.svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import PageHeader from '$lib/components/ui/PageHeader.svelte';
  import { fullName } from '$lib/components/patients/util';
  import type { Patient } from '$lib/types';

  const id = $derived(page.params.id ?? '');
  let patient = $state<Patient | null>(null);
  let status = $state<'loading' | 'ok' | 'notFound' | 'error'>('loading');
  let message = $state('');

  $effect(() => {
    const current = id;
    status = 'loading';
    patient = null;
    api.patients.get(current).then(
      (p) => {
        patient = p;
        status = 'ok';
      },
      (e) => {
        if (e instanceof ApiError && e.status === 404) status = 'notFound';
        else {
          status = 'error';
          message = e instanceof Error ? e.message : 'No se pudo cargar el expediente.';
        }
      }
    );
  });
</script>

<svelte:head><title>Editar paciente · Caresia</title></svelte:head>

<Guard title="Pacientes" permissions={['adminHistorials']}>
  <a href="/pacientes/{id}" class="mb-3 inline-block text-sm text-app-muted hover:text-app-ink">← Expediente</a>
  {#if status === 'loading'}
    <div class="card"><LoadingRows /></div>
  {:else if status === 'notFound'}
    <div class="card"><EmptyState icon="search" title="No encontramos este expediente" text="Puede que el enlace sea incorrecto."><a href="/pacientes" class="btn-secondary">Ver pacientes</a></EmptyState></div>
  {:else if status === 'error'}
    <p class="alert" role="alert"><Icon name="alert" size={18} />{message}</p>
  {:else if patient}
    <PageHeader title="Editar expediente" subtitle="{fullName(patient)} · #{patient.file_number}" />
    {#key patient.id}<PatientForm {patient} />{/key}
  {/if}
</Guard>
