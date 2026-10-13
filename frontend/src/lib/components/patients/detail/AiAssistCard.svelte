<script lang="ts">
  import { onMount } from 'svelte';
  import { specialtyApi } from '$lib/api/specialty';
  import { Op } from '$lib/op.svelte';
  import { dateTime, moneyCents } from '$lib/format';
  import { printAiDoc } from '$lib/print';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { Patient } from '$lib/types';
  import type { AiResult, AiTaskInfo } from '$lib/types/specialty';
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

  async function copy() {
    const r = result?.result;
    if (!r) return;
    const text = r.text || [r.title, r.summary, ...r.sections.flatMap((s) => [s.heading, ...s.items.map((i) => `• ${i}`)])].filter(Boolean).join('\n');
    try {
      await navigator.clipboard.writeText(text);
      toast.show('Copiado');
    } catch {
      toast.show('No se pudo copiar; selecciona el texto a mano');
    }
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
        {@const r = result.result}
        <div class="mt-4 space-y-4" aria-live="polite">
          <p class="rounded-lg bg-amber-50 px-3 py-2 text-sm text-amber-900" role="note"><strong>{result.draft ? 'Borrador.' : 'Solo es una sugerencia.'}</strong> {result.disclaimer}</p>
          {#if r.title}<h3 class="display text-xl">{r.title}</h3>{/if}
          {#if r.summary}<p class="text-[15px] leading-relaxed">{r.summary}</p>{/if}
          {#each r.sections as s (s.heading)}
            <div>
              <p class="section-title mb-1">{s.heading}</p>
              <ul class="list-disc space-y-0.5 pl-5 text-sm">{#each s.items as it}<li>{it}</li>{/each}</ul>
            </div>
          {/each}
          {#if r.text}
            <div>
              <p class="section-title mb-1">Texto</p>
              <p class="whitespace-pre-wrap rounded-xl bg-app-ink/5 p-3 text-sm">{r.text}</p>
            </div>
          {/if}
          {#if r.lines.length}
            <ul class="divide-y divide-app-ink/8 rounded-xl border border-app-ink/12 text-sm">
              {#each r.lines as l, i (i)}
                <li class="px-3 py-2">
                  <p class="font-medium">{l.name}{#if l.qty > 1} <span class="text-app-muted">× {l.qty}</span>{/if}{#if l.unit_price_cents} <span class="float-right">{moneyCents(l.unit_price_cents * l.qty)}</span>{/if}</p>
                  {#if l.detail}<p class="text-app-muted">{l.detail}</p>{/if}
                  {#each l.alerts ?? [] as a}<p class="mt-1 rounded bg-red-50 px-2 py-1 text-red-800" role="alert">{a}</p>{/each}
                </li>
              {/each}
            </ul>
            {#if result.task === 'dental_budget'}
              <OpError op={budgetOp} />
              <button type="button" class="btn-primary" disabled={budgetOp.phase === 'loading'} onclick={makeBudget}><Icon name="plus" size={18} />Crear presupuesto con estas líneas</button>
            {/if}
          {/if}
          {#each r.notices ?? [] as n}<p class="text-sm text-app-muted">{n}</p>{/each}
          {#if result.basis}
            <details class="text-sm"><summary class="cursor-pointer text-app-muted">Cifras en las que se basa (calculadas por el sistema)</summary><p class="mt-1 whitespace-pre-line">{result.basis}</p></details>
          {/if}
          <div class="flex flex-wrap gap-2">
            <button type="button" class="btn-secondary" onclick={copy}><Icon name="file" size={16} />Copiar</button>
            <button type="button" class="btn-secondary" onclick={print}><Icon name="receipt" size={16} />Imprimir</button>
          </div>
          <p class="text-xs text-app-muted">Generado {dateTime(result.generated_at)}.</p>
        </div>
      {/if}
    {/if}
  </section>
{/if}
