<script lang="ts">
  // Texto de un artículo: solo admite **negritas**; todo lo demás se muestra tal cual (Svelte lo escapa).
  let { text }: { text: string } = $props();
  const parts = $derived(text.split(/\*\*(.+?)\*\*/g));
</script>

{#each parts as part, i}{#if i % 2}<strong class="font-semibold text-ink">{part}</strong>{:else}{part}{/if}{/each}
