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
  <section aria-label="Requiere atención" class="card flex flex-wrap items-center gap-x-5 gap-y-1 px-5 py-3 sm:px-6">
    <p class="section-title flex items-center gap-2"><span class="h-2 w-2 rounded-full bg-app-warning"></span>Requiere atención</p>
    <ul class="flex flex-wrap gap-x-5" aria-live="polite">
      {#each items as item}
        <li>
          <a href={item.href} class="flex min-h-11 items-center gap-2 text-sm font-medium text-app-ink hover:underline">
            <span class="pill pill-warn tabular-nums">{item.count}</span>{item.label}
          </a>
        </li>
      {/each}
    </ul>
  </section>
{/if}
