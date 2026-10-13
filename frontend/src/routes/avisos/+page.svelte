<script lang="ts">
  import Segmented from '$lib/components/ui/Segmented.svelte';
  import { Loader } from '$lib/loader.svelte';
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { notificationsApi } from '$lib/api/waitlist';
  import { Op } from '$lib/op.svelte';
  import type { AppNotification } from '$lib/types/waitlist';
  import Guard from '$lib/components/Guard.svelte';
  import PageHeader from '$lib/components/ui/PageHeader.svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import { timeAgo } from '$lib/components/notifications/util';

  let items = $state<AppNotification[]>([]);
  let unread = $state(0);
  let onlyUnread = $state(false);
  const load = new Loader('No se pudieron cargar los avisos.');
  const act = new Op();

  const shown = $derived(onlyUnread ? items.filter((n) => !n.read) : items);

  async function refresh() {
    await load.run(async () => {
      const r = await notificationsApi.list(false, 100);
      items = r.notifications;
      unread = r.unread;
    });
  }

  async function open(n: AppNotification) {
    if (!n.read) {
      items = items.map((x) => (x.id === n.id ? { ...x, read: true } : x));
      unread = Math.max(0, unread - 1);
      notificationsApi.read(n.id).then((u) => (unread = u)).catch(() => {});
    }
    if (n.link) await goto(n.link);
  }

  async function readAll() {
    await act.run(async () => {
      unread = await notificationsApi.readAll();
      items = items.map((x) => ({ ...x, read: true }));
    });
  }

  onMount(refresh);
</script>

<svelte:head><title>Avisos · Caresia</title></svelte:head>

<Guard title="Avisos">
  <PageHeader title="Avisos" subtitle="Lo que pasó en tu consultorio: reservas, cancelaciones, lista de espera e inventario.">
    {#snippet actions()}
      <button type="button" class="btn-secondary" onclick={readAll} disabled={unread === 0 || act.phase === 'loading'}><Icon name="check" size={18} />Marcar todo como leído</button>
    {/snippet}
  </PageHeader>

  <Segmented class="mb-4" label="Filtro" value={onlyUnread ? 'unread' : 'all'} options={[{ value: 'all', label: 'Todos' }, { value: 'unread', label: `Sin leer${unread ? ` (${unread})` : ''}` }]} onselect={(v) => (onlyUnread = v === 'unread')} />

  <OpError op={act} class="mb-4" />

  {#if load.loading && items.length === 0}
    <LoadingRows />
  {:else if load.error}
    <Alert>{load.error}</Alert>
  {:else if shown.length === 0}
    <div class="card"><EmptyState icon="check" title={onlyUnread ? 'Estás al día' : 'Sin avisos'} text="Cuando algo requiera tu atención aparecerá aquí." /></div>
  {:else}
    <ul class="card divide-y divide-app-ink/8 overflow-hidden p-0">
      {#each shown as n (n.id)}
        <li>
          <button type="button" class="flex w-full items-start gap-3 px-4 py-4 text-left transition hover:bg-app-ink/5 sm:px-5" onclick={() => open(n)}>
            <span class="mt-1.5 h-2.5 w-2.5 flex-none rounded-full {n.read ? 'bg-app-ink/15' : 'bg-app-primary'}" aria-hidden="true"></span>
            <span class="min-w-0 flex-1">
              <span class="block {n.read ? '' : 'font-semibold'}">{#if !n.read}<span class="sr-only">Sin leer: </span>{/if}{n.title}</span>
              {#if n.body}<span class="mt-0.5 block text-sm text-app-muted [overflow-wrap:anywhere]">{n.body}</span>{/if}
            </span>
            <time class="flex-none font-mono text-xs text-app-muted" datetime={n.created_at}>{timeAgo(n.created_at)}</time>
          </button>
        </li>
      {/each}
    </ul>
    <p class="hint mt-3">Se muestran los avisos de los últimos 60 días.</p>
  {/if}
</Guard>
