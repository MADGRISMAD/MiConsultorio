<script lang="ts">
  import { i18n, LOCALES, t } from '$lib/i18n/index.svelte';

  let { class: cls = '' }: { class?: string } = $props();
</script>

<div class="inline-flex items-center rounded-full border border-app-ink/12 p-0.5 text-xs {cls}" role="group" aria-label={t('lang.label')}>
  {#each LOCALES as l (l.code)}
    <button
      type="button"
      class="min-h-8 min-w-9 rounded-full px-2.5 font-mono font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-app-primary/60
        {i18n.locale === l.code ? 'bg-app-ink text-app-surface' : 'text-app-muted hover:text-app-ink'}"
      lang={l.code === 'en' ? 'en' : 'es'}
      aria-pressed={i18n.locale === l.code}
      aria-label={l.label}
      onclick={() => i18n.set(l.code)}>{l.short}</button
    >
  {/each}
</div>
