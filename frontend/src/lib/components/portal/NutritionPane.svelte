<script lang="ts">
  import { onMount } from 'svelte';
  import { portalApi } from '$lib/api/portal';
  import { Op } from '$lib/op.svelte';
  import type { PortalNutritionPlan } from '$lib/types/portal';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import { fmtDate } from '$lib/i18n/index.svelte';
  import { printPortalPlan } from './print';

  let { plans, patientId, multi, clinic, loaded, error }: { plans: PortalNutritionPlan[]; patientId: string; multi: boolean; clinic: { name: string; address: string; phone: string }; loaded: boolean; error: string } = $props();

  const shown = $derived(plans.filter((p) => !patientId || p.patient_id === patientId));
  let open = $state<Record<string, boolean>>({});
  let first = true;
  $effect(() => {
    // the newest plan starts expanded
    if (first && shown.length) {
      open[shown[0].id] = true;
      first = false;
    }
  });
  const printOp = new Op();
  const fmt = (iso: string) => fmtDate(iso, { day: 'numeric', month: 'long', year: 'numeric' });
  const daysOf = (p: PortalNutritionPlan) => (p.data.days?.length ? p.data.days : p.data.meals?.length ? [{ name: 'Día tipo', meals: p.data.meals }] : []);
</script>

{#if !loaded && !error}
  <LoadingRows />
{:else if error}
  <p class="alert" role="alert"><Icon name="alert" size={18} />{error}</p>
{:else if shown.length === 0}
  <div class="card px-5 py-8 text-center text-sm text-app-muted">Aún no tienes un plan nutricional.</div>
{:else}
  {#if printOp.phase === 'error'}<p class="alert mb-3" role="alert"><Icon name="alert" size={18} />{printOp.message}</p>{/if}
  <div class="card overflow-x-auto">
    <table class="w-full min-w-[34rem] text-sm">
      <thead>
        <tr class="text-left text-xs uppercase tracking-wide text-app-muted">
          <th class="w-10 px-3 py-2"><span class="sr-only">Expandir</span></th>
          <th class="px-3 py-2 font-medium">Fecha</th>
          <th class="px-3 py-2 font-medium">Objetivo</th>
          <th class="px-3 py-2 font-medium">Energía</th>
          <th class="px-3 py-2 font-medium">Elaboró</th>
          {#if multi}<th class="px-3 py-2 font-medium">Paciente</th>{/if}
          <th class="px-3 py-2"><span class="sr-only">Acciones</span></th>
        </tr>
      </thead>
      {#each shown as p, i (p.id)}
        {@const isOpen = !!open[p.id]}
        <tbody class="border-t border-app-ink/8">
          <tr>
            <td class="px-3 py-2">
              <button class="icon-btn" aria-expanded={isOpen} aria-label={isOpen ? 'Contraer plan' : 'Expandir plan'} onclick={() => (open[p.id] = !isOpen)}>
                <span class="transition {isOpen ? 'rotate-180' : ''}"><Icon name="chevron-down" size={18} /></span>
              </button>
            </td>
            <td class="whitespace-nowrap px-3 py-2 font-medium">{fmt(p.created_at)}{#if i === 0}<span class="pill pill-ok ml-2">Más reciente</span>{/if}</td>
            <td class="px-3 py-2">{p.data.goal || '—'}</td>
            <td class="whitespace-nowrap px-3 py-2">{p.data.kcal ? `${p.data.kcal} kcal` : '—'}</td>
            <td class="px-3 py-2 text-app-muted">{p.by || '—'}</td>
            {#if multi}<td class="px-3 py-2">{p.patient_name}</td>{/if}
            <td class="px-3 py-2 text-right">
              <button class="btn-secondary !min-h-9" disabled={printOp.phase === 'loading'} onclick={() => printOp.run(() => printPortalPlan(p, clinic))}><Icon name="receipt" size={16} />PDF</button>
            </td>
          </tr>
          {#if isOpen}
            <tr>
              <td></td>
              <td colspan={multi ? 6 : 5} class="px-3 pb-4">
                {#if daysOf(p).length}
                  {@const cols = daysOf(p)[0].meals.map((m) => m.name)}
                  <div class="overflow-x-auto">
                    <table class="w-full min-w-[32rem] border-collapse text-xs">
                      <thead>
                        <tr><th class="border border-app-ink/12 bg-app-ink/5 px-2 py-1 text-left"></th>{#each cols as c}<th class="border border-app-ink/12 bg-app-ink/5 px-2 py-1 text-left">{c}</th>{/each}</tr>
                      </thead>
                      <tbody>
                        {#each daysOf(p) as day}
                          <tr>
                            <th class="border border-app-ink/12 px-2 py-1 text-left align-top">{day.name}</th>
                            {#each cols as _, k}<td class="whitespace-pre-line border border-app-ink/12 px-2 py-1 align-top">{day.meals[k]?.items ?? ''}{day.meals[k]?.kcal ? ` · ${day.meals[k].kcal} kcal` : ''}</td>{/each}
                          </tr>
                        {/each}
                      </tbody>
                    </table>
                  </div>
                {/if}
                <div class="mt-3 grid gap-3 text-sm sm:grid-cols-2">
                  {#each [['Recomendaciones', p.data.recommendations], ['Alimentos o hábitos a evitar', p.data.avoid], ['Suplementos', p.data.supplements]] as [title, text]}
                    {#if text?.trim()}<div><p class="section-title mb-1">{title}</p><p class="whitespace-pre-line">{text}</p></div>{/if}
                  {/each}
                </div>
                {#if p.data.follow_up_days}<p class="mt-2 text-sm text-app-muted">Siguiente cita en aproximadamente {p.data.follow_up_days} días.</p>{/if}
              </td>
            </tr>
          {/if}
        </tbody>
      {/each}
    </table>
  </div>
{/if}
