<script lang="ts">
  import { api } from '$lib/api';
  import { specialtyApi } from '$lib/api/specialty';
  import { moneyCents } from '$lib/format';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import type { CatalogItem } from '$lib/types';
  import type { PlanItemInput, TreatmentPlan } from '$lib/types/specialty';
  import Modal from '../Modal.svelte';
  import Icon from '../ui/Icon.svelte';

  interface Props {
    open: boolean;
    patientId: string;
    /** 'create' a plan, 'edit' a draft or proposed one, 'add' items to an accepted one */
    mode: 'create' | 'edit' | 'add';
    plan?: TreatmentPlan | null;
    dental?: boolean;
    onclose: () => void;
    onsaved: (plan: TreatmentPlan) => void;
  }
  let { open, patientId, mode, plan = null, dental = false, onclose, onsaved }: Props = $props();

  interface Row {
    phase: number;
    description: string;
    tooth: string;
    catalog_item_id: string;
    qty: number;
    price: number; // pesos
    tax_rate: number;
  }
  let title = $state('');
  let notes = $state('');
  let reason = $state('');
  let rows = $state<Row[]>([]);
  let catalog = $state<CatalogItem[]>([]);
  const op = new Op();
  const canCatalog = $derived(session.has('pos') && session.cobros);
  const uid = $props.id();

  $effect(() => {
    if (!open) return;
    op.reset();
    reason = '';
    title = mode === 'add' ? (plan?.title ?? '') : (plan?.title ?? '');
    notes = plan?.notes ?? '';
    rows =
      mode === 'edit' && plan
        ? plan.items.map((i) => ({ phase: i.phase, description: i.description, tooth: i.tooth, catalog_item_id: i.catalog_item_id ?? '', qty: i.qty, price: i.unit_price_cents / 100, tax_rate: i.tax_rate }))
        : [blank(1)];
    if (canCatalog && catalog.length === 0) api.pos.items({ active: true }).then((r) => (catalog = r.items)).catch(() => {});
  });

  function blank(phase: number): Row {
    return { phase, description: '', tooth: '', catalog_item_id: '', qty: 1, price: 0, tax_rate: 0 };
  }
  function addRow(phase?: number) {
    rows = [...rows, blank(phase ?? rows[rows.length - 1]?.phase ?? 1)];
  }
  function removeRow(i: number) {
    rows = rows.filter((_, k) => k !== i);
  }
  function describe(i: number) {
    const hit = catalog.find((c) => c.name.toLowerCase() === rows[i].description.trim().toLowerCase());
    if (hit) {
      rows[i].catalog_item_id = hit.id;
      rows[i].price = hit.price_cents / 100;
      rows[i].tax_rate = hit.tax_rate;
    } else rows[i].catalog_item_id = '';
  }
  const total = $derived(rows.reduce((s, r) => s + Math.round(r.qty * Math.round(r.price * 100)), 0));

  function items(): PlanItemInput[] {
    return rows
      .filter((r) => r.description.trim())
      .map((r) => ({
        phase: Math.max(1, Math.round(r.phase) || 1),
        description: r.description.trim(),
        tooth: r.tooth.trim(),
        ...(r.catalog_item_id ? { catalog_item_id: r.catalog_item_id } : {}),
        qty: r.qty > 0 ? r.qty : 1,
        unit_price_cents: Math.max(0, Math.round(r.price * 100)),
        tax_rate: r.tax_rate
      }));
  }
  async function submit() {
    const list = items();
    if (mode !== 'add' && !title.trim()) return op.fail('Escribe el título del plan.');
    if (list.length === 0) return op.fail('Agrega al menos un concepto con descripción.');
    let saved: TreatmentPlan | undefined;
    const ok = await op.run(async () => {
      if (mode === 'create') saved = await specialtyApi.createPlan(patientId, { title: title.trim(), notes: notes.trim(), items: list });
      else if (mode === 'edit') saved = await specialtyApi.updatePlan(plan!.id, { title: title.trim(), notes: notes.trim(), items: list });
      else saved = await specialtyApi.addPlanItems(plan!.id, list, reason.trim());
    });
    if (ok && saved) onsaved(saved);
  }
</script>

