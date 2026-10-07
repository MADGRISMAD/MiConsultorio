<script lang="ts">
  import { api } from '$lib/api';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import { moneyCents } from '$lib/format';
  import type { CatalogItem } from '$lib/types';
  import Modal from '$lib/components/Modal.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { parsePesos, toCents, toInput, toPesos } from './helpers';
  import { magicMessage } from './magicErrors';

  interface Props {
    open: boolean;
    /** Active catalog items; only those with a cost can be priced. */
    items: CatalogItem[];
    ondone: () => void;
    onclose: () => void;
  }
  let { open, items, ondone, onclose }: Props = $props();

  interface Sug {
    item: CatalogItem;
    price: string;
    note: string;
    accept: boolean;
  }

  const uid = $props.id();
  const askOp = new Op();
  const saveOp = new Op();
  const candidates = $derived(items.filter((i) => i.active && i.cost_cents > 0));
  let selected = $state<Set<string>>(new Set());
  let margin = $state('30');
  let sugs = $state<Sug[] | null>(null);
  let usedAi = $state(false);
  let usage = $state<{ limit: number; used: number } | null>(null);

  $effect(() => {
    if (!open) return;
    selected = new Set(candidates.map((c) => c.id));
    margin = '30';
    sugs = null;
    askOp.reset();
    saveOp.reset();
    api.pos.magic().then((u) => (usage = u)).catch(() => (usage = null));
  });

  const marginNum = $derived(parsePesos(margin));
  const marginOk = $derived(!Number.isNaN(marginNum) && marginNum > 0 && marginNum < 95);
  const allOn = $derived(candidates.length > 0 && candidates.every((c) => selected.has(c.id)));

  function toggle(id: string) {
    const s = new Set(selected);
    if (s.has(id)) s.delete(id);
    else s.add(id);
    selected = s;
  }
  const toggleAll = () => (selected = allOn ? new Set() : new Set(candidates.map((c) => c.id)));

  async function ask() {
    const chosen = candidates.filter((c) => selected.has(c.id));
    let res: Awaited<ReturnType<typeof api.pos.magicPrice>> | undefined;
    const ok = await askOp.run(async () => {
      try {
        res = await api.pos.magicPrice(marginNum, chosen.map((c) => ({ id: c.id, name: c.name, cost_cents: c.cost_cents })));
      } catch (e) {
        throw new Error(magicMessage(e));
      }
    });
    if (!ok || !res) return;
    usedAi = res.ai;
    const byId = new Map(res.suggestions.map((s) => [s.id, s]));
    sugs = chosen
      .filter((c) => byId.has(c.id))
      .map((c) => {
        const s = byId.get(c.id)!;
        return { item: c, price: toPesos(s.price_cents), note: s.note, accept: s.price_cents !== c.price_cents };
      });
    api.pos.magic().then((u) => (usage = u)).catch(() => {});
  }

  const rowBad = (s: Sug) => Number.isNaN(parsePesos(s.price)) || parsePesos(s.price) < 0;
  const toApply = $derived(sugs ? sugs.filter((s) => s.accept && !rowBad(s)) : []);

  async function apply() {
    const list = toApply;
    let done = 0;
    const ok = await saveOp.run(async () => {
      for (const s of list) {
        await api.pos.updateItem(s.item.id, toInput(s.item, { price_cents: toCents(parsePesos(s.price)) }));
        done++;
        s.accept = false; // so a retry only redoes what is left
      }
    });
    if (done) {
      toast.show(`${done} ${done === 1 ? 'precio actualizado' : 'precios actualizados'}`);
      ondone();
    }
    if (ok) onclose();
  }
</script>

