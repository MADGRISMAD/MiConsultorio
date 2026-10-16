<script lang="ts">
  /** Pinta «texto *con acento*» con el acento en cursiva azul. */
  let { text }: { text: string } = $props();
  const parts = $derived(text.split('*'));
</script>

{#each parts as part, i}{#if i % 2}<em class="italic text-signal">{part}</em>{:else}{part}{/if}{/each}
