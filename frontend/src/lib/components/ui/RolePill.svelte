<script lang="ts">
  import { ROLES, type Role } from '$lib/types';
  import Icon from './Icon.svelte';
  import Pill from './Pill.svelte';

  let { role, long = false }: { role: Role; long?: boolean } = $props();
  const info = $derived(ROLES[role]);
</script>

<Pill tone={info?.tone ?? 'muted'}>
  {#if role === 'admin' || role === 'platform_admin'}<Icon name="shield" size={13} />{/if}
  {long ? info?.label : info?.short}
</Pill>
