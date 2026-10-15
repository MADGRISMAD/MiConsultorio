<script lang="ts">
  import { goto } from '$app/navigation';
  import { api } from '$lib/api';
  import type { PatientRow } from '$lib/types';
  import Icon from '../ui/Icon.svelte';

  let query = $state('');
  let results = $state<PatientRow[]>([]);
  let loading = $state(false);
  let failed = $state(false);
  let expanded = $state(false);
  let selected = $state(-1);
  let input: HTMLInputElement;
  let root: HTMLDivElement;
  const uid = $props.id();

  $effect(() => {
    const q = query.trim();
    let current = true;
    results = [];
    selected = -1;
    failed = false;
    loading = !!q;
    const timer = setTimeout(async () => {
      if (!q) return;
      try {
        const rows = await api.patients.lookup(q);
        if (current) results = rows;
      } catch {
        if (current) failed = true;
      } finally {
        if (current) loading = false;
      }
    }, 300);
    return () => { current = false; clearTimeout(timer); };
  });

  function shortcut(e: KeyboardEvent) {
    const editing = (e.target as HTMLElement)?.closest('input, textarea, select, [contenteditable="true"]');
    if ((!editing && e.key === '/') || ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k')) {
      e.preventDefault();
      input?.focus();
    }
  }
  function navigate(e: KeyboardEvent) {
    if (e.key === 'Escape') { expanded = false; selected = -1; }
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      expanded = true;
      selected = results.length ? (selected < 0 ? (e.key === 'ArrowDown' ? 0 : results.length - 1) : (selected + (e.key === 'ArrowDown' ? 1 : results.length - 1)) % results.length) : -1;
      if (selected >= 0) document.getElementById(`${uid}-${selected}`)?.scrollIntoView({ block: 'nearest' });
    }
    if (e.key === 'Enter' && expanded && results[selected]) {
      e.preventDefault();
      void goto(`/pacientes/${encodeURIComponent(results[selected].id)}`);
    }
  }
</script>

<svelte:window onkeydown={shortcut} onpointerdown={(e) => { if (root && !root.contains(e.target as Node)) expanded = false; }} />
<div class="relative min-w-0 flex-1" bind:this={root} onfocusout={(e) => { if (!root.contains(e.relatedTarget as Node)) expanded = false; }}>
  <Icon name="search" size={18} class="pointer-events-none absolute left-3 top-3.5 text-app-muted" />
  <input bind:this={input} bind:value={query} oninput={() => (expanded = true)} onfocus={() => (expanded = true)} onkeydown={navigate}
    role="combobox" aria-label="Buscar paciente" aria-autocomplete="list" aria-expanded={expanded && !!query.trim()} aria-controls="{uid}-results"
    aria-activedescendant={expanded && selected >= 0 ? `${uid}-${selected}` : undefined}
    class="field min-h-11 pl-10" placeholder="Buscar paciente…" autocomplete="off" />
  {#if expanded && query.trim()}
    <div class="absolute inset-x-0 top-full z-30 mt-2 max-h-80 overflow-y-auto rounded-xl border border-app-ink/15 bg-app-panel p-1 shadow-xl">
      <p class="px-3 text-sm text-app-muted" role="status">{loading ? 'Buscando…' : failed ? 'No pudimos buscar. Intenta de nuevo.' : results.length === 0 ? 'Sin resultados. Prueba otro nombre, CURP o teléfono.' : `${results.length} resultados`}</p>
      <ul id="{uid}-results" role="listbox" aria-label="Pacientes">
        {#each results as p, i (p.id)}
          <li role="option" aria-selected={selected === i} id="{uid}-{i}">
            <a href="/pacientes/{encodeURIComponent(p.id)}" class="block min-h-11 rounded-lg px-3 py-2 focus-visible:ring-2 focus-visible:ring-app-primary hover:bg-app-primary/10 {selected === i ? 'bg-app-primary/10' : ''}">
              <span class="block font-semibold">{p.names} {p.last_names}</span>
              <span class="text-xs text-app-muted">#{p.file_number}{p.phone ? ` · ${p.phone}` : ''}</span>
            </a>
          </li>
        {/each}
      </ul>
    </div>
  {/if}
</div>
