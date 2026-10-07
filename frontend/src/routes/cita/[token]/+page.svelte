<script lang="ts">
  import { page } from '$app/state';
  import { t } from '$lib/i18n/index.svelte';
  import AppointmentManage from '$lib/components/booking/AppointmentManage.svelte';
  import PublicShell from '$lib/components/booking/PublicShell.svelte';
</script>

<svelte:head>
  <title>{t('appt.pageTitle')}</title>
  <meta name="robots" content="noindex, nofollow" />
  <meta name="referrer" content="no-referrer" />
</svelte:head>

<PublicShell>
  {#key page.params.token}
    <AppointmentManage token={page.params.token ?? ''} />
  {/key}
</PublicShell>
