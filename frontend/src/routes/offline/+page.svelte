<script lang="ts">
  import { page } from '$app/state';
  import PublicShell from '$lib/components/booking/PublicShell.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { t } from '$lib/i18n/index.svelte';

  // The service worker sends failed navigations here with ?from=<path>; only same-site paths are followed.
  const from = $derived.by(() => {
    const f = page.url.searchParams.get('from') ?? '';
    return f.startsWith('/') && !f.startsWith('//') && !f.startsWith('/offline') ? f : '/';
  });
</script>

<svelte:head>
  <title>{t('offline.pageTitle')}</title>
  <meta name="robots" content="noindex" />
</svelte:head>

<PublicShell>
  <section class="card px-6 py-9" aria-live="polite">
    <div class="grid h-12 w-12 place-items-center rounded-full bg-app-ink/8 text-app-muted"><Icon name="alert" size={24} /></div>
    <h1 class="display mt-4 text-3xl leading-tight">{t('offline.title')}</h1>
    <p class="mt-3 text-app-muted">{t('offline.text')}</p>
    <a class="btn-primary mt-5" href={from} data-sveltekit-reload>{t('offline.retry')}</a>
  </section>
</PublicShell>
