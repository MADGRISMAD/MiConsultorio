<script lang="ts">
  import type { Op } from '$lib/op.svelte';
  import Alert from './Alert.svelte';

  /** The error of an operation, shown only while it is in the error phase. */
  let { op, class: cls = '' }: { op: Op; class?: string } = $props();
</script>

{#if op.phase === 'error'}<Alert class={cls}>{op.message}</Alert>{/if}
