<script lang="ts">
  import { page } from '$app/state';
  import { arcoApi } from '$lib/api/arco';
  import { ApiError } from '$lib/api';
  import type { ArcoPublicStatus } from '$lib/types/arco';
  import ArcoShell from '$lib/components/arco/ArcoShell.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { statusPill } from '$lib/components/arco/labels';
  import { fmtDay, i18n, t } from '$lib/i18n/index.svelte';

  const token = $derived(page.url.searchParams.get('t') ?? '');
  let req = $state<ArcoPublicStatus | null>(null);
  let view = $state<'loading' | 'ready' | 'missing' | 'error'>('loading');

  $effect(() => {
    const tk = token;
    if (!tk) {
      view = 'missing';
      return;
    }
    view = 'loading';
    arcoApi
      .publicStatus(tk)
      .then((r) => {
        req = r;
        view = 'ready';
      })
      .catch((e) => {
        view = e instanceof ApiError && e.status === 404 ? 'missing' : 'error';
      });
  });
</script>

<svelte:head>
  <title>{t('arco.status.pageTitle')}</title>
  <meta name="robots" content="noindex" />
</svelte:head>

<ArcoShell>
  {#if view === 'loading'}
    <div class="card px-6 py-10 text-center text-sm text-app-muted" role="status">{t('common.loading')}</div>
  {:else if view === 'ready' && req}
    <section class="card px-6 py-8 sm:px-9">
      <p class="section-title">{req.clinic}</p>
      <h1 class="display mt-3 text-4xl leading-tight">{t('arco.status.title')}</h1>
      <dl class="mt-6 grid gap-4 sm:grid-cols-2">
        <div><dt class="section-title">{t('arco.status.folio')}</dt><dd class="mt-1 font-mono text-lg font-semibold" data-testid="arco-status-folio">{req.folio}</dd></div>
        <div><dt class="section-title">{t('arco.status.type')}</dt><dd class="mt-1 text-[15px]">{i18n.locale === 'en' ? t(`arco.kind.${req.kind}`) : req.kind_label}</dd></div>
        <div><dt class="section-title">{t('arco.status.received')}</dt><dd class="mt-1 text-[15px]">{fmtDay(req.received_on.slice(0, 10), { day: 'numeric', month: 'short', year: 'numeric' })}</dd></div>
        <div><dt class="section-title">{t('arco.status.state')}</dt><dd class="mt-1"><span class="pill {statusPill(req.status)}">{req.status === 'negada' ? t('arco.status.answered') : t(`arco.status.${req.status}`)}</span></dd></div>
      </dl>
      <p class="mt-5 rounded-xl bg-app-ink/5 px-4 py-3 text-sm text-app-muted">{i18n.locale === 'en' && req.status !== 'vencida' ? t(`arco.statusText.${req.status}`) : req.status_label}</p>
      <p class="hint mt-4">{t('arco.status.safety')}</p>
    </section>
  {:else if view === 'missing'}
    <section class="card px-6 py-9">
      <div class="grid h-12 w-12 place-items-center rounded-full bg-app-ink/8 text-app-muted"><Icon name="search" size={24} /></div>
      <h1 class="display mt-4 text-3xl leading-tight">{t('arco.status.notFoundTitle')}</h1>
      <p class="mt-3 text-app-muted">{t('arco.status.notFoundText')}</p>
    </section>
  {:else}
    <section class="card px-6 py-9" role="alert">
      <p class="alert"><Icon name="alert" size={18} />{t('booking.loadError')}</p>
      <button class="btn-secondary mt-5" onclick={() => location.reload()}>{t('common.retry')}</button>
    </section>
  {/if}
</ArcoShell>
