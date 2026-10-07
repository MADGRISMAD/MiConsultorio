<script lang="ts">
  import type { Snippet } from 'svelte';
  import { theme } from '$lib/theme.svelte';
  import { i18n, t } from '$lib/i18n/index.svelte';
  import Brand from '$lib/components/ui/Brand.svelte';
  import LanguageSwitch from '$lib/components/ui/LanguageSwitch.svelte';

  let { children, wide = false }: { children: Snippet; wide?: boolean } = $props();

  $effect(() => theme.init());
  // Public pages follow the visitor's language; leaving them puts the Spanish app default back on <html lang>.
  $effect(() => {
    i18n.initPublic();
    i18n.applyDocumentLang();
    return () => i18n.restoreDocumentLang();
  });
</script>

<!-- Public pages (no session): brand frame only, no AppShell -->
<div
  class="app fixed inset-0 overflow-y-auto overflow-x-hidden px-4 py-6 sm:py-10"
  data-theme={theme.mode}
  style="background-image: radial-gradient(ellipse 60% 420px at 50% 0%, rgb(var(--app-primary) / 0.1), transparent 75%); background-repeat: no-repeat"
>
  <div class="mx-auto w-full {wide ? 'max-w-2xl' : 'max-w-lg'}">
    <div class="mb-2 flex justify-end"><LanguageSwitch /></div>
    {@render children()}
    <p class="mt-6 flex items-center justify-center gap-2 text-xs text-app-muted">
      {t('shell.bookedWith')} <a href="/" class="inline-flex" aria-label="Caresia"><Brand size={16} class="text-[0.7rem]" /></a>
    </p>
  </div>
</div>
