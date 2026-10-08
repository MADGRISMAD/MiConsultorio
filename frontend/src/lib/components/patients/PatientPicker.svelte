<script lang="ts">
  import { api } from '$lib/api';
  import type { Patient, PatientRow } from '$lib/types';
  import Icon from '../ui/Icon.svelte';
  import QuickPatientModal from './QuickPatientModal.svelte';
  import { fullName, subtitle } from './util';

  interface Props {
    value?: PatientRow | null;
    onpick?: (p: PatientRow) => void;
    /** called when the person clears the selection ("Cambiar") */
    onclear?: () => void;
    label?: string;
  }
  let { value = $bindable(null), onpick, onclear, label = 'Paciente registrado' }: Props = $props();

  const uid = $props.id();
  let query = $state('');
  let results = $state<PatientRow[]>([]);
  let open = $state(false);
  let loading = $state(false);
  let error = $state('');
  let active = $state(0);
  let quickOpen = $state(false);
  let input = $state<HTMLInputElement>();
  let timer: ReturnType<typeof setTimeout> | undefined;
  let seq = 0;

  const q = $derived(query.trim());
  // the last option is always "register new"
  const optionCount = $derived(results.length + 1);

  function onInput() {
    open = true;
    active = 0;
    clearTimeout(timer);
    error = '';
    if (q.length < 2) {
      results = [];
      loading = false;
      return;
    }
    loading = true;
    const mine = ++seq;
    timer = setTimeout(async () => {
      try {
        const r = await api.patients.lookup(q);
        if (mine === seq) results = r;
      } catch (e) {
        if (mine === seq) {
          results = [];
          error = e instanceof Error ? e.message : 'No se pudo buscar.';
        }
      } finally {
        if (mine === seq) loading = false;
      }
    }, 250);
  }

  function pick(p: PatientRow) {
    value = p;
    query = '';
    results = [];
    open = false;
    onpick?.(p);
  }
  function clear() {
    value = null;
    onclear?.();
    queueMicrotask(() => input?.focus());
  }
  function openQuick() {
    open = false;
    quickOpen = true;
  }
  function created(p: Patient) {
    quickOpen = false;
    pick({
      id: p.id,
      file_number: p.file_number,
      subject: p.subject,
      names: p.names,
      last_names: p.last_names,
      age: p.age,
      phone: p.phone,
      guardian_name: p.guardian_name,
      species: typeof p.profile?.species === 'string' ? (p.profile.species as string) : undefined,
      incomplete: p.incomplete,
      no_privacy_notice: !p.privacy_notice_at,
      last_encounter_at: p.last_encounter_at,
      archived_at: p.archived_at
    });
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      if (!open) open = true;
      else active = (active + 1) % optionCount;
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      active = (active - 1 + optionCount) % optionCount;
    } else if (e.key === 'Enter') {
      if (!open) return;
      e.preventDefault();
      if (active < results.length) pick(results[active]);
      else openQuick();
    } else if (e.key === 'Escape' && open) {
      e.stopPropagation();
      open = false;
    }
  }
  function onfocusout(e: FocusEvent) {
    const next = e.relatedTarget as Node | null;
    if (!next || !(e.currentTarget as HTMLElement).contains(next)) open = false;
  }
</script>

