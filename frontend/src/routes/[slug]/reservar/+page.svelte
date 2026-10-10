<script lang="ts">
  import { page } from '$app/state';
  import { bookingApi } from '$lib/api/booking';
  import { ApiError } from '$lib/api';
  import { t } from '$lib/i18n/index.svelte';
  import type { BookingInfo } from '$lib/types/booking';
  import BookingWizard from '$lib/components/booking/BookingWizard.svelte';
  import PublicShell from '$lib/components/booking/PublicShell.svelte';
  import BookingFix from '$lib/components/booking/BookingFix.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';

  const slug = $derived(page.params.slug ?? '');
  let info = $state<BookingInfo | null>(null);
  let view = $state<'loading' | 'ready' | 'missing' | 'error'>('loading');

  $effect(() => {
    const s = slug;
    view = 'loading';
    bookingApi
      .info(s)
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
  <title>{info ? t('booking.pageTitleClinic', { clinic: info.clinic.name }) : t('booking.pageTitle')}</title>
  <meta name="robots" content="noindex" />
</svelte:head>

<PublicShell wide>
  {#if view === 'loading'}
    <div class="card-empty" role="status">{t('common.loading')}</div>
  {:else if view === 'ready' && info}
    {#key slug}<BookingWizard {slug} {info} />{/key}
  {:else if view === 'missing'}
    <section class="card px-6 py-9">
      <div class="grid h-12 w-12 place-items-center rounded-full bg-app-ink/8 text-app-muted"><Icon name="calendar" size={24} /></div>
      <h1 class="display mt-4 text-3xl leading-tight">{t('booking.missingTitle')}</h1>
      <p class="mt-3 text-app-muted">{t('booking.missingText')}</p>
    </section>
    <BookingFix {slug} />
  {:else}
    <section class="card px-6 py-9" role="alert">
      <p class="alert"><Icon name="alert" size={18} />{t('booking.loadError')}</p>
      <button class="btn-secondary mt-5" onclick={() => location.reload()}>{t('common.retry')}</button>
    </section>
    <BookingFix {slug} />
  {/if}
</PublicShell>
