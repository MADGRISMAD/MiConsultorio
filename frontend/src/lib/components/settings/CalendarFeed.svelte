<script lang="ts">
  import { publicOrigin } from '$lib/site';
  import OpError from '$lib/components/ui/OpError.svelte';
  import { onMount } from 'svelte';
  import { calendarApi, type CalendarFeed } from '$lib/api/profile';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import Icon from '../ui/Icon.svelte';

  let feed = $state<CalendarFeed | null>(null);
  let names = $state(false);
  const op = new Op();

  onMount(async () => {
    try {
      feed = await calendarApi.get();
      names = feed.show_names;
    } catch {
      feed = null;
    }
  });

  const url = $derived(feed?.enabled ? `${publicOrigin()}${feed.path}` : '');
  // Google Calendar subscribes by address: "Other calendars (+)" > "From URL"
  const googleAdd = $derived(url ? `https://calendar.google.com/calendar/r?cid=${encodeURIComponent(url.replace(/^https:/, 'webcal:'))}` : '');

  async function set(action: 'enable' | 'rotate' | 'disable') {
    if (await op.run(async () => {
      feed = await calendarApi.set(action, names);
      names = feed.show_names;
    })) toast.show(action === 'disable' ? 'Calendario desactivado' : action === 'rotate' ? 'Dirección nueva generada' : 'Calendario actualizado');
  }
  async function copy() {
    try {
      await navigator.clipboard.writeText(url);
      toast.show('Dirección copiada');
    } catch {
      toast.show('No se pudo copiar. Selecciona la dirección y cópiala a mano.', 'error');
    }
  }
</script>

{#if feed}
  <section class="card mt-6 p-5 sm:p-6" aria-labelledby="cal-h">
    <h3 id="cal-h" class="display text-xl">Mis citas en Google Calendar</h3>
    <p class="mt-1 text-sm text-app-muted">
      Suscribe tu calendario personal a tus citas de Caresia y verás tu agenda junto a todo lo demás. Es de un solo sentido: Caresia publica tus citas, pero nunca lee ni guarda nada de tu calendario.
    </p>
    <label class="mt-4 flex cursor-pointer items-start gap-3 text-sm">
      <input type="checkbox" class="mt-0.5 h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={names} onchange={() => feed?.enabled && set('enable')} />
      <span>Mostrar el nombre del paciente en cada evento<br /><span class="text-app-muted">Apagado, los eventos dicen solo «Cita» y la hora, para que ningún dato de tus pacientes quede en el calendario de Google.</span></span>
    </label>
    {#if feed.enabled}
      <p class="mt-4 text-sm font-medium">Tu dirección privada</p>
      <div class="mt-1 flex flex-wrap items-center gap-2">
        <input class="field min-w-0 flex-1 font-mono text-xs" readonly value={url} onfocus={(e) => e.currentTarget.select()} aria-label="Dirección del calendario" />
        <button type="button" class="btn-secondary" onclick={copy}>Copiar</button>
      </div>
      <div class="mt-3 flex flex-wrap gap-2">
        <a class="btn-primary" href={googleAdd} target="_blank" rel="noopener noreferrer"><Icon name="calendar" size={18} />Agregar a Google Calendar</a>
        <button type="button" class="btn-ghost" disabled={op.phase === 'loading'} onclick={() => set('rotate')}>Generar dirección nueva</button>
        <button type="button" class="btn-ghost text-app-danger" disabled={op.phase === 'loading'} onclick={() => set('disable')}>Desactivar</button>
      </div>
      <p class="hint mt-2">Cualquiera que tenga esta dirección puede ver tus horarios: no la compartas. Si se filtra, genera una nueva y la anterior deja de funcionar. Google actualiza los calendarios suscritos cada varias horas, así que una cita nueva puede tardar en aparecer.</p>
    {:else}
      <button type="button" class="btn-primary mt-4" disabled={op.phase === 'loading'} onclick={() => set('enable')}>Activar mi calendario</button>
    {/if}
    <OpError {op} class="mt-3" />
  </section>
{/if}
