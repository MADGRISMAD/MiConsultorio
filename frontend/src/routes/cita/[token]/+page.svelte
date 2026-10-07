<script lang="ts">
  import { page } from '$app/state';
  import AppointmentManage from '$lib/components/booking/AppointmentManage.svelte';
  import PublicShell from '$lib/components/booking/PublicShell.svelte';
</script>

<svelte:head>
  <title>Tu cita · Caresia</title>
  <meta name="robots" content="noindex, nofollow" />
  <meta name="referrer" content="no-referrer" />
</svelte:head>

<PublicShell>
  {#key page.params.token}
    <AppointmentManage token={page.params.token ?? ''} />
  {/key}
</PublicShell>
