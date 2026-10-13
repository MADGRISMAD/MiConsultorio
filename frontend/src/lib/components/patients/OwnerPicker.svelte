<script lang="ts">
  import { ownersApi } from '$lib/api/owners';
  import type { OwnerListItem } from '$lib/types/owners';
  import Combobox from '../ui/Combobox.svelte';
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
  let busy = $state(false);
  let error = $state('');
  let seq = 0;

  $effect(() => {
    const text = q.trim();
    if (selected || text.length < 2) {
      results = [];
      return;
    }
    const mine = ++seq;
    busy = true;
    error = '';
    const t = setTimeout(async () => {
      try {
        const r = await ownersApi.search(text);
        if (mine === seq) results = r;
      } catch (e) {
        if (mine === seq) {
          results = [];
          error = e instanceof Error ? e.message : 'No se pudo buscar.';
        }
      } finally {
        if (mine === seq) busy = false;
      }
    }, 250);
    return () => clearTimeout(t);
  });

  const petsText = (o: OwnerListItem) => (o.pets.length ? o.pets.map((p) => (p.species ? `${p.names} (${p.species})` : p.names)).join(', ') : 'Sin mascotas activas');

  function pick(o: OwnerListItem) {
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
  <Combobox
    {id}
    label="¿Ya está registrado el propietario?"
    type="search"
    placeholder="Busca por nombre o teléfono del propietario…"
    bind:query={q}
    options={results.map((o) => ({ key: o.id, value: o }))}
    status={busy ? 'Buscando…' : error ? error : results.length === 0 ? 'No hay propietarios con ese dato: escribe sus datos abajo para registrarlo.' : ''}
    statusTone={error ? 'danger' : 'muted'}
    minChars={2}
    onpick={(o) => pick(o.value)}
  >
    {#snippet option(o)}
      <span class="block px-3 py-2.5">
        <span class="block font-medium">{o.value.name} <span class="font-normal text-app-muted">· {o.value.phone || 'sin teléfono'}</span></span>
        <span class="block text-xs text-app-muted">{o.value.pet_count === 1 ? '1 mascota' : `${o.value.pet_count} mascotas`}: {petsText(o.value)}</span>
      </span>
    {/snippet}
  </Combobox>
{/if}
