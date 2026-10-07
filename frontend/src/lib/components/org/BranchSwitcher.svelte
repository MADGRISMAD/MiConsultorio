<script lang="ts">
  // Branch switcher for the top bar. Self-contained: it only shows up for the owner of an organization
  // with more than one branch in service, and asks the server (which re-checks ownership) to move.
  import { enterBranch, orgApi } from '$lib/api/org';
  import { session } from '$lib/session.svelte';
  import type { OrgBranch } from '$lib/types/org';
  import Icon from '$lib/components/ui/Icon.svelte';

  const uid = $props.id();

  let branches = $state<OrgBranch[]>([]);
  let isOwner = $state(false);
  let open = $state(false);
  let busy = $state('');
  let error = $state('');
  let root = $state<HTMLDivElement>();
  let button = $state<HTMLButtonElement>();

  const active = $derived(session.status === 'authenticated' && !session.isPlatform && !session.locked);
  const usable = $derived(branches.filter((b) => !b.suspended));
  const visible = $derived(active && isOwner && usable.length > 1);
  const current = $derived(branches.find((b) => b.current));

  let loadedFor = '';
  $effect(() => {
    const clinic = session.user?.clinicId ?? '';
    if (!active || !clinic || clinic === loadedFor) return;
    loadedFor = clinic;
    orgApi
      .overview()
      .then((o) => {
        isOwner = o.is_owner;
        branches = o.is_owner ? o.branches : [];
      })
      .catch(() => {
        isOwner = false;
        branches = [];
      });
  });

  function close(refocus = false) {
    open = false;
    error = '';
    if (refocus) button?.focus();
  }

  async function pick(b: OrgBranch) {
    if (b.current) return close(true);
    busy = b.id;
    error = '';
    try {
      await enterBranch(b.id);
    } catch (e) {
      busy = '';
      error = e instanceof Error ? e.message : 'No se pudo cambiar de sucursal.';
    }
  }

  function onKey(e: KeyboardEvent) {
    if (!open) return;
    if (e.key === 'Escape') {
      e.stopPropagation();
      close(true);
    } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      const items = [...(root?.querySelectorAll<HTMLButtonElement>('[role="menuitemradio"]') ?? [])];
      if (!items.length) return;
      e.preventDefault();
      const i = items.indexOf(document.activeElement as HTMLButtonElement);
      items[(i + (e.key === 'ArrowDown' ? 1 : -1) + items.length) % items.length].focus();
    }
  }

  function onPointer(e: PointerEvent) {
    if (open && root && !root.contains(e.target as Node)) close();
  }

  async function toggle() {
    open = !open;
    if (open) {
      await Promise.resolve();
      root?.querySelector<HTMLButtonElement>('[aria-checked="true"]')?.focus();
    }
  }
</script>

<svelte:window onkeydown={onKey} onpointerdown={onPointer} />

{#if visible}
  <div class="relative min-w-0" bind:this={root}>
    <button
      type="button"
      bind:this={button}
      class="inline-flex max-w-[11rem] items-center gap-1.5 rounded-full bg-app-ink/5 px-3 py-1.5 text-sm font-medium transition hover:bg-app-ink/10 sm:max-w-[16rem]"
      aria-haspopup="menu"
      aria-expanded={open}
      aria-controls="{uid}-menu"
      aria-label="Cambiar de sucursal. Sucursal actual: {current?.name ?? ''}"
      onclick={toggle}
    >
      <Icon name="building" size={16} />
      <span class="min-w-0 truncate">{current?.name ?? 'Sucursal'}</span>
      <Icon name="chevron-down" size={14} />
    </button>

    {#if open}
      <div
        id="{uid}-menu"
        role="menu"
        aria-label="Sucursales"
        class="fixed inset-x-3 top-16 z-50 max-h-[70vh] overflow-y-auto rounded-2xl border border-app-ink/10 bg-app-panel p-1.5 shadow-xl sm:absolute sm:inset-x-auto sm:left-0 sm:top-full sm:mt-2 sm:w-72"
      >
        {#each usable as b (b.id)}
          <button
            type="button"
            role="menuitemradio"
            aria-checked={b.current}
            class="flex w-full items-center gap-2 rounded-xl px-3 py-2.5 text-left text-sm transition hover:bg-app-ink/5 focus-visible:bg-app-ink/5 {b.current ? 'font-semibold' : ''}"
            disabled={busy !== ''}
            onclick={() => pick(b)}
          >
            <span class="min-w-0 flex-1">
              <span class="block truncate">{b.name}</span>
              {#if b.is_matrix}<span class="block text-xs font-normal text-app-muted">Matriz</span>{/if}
            </span>
            {#if busy === b.id}<span class="spin" aria-hidden="true"></span>{:else if b.current}<Icon name="check" size={16} />{/if}
          </button>
        {/each}
        {#if error}<p class="alert m-1.5" role="alert"><Icon name="alert" size={16} />{error}</p>{/if}
        <a href="/organizacion" class="mt-1 block rounded-xl px-3 py-2.5 text-sm text-app-primary hover:bg-app-ink/5" onclick={() => close()}>Administrar sucursales y reportes</a>
      </div>
    {/if}
  </div>
{/if}
