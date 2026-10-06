<script lang="ts">
  import Mark from './Mark.svelte';
  let { light = false }: { light?: boolean } = $props();
</script>

<span class="inline-flex items-center gap-2.5 {light ? 'text-paper' : 'text-ink'}">
  <Mark {light} />
  <span class="font-display text-[26px] leading-none tracking-[-0.01em]">Caresia</span>
</span>
