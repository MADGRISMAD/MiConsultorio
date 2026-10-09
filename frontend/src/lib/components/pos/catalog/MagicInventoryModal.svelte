<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { api } from '$lib/api';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { CatalogInput, ItemKind } from '$lib/types';
  import Modal from '$lib/components/Modal.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { MAGIC_MAX_BYTES, MAGIC_MIMES, blankInput, imageToBase64, parsePesos, toCents, toPesos } from './helpers';
  import { magicMessage } from './magicErrors';

  interface Props {
    open: boolean;
    ondone: () => void;
    onclose: () => void;
  }
  let { open, ondone, onclose }: Props = $props();

  interface Row {
    key: number;
    kind: ItemKind;
    name: string;
    category: string;
    price: string;
    cost: string;
    stock: string;
    unit: string;
    barcode: string;
  }

  const uid = $props.id();
  const askOp = new Op();
  const saveOp = new Op();
  let mode = $state<'text' | 'photo'>('text');
  let text = $state('');
  let photo = $state<{ base64: string; mime: string; preview: string; name: string } | null>(null);
  let photoError = $state('');
  let rows = $state<Row[] | null>(null);
  let usage = $state<{ available: boolean; limit: number; used: number } | null>(null);
  let nextKey = 1;

  async function loadUsage() {
    try {
      usage = await api.pos.magic();
    } catch {
      usage = null;
    }
  }

  $effect(() => {
    if (!open) return;
    text = '';
    photo = null;
    photoError = '';
    rows = null;
    mode = 'text';
    askOp.reset();
    saveOp.reset();
    loadUsage();
  });

  const left = $derived(usage ? Math.max(0, usage.limit - usage.used) : null);
  const blocked = $derived(usage !== null && (!usage.available || left === 0));

  async function onphoto(e: Event) {
    const input = e.currentTarget as HTMLInputElement;
    const f = input.files?.[0];
    input.value = '';
    photoError = '';
    if (!f) return;
    if (!MAGIC_MIMES.includes(f.type)) return void (photoError = 'Usa una foto JPG, PNG o WebP.');
    if (f.size > MAGIC_MAX_BYTES) return void (photoError = 'La foto pesa más de 5 MB. Toma una con menos resolución.');
    try {
      const r = await imageToBase64(f);
      photo = { ...r, name: f.name };
    } catch (err) {
      photoError = err instanceof Error ? err.message : 'No se pudo leer la foto.';
    }
  }

  const canAsk = $derived(!blocked && (mode === 'text' ? text.trim().length > 2 : !!photo));

  async function ask() {
    let out: Awaited<ReturnType<typeof api.pos.magicInventory>> = [];
    const ok = await askOp.run(async () => {
      try {
        out = await api.pos.magicInventory(mode === 'text' ? { text: text.trim() } : { image_base64: photo!.base64, mime: photo!.mime });
      } catch (e) {
        throw new Error(magicMessage(e));
      }
    });
    loadUsage();
    if (!ok) return;
    if (!out.length) return askOp.fail('No encontramos artículos ahí. Intenta con una lista más clara o una foto más cercana.');
    rows = out.map((m) => ({
      key: nextKey++,
      kind: m.kind,
      name: m.name,
      category: m.category,
      price: toPesos(m.price_cents),
      cost: toPesos(m.cost_cents),
      stock: m.stock ? String(m.stock) : '',
      unit: m.unit,
      barcode: m.barcode
    }));
  }

  function rowProblem(r: Row): string {
    if (!r.name.trim()) return 'Falta el nombre';
    for (const v of [r.price, r.cost, r.stock]) if (v.trim() !== '' && (Number.isNaN(parsePesos(v)) || parsePesos(v) < 0)) return 'Número no válido';
    return '';
  }
  const problems = $derived(rows ? rows.filter((r) => rowProblem(r)).length : 0);

  function toItem(r: Row): CatalogInput {
    const stock = r.stock.trim() === '' ? 0 : parsePesos(r.stock);
    const track = r.kind === 'product' && stock > 0;
    return {
      ...blankInput(r.kind),
      name: r.name.trim(),
      category: r.category.trim(),
      barcode: r.barcode.trim(),
      unit: r.unit.trim() || (r.kind === 'product' ? 'pza' : 'sesión'),
      price_cents: toCents(r.price.trim() === '' ? 0 : parsePesos(r.price)),
      cost_cents: toCents(r.cost.trim() === '' ? 0 : parsePesos(r.cost)),
      tax_rate: 0,
      track_stock: track,
      stock: track ? stock : undefined
    };
  }

  async function confirm() {
    if (!rows?.length || problems) return;
    let res: { created: number; skipped: string[] } | undefined;
    const ok = await saveOp.run(async () => {
      res = await api.pos.importItems(rows!.map(toItem));
    });
    if (!ok || !res) return;
    const skipped = res.skipped?.length ? ` (${res.skipped.length} omitidos por estar repetidos)` : '';
    toast.show(`${res.created} ${res.created === 1 ? 'artículo agregado' : 'artículos agregados'}${skipped}`);
    ondone();
    onclose();
  }

  const cell = 'field !min-h-9 !px-2 !text-sm';
