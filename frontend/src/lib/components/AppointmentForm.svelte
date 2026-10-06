<script lang="ts">
  import type { AppointmentInput } from '$lib/types';

  let { data = $bindable() }: { data: AppointmentInput } = $props();
  const today = new Date().toISOString().slice(0, 10);
</script>

<div class="grid gap-3 text-left sm:grid-cols-2">
  <label class="block">
    <span class="label">Nombre(s) *</span>
    <input type="text" class="field" bind:value={data.names} required autocomplete="off" />
  </label>
  <label class="block">
    <span class="label">Apellido(s) *</span>
    <input type="text" class="field" bind:value={data.last_names} required autocomplete="off" />
  </label>
  <label class="block sm:col-span-2">
    <span class="label">CURP *</span>
    <input type="text" class="field uppercase" bind:value={data.CURP} required maxlength="18" minlength="18" autocomplete="off" />
  </label>
  <label class="block">
    <span class="label">Fecha *</span>
    <input type="date" class="field" min={today} max="2100-12-30" bind:value={data.date} required />
  </label>
  <span class="hidden sm:block"></span>
  <label class="block">
    <span class="label">Hora de inicio *</span>
    <input type="time" step="900" class="field" bind:value={data.startHour} required />
  </label>
  <label class="block">
    <span class="label">Hora de finalización *</span>
    <input type="time" step="900" class="field" bind:value={data.endHour} required />
  </label>
  <label class="block sm:col-span-2">
    <span class="label">Detalles de la cita</span>
    <textarea class="field min-h-24" bind:value={data.details} maxlength="2000"></textarea>
  </label>
</div>
