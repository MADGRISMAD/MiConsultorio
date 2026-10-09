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
  let openId = $state('');
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
  <ul class="grid gap-3">
    {#each shown as p, i (p.id)}
      {@const isOpen = openId === p.id || (openId === '' && i === 0)}
      <li class="card px-4 py-4 sm:px-5">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0">
            <p class="font-medium">Plan nutricional{i === 0 ? ' (el más reciente)' : ''}</p>
            <p class="text-sm text-app-muted">{fmt(p.created_at)}{p.by ? ` · ${p.by}` : ''}</p>
            {#if multi}<p class="mt-0.5 text-sm">Paciente: <strong>{p.patient_name}</strong></p>{/if}
            {#if p.data.goal}<p class="mt-1 text-sm">Objetivo: <strong>{p.data.goal}</strong></p>{/if}
          </div>
          {#if p.data.kcal}<span class="pill pill-ok">{p.data.kcal} kcal al día</span>{/if}
        </div>
        {#if isOpen}
          <div class="mt-4 grid gap-4 border-t border-app-ink/10 pt-4 text-sm">
            {#each daysOf(p) as day}
              <div>
                <p class="section-title mb-1">{day.name}</p>
                <ul class="grid gap-1">
                  {#each day.meals as m}
                    <li><strong>{m.name}</strong>{m.time ? ` · ${m.time}` : ''}<span class="block whitespace-pre-line text-app-muted">{m.items}</span></li>
                  {/each}
                </ul>
              </div>
            {/each}
            {#each [['Recomendaciones', p.data.recommendations], ['Alimentos o hábitos a evitar', p.data.avoid], ['Suplementos', p.data.supplements]] as [title, text]}
              {#if text?.trim()}<div><p class="section-title mb-1">{title}</p><p class="whitespace-pre-line">{text}</p></div>{/if}
            {/each}
            {#if p.data.follow_up_days}<p class="text-app-muted">Siguiente cita en aproximadamente {p.data.follow_up_days} días.</p>{/if}
          </div>
        {/if}
        <div class="mt-3 flex flex-wrap gap-2">
          <button class="btn-secondary !min-h-9" aria-expanded={isOpen} onclick={() => (openId = isOpen ? '-' : p.id)}><Icon name="eye" size={16} />{isOpen ? 'Ocultar' : 'Ver plan'}</button>
          <button class="btn-secondary !min-h-9" disabled={printOp.phase === 'loading'} onclick={() => printOp.run(() => printPortalPlan(p, clinic))}><Icon name="receipt" size={16} />Imprimir o guardar PDF</button>
        </div>
      </li>
    {/each}
  </ul>
{/if}
