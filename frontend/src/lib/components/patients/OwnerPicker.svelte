<script lang="ts">
  import { ownersApi } from '$lib/api/owners';
  import type { OwnerListItem } from '$lib/types/owners';
  import Icon from '../ui/Icon.svelte';

  interface Props {
    /** the owner picked from the clinic's list (null = a new owner is typed in the fields) */
    selected: OwnerListItem | null;
    onpick: (o: OwnerListItem) => void;
    onclear: () => void;
    /** id for the search box (label association) */
    id?: string;
  }
  let { selected, onpick, onclear, id = 'owner-search' }: Props = $props();

  let q = $state('');
  let results = $state<OwnerListItem[]>([]);
  let open = $state(false);
  let busy = $state(false);
  let seq = 0;

  $effect(() => {
    const text = q.trim();
    if (selected || text.length < 2) {
      results = [];
      return;
    }
    const mine = ++seq;
    busy = true;
    const t = setTimeout(async () => {
      try {
        const r = await ownersApi.search(text);
        if (mine === seq) results = r;
      } catch {
        if (mine === seq) results = [];
      } finally {
        if (mine === seq) busy = false;
      }
    }, 250);
    return () => clearTimeout(t);
  });

  const petsText = (o: OwnerListItem) => (o.pets.length ? o.pets.map((p) => p.names).join(', ') : 'Sin mascotas activas');

  function pick(o: OwnerListItem) {
    open = false;
    q = '';
    onpick(o);
  }
</script>

{#if selected}
  <div class="rounded-xl border border-app-primary/30 bg-app-primary/5 p-3.5 text-sm" role="group" aria-label="Propietario elegido">
    <div class="flex flex-wrap items-start justify-between gap-2">
      <div class="min-w-0">
        <p class="flex items-center gap-2 font-semibold"><Icon name="user" size={16} class="text-app-primary" />{selected.name}</p>
        <p class="mt-0.5 text-app-muted">{[selected.phone, selected.email].filter(Boolean).join(' · ') || 'Sin datos de contacto'}</p>
        <p class="mt-1 text-xs text-app-muted">Mascotas registradas: {petsText(selected)}</p>
      </div>
      <button type="button" class="btn-ghost !min-h-8 px-2.5 text-xs" onclick={onclear}>Es otro propietario</button>
    </div>
  </div>
{:else}
  <div class="relative">
    <label class="label" for={id}>¿Ya está registrado el propietario?</label>
    <div class="relative">
      <Icon name="search" size={17} class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-app-muted" />
      <input
        {id}
        type="search"
        class="field pl-10"
        placeholder="Busca por nombre o teléfono del propietario…"
        autocomplete="off"
        role="combobox"
        aria-expanded={open && results.length > 0}
        aria-controls="{id}-list"
        bind:value={q}
        onfocus={() => (open = true)}
        onblur={() => setTimeout(() => (open = false), 150)}
      />
    </div>
    {#if open && q.trim().length >= 2}
      <ul id="{id}-list" role="listbox" class="absolute z-30 mt-1 max-h-72 w-full overflow-auto rounded-xl border border-app-ink/12 bg-app-panel p-1 shadow-app">
        {#each results as o (o.id)}
          <li role="option" aria-selected="false">
            <button type="button" class="w-full rounded-lg px-3 py-2.5 text-left hover:bg-app-ink/5 focus-visible:bg-app-ink/5 focus-visible:outline-none" onmousedown={(e) => e.preventDefault()} onclick={() => pick(o)}>
              <span class="block font-medium">{o.name} <span class="font-normal text-app-muted">· {o.phone || 'sin teléfono'}</span></span>
              <span class="block text-xs text-app-muted">{o.pet_count === 1 ? '1 mascota' : `${o.pet_count} mascotas`}: {petsText(o)}</span>
            </button>
          </li>
        {:else}
          <li class="px-3 py-2.5 text-sm text-app-muted" role="status">{busy ? 'Buscando…' : 'No hay propietarios con ese dato: escribe sus datos abajo para registrarlo.'}</li>
        {/each}
      </ul>
    {/if}
  </div>
{/if}
