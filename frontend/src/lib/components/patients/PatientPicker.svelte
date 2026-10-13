<script lang="ts">
  import { api } from '$lib/api';
  import type { Patient, PatientRow } from '$lib/types';
  import Combobox, { type Option } from '../ui/Combobox.svelte';
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
  let loading = $state(false);
  let error = $state('');
  let quickOpen = $state(false);
  let timer: ReturnType<typeof setTimeout> | undefined;
  let seq = 0;

  const q = $derived(query.trim());
  // the last option is always "register new" (value null)
  const options = $derived<Option<PatientRow | null>[]>([...results.map((p) => ({ key: p.id, value: p as PatientRow | null })), { key: 'new', value: null }]);
  const status = $derived(
    error ? error : q.length < 2 ? 'Escribe al menos 2 letras para buscar.' : loading ? 'Buscando…' : results.length === 0 ? 'Sin coincidencias.' : `${results.length} ${results.length === 1 ? 'resultado' : 'resultados'}.`
  );

  function oninput(text: string) {
    const term = text.trim();
    clearTimeout(timer);
    error = '';
    if (term.length < 2) {
      results = [];
      loading = false;
      return;
    }
    loading = true;
    const mine = ++seq;
    timer = setTimeout(async () => {
      try {
        const r = await api.patients.lookup(term);
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
    onpick?.(p);
  }
  function clear() {
    value = null;
    onclear?.();
  }
  function openQuick() {
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
    <Combobox
      id="{uid}-in"
      {label}
      placeholder="Buscar por nombre, teléfono o número de expediente…"
      bind:query
      options={options}
      status={status}
      statusTone={error ? 'danger' : 'muted'}
      {oninput}
      onpick={(o) => (o.value ? pick(o.value) : openQuick())}
    >
      {#snippet option(o)}
        {#if o.value}
          {@const p = o.value}
          <span class="flex items-center gap-3 px-3 py-2">
            <span class="grid h-8 w-8 flex-none place-items-center rounded-full bg-app-ink/8 text-app-muted"><Icon name={p.subject === 'animal' ? 'paw' : 'user'} size={16} /></span>
            <span class="min-w-0 flex-1">
              <span class="flex items-center gap-2 text-sm font-semibold"><span class="truncate">{fullName(p)} <span class="font-normal text-app-muted">#{p.file_number}</span></span>{#if p.subject === 'animal' && p.species}<span class="pill pill-info flex-none">{p.species}</span>{/if}</span>
              <span class="block truncate text-xs text-app-muted">{[subtitle(p, false), p.phone || p.guardian_phone].filter(Boolean).join(' · ') || 'Sin más datos'}</span>
            </span>
          </span>
        {:else}
          <span class="flex items-center gap-3 px-3 py-2.5 text-sm font-medium text-app-primary"><Icon name="user-plus" size={18} />+ Registrar paciente nuevo</span>
        {/if}
      {/snippet}
    </Combobox>
  {/if}
</div>

<QuickPatientModal open={quickOpen} onclose={() => (quickOpen = false)} oncreated={created} />
