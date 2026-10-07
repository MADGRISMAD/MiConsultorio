<script lang="ts">
  import { page } from '$app/state';
  import PublicShell from '$lib/components/booking/PublicShell.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { t } from '$lib/i18n/index.svelte';

  const missing = $derived(page.status === 404);
</script>

<svelte:head>
  <title>{missing ? t('error.notFoundTitle') : t('error.genericTitle')} · Caresia</title>
  <meta name="robots" content="noindex" />
</svelte:head>

<PublicShell>
  <section class="card px-6 py-9" role={missing ? undefined : 'alert'}>
    <div class="grid h-12 w-12 place-items-center rounded-full bg-app-ink/8 text-app-muted"><Icon name="alert" size={24} /></div>
    <p class="mt-4 font-mono text-xs text-app-muted">{page.status}</p>
    <h1 class="display mt-1 text-3xl leading-tight">{missing ? t('error.notFoundTitle') : t('error.genericTitle')}</h1>
    <p class="mt-3 text-app-muted">{missing ? t('error.notFoundText') : t('error.genericText')}</p>
    <div class="mt-5 flex flex-wrap gap-2">
      <a class="btn-primary" href="/">{t('error.home')}</a>
      {#if !missing}<button class="btn-secondary" onclick={() => location.reload()}>{t('common.retry')}</button>{/if}
    </div>
  </section>
</PublicShell>
