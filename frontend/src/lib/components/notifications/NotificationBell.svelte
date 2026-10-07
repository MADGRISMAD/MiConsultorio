<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { notificationsApi } from '$lib/api/waitlist';
  import { session } from '$lib/session.svelte';
  import type { AppNotification } from '$lib/types/waitlist';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { timeAgo } from './util';

  const POLL_MS = 60_000;
  const uid = $props.id();

  let unread = $state(0);
  let items = $state<AppNotification[]>([]);
  let open = $state(false);
  let loading = $state(false);
  let failed = $state(false);
  let root = $state<HTMLDivElement>();
  let button = $state<HTMLButtonElement>();

  const active = $derived(session.status === 'authenticated' && !session.isPlatform && !session.locked);

  async function refreshCount() {
    if (!active || document.visibilityState !== 'visible') return;
    try {
      unread = await notificationsApi.count();
    } catch {
      /* the bell is a convenience: stay quiet */
    }
  }

  async function loadList() {
    loading = true;
    failed = false;
    try {
      const r = await notificationsApi.list(false, 15);
      items = r.notifications;
      unread = r.unread;
    } catch {
      failed = true;
    } finally {
      loading = false;
    }
  }

  async function toggle() {
    open = !open;
    if (open) await loadList();
  }

  function close(refocus = false) {
    open = false;
    if (refocus) button?.focus();
  }

  async function openItem(n: AppNotification) {
    close();
    if (!n.read) {
      items = items.map((x) => (x.id === n.id ? { ...x, read: true } : x));
      unread = Math.max(0, unread - 1);
      notificationsApi.read(n.id).then((u) => (unread = u)).catch(() => {});
    }
    if (n.link) await goto(n.link);
  }

  async function readAll() {
    try {
      unread = await notificationsApi.readAll();
      items = items.map((x) => ({ ...x, read: true }));
    } catch {
      failed = true;
    }
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape' && open) {
      e.stopPropagation();
      close(true);
    }
  }

  function onPointer(e: PointerEvent) {
    if (open && root && !root.contains(e.target as Node)) close();
  }

  onMount(() => {
    const t = setInterval(refreshCount, POLL_MS);
    const vis = () => document.visibilityState === 'visible' && refreshCount();
    document.addEventListener('visibilitychange', vis);
    return () => {
      clearInterval(t);
      document.removeEventListener('visibilitychange', vis);
    };
  });

  $effect(() => {
    if (active) refreshCount();
  });
</script>

<svelte:window onkeydown={onKey} onpointerdown={onPointer} />

{#if active}
  <div class="relative" bind:this={root}>
    <button
      type="button"
      bind:this={button}
      class="icon-btn relative"
      aria-haspopup="true"
      aria-expanded={open}
      aria-controls="{uid}-panel"
      aria-label={unread > 0 ? `Avisos, ${unread} sin leer` : 'Avisos'}
      onclick={toggle}
    >
      <Icon name="bell" size={20} />
      {#if unread > 0}
        <span class="absolute -right-0.5 -top-0.5 grid min-w-[1.1rem] place-items-center rounded-full bg-app-danger px-1 text-[10px] font-semibold leading-[1.1rem] text-white" aria-hidden="true">{unread > 99 ? '99+' : unread}</span>
      {/if}
    </button>

    {#if open}
      <div
        id="{uid}-panel"
        role="region"
        aria-label="Avisos"
        class="fixed inset-x-3 top-16 z-50 max-h-[75vh] overflow-hidden rounded-2xl border border-app-ink/10 bg-app-panel shadow-xl sm:absolute sm:inset-x-auto sm:right-0 sm:top-full sm:mt-2 sm:w-[22rem]"
      >
        <div class="flex items-center justify-between gap-2 border-b border-app-ink/10 px-4 py-3">
          <h2 class="text-sm font-semibold">Avisos</h2>
          <button type="button" class="text-xs font-medium text-app-primary underline disabled:opacity-40 disabled:no-underline" onclick={readAll} disabled={unread === 0}>Marcar todo como leído</button>
        </div>
        <div class="max-h-[55vh] overflow-y-auto" aria-live="polite">
          {#if loading && items.length === 0}
            <p class="px-4 py-8 text-center text-sm text-app-muted" role="status">Cargando…</p>
          {:else if failed}
            <p class="px-4 py-8 text-center text-sm text-app-danger" role="alert">No se pudieron cargar los avisos.</p>
          {:else if items.length === 0}
            <p class="px-4 py-8 text-center text-sm text-app-muted">No tienes avisos por ahora.</p>
          {:else}
            <ul>
              {#each items as n (n.id)}
                <li class="border-b border-app-ink/6 last:border-0">
                  <button type="button" class="flex w-full items-start gap-3 px-4 py-3 text-left transition hover:bg-app-ink/5 focus-visible:bg-app-ink/5" onclick={() => openItem(n)}>
                    <span class="mt-1.5 h-2 w-2 flex-none rounded-full {n.read ? 'bg-transparent' : 'bg-app-primary'}" aria-hidden="true"></span>
                    <span class="min-w-0 flex-1">
                      <span class="block text-sm {n.read ? 'font-normal' : 'font-semibold'}">{#if !n.read}<span class="sr-only">Sin leer: </span>{/if}{n.title}</span>
                      {#if n.body}<span class="mt-0.5 block text-xs text-app-muted [overflow-wrap:anywhere]">{n.body}</span>{/if}
                      <span class="mt-1 block font-mono text-[11px] text-app-muted">{timeAgo(n.created_at)}</span>
                    </span>
                  </button>
                </li>
              {/each}
            </ul>
          {/if}
        </div>
        <a href="/avisos" class="block border-t border-app-ink/10 px-4 py-3 text-center text-sm font-medium text-app-primary hover:bg-app-ink/5" onclick={() => close()}>Ver todos los avisos</a>
      </div>
    {/if}
  </div>
{/if}
