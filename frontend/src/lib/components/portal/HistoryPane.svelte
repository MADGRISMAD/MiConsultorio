<script lang="ts">
  import type { PortalHistoryItem } from '$lib/types/portal';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import { fmtDate } from '$lib/i18n/index.svelte';

  let { items, areas, patientId, multi, loaded, error }: { items: PortalHistoryItem[]; areas: { id: string; label: string }[]; patientId: string; multi: boolean; loaded: boolean; error: string } = $props();

  const shown = $derived(items.filter((i) => !patientId || i.patient_id === patientId));
  // one section per branch of the clinic that has something to show
  const groups = $derived(areas.map((a) => ({ ...a, rows: shown.filter((i) => i.area === a.id) })).filter((g) => g.rows.length > 0));
  let closed = $state<Record<string, boolean>>({});
  const fmt = (iso: string) => fmtDate(iso, { day: 'numeric', month: 'short', year: 'numeric' });
  const TYPES: Record<string, string> = { consulta: 'Consulta', receta: 'Receta', plan: 'Plan', vacuna: 'Vacuna', cita: 'Cita atendida' };
  const allClosed = $derived(groups.length > 0 && groups.every((g) => closed[g.id]));
  function setAll(v: boolean) {
    closed = Object.fromEntries(groups.map((g) => [g.id, v]));
  }
</script>

{#if !loaded && !error}
  <LoadingRows />
{:else if error}
  <p class="alert" role="alert"><Icon name="alert" size={18} />{error}</p>
{:else if groups.length === 0}
  <div class="card px-5 py-8 text-center text-sm text-app-muted">Aún no hay movimientos en tu historial.</div>
{:else}
  <div class="mb-3 flex justify-end">
    <button class="btn-ghost !min-h-9" onclick={() => setAll(!allClosed)}>{allClosed ? 'Expandir todo' : 'Contraer todo'}</button>
  </div>
  <div class="grid gap-3">
    {#each groups as g (g.id)}
      {@const isClosed = !!closed[g.id]}
      <section class="card overflow-hidden">
        <button class="flex w-full items-center justify-between gap-3 px-4 py-3 text-left sm:px-5" aria-expanded={!isClosed} onclick={() => (closed[g.id] = !isClosed)}>
          <span class="font-semibold">{g.label} <span class="ml-1.5 rounded-full bg-app-ink/8 px-1.5 py-0.5 font-mono text-[11px] font-normal">{g.rows.length}</span></span>
          <span class="text-app-muted transition {isClosed ? '' : 'rotate-180'}"><Icon name="chevron-down" size={18} /></span>
        </button>
        {#if !isClosed}
          <div class="overflow-x-auto border-t border-app-ink/10">
            <table class="w-full min-w-[34rem] text-sm">
              <thead>
                <tr class="text-left text-xs uppercase tracking-wide text-app-muted">
                  <th class="px-4 py-2 font-medium sm:px-5">Fecha</th>
                  <th class="px-3 py-2 font-medium">Qué se hizo</th>
                  <th class="px-3 py-2 font-medium">Atendió</th>
                  {#if multi}<th class="px-3 py-2 font-medium">Paciente</th>{/if}
                </tr>
              </thead>
              <tbody class="divide-y divide-app-ink/8">
                {#each g.rows as r (r.type + r.id)}
                  <tr>
                    <td class="whitespace-nowrap px-4 py-2 sm:px-5">{fmt(r.at)}</td>
                    <td class="px-3 py-2"><span class="mr-1.5 rounded-full bg-app-ink/6 px-2 py-0.5 text-[11px] text-app-muted">{TYPES[r.type] ?? r.type}</span>{r.title}</td>
                    <td class="px-3 py-2 text-app-muted">{r.by || '—'}</td>
                    {#if multi}<td class="px-3 py-2">{r.patient_name}</td>{/if}
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </section>
    {/each}
  </div>
{/if}
