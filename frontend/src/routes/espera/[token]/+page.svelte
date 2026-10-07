<script lang="ts">
  import { page } from '$app/state';
  import WaitlistOffer from '$lib/components/booking/WaitlistOffer.svelte';
  import PublicShell from '$lib/components/booking/PublicShell.svelte';
</script>

<svelte:head>
  <title>Lista de espera · Caresia</title>
  <meta name="robots" content="noindex, nofollow" />
  <meta name="referrer" content="no-referrer" />
</svelte:head>

<PublicShell>
  {#key page.params.token}
    <WaitlistOffer token={page.params.token ?? ''} />
  {/key}
</PublicShell>
