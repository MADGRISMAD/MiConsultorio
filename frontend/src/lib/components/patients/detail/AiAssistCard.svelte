<script lang="ts">
  import { onMount } from 'svelte';
  import { specialtyApi } from '$lib/api/specialty';
  import { Op } from '$lib/op.svelte';
    import { printAiDoc } from '$lib/print';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { Patient } from '$lib/types';
  import type { AiResult, AiTaskInfo } from '$lib/types/specialty';
  import AiResultView from './AiResultView.svelte';
  import OpError from '../../ui/OpError.svelte';
  import Icon from '../../ui/Icon.svelte';

  let { patient, oncreated }: { patient: Patient; oncreated?: () => void } = $props();

  /** Tasks that live elsewhere in the screen (the code finder sits in the consultation form, the rehab advice has its own card). */
  const ELSEWHERE = ['icd_suggest'];

  let tasks = $state<AiTaskInfo[]>([]);
  let open = $state(false);
  let current = $state<AiTaskInfo | null>(null);
  let param = $state('');
  let result = $state<AiResult | null>(null);
  const op = new Op();
  const budgetOp = new Op();

  onMount(async () => {
    if (!session.magic) return;
    try {
      tasks = (await specialtyApi.aiTasks(patient.id)).filter((t) => !ELSEWHERE.includes(t.key));
    } catch {
      tasks = [];
    }
  });

  function pick(t: AiTaskInfo) {
    current = t;
    param = '';
    result = null;
    op.reset?.();
    if (!t.param && !t.disabled) void run();
  }

  async function run() {
    const t = current;
    if (!t) return;
    if (t.param?.required && !param.trim()) return op.fail(`Falta: ${t.param.label.toLowerCase()}.`);
    await op.run(async () => {
      result = await specialtyApi.aiRun(patient.id, t.key, param.trim());
    });
  }

  const print = () => {
    const r = result;
    if (!r) return;
    // a message prints as the text; the rest print their lists
    return printAiDoc(patient, { title: r.result.title || r.label, summary: r.result.summary, sections: r.task === 'patient_message' ? [] : r.result.sections, text: r.result.text }, session.user?.name ?? '');
  };

  async function makeBudget() {
    const r = result;
    if (!r) return;
    await budgetOp.run(async () => {
      await specialtyApi.createPlan(patient.id, {
        title: 'Presupuesto (borrador sugerido)',
        notes: 'Borrador generado con IA a partir del odontograma: revisa piezas, cantidades y precios antes de proponerlo al paciente.',
        items: r.result.lines.map((l) => ({ phase: 1, description: l.name, tooth: l.detail === 'general' ? '' : l.detail, catalog_item_id: l.catalog_item_id, qty: l.qty, unit_price_cents: l.unit_price_cents ?? 0, tax_rate: l.tax_rate ?? 0 }))
      });
      toast.show('Presupuesto creado como borrador en «Plan de tratamiento»');
      oncreated?.();
    });
  }
</script>

{#if session.magic && tasks.length}
  <section class="card p-5 sm:p-6" aria-labelledby="ai-assist-h">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 id="ai-assist-h" class="display text-2xl">Ayudas con IA</h2>
        <p class="mt-1 max-w-xl text-sm text-app-muted">Borradores y sugerencias para ahorrarte tiempo. Tú los revisas y decides: la IA no ve el nombre del paciente y nada se guarda solo. Cada una usa 1 uso de magia.</p>
      </div>
      <button type="button" class="btn-secondary" aria-expanded={open} onclick={() => (open = !open)}><Icon name="sparkles" size={18} />{open ? 'Cerrar' : 'Ver ayudas'}</button>
    </div>

    {#if open}
      <div class="mt-4 flex flex-wrap gap-2" role="group" aria-label="Ayudas disponibles">
        {#each tasks as t (t.key)}
          <button type="button" class="chip" class:chip-on={current?.key === t.key} aria-pressed={current?.key === t.key} title={t.disabled || t.desc} onclick={() => pick(t)}>{t.label}</button>
        {/each}
      </div>

      {#if current}
        <div class="mt-4 rounded-xl border border-app-ink/12 p-4">
          <p class="text-sm font-medium">{current.label}</p>
          <p class="text-sm text-app-muted">{current.desc}</p>
          {#if current.disabled}
            <p class="mt-2 text-sm" role="status">{current.disabled}</p>
          {:else}
            {#if current.param}
              <label class="mt-3 block">
                <span class="label">{current.param.label}</span>
                {#if current.param.options?.length}
                  <select class="field" bind:value={param}>
                    <option value="">Elige…</option>
                    {#each current.param.options as o}<option value={o}>{o}</option>{/each}
                  </select>
                {:else}
                  <input class="field" maxlength="400" placeholder={current.param.hint ?? ''} bind:value={param} />
                {/if}
              </label>
              <button type="button" class="btn-primary mt-3" disabled={op.phase === 'loading'} onclick={run}>
                {#if op.phase === 'loading'}<span class="spin"></span>Preparando…{:else}<Icon name="sparkles" size={18} />{result ? 'Volver a generar' : 'Generar'}{/if}
              </button>
            {:else if op.phase === 'loading'}
              <p class="mt-3 text-sm"><span class="spin"></span> Preparando…</p>
            {/if}
          {/if}
          <OpError {op} class="mt-3" />
        </div>
      {/if}

      {#if result}
        <div class="mt-4">
          <AiResultView {result} onprint={print}>
            {#if result.task === 'dental_budget' && result.result.lines.length}
              <OpError op={budgetOp} />
              <button type="button" class="btn-primary" disabled={budgetOp.phase === 'loading'} onclick={makeBudget}><Icon name="plus" size={18} />Crear presupuesto con estas líneas</button>
            {/if}
          </AiResultView>
        </div>
      {/if}
    {/if}
  </section>
{/if}
