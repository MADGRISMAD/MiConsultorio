<script lang="ts">
  import { BILLING_STATES, type BillingState } from '$lib/types';
  import Pill from './Pill.svelte';

  let { state }: { state: BillingState } = $props();
  const info = $derived(BILLING_STATES[state]);
</script>

<Pill tone={info?.tone ?? 'muted'}>{info?.label ?? state}</Pill>
