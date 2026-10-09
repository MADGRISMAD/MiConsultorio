<script lang="ts">
  import { addMinutes, outsideHours } from '$lib/clinic';
  import { session } from '$lib/session.svelte';
  import type { PatientRow } from '$lib/types';
  import type { ApptInput, Professional, ServiceOption } from '$lib/types/agenda';
  import PatientPicker from './patients/PatientPicker.svelte';
  import Icon from './ui/Icon.svelte';

  interface Props {
    data: ApptInput;
    professionals?: Professional[];
    rooms?: string[];
    services?: ServiceOption[];
    /** server answer to the last save: SLOT_TAKEN can be overridden with overbook, SLOT_BLOCKED cannot */
    conflict?: { code: string; message: string } | null;
    slotMinutes?: number;
  }
  let { data = $bindable(), professionals = [], rooms = [], services = [], conflict = null, slotMinutes }: Props = $props();

  const settings = $derived(session.clinic?.settings);
  const minutes = $derived(slotMinutes ?? settings?.appointment_minutes ?? 30);

  // propose the end time from the usual appointment length, without overriding one already set
  function onStart() {
    if (data.startHour && !data.endHour) data.endHour = addMinutes(data.startHour, minutes);
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
    if (!data.phone && p.phone) data.phone = p.phone;
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
    <span class="label">Profesional</span>
    <select class="field" bind:value={data.professional_id}>
      <option value={null}>Sin asignar</option>
      {#each professionals as p (p.id)}<option value={p.id}>{p.name}</option>{/each}
    </select>
  </label>
  <label class="block">
    <span class="label">Servicio</span>
    <select class="field" bind:value={data.service_id}>
      <option value={null}>Sin especificar</option>
      {#each services as s (s.id)}<option value={s.id}>{s.name}</option>{/each}
    </select>
  </label>

  <label class="block">
    <span class="label">Fecha *</span>
    <input type="date" class="field" max="2100-12-30" bind:value={data.date} required />
  </label>
  <label class="block">
    <span class="label">Sala</span>
    {#if rooms.length}
      <select class="field" bind:value={data.room}>
        <option value="">Sin sala</option>
        {#each rooms as r}<option value={r}>{r}</option>{/each}
        {#if data.room && !rooms.includes(data.room)}<option value={data.room}>{data.room}</option>{/if}
      </select>
    {:else}
      <input type="text" class="field" bind:value={data.room} maxlength="40" placeholder="Consultorio 1" autocomplete="off" />
    {/if}
  </label>
  <label class="block">
    <span class="label">Hora de inicio *</span>
    <input type="time" step="300" class="field" bind:value={data.startHour} onchange={onStart} required />
  </label>
  <label class="block">
    <span class="label">Hora de finalización *</span>
    <input type="time" step="300" class="field" bind:value={data.endHour} required />
  </label>
  {#if warning}
    <p class="flex items-start gap-2 rounded-xl bg-app-warning/12 px-3.5 py-2.5 text-sm font-medium text-app-warning sm:col-span-2" role="status"><Icon name="clock" size={17} class="mt-0.5 flex-none" />{warning} Puedes agendarla de todos modos.</p>
  {/if}

  <label class="block">
    <span class="label">Teléfono de contacto</span>
    <input type="tel" class="field" bind:value={data.phone} maxlength="30" autocomplete="off" inputmode="tel" />
  </label>
  <label class="block">
    <span class="label">Correo de contacto</span>
    <input type="email" class="field" bind:value={data.email} maxlength="200" autocomplete="off" />
  </label>
  <label class="flex cursor-pointer items-start gap-2 text-sm sm:col-span-2">
    <input type="checkbox" bind:checked={data.reminders_consent} class="mt-0.5 h-4 w-4 accent-[rgb(var(--app-primary))]" />
    El paciente recibe recordatorios de su cita (un día antes, por correo; puede pedir no recibirlos)
  </label>
  <label class="block sm:col-span-2">
    <span class="label">Notas de la cita</span>
    <textarea class="field min-h-20" bind:value={data.details} maxlength="2000"></textarea>
  </label>

  {#if conflict}
    <div class="rounded-xl bg-app-warning/12 px-3.5 py-3 text-sm sm:col-span-2" role="alert">
      <p class="flex items-start gap-2 font-medium text-app-warning"><Icon name="alert" size={17} class="mt-0.5 flex-none" />{conflict.message}</p>
      {#if conflict.code === 'SLOT_TAKEN'}
        <label class="mt-2 flex cursor-pointer items-center gap-2 text-app-ink">
          <input type="checkbox" bind:checked={data.overbook} class="h-4 w-4 accent-[rgb(var(--app-primary))]" />
          Agendar como sobreturno (se encima con otra cita)
        </label>
      {:else}
        <p class="mt-1 text-app-muted">Elige otra hora o quita el bloqueo desde la agenda.</p>
      {/if}
    </div>
  {/if}
</div>
