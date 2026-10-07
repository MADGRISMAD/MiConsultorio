
<script lang="ts">
  import type { Snippet } from 'svelte';
  import { theme } from '$lib/theme.svelte';
  import Brand from '$lib/components/ui/Brand.svelte';
  import LanguageSwitch from '$lib/components/ui/LanguageSwitch.svelte';
  import { i18n, t } from '$lib/i18n/index.svelte';

  let { children }: { children: Snippet } = $props();

  $effect(() => theme.init());
  $effect(() => {
    i18n.initPublic();
    i18n.applyDocumentLang();
    return () => i18n.restoreDocumentLang();
  });
</script>

<!-- Public ARCO pages (no session): brand frame only -->
<div
  class="app fixed inset-0 overflow-y-auto overflow-x-hidden px-4 py-6 sm:py-10"
  data-theme={theme.mode}
  style="background-image: radial-gradient(ellipse 60% 420px at 50% 0%, rgb(var(--app-primary) / 0.1), transparent 75%); background-repeat: no-repeat"
>
  <div class="mx-auto w-full max-w-2xl">
    <div class="mb-2 flex justify-end"><LanguageSwitch /></div>
    {@render children()}
    <p class="mt-6 flex flex-wrap items-center justify-center gap-x-3 gap-y-1 text-xs text-app-muted">
      <span class="inline-flex items-center gap-2">{t('arco.shell.poweredBy')} <a href="/" class="inline-flex" aria-label="Caresia"><Brand size={16} class="text-[0.7rem]" /></a></span>
      <a href="/privacidad" class="underline">{t('arco.shell.privacy')}</a>
    </p>
  </div>
</div>
