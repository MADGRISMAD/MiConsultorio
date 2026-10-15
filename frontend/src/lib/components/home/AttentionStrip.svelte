<script lang="ts">
  import { onMount } from 'svelte';
  import { notificationsApi } from '$lib/api/waitlist';
  import { session } from '$lib/session.svelte';
  import { PERMISSIONS } from '$lib/types';
  import type { Appt } from '$lib/types/agenda';
  let { today = [], tomorrow = [], now = Date.now() }: { today?: Appt[]; tomorrow?: Appt[]; now?: number } = $props();
  let unread = $state(0);
  onMount(() => {
    const refresh = () => notificationsApi.count().then((n) => (unread = n), () => {});
    refresh();
    const timer = setInterval(refresh, 30000);
    return () => clearInterval(timer);
  });
  const waiting = $derived(today.filter((a) => a.status === 'arrived' && a.arrived_at && now - Date.parse(a.arrived_at) > 15 * 60000));
  const unregistered = $derived(today.filter((a) => !a.patient_id && ['scheduled', 'confirmed', 'arrived', 'in_progress'].includes(a.status)));
  const confirming = $derived([...today, ...tomorrow].filter((a) => a.status === 'scheduled'));
  const items = $derived([
    { count: waiting.length, label: 'en espera más de 15 min', href: '/en-proceso' },
    { count: session.has(PERMISSIONS.adminHistorials) ? unregistered.length : 0, label: 'sin paciente registrado', href: unregistered[0] ? `/pacientes/nuevo?cita=${encodeURIComponent(unregistered[0].id)}` : '' },
    { count: session.has(PERMISSIONS.adminAppointments) ? confirming.length : 0, label: 'por confirmar hoy o mañana', href: confirming[0] ? `/admin/navegar-citas?cita=${encodeURIComponent(confirming[0].id)}` : '' },
    { count: unread, label: 'avisos sin leer', href: '/avisos' }
  ].filter((i) => i.count > 0).slice(0, 3));
</script>

{#if items.length}
  <section aria-label="Requiere atención" class="rounded-xl border border-app-warning/25 bg-app-warning/5 px-4 py-2">
    <h2 class="text-sm font-semibold">Requiere atención</h2>
    <ul class="flex flex-wrap gap-x-5" aria-live="polite">
      {#each items as item}
        <li><a href={item.href} class="flex min-h-11 items-center gap-2 text-sm text-app-primary underline underline-offset-4"><strong>{item.count}</strong> {item.label}</a></li>
      {/each}
    </ul>
  </section>
{/if}
