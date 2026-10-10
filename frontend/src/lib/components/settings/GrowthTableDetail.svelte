<script lang="ts">
  import { growthApi } from '$lib/api/lab';
  import { dateTime as fmtDate } from '$lib/format';
  import { Op } from '$lib/op.svelte';
  import type { GrowthCoverage, GrowthImportRecord } from '$lib/types/lab';
  import Modal from '../Modal.svelte';
  import OpError from '../ui/OpError.svelte';
  import LoadingRows from '../ui/LoadingRows.svelte';

  let { table, onclose }: { table: GrowthImportRecord | null; onclose: () => void } = $props();

  let groups = $state<GrowthCoverage[]>([]);
  const op = new Op();
  $effect(() => {
    const t = table;
    groups = [];
    if (!t) return;
    void op.run(async () => {
      groups = (await growthApi.detail(t.id)).groups;
    });
  });

  const INDICATORS: [string, string][] = [
    ['weight_for_age', 'Peso para la edad'],
    ['length_height_for_age', 'Talla / longitud para la edad'],
    ['bmi_for_age', 'IMC para la edad'],
    ['head_circumference_for_age', 'Perímetro cefálico para la edad']
  ];
  /** the axis goes to the end of the table (5 years for the WHO, 20 for the CDC), never beyond 20 */
  const axisMax = $derived(Math.max(60, Math.ceil(Math.max(0, ...groups.map((g) => g.max_age_months)) / 60) * 60));
  const years = (m: number) => (m < 24 ? `${Math.round(m * 10) / 10} m` : `${Math.round((m / 12) * 10) / 10} años`);
  const get = (ind: string, sex: string) => groups.find((g) => g.indicator === ind && g.sex === sex);
  const ticks = $derived(Array.from({ length: axisMax / 60 + 1 }, (_, i) => i * 60));
  // the source as the clinic reads it: no web addresses
  const clean = (s: string) => s.replace(/\s*https?:\/\/\S+/g, '').trim();
</script>

<Modal open={!!table} title={table ? `${table.standard} · versión ${table.version}` : ''} onclose={onclose} wide>
  {#if table}
    <p class="text-sm text-app-muted">{clean(table.source_name)}</p>
    <p class="mt-1 flex flex-wrap items-center gap-2 text-xs text-app-muted">
      <span>{table.row_count} filas</span>·
      {#if table.platform}<span class="font-semibold text-app-primary">Incluida con Caresia</span>{:else}<span>Cargada el {fmtDate(table.created_at)} por {table.created_by_name}</span>{/if}
    </p>

    <h4 class="section-title mb-2 mt-5">Qué edades cubre</h4>
    <OpError {op} />
    {#if op.phase === 'loading'}
      <LoadingRows />
    {:else if groups.length}
      <div class="grid gap-4">
        {#each INDICATORS as [ind, label]}
          {#if get(ind, 'M') || get(ind, 'F')}
            <div>
              <p class="mb-1.5 text-sm font-medium">{label}</p>
              {#each [['M', 'Niños', 'bg-app-primary/80'], ['F', 'Niñas', 'bg-app-warning/80']] as [sex, who, tone]}
                {@const g = get(ind, sex)}
                <div class="grid grid-cols-[3.5rem_1fr] items-center gap-2 py-0.5 text-xs">
                  <span class="text-app-muted">{who}</span>
                  <div class="relative h-5 rounded-full bg-app-ink/6" role="img" aria-label={g ? `${who}: de ${years(g.min_age_months)} a ${years(g.max_age_months)}, ${g.rows} filas` : `${who}: sin datos`}>
                    {#if g}
                      <div class="absolute inset-y-0 rounded-full {tone}" style="left: {(g.min_age_months / axisMax) * 100}%; width: {Math.max(2, ((g.max_age_months - g.min_age_months) / axisMax) * 100)}%"></div>
                      <span class="absolute inset-y-0 flex items-center whitespace-nowrap px-2 font-medium text-app-ink" style="left: {(g.min_age_months / axisMax) * 100}%">{years(g.min_age_months)} – {years(g.max_age_months)}</span>
                    {:else}
                      <span class="absolute inset-0 grid place-items-center text-app-muted">sin datos</span>
                    {/if}
                  </div>
                </div>
              {/each}
            </div>
          {/if}
        {/each}
        <div class="grid grid-cols-[3.5rem_1fr] gap-2 text-[11px] text-app-muted" aria-hidden="true">
          <span></span>
          <div class="relative h-4">
            {#each ticks as t}<span class="absolute whitespace-nowrap -translate-x-1/2 first:translate-x-0 last:-translate-x-full" style="left: {(t / axisMax) * 100}%">{t === 0 ? 'Nac.' : `${t / 12} años`}</span>{/each}
          </div>
        </div>
      </div>
      <p class="mt-4 text-xs text-app-muted">Cada barra muestra desde qué edad hasta qué edad hay valores de referencia. Fuera de ese rango no se calcula percentil ni Z (no se extrapola).</p>
    {/if}
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>Cerrar</button>
  {/snippet}
</Modal>
