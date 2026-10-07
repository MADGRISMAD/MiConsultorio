<script lang="ts">
  import { isPending } from './operator';

  let { v }: { v: string } = $props();
</script>

{#if isPending(v)}<mark class="rounded bg-app-warning/20 px-1 font-semibold text-app-warning" title="Dato pendiente de completar por el operador">{v}</mark>{:else}{v}{/if}