<div class="text-left">
  {#if value}
    <span class="label" id="{uid}-l">{label}</span>
    <div class="flex items-center gap-3 rounded-xl border border-app-ink/15 bg-app-elevated px-3.5 py-2.5" aria-labelledby="{uid}-l" role="group">
      <span class="grid h-9 w-9 flex-none place-items-center rounded-full bg-app-primary/12 text-app-primary"><Icon name={value.subject === 'animal' ? 'paw' : 'user'} size={18} /></span>
      <span class="min-w-0 flex-1">
        <span class="flex items-center gap-2"><span class="truncate font-semibold">{fullName(value)}</span>{#if value.subject === 'animal' && value.species}<span class="pill pill-info flex-none">{value.species}</span>{/if}</span>
        <span class="block truncate text-xs text-app-muted">#{value.file_number}{subtitle(value, false) ? ` · ${subtitle(value, false)}` : ''}{value.phone ? ` · ${value.phone}` : ''}</span>
      </span>
      <button type="button" class="btn-ghost -mr-1 min-h-9 px-3" onclick={clear}>Cambiar</button>
    </div>
  {:else}
    <label class="label" for="{uid}-in">{label}</label>
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="relative" {onfocusout}>
      <Icon name="search" size={18} class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-app-muted" />
      <input
        bind:this={input}
        id="{uid}-in"
        class="field pl-10"
        type="text"
        role="combobox"
        autocomplete="off"
        placeholder="Buscar por nombre, teléfono o número de expediente…"
        aria-expanded={open}
        aria-controls="{uid}-list"
        aria-autocomplete="list"
        aria-activedescendant={open ? `${uid}-o${active}` : undefined}
        bind:value={query}
        oninput={onInput}
        onfocus={() => (open = true)}
        {onkeydown}
      />
      {#if open}
        <div class="absolute inset-x-0 top-full z-30 mt-1.5 overflow-hidden rounded-2xl border border-app-ink/10 bg-app-panel shadow-app">
        <ul id="{uid}-list" role="listbox" aria-label="Pacientes" class="max-h-64 overflow-y-auto p-1.5">
          {#each results as p, i (p.id)}
            <li
              id="{uid}-o{i}"
              role="option"
              aria-selected={active === i}
              class="flex cursor-pointer items-center gap-3 rounded-xl px-3 py-2 {active === i ? 'bg-app-primary/10' : 'hover:bg-app-ink/5'}"
              onmousedown={(e) => e.preventDefault()}
              onclick={() => pick(p)}
              onkeydown={() => {}}
              onmousemove={() => (active = i)}
            >
              <span class="grid h-8 w-8 flex-none place-items-center rounded-full bg-app-ink/8 text-app-muted"><Icon name={p.subject === 'animal' ? 'paw' : 'user'} size={16} /></span>
              <span class="min-w-0 flex-1">
                <span class="flex items-center gap-2 text-sm font-semibold"><span class="truncate">{fullName(p)} <span class="font-normal text-app-muted">#{p.file_number}</span></span>{#if p.subject === 'animal' && p.species}<span class="pill pill-info flex-none">{p.species}</span>{/if}</span>
                <span class="block truncate text-xs text-app-muted">{[subtitle(p, false), p.phone || p.guardian_phone].filter(Boolean).join(' · ') || 'Sin más datos'}</span>
              </span>
            </li>
          {/each}
          <li
            id="{uid}-o{results.length}"
            role="option"
            aria-selected={active === results.length}
            class="flex cursor-pointer items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium text-app-primary {active === results.length ? 'bg-app-primary/10' : 'hover:bg-app-ink/5'}"
            onmousedown={(e) => e.preventDefault()}
            onclick={openQuick}
            onkeydown={() => {}}
            onmousemove={() => (active = results.length)}
          >
            <Icon name="user-plus" size={18} />+ Registrar paciente nuevo
          </li>
        </ul>
          <p class="border-t border-app-ink/8 px-4 py-2 text-xs text-app-muted" aria-live="polite">
            {#if error}<span class="text-app-danger">{error}</span>
            {:else if q.length < 2}Escribe al menos 2 letras para buscar.
            {:else if loading}Buscando…
            {:else if results.length === 0}Sin coincidencias.
            {:else}{results.length} {results.length === 1 ? 'resultado' : 'resultados'}.{/if}
          </p>
        </div>
      {/if}
    </div>
  {/if}
</div>

<QuickPatientModal open={quickOpen} onclose={() => (quickOpen = false)} oncreated={created} />