<Modal {open} title="Precio Mágico" {onclose} wide>
  <div class="space-y-4">
    {#if usage}<p class="flex items-center gap-2 text-xs text-app-muted"><Icon name="sparkles" size={14} />Usos de magia: {usage.used} de {usage.limit} este mes</p>{/if}

    {#if sugs === null}
      <p class="text-sm text-app-muted">Elige los artículos y el margen de ganancia que quieres; te proponemos un precio de venta a partir de su costo. Solo aparecen artículos con costo registrado.</p>
      {#if !candidates.length}
        <div class="rounded-2xl bg-app-elevated p-5 text-center text-sm text-app-muted">Aún no hay artículos con costo. Agrega el costo al editar un producto para poder sugerir su precio.</div>
      {:else}
        <div class="max-w-[12rem]">
          <label class="label" for="{uid}-m">Margen deseado (%)</label>
          <input id="{uid}-m" class="field" inputmode="decimal" bind:value={margin} aria-invalid={!marginOk} />
          {#if !marginOk}<p class="mt-1 text-xs text-app-danger" role="alert">Escribe un margen entre 1 y 94.</p>{/if}
        </div>
        <div class="overflow-x-auto rounded-2xl border border-app-ink/10">
          <table class="w-full min-w-[30rem]">
            <thead class="border-b border-app-ink/10 bg-app-elevated">
              <tr>
                <th class="th w-10"><input type="checkbox" class="h-4 w-4 accent-[rgb(var(--app-primary))]" checked={allOn} onchange={toggleAll} aria-label="Seleccionar todos" /></th>
                <th class="th">Artículo</th><th class="th text-right">Costo</th><th class="th text-right">Precio actual</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-app-ink/8">
              {#each candidates as c (c.id)}
                <tr>
                  <td class="td"><input type="checkbox" class="h-4 w-4 accent-[rgb(var(--app-primary))]" checked={selected.has(c.id)} onchange={() => toggle(c.id)} aria-label="Seleccionar {c.name}" /></td>
                  <td class="td font-medium">{c.name}</td>
                  <td class="td text-right tabular-nums">{moneyCents(c.cost_cents)}</td>
                  <td class="td text-right tabular-nums">{moneyCents(c.price_cents)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
      <div aria-live="polite">
        {#if askOp.phase === 'loading'}<p class="flex items-center gap-2 text-sm text-app-muted"><span class="spin"></span>Calculando precios sugeridos…</p>
        {:else if askOp.phase === 'error'}<p class="alert" role="alert"><Icon name="alert" size={18} />{askOp.message}</p>{/if}
      </div>
    {:else}
      <p class="flex items-center gap-2 text-sm">
        {#if usedAi}<span class="pill pill-info"><Icon name="sparkles" size={12} />Sugerido con IA</span>{:else}<span class="pill">Solo fórmula de margen</span>{/if}
        <span class="text-app-muted">Margen objetivo {marginNum} %. Edita o desmarca lo que no quieras cambiar.</span>
      </p>
      <div class="overflow-x-auto rounded-2xl border border-app-ink/10">
        <table class="w-full min-w-[42rem]">
          <thead class="border-b border-app-ink/10 bg-app-elevated">
            <tr><th class="th w-10"><span class="sr-only">Aplicar</span></th><th class="th">Artículo</th><th class="th text-right">Actual</th><th class="th">Sugerido (MXN)</th><th class="th">Nota</th></tr>
          </thead>
          <tbody class="divide-y divide-app-ink/8">
            {#each sugs as s (s.item.id)}
              <tr>
                <td class="td"><input type="checkbox" class="h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={s.accept} aria-label="Aplicar a {s.item.name}" /></td>
                <td class="td font-medium">{s.item.name}<span class="block text-xs font-normal text-app-muted">Costo {moneyCents(s.item.cost_cents)}</span></td>
                <td class="td text-right tabular-nums text-app-muted">{moneyCents(s.item.price_cents)}</td>
                <td class="td"><input class="field !min-h-9 !w-28 !px-2 !text-sm" inputmode="decimal" bind:value={s.price} aria-label="Precio sugerido para {s.item.name}" aria-invalid={rowBad(s)} /></td>
                <td class="td text-xs text-app-muted">{s.note || '—'}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
      {#if saveOp.phase === 'error'}<p class="alert" role="alert"><Icon name="alert" size={18} />{saveOp.message}</p>{/if}
    {/if}
  </div>
  {#snippet footer()}
    {#if sugs === null}
      <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
      <button type="button" class="btn-primary" disabled={!marginOk || !selected.size || askOp.phase === 'loading'} onclick={ask}><Icon name="sparkles" size={16} />Sugerir precios</button>
    {:else}
      <button type="button" class="btn-secondary" onclick={() => (sugs = null)}>Volver</button>
      <button type="button" class="btn-primary" disabled={!toApply.length || saveOp.phase === 'loading'} onclick={apply}>
        {#if saveOp.phase === 'loading'}<span class="spin"></span>{/if}Aplicar {toApply.length} {toApply.length === 1 ? 'precio' : 'precios'}
      </button>
    {/if}
  {/snippet}
</Modal>
