<script lang="ts">
  import type { Snippet } from 'svelte';
  import { theme } from '$lib/theme.svelte';
  import Brand from '$lib/components/ui/Brand.svelte';

  let { children, wide = false }: { children: Snippet; wide?: boolean } = $props();

  $effect(() => theme.init());
</script>

<!-- Public pages (no session): brand frame only, no AppShell -->
<div
  class="app fixed inset-0 overflow-y-auto overflow-x-hidden px-4 py-6 sm:py-10"
  data-theme={theme.mode}
  style="background-image: radial-gradient(ellipse 60% 420px at 50% 0%, rgb(var(--app-primary) / 0.1), transparent 75%); background-repeat: no-repeat"
>
  <div class="mx-auto w-full {wide ? 'max-w-2xl' : 'max-w-lg'}">
    {@render children()}
    <p class="mt-6 flex items-center justify-center gap-2 text-xs text-app-muted">
      Agenda con <a href="/" class="inline-flex" aria-label="Caresia"><Brand size={16} class="text-[0.7rem]" /></a>
    </p>
  </div>
</div>
