<script lang="ts">
  import { pwa } from '$lib/pwa/pwa.svelte';
  import { t } from '$lib/i18n/index.svelte';
</script>

<!-- Self-contained: mount once anywhere (e.g. the app shell). Appears only when a new version is waiting. -->
{#if pwa.updateReady}
  <div
    class="fixed inset-x-4 bottom-4 z-[60] mx-auto flex max-w-md flex-wrap items-center justify-between gap-3 rounded-2xl border border-app-ink/12 bg-app-panel px-4 py-3 text-sm shadow-app"
    role="status"
    data-testid="update-banner"
  >
    <span>{t('pwa.update')}</span>
    <span class="flex gap-2">
      <button type="button" class="btn-ghost !min-h-9" onclick={() => pwa.dismissUpdate()}>{t('pwa.updateLater')}</button>
      <button type="button" class="btn-primary !min-h-9" onclick={() => pwa.applyUpdate()}>{t('pwa.updateAction')}</button>
    </span>
  </div>
{/if}
