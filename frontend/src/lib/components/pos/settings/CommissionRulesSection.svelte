<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { pos2 } from '$lib/api/pos2';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { CatalogItem } from '$lib/types';
  import type { CommissionRule, Professional } from '$lib/types/pos2';
  import ConfirmModal from '$lib/components/ConfirmModal.svelte';
  import Modal from '$lib/components/Modal.svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import { parsePesos } from '../catalog/helpers';

  const uid = $props.id();
  let rules = $state<CommissionRule[]>([]);
  let pros = $state<Professional[]>([]);
  let items = $state<CatalogItem[]>([]);
  let categories = $state<string[]>([]);
  let loading = $state(true);
  let error = $state('');

  async function load() {
    try {
      const [r, p, i] = await Promise.all([pos2.rules(), pos2.professionals(), api.pos.items({ active: true })]);
      rules = r;
      pros = p;
      items = i.items;
      categories = i.categories;
      error = '';
    } catch (e) {
      error = e instanceof Error ? e.message : 'No se pudieron cargar las reglas.';
    } finally {
      loading = false;
    }
  }
  onMount(load);

  // ---- describe a rule ----
  const scope = (r: CommissionRule) => (r.item_id ? `Artículo: ${r.item_name}` : r.category ? `Categoría: ${r.category}` : 'Todos los conceptos');
  const who = (r: CommissionRule) => (r.user_id ? r.user_name || 'Profesional' : 'Todos los profesionales');
  const pctText = (n: number) => `${n.toLocaleString('es-MX', { maximumFractionDigits: 2 })} %`;
  const priority = (r: CommissionRule) => (r.item_id ? 'Artículo' : r.category ? 'Categoría' : r.user_id ? 'Profesional' : 'General');

  // ---- form ----
  let formOpen = $state(false);
  let editing = $state<CommissionRule | null>(null);
  let appliesTo = $state<'all' | 'category' | 'item'>('all');
  let userId = $state('');
  let category = $state('');
  let itemId = $state('');
  let percentStr = $state('');
  let active = $state(true);
  let formError = $state('');
  const op = new Op();

  function openForm(r: CommissionRule | null) {
    editing = r;
    appliesTo = r?.item_id ? 'item' : r?.category ? 'category' : 'all';
    userId = r?.user_id ?? '';
    category = r?.category ?? '';
    itemId = r?.item_id ?? '';
    percentStr = r ? String(r.percent) : '';
    active = r?.active ?? true;
    formError = '';
    op.reset();
    formOpen = true;
  }

  async function save(ev: SubmitEvent) {
    ev.preventDefault();
    const percent = parsePesos(percentStr);
    if (percentStr.trim() === '' || Number.isNaN(percent) || percent < 0 || percent > 100) return void (formError = 'El porcentaje debe estar entre 0 y 100.');
    if (appliesTo === 'category' && !category.trim()) return void (formError = 'Elige la categoría.');
    if (appliesTo === 'item' && !itemId) return void (formError = 'Elige el artículo.');
    formError = '';
    const body = {
      percent,
      active,
      user_id: userId || undefined,
      category: appliesTo === 'category' ? category.trim() : undefined,
      item_id: appliesTo === 'item' ? itemId : undefined
    };
    const cur = editing;
    if (await op.run(() => (cur ? pos2.updateRule(cur.id, body) : pos2.createRule(body)))) {
      formOpen = false;
      toast.show(cur ? 'Regla actualizada' : 'Regla creada');
      void load();
    }
  }

  async function toggle(r: CommissionRule) {
    try {
      await pos2.updateRule(r.id, { percent: r.percent, active: !r.active, user_id: r.user_id ?? undefined, item_id: r.item_id ?? undefined, category: r.category ?? undefined });
      void load();
    } catch (e) {
      toast.show(e instanceof Error ? e.message : 'No se pudo actualizar.', 'error');
    }
  }

  let removing = $state<CommissionRule | null>(null);
  const delOp = new Op();
  async function remove() {
    const r = removing;
    if (r && (await delOp.run(() => pos2.deleteRule(r.id)))) {
      removing = null;
      toast.show('Regla eliminada');
      void load();
    }
  }
</script>