</script>

<Modal {open} title="Inventario Mágico" {onclose} wide>
  <div class="space-y-4">
    {#if usage}
      <p class="flex items-center gap-2 text-xs text-app-muted" aria-live="polite">
        <Icon name="sparkles" size={14} />Usos de magia: {usage.used} de {usage.limit} este mes
      </p>
    {/if}

    {#if rows === null}
      <p class="text-sm text-app-muted">Pega la lista de lo que vendes o sube una foto de tu lista, factura o estantería. Nosotros armamos el catálogo y tú lo revisas antes de guardar.</p>

      <div role="tablist" aria-label="Origen" class="inline-flex rounded-full bg-app-ink/6 p-1">
        {#each [['text', 'Pegar lista'], ['photo', 'Subir foto']] as [m, label]}
          <button type="button" role="tab" aria-selected={mode === m} class="rounded-full px-4 py-1.5 text-sm font-medium transition {mode === m ? 'bg-app-panel shadow-sm' : 'text-app-muted'}" onclick={() => (mode = m as 'text' | 'photo')}>{label}</button>
        {/each}
      </div>

      {#if mode === 'text'}
        <div>
          <label class="label" for="{uid}-t">Tu lista</label>
          <textarea id="{uid}-t" class="field" rows="8" bind:value={text} placeholder={'Consulta general 500\nLimpieza dental 800\nParacetamol 500mg caja c/20, costo 25, vendo 45, tengo 30\n…'}></textarea>
        </div>
      {:else}
        <div>
          <span class="label">Foto (JPG, PNG o WebP, máx. 5 MB)</span>
          <label class="flex cursor-pointer flex-col items-center gap-2 rounded-2xl border-2 border-dashed border-app-ink/20 p-6 text-center transition hover:border-app-primary focus-within:ring-4 focus-within:ring-app-primary/15">
            {#if photo}
              <img src={photo.preview} alt="Vista previa de la foto" class="max-h-52 rounded-xl" />
              <span class="text-xs text-app-muted">{photo.name} · toca para cambiarla</span>
            {:else}
              <Icon name="plus" size={26} class="text-app-primary" />
              <span class="text-sm font-medium">Elegir o tomar una foto</span>
            {/if}
            <input type="file" accept="image/jpeg,image/png,image/webp" capture="environment" class="sr-only" onchange={onphoto} />
          </label>
          {#if photoError}<p class="mt-1.5 text-xs text-app-danger" role="alert">{photoError}</p>{/if}
        </div>
      {/if}

      {#if blocked}
        <Alert>{usage && !usage.available ? 'La magia no está disponible en tu cuenta (no está configurada en el servidor o tu plan no la incluye).' : 'Ya usaste toda la magia de este mes. Se renueva el mes siguiente.'}</Alert>
      {/if}
      <div aria-live="polite">
        {#if askOp.phase === 'loading'}
          <p class="flex items-center gap-2 text-sm text-app-muted"><span class="spin"></span>Leyendo tu lista y armando el catálogo… puede tardar unos segundos.</p>
        {:else if askOp.phase === 'error'}
          <Alert>{askOp.message}</Alert>
        {/if}
      </div>
    {:else}
      <p class="text-sm text-app-muted">Revisa y corrige lo que propusimos. Los productos con existencia mayor a 0 quedan con control de inventario; el IVA queda en 0 % (puedes cambiarlo después).</p>
      <div class="overflow-x-auto rounded-2xl border border-app-ink/10">
        <table class="w-full min-w-[56rem]">
          <thead class="border-b border-app-ink/10 bg-app-elevated">
            <tr>
              <th class="th">Tipo</th><th class="th">Nombre</th><th class="th">Categoría</th><th class="th">Precio</th><th class="th">Costo</th><th class="th">Existencia</th><th class="th"><span class="sr-only">Quitar</span></th>
            </tr>
          </thead>
          <tbody class="divide-y divide-app-ink/8">
            {#each rows as r, i (r.key)}
              {@const p = rowProblem(r)}
              <tr>
                <td class="td !px-2">
                  <button type="button" class="pill {r.kind === 'product' ? 'pill-info' : 'pill-ok'}" aria-label="Cambiar tipo, ahora es {r.kind === 'product' ? 'producto' : 'servicio'}" onclick={() => (r.kind = r.kind === 'product' ? 'service' : 'product')}>{r.kind === 'product' ? 'Producto' : 'Servicio'}</button>
                </td>
                <td class="td !px-2"><input class={cell} aria-label="Nombre" aria-invalid={!!p} bind:value={r.name} /></td>
                <td class="td !px-2"><input class={cell} aria-label="Categoría" bind:value={r.category} /></td>
                <td class="td !px-2"><input class="{cell} w-24" aria-label="Precio" inputmode="decimal" bind:value={r.price} /></td>
                <td class="td !px-2"><input class="{cell} w-24" aria-label="Costo" inputmode="decimal" bind:value={r.cost} /></td>
                <td class="td !px-2"><input class="{cell} w-20" aria-label="Existencia" inputmode="decimal" disabled={r.kind === 'service'} bind:value={r.stock} /></td>
                <td class="td !px-2">
                  <button type="button" class="icon-btn danger" aria-label="Quitar {r.name || 'fila'}" onclick={() => (rows = rows!.filter((_, j) => j !== i))}><Icon name="trash" size={16} /></button>
                </td>
              </tr>
              {#if p}<tr><td colspan="7" class="px-4 pb-2 text-xs text-app-danger"><span role="alert">{p}</span></td></tr>{/if}
            {/each}
          </tbody>
        </table>
      </div>
      <p class="text-xs text-app-muted">{rows.length} {rows.length === 1 ? 'artículo' : 'artículos'} por agregar.</p>
      <OpError op={saveOp} />
    {/if}
  </div>
  {#snippet footer()}
    {#if rows === null}
      <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
      <button type="button" class="btn-primary" disabled={!canAsk || askOp.phase === 'loading'} onclick={ask}><Icon name="sparkles" size={16} />Armar mi catálogo</button>
    {:else}
      <button type="button" class="btn-secondary" onclick={() => (rows = null)}>Volver</button>
      <button type="button" class="btn-primary" disabled={!rows.length || !!problems || saveOp.phase === 'loading'} onclick={confirm}>
        {#if saveOp.phase === 'loading'}<span class="spin"></span>{/if}Agregar al catálogo
      </button>
    {/if}
  {/snippet}
</Modal>