<Modal {open} title={mode === 'create' ? 'Nuevo plan de tratamiento' : mode === 'edit' ? 'Editar plan' : 'Agregar conceptos al plan'} {onclose} wide>
  <div class="space-y-4">
    {#if mode === 'add'}
      <p class="rounded-xl bg-app-warning/12 p-3 text-sm">El plan ya fue aceptado. Los conceptos nuevos crean la versión {(plan?.version ?? 1) + 1} y piden una nueva firma; lo ya aceptado se conserva.</p>
      <div>
        <label class="label" for="{uid}-reason">Motivo</label>
        <input id="{uid}-reason" class="field" maxlength="300" bind:value={reason} placeholder="Ej. Hallazgo en la revisión del 12 de marzo" />
      </div>
    {:else}
      <div>
        <label class="label" for="{uid}-title">Título</label>
        <input id="{uid}-title" class="field" maxlength="150" bind:value={title} placeholder="Ej. Rehabilitación oral" />
      </div>
      <div>
        <label class="label" for="{uid}-notes">Notas para el paciente (opcional)</label>
        <textarea id="{uid}-notes" class="field" rows="2" bind:value={notes}></textarea>
      </div>
    {/if}

    <datalist id="{uid}-catalog">{#each catalog as c (c.id)}<option value={c.name}></option>{/each}</datalist>
    <div class="space-y-3">
      {#each rows as r, i (i)}
        <fieldset class="rounded-2xl border border-app-ink/10 p-3">
          <legend class="px-1 text-xs text-app-muted">Concepto {i + 1}</legend>
          <div class="grid grid-cols-6 gap-3">
            <div class="col-span-6 sm:col-span-4">
              <label class="label" for="{uid}-d{i}">Descripción</label>
              <input id="{uid}-d{i}" class="field" maxlength="300" list={canCatalog ? `${uid}-catalog` : undefined} bind:value={r.description} oninput={() => describe(i)} placeholder={canCatalog ? 'Escribe o elige del catálogo' : 'Ej. Resina en pieza 16'} />
              {#if r.catalog_item_id}<p class="hint">Del catálogo de cobros: precio y IVA tomados de ahí.</p>{/if}
            </div>
            <div class="col-span-3 sm:col-span-1">
              <label class="label" for="{uid}-p{i}">Fase</label>
              <input id="{uid}-p{i}" class="field" type="number" min="1" max="50" step="1" bind:value={r.phase} />
            </div>
            <div class="col-span-3 sm:col-span-1">
              <label class="label" for="{uid}-t{i}">{dental ? 'Pieza' : 'Zona'}</label>
              <input id="{uid}-t{i}" class="field" maxlength="40" bind:value={r.tooth} placeholder={dental ? '16' : ''} />
            </div>
            <div class="col-span-3 sm:col-span-2">
              <label class="label" for="{uid}-q{i}">Cantidad</label>
              <input id="{uid}-q{i}" class="field" type="number" min="0.001" step="any" bind:value={r.qty} />
            </div>
            <div class="col-span-3 sm:col-span-2">
              <label class="label" for="{uid}-u{i}">Precio unitario ($)</label>
              <input id="{uid}-u{i}" class="field" type="number" min="0" step="0.01" bind:value={r.price} />
            </div>
            <div class="col-span-6 flex items-end justify-between gap-2 sm:col-span-2">
              <p class="pb-2 text-sm"><span class="text-app-muted">Importe</span> <strong>{moneyCents(Math.round(r.qty * Math.round(r.price * 100)))}</strong></p>
              {#if rows.length > 1}<button type="button" class="btn-ghost text-app-danger" aria-label="Quitar concepto {i + 1}" onclick={() => removeRow(i)}><Icon name="trash" size={16} />Quitar</button>{/if}
            </div>
          </div>
        </fieldset>
      {/each}
    </div>
    <div class="flex flex-wrap items-center justify-between gap-2">
      <div class="flex gap-2">
        <button type="button" class="btn-secondary" onclick={() => addRow()}><Icon name="plus" size={18} />Agregar concepto</button>
        <button type="button" class="btn-ghost" onclick={() => addRow(Math.max(...rows.map((r) => r.phase), 0) + 1)}>Nueva fase</button>
      </div>
      <p class="text-sm">Total: <strong class="text-base">{moneyCents(total)}</strong></p>
    </div>
    {#if !canCatalog}<p class="hint">Sin acceso al catálogo de cobros: escribe la descripción y el precio a mano.</p>{/if}
    {#if op.phase === 'error'}<p class="alert" role="alert"><Icon name="alert" size={18} />{op.message}</p>{/if}
  </div>
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
    <button type="button" class="btn-primary" disabled={op.phase === 'loading'} onclick={submit}>{#if op.phase === 'loading'}<span class="spin"></span>{/if}{mode === 'add' ? 'Agregar' : 'Guardar'}</button>
  {/snippet}
</Modal>
