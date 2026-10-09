<script lang="ts">
  import { page } from '$app/state';
  import { arcoApi } from '$lib/api/arco';
  import { ApiError } from '$lib/api';
  import { t } from '$lib/i18n/index.svelte';
  import type { ArcoPublicInfo } from '$lib/types/arco';
  import ArcoForm from '$lib/components/arco/ArcoForm.svelte';
  import ArcoShell from '$lib/components/arco/ArcoShell.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';

  const slug = $derived(page.params.slug ?? '');
  let info = $state<ArcoPublicInfo | null>(null);
  let view = $state<'loading' | 'ready' | 'missing' | 'error'>('loading');

  $effect(() => {
    const s = slug;
    view = 'loading';
    arcoApi
      .publicInfo(s)
      .then((r) => {
        info = r;
        view = 'ready';
      })
      .catch((e) => {
        view = e instanceof ApiError && e.status === 404 ? 'missing' : 'error';
      });
  });
</script>

<svelte:head>
  <title>{info ? t('arco.pageTitleClinic', { clinic: info.clinic.name }) : t('arco.pageTitle')}</title>
  <meta name="robots" content="noindex" />
</svelte:head>

<ArcoShell>
  {#if view === 'loading'}
    <div class="card-empty" role="status">{t('common.loading')}</div>
  {:else if view === 'ready' && info}
    {#key slug}<ArcoForm {slug} {info} />{/key}
  {:else if view === 'missing'}
    <section class="card px-6 py-9">
      <div class="grid h-12 w-12 place-items-center rounded-full bg-app-ink/8 text-app-muted"><Icon name="shield" size={24} /></div>
      <h1 class="display mt-4 text-3xl leading-tight">{t('arco.missingTitle')}</h1>
      <p class="mt-3 text-app-muted">{t('arco.missingText')}</p>
    </section>
  {:else}
    <section class="card px-6 py-9" role="alert">
      <p class="alert"><Icon name="alert" size={18} />{t('booking.loadError')}</p>
      <button class="btn-secondary mt-5" onclick={() => location.reload()}>{t('common.retry')}</button>
    </section>
  {/if}
</ArcoShell>
