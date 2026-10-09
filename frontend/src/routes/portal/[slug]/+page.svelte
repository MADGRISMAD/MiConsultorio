<script lang="ts">
  import { page } from '$app/state';
  import { ApiError } from '$lib/api';
  import { t } from '$lib/i18n/index.svelte';
  import { portalApi } from '$lib/api/portal';
  import type { PortalInfo } from '$lib/types/portal';
  import PublicShell from '$lib/components/booking/PublicShell.svelte';
  import PortalApp from '$lib/components/portal/PortalApp.svelte';
  import PortalLogin from '$lib/components/portal/PortalLogin.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';

  const slug = $derived(page.params.slug ?? '');
  let info = $state<PortalInfo | null>(null);
  let view = $state<'loading' | 'login' | 'app' | 'missing' | 'error'>('loading');
  let expired = $state(false);

  // The session cookie is HttpOnly, so we find out by asking: /portal/me answers 401 without one.
  $effect(() => {
    const s = slug;
    view = 'loading';
    portalApi
      .info(s)
      .then(async (i) => {
        info = i;
        try {
          await portalApi.me();
          view = 'app';
        } catch {
          view = 'login';
        }
      })
      .catch((e) => {
        view = e instanceof ApiError && e.status === 404 ? 'missing' : 'error';
      });
  });
</script>

<svelte:head>
  <title>{info ? t('portal.pageTitleClinic', { clinic: info.name }) : t('portal.pageTitle')}</title>
  <meta name="robots" content="noindex" />
</svelte:head>

<PublicShell wide="xl">
  {#if view === 'loading'}
    <div class="card-empty" role="status">{t('common.loading')}</div>
  {:else if view === 'login' && info}
    {#if expired}<p class="mb-4 rounded-xl bg-app-warning/14 px-3.5 py-3 text-sm font-medium text-app-warning" role="status">{t('portal.sessionEnded')}</p>{/if}
    <PortalLogin {slug} {info} onsignedin={() => { expired = false; view = 'app'; }} />
  {:else if view === 'app'}
    {#key slug}
      <PortalApp {slug} onexit={(exp) => { expired = exp; view = 'login'; }} />
    {/key}
  {:else if view === 'missing'}
    <section class="card px-6 py-9">
      <div class="grid h-12 w-12 place-items-center rounded-full bg-app-ink/8 text-app-muted"><Icon name="user" size={24} /></div>
      <h1 class="display mt-4 text-3xl leading-tight">{t('portal.missingTitle')}</h1>
      <p class="mt-3 text-app-muted">{t('booking.missingText')}</p>
    </section>
  {:else}
    <section class="card px-6 py-9" role="alert">
      <p class="alert"><Icon name="alert" size={18} />{t('booking.loadError')}</p>
      <button class="btn-secondary mt-5" onclick={() => location.reload()}>{t('common.retry')}</button>
    </section>
  {/if}
</PublicShell>
