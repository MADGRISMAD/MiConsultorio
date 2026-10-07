<script lang="ts">
  import { addMinutes, outsideHours } from '$lib/clinic';
  import { session } from '$lib/session.svelte';
  import type { AppointmentInput, PatientRow } from '$lib/types';
  import PatientPicker from './patients/PatientPicker.svelte';
  import Icon from './ui/Icon.svelte';

  let { data = $bindable() }: { data: AppointmentInput } = $props();
  const today = new Date().toISOString().slice(0, 10);

  const settings = $derived(session.clinic?.settings);
  // propose the end time from the clinic's usual appointment length, without overriding one already set
  function onStart() {
    if (data.startHour && !data.endHour) data.endHour = addMinutes(data.startHour, settings?.appointment_minutes ?? 30);
  }

  // Registered patient chosen with the picker. When editing, the row is rebuilt from the appointment itself.
  let picked = $state<PatientRow | null>(null);
  $effect(() => {
    const id = data.patient_id;
    if (!id) {
      if (picked) picked = null;
    } else if (picked?.id !== id) {
      picked = { id, file_number: 0, subject: data.last_names ? 'person' : 'animal', names: data.names, last_names: data.last_names, age: null, phone: '', guardian_name: '', incomplete: false, no_privacy_notice: false, last_encounter_at: null, archived_at: null };
    }
  });
  function onPick(p: PatientRow) {
    data.patient_id = p.id;
    data.names = p.names;
    data.last_names = p.last_names;
    data.CURP = '';
  }
  function onClear() {
    data.patient_id = null;
    data.names = '';
    data.last_names = '';
    data.CURP = '';
  }
  const registered = $derived(!!data.patient_id);
  const warning = $derived(outsideHours(settings, data.date, data.startHour, data.endHour));
</script>

<div class="grid gap-3 text-left sm:grid-cols-2">
  <div class="sm:col-span-2">
    <PatientPicker bind:value={picked} onpick={onPick} onclear={onClear} />
    {#if !registered}<p class="hint">Busca un paciente registrado o escribe los datos de quien aún no lo está.</p>{/if}
  </div>
  {#if !registered}
    <label class="block">
      <span class="label">Nombre(s) *</span>
      <input type="text" class="field" bind:value={data.names} required autocomplete="off" />
    </label>
    <label class="block">
      <span class="label">Apellido(s) *</span>
      <input type="text" class="field" bind:value={data.last_names} required autocomplete="off" />
    </label>
    <label class="block sm:col-span-2">
      <span class="label">CURP <span class="font-normal text-app-muted">(opcional)</span></span>
      <input type="text" class="field uppercase" bind:value={data.CURP} maxlength="18" autocomplete="off" />
    </label>
  {/if}
  <label class="block">
    <span class="label">Fecha *</span>
    <input type="date" class="field" min={today} max="2100-12-30" bind:value={data.date} required />
  </label>
  <span class="hidden sm:block"></span>
  <label class="block">
    <span class="label">Hora de inicio *</span>
    <input type="time" step="900" class="field" bind:value={data.startHour} onchange={onStart} required />
  </label>
  <label class="block">
    <span class="label">Hora de finalización *</span>
    <input type="time" step="900" class="field" bind:value={data.endHour} required />
  </label>
  {#if warning}
    <p class="flex items-start gap-2 rounded-xl bg-app-warning/12 px-3.5 py-2.5 text-sm font-medium text-app-warning sm:col-span-2" role="status"><Icon name="clock" size={17} class="mt-0.5 flex-none" />{warning} Puedes agendarla de todos modos.</p>
  {/if}
  <label class="block sm:col-span-2">
    <span class="label">Detalles de la cita</span>
    <textarea class="field min-h-24" bind:value={data.details} maxlength="2000"></textarea>
  </label>
</div>
