<script lang="ts">
  import { DURATIONS } from '$lib/clinic';
  import { WEEKDAYS, type ClinicSettings } from '$lib/types';

  let { settings = $bindable() }: { settings: ClinicSettings } = $props();

  function preset(days: number, start: string, end: string, satEnd?: string) {
    WEEKDAYS.forEach(([k], i) => {
      settings.hours[k] = { open: i < days, start, end };
    });
    if (satEnd) settings.hours.sat = { open: true, start, end: satEnd };
  }
  const custom = $derived(!DURATIONS.includes(settings.appointment_minutes));
</script>

<fieldset>
  <legend class="label">Días y horario de atención</legend>
  <div class="mb-3 flex flex-wrap gap-2">
    <button type="button" class="btn-secondary !min-h-8 !px-3.5 text-[13px]" onclick={() => preset(5, '09:00', '18:00')}>Lun–Vie 9:00–18:00</button>
    <button type="button" class="btn-secondary !min-h-8 !px-3.5 text-[13px]" onclick={() => preset(5, '09:00', '18:00', '14:00')}>Lun–Vie + Sáb medio día</button>
    <button type="button" class="btn-secondary !min-h-8 !px-3.5 text-[13px]" onclick={() => preset(7, '08:00', '20:00')}>Todos los días</button>
  </div>
  <ul class="divide-y divide-app-ink/8 overflow-hidden rounded-xl border border-app-ink/10">
    {#each WEEKDAYS as [key, label]}
      {@const h = settings.hours[key]}
      <li class="flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-2.5 {h.open ? '' : 'bg-app-ink/[0.03]'}">
        <label class="flex w-36 cursor-pointer items-center gap-3 text-sm font-medium">
          <input type="checkbox" class="peer sr-only" bind:checked={h.open} />
          <span class="relative h-5 w-9 flex-none rounded-full bg-app-ink/15 transition peer-checked:bg-app-accent peer-focus-visible:outline peer-focus-visible:outline-2 peer-focus-visible:outline-app-primary/60 after:absolute after:left-0.5 after:top-0.5 after:h-4 after:w-4 after:rounded-full after:bg-white after:shadow after:transition peer-checked:after:translate-x-4" aria-hidden="true"></span>
          <span class={h.open ? '' : 'text-app-muted'}>{label}</span>
        </label>
        {#if h.open}
          <div class="flex items-center gap-2 text-sm">
            <input type="time" step="900" class="field !min-h-9 !w-auto !py-1" bind:value={h.start} aria-label="Apertura del {label}" required />
            <span class="text-app-muted">a</span>
            <input type="time" step="900" class="field !min-h-9 !w-auto !py-1" bind:value={h.end} aria-label="Cierre del {label}" required />
          </div>
        {:else}
          <span class="text-sm text-app-muted">Cerrado</span>
        {/if}
      </li>
    {/each}
  </ul>
</fieldset>

<fieldset class="mt-6">
  <legend class="label">Duración normal de una cita</legend>
  <p class="hint mb-3 !mt-0">Al agendar, la hora de fin se propone sola. Siempre puedes cambiarla.</p>
  <div class="flex flex-wrap items-center gap-2">
    {#each DURATIONS as m}
      <button type="button" aria-pressed={settings.appointment_minutes === m} class="rounded-full px-4 py-1.5 text-sm font-medium transition {settings.appointment_minutes === m ? 'bg-app-ink text-app-surface' : 'bg-app-ink/6 text-app-muted hover:bg-app-ink/10 hover:text-app-ink'}" onclick={() => (settings.appointment_minutes = m)}>{m} min</button>
    {/each}
    <label class="ml-1 flex items-center gap-2 text-sm text-app-muted">
      Otra
      <input type="number" min="5" max="240" step="5" class="field !min-h-9 !w-20 !py-1 {custom ? 'border-app-primary' : ''}" bind:value={settings.appointment_minutes} aria-label="Duración personalizada en minutos" />
      min
    </label>
  </div>
</fieldset>
