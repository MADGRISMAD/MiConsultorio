<script lang="ts">
  import { onMount } from 'svelte';
  import { filesApi } from '$lib/api/files';
  import { specialtyApi } from '$lib/api/specialty';
  import type { Attachment } from '$lib/types/files';
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

  // photo comparison: two pictures of the patient chosen by the professional
  let photos = $state<Attachment[]>([]);
  let before = $state('');
  let after = $state('');
  let zone = $state('');
  async function loadPhotos() {
    try {
      photos = (await filesApi.list(patient.id)).files.filter((f) => ['image/jpeg', 'image/png', 'image/webp'].includes(f.mime) && f.size_bytes <= 4 << 20 && !f.archived_at);
    } catch {
      photos = [];
    }
  }
  const photoLabel = (f: Attachment) => `${f.created_at.slice(0, 10)} · ${f.title || f.original_name}`;

  function pick(t: AiTaskInfo) {
    current = t;
    param = '';
    result = null;
    op.reset?.();
    if (t.key === 'photo_compare') void loadPhotos();
    else if (!t.param && !t.disabled) void run();
  }

  async function comparePhotos() {
    if (!before || !after || before === after) return op.fail('Elige dos fotos distintas: antes y después.');
    await op.run(async () => {
      result = await specialtyApi.aiPhotoCompare(patient.id, { before, after, zone: zone.trim() });
    });
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
          {:else if current.key === 'photo_compare'}
            {#if photos.length < 2}
              <p class="mt-2 text-sm" role="status">Sube al menos dos fotos del paciente en la pestaña de archivos para compararlas.</p>
            {:else}
              <div class="mt-3 grid gap-3 sm:grid-cols-2">
                <label class="block"><span class="label">Foto de antes</span>
                  <select class="field" bind:value={before}><option value="">Elige…</option>{#each photos as f (f.id)}<option value={f.id}>{photoLabel(f)}</option>{/each}</select>
                </label>
                <label class="block"><span class="label">Foto de después</span>
                  <select class="field" bind:value={after}><option value="">Elige…</option>{#each photos as f (f.id)}<option value={f.id}>{photoLabel(f)}</option>{/each}</select>
                </label>
                <label class="block sm:col-span-2"><span class="label">Zona (opcional)</span><input class="field" maxlength="120" placeholder="Antebrazo derecho" bind:value={zone} /></label>
              </div>
              <p class="hint">Las dos fotos se envían a la IA solo para esta comparación y no se guardan en otro lugar. Describe cambios visibles; no diagnostica. Usa 1 uso de magia.</p>
              <button type="button" class="btn-primary mt-3" disabled={op.phase === 'loading'} onclick={comparePhotos}>
                {#if op.phase === 'loading'}<span class="spin"></span>Comparando…{:else}<Icon name="sparkles" size={18} />Comparar fotos{/if}
              </button>
            {/if}
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
