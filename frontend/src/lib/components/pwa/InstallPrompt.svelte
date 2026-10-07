<script lang="ts">
  import { pwa } from '$lib/pwa/pwa.svelte';
  import { t } from '$lib/i18n/index.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';

  /** `card` is a block for a settings or account page; `banner` is a floating hint. */
  let { variant = 'card' }: { variant?: 'card' | 'banner' } = $props();

  let showSteps = $state(false);
  const visible = $derived(!pwa.installed && !pwa.dismissed && (pwa.canInstall || pwa.iosHint));
</script>

<!-- Self-contained: shows only where the app can be installed (Chromium prompt, or iOS Safari steps). -->
{#if visible}
  <section
    class={variant === 'banner' ? 'fixed inset-x-4 bottom-20 z-[55] mx-auto max-w-md rounded-2xl border border-app-ink/12 bg-app-panel p-4 text-sm shadow-app' : 'card px-5 py-4 text-sm'}
    aria-label={t('pwa.install')}
    data-testid="install-prompt"
  >
    <p class="flex items-start gap-2"><Icon name="download" size={18} class="mt-0.5 flex-none text-app-primary" />{t('pwa.installText')}</p>
    {#if showSteps && pwa.iosHint && !pwa.canInstall}
      <div class="mt-3 rounded-xl bg-app-elevated px-3.5 py-3">
        <p class="font-medium">{t('pwa.iosTitle')}</p>
        <ol class="mt-1 list-decimal pl-5 text-app-muted">
          <li>{t('pwa.iosStep1')}</li>
          <li>{t('pwa.iosStep2')}</li>
          <li>{t('pwa.iosStep3')}</li>
        </ol>
      </div>
    {/if}
    <div class="mt-3 flex flex-wrap gap-2">
      {#if pwa.canInstall}
        <button type="button" class="btn-primary !min-h-9" onclick={() => pwa.install()}>{t('pwa.install')}</button>
      {:else}
        <button type="button" class="btn-primary !min-h-9" aria-expanded={showSteps} onclick={() => (showSteps = !showSteps)}>{t('pwa.install')}</button>
      {/if}
      <button type="button" class="btn-ghost !min-h-9" onclick={() => pwa.dismissInstall()}>{t('pwa.installDismiss')}</button>
    </div>
  </section>
{/if}
