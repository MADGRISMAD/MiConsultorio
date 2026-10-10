<script lang="ts">
  import Guard from '$lib/components/Guard.svelte';
  import AppointmentsPanel from '$lib/components/AppointmentsPanel.svelte';
  import SectionTabs from '$lib/components/nav/SectionTabs.svelte';
  import { AGENDA_TABS } from '$lib/components/nav/tabs';
  import { session } from '$lib/session.svelte';
</script>

<svelte:head><title>Agenda · Caresia</title></svelte:head>

<!-- One agenda for everyone: whoever can manage appointments gets the buttons to create, move and cancel -->
<Guard title="Agenda" permissions={['navAppointments', 'adminAppointments']}>
  <SectionTabs tabs={AGENDA_TABS} label="Agenda" />
  <AppointmentsPanel admin={session.has('adminAppointments')} />
</Guard>
