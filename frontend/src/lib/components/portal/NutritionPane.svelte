<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onMount } from 'svelte';
  import { portalApi } from '$lib/api/portal';
  import { Op } from '$lib/op.svelte';
  import type { PortalNutritionPlan } from '$lib/types/portal';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import { fmtDate, t } from '$lib/i18n/index.svelte';
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
  <Alert>{error}</Alert>
{:else if shown.length === 0}
  <div class="card-empty">Aún no tienes un plan nutricional.</div>
{:else}
  <OpError op={printOp} class="mb-3" />
  <ul class="grid gap-3">
    {#each shown as p, i (p.id)}
      {@const isOpen = !!open[p.id]}
      <li class="card px-4 py-4 sm:px-5">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0">
            <p class="font-medium">Plan nutricional del {fmt(p.created_at)}{#if i === 0}<span class="pill pill-ok ml-2">Más reciente</span>{/if}</p>
            {#if p.data.goal}<p class="mt-0.5 text-sm">Objetivo: {p.data.goal}</p>{/if}
            <p class="mt-0.5 text-sm text-app-muted">{p.data.kcal ? `${p.data.kcal} kcal al día` : ''}{p.data.kcal && p.by ? ' · ' : ''}{p.by ? `Elaboró ${p.by}` : ''}</p>
            {#if multi}<p class="mt-0.5 text-sm">Paciente: <strong>{p.patient_name}</strong></p>{/if}
          </div>
        </div>
        {#if isOpen}
          <div class="mt-4 border-t border-app-ink/10 pt-4">
            {#if daysOf(p).length}
              {@const cols = daysOf(p)[0].meals.map((m) => m.name)}
              <table class="w-full table-fixed border-collapse text-[11px] leading-snug sm:text-xs">
                <thead>
                  <tr>
                    <th class="w-[13%] border border-app-ink/12 bg-app-ink/5 px-1.5 py-1 text-left"></th>
                    {#each cols as c}<th class="border border-app-ink/12 bg-app-ink/5 px-1.5 py-1 text-left">{c}</th>{/each}
                  </tr>
                </thead>
                <tbody>
                  {#each daysOf(p) as day}
                    <tr>
                      <th class="break-words border border-app-ink/12 px-1.5 py-1 text-left align-top">{day.name}</th>
                      {#each cols as _, k}<td class="whitespace-pre-line break-words border border-app-ink/12 px-1.5 py-1 align-top">{day.meals[k]?.items ?? ''}</td>{/each}
                    </tr>
                  {/each}
                </tbody>
              </table>
            {/if}
            <div class="mt-4 grid gap-3 text-sm">
              {#each [['Recomendaciones', p.data.recommendations], ['Alimentos o hábitos a evitar', p.data.avoid], ['Suplementos', p.data.supplements]] as [title, text]}
                {#if text?.trim()}<div><p class="section-title mb-1">{title}</p><p class="whitespace-pre-line">{text}</p></div>{/if}
              {/each}
              {#if p.data.follow_up_days}<p class="text-app-muted">Siguiente cita en aproximadamente {p.data.follow_up_days} días.</p>{/if}
            </div>
          </div>
        {/if}
        <div class="mt-3 flex flex-wrap gap-2">
          <button class="btn-secondary !min-h-9" aria-expanded={isOpen} onclick={() => (open[p.id] = !isOpen)}>
            <Icon name="eye" size={16} />{isOpen ? t('portal.rx.hide') : t('portal.rx.detail')}
          </button>
          <button class="btn-secondary !min-h-9" disabled={printOp.phase === 'loading'} onclick={() => printOp.run(() => printPortalPlan(p, clinic))}><Icon name="receipt" size={16} />{t('portal.rx.print')}</button>
        </div>
      </li>
    {/each}
  </ul>
{/if}