<section class="card p-6" aria-labelledby="{uid}-t">
  <div class="flex flex-wrap items-start justify-between gap-3">
    <div>
      <h2 id="{uid}-t" class="display text-2xl">Comisiones</h2>
      <p class="mt-1 max-w-prose text-sm text-app-muted">
        Define qué porcentaje gana cada profesional. Si varias reglas aplican a un concepto gana la más específica:
        <strong>artículo</strong>, luego <strong>categoría</strong>, luego <strong>profesional</strong> y por último la <strong>general</strong>. Se calcula sobre lo cobrado sin IVA, después de descuentos, y queda fijo al vender.
      </p>
    </div>
    <button type="button" class="btn-primary" onclick={() => openForm(null)}><Icon name="plus" size={18} />Nueva regla</button>
  </div>

  <div class="mt-5">
    {#if loading}
      <LoadingRows />
    {:else if error}
      <p class="alert" role="alert"><Icon name="alert" size={18} />{error}</p>
    {:else if !rules.length}
      <EmptyState icon="tag" title="Sin reglas de comisión" text="Crea una regla general (por ejemplo 30 % para todos) y afínala por profesional, categoría o artículo.">
        <button type="button" class="btn-primary" onclick={() => openForm(null)}>Nueva regla</button>
      </EmptyState>
    {:else}
      <ul class="divide-y divide-app-ink/10 rounded-2xl ring-1 ring-inset ring-app-ink/10">
        {#each rules as r (r.id)}
          <li class="flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3 {r.active ? '' : 'opacity-60'}">
            <span class="display w-20 text-2xl tabular-nums">{pctText(r.percent)}</span>
            <span class="min-w-0 flex-1 text-sm">
              <span class="block font-medium">{scope(r)}</span>
              <span class="block text-xs text-app-muted">{who(r)} · prioridad {priority(r).toLowerCase()}</span>
            </span>
            <button type="button" class="btn-ghost min-h-9 px-3 text-xs" role="switch" aria-checked={r.active} onclick={() => toggle(r)}>{r.active ? 'Activa' : 'Pausada'}</button>
            <button type="button" class="icon-btn" aria-label="Editar regla de {scope(r)}" onclick={() => openForm(r)}><Icon name="edit" size={17} /></button>
            <button type="button" class="icon-btn danger" aria-label="Eliminar regla de {scope(r)}" onclick={() => ((removing = r), delOp.reset())}><Icon name="trash" size={17} /></button>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
</section>

<Modal open={formOpen} title={editing ? 'Editar regla' : 'Nueva regla de comisión'} onclose={() => (formOpen = false)}>
  <form id="{uid}-f" class="space-y-4" onsubmit={save} novalidate>
    <div>
      <label class="label" for="{uid}-pro">Profesional</label>
      <select id="{uid}-pro" class="field" bind:value={userId}>
        <option value="">Todos los profesionales</option>
        {#each pros as p (p.id)}<option value={p.id}>{p.name}</option>{/each}
      </select>
    </div>
    <div>
      <label class="label" for="{uid}-ap">Aplica a</label>
      <select id="{uid}-ap" class="field" bind:value={appliesTo}>
        <option value="all">Todos los conceptos</option>
        <option value="category">Una categoría</option>
        <option value="item">Un artículo o servicio</option>
      </select>
    </div>
    {#if appliesTo === 'category'}
      <div>
        <label class="label" for="{uid}-cat">Categoría</label>
        <input id="{uid}-cat" class="field" bind:value={category} list="{uid}-cats" maxlength="60" autocomplete="off" />
        <datalist id="{uid}-cats">{#each categories as c}<option value={c}></option>{/each}</datalist>
      </div>
    {:else if appliesTo === 'item'}
      <div>
        <label class="label" for="{uid}-item">Artículo o servicio</label>
        <select id="{uid}-item" class="field" bind:value={itemId}>
          <option value="">Elige uno…</option>
          {#each items as i (i.id)}<option value={i.id}>{i.name}</option>{/each}
        </select>
      </div>
    {/if}
    <div>
      <label class="label" for="{uid}-pct">Porcentaje</label>
      <div class="relative">
        <input id="{uid}-pct" class="field pr-8 text-right tabular-nums" inputmode="decimal" bind:value={percentStr} autocomplete="off" placeholder="0" aria-invalid={!!formError} />
        <span class="pointer-events-none absolute inset-y-0 right-3.5 grid place-items-center text-app-muted" aria-hidden="true">%</span>
      </div>
    </div>
    <label class="flex cursor-pointer items-center gap-3 text-sm"><input type="checkbox" class="h-4 w-4" bind:checked={active} />Regla activa</label>
    {#if formError}<p class="alert" role="alert"><Icon name="alert" size={18} />{formError}</p>{/if}
    {#if op.phase === 'error'}<p class="alert" role="alert"><Icon name="alert" size={18} />{op.message}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={() => (formOpen = false)}>Cancelar</button>
    <button type="submit" form="{uid}-f" class="btn-primary" disabled={op.phase === 'loading'}>{#if op.phase === 'loading'}<span class="spin"></span>{/if}Guardar</button>
  {/snippet}
</Modal>

<ConfirmModal open={!!removing} title="Eliminar regla" op={delOp} confirmLabel="Eliminar" onconfirm={remove} onclose={() => (removing = null)}>
  {#if removing}La regla «{scope(removing)} · {pctText(removing.percent)}» dejará de aplicarse a nuevas ventas. Las comisiones ya calculadas no cambian.{/if}
</ConfirmModal>
