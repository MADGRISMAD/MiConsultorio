<script lang="ts">
  import { downloadIcs, googleCalendarUrl, type CalendarEvent } from '$lib/calendar';
  import Icon from './Icon.svelte';

  let { event, size = 'normal' }: { event: CalendarEvent; size?: 'normal' | 'small' } = $props();
  const cls = $derived(size === 'small' ? 'btn-secondary !min-h-9' : 'btn-secondary btn-lg');
</script>

<div class="flex flex-wrap gap-2">
  <a class={cls} href={googleCalendarUrl(event)} target="_blank" rel="noopener"><Icon name="calendar" size={size === 'small' ? 16 : 18} />Agregar a Google Calendar</a>
  <button type="button" class={cls} onclick={() => downloadIcs(event)}>Descargar .ics</button>
</div>
