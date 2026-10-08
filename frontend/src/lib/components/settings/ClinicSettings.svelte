<script lang="ts">
  import { api } from '$lib/api';
  import { summarizeHours } from '$lib/clinic';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { ClinicKind, ClinicSettings } from '$lib/types';
  import ClinicFields from '$lib/components/setup/ClinicFields.svelte';
  import HoursEditor from '$lib/components/setup/HoursEditor.svelte';
  import SpecialtyPicker from '$lib/components/setup/SpecialtyPicker.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';

  let ready = $state(false);
  let name = $state('');
  let phone = $state('');
  let address = $state('');
  let kind = $state<ClinicKind>('GENERAL_MEDICAL');
  let specialties = $state<ClinicKind[]>([]);
  let settings = $state<ClinicSettings>({ hours: {} as ClinicSettings['hours'], appointment_minutes: 30 });

  $effect(() => {
    if (session.status === 'authenticated') void session.loadClinic();
  });
  $effect(() => {
    const c = session.clinic;
    if (!c || ready) return;
    ({ name, kind } = c);
    phone = c.phone_number;
    address = c.address;
    specialties = [...c.specialties];
    settings = structuredClone($state.snapshot(c.settings));
    ready = true;
  });

  const dataOp = new Op();
  const kindOp = new Op();
  const hoursOp = new Op();
  async function save(op: Op, patch: Parameters<typeof api.updateClinic>[0], done: string) {
    if (await op.run(async () => session.setClinic(await api.updateClinic(patch)))) toast.show(done);
  }
</script>

{#if !ready}
  <div class="card"><LoadingRows /></div>
{:else}
  <div class="grid gap-4">
    <form class="card p-4 sm:p-6" onsubmit={(e) => { e.preventDefault(); void save(dataOp, { name, phone_number: phone, address }, 'Datos guardados'); }}>
      <h2 class="display mb-5 text-3xl">Nombre y contacto</h2>
      <ClinicFields bind:name bind:phone bind:address />
      {#if dataOp.phase === 'error'}<p class="alert mt-4" role="alert"><Icon name="alert" size={18} />{dataOp.message}</p>{/if}
      <div class="mt-5"><button type="submit" class="btn-primary" disabled={dataOp.phase === 'loading'}>Guardar datos</button></div>
    </form>

    <form class="card p-4 sm:p-6" onsubmit={(e) => { e.preventDefault(); void save(kindOp, { kind, specialties }, 'Giro guardado'); }}>
      <h2 class="display mb-5 text-3xl">Giro y especialidades</h2>
      <SpecialtyPicker bind:kind bind:specialties />
      {#if kindOp.phase === 'error'}<p class="alert mt-4" role="alert"><Icon name="alert" size={18} />{kindOp.message}</p>{/if}
      <div class="mt-5"><button type="submit" class="btn-primary" disabled={kindOp.phase === 'loading'}>Guardar giro</button></div>
    </form>

    <form class="card p-4 sm:p-6" onsubmit={(e) => { e.preventDefault(); void save(hoursOp, { settings: $state.snapshot(settings) }, 'Horario guardado'); }}>
      <h2 class="display mb-1 text-3xl">Horario y citas</h2>
      <p class="mb-5 text-sm text-app-muted">Ahora: {summarizeHours(settings)} · citas de {settings.appointment_minutes} min</p>
      <HoursEditor bind:settings />
      {#if hoursOp.phase === 'error'}<p class="alert mt-4" role="alert"><Icon name="alert" size={18} />{hoursOp.message}</p>{/if}
      <div class="mt-5"><button type="submit" class="btn-primary" disabled={hoursOp.phase === 'loading'}>Guardar horario</button></div>
    </form>
  </div>
{/if}
