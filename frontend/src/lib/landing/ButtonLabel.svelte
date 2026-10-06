<script lang="ts">
  import type { Snippet } from 'svelte';
  let { children }: { children: Snippet } = $props();
</script>

<!-- Label slides up and a copy slides in from below on hover -->
<span class="relative block overflow-hidden">
  <span class="block transition-transform duration-500 ease-out-strong group-hover:-translate-y-full">{@render children()}</span>
  <span aria-hidden="true" class="absolute inset-0 block translate-y-full transition-transform duration-500 ease-out-strong group-hover:translate-y-0">{@render children()}</span>
</span>
