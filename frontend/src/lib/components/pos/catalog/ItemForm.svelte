<script lang="ts">
  import { untrack } from 'svelte';
  import { api } from '$lib/api';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { CatalogInput, CatalogItem, ItemKind } from '$lib/types';
  import Modal from '$lib/components/Modal.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import ConsumablesEditor from './ConsumablesEditor.svelte';
  import { UNITS, marginPct, parsePesos, pct, toCents, toPesos } from './helpers';

  interface Props {
    open: boolean;
    /** Item being edited; null to create. */
    item: CatalogItem | null;
    categories: string[];
    /** Starting kind when creating. */
    initialKind?: ItemKind;
    onsaved: (item: CatalogItem) => void;
    onclose: () => void;
  }
  let { open, item, categories, initialKind = 'service', onsaved, onclose }: Props = $props();

  const uid = $props.id();
  const op = new Op();

  let kind = $state<ItemKind>('service');
  let name = $state('');
  let sku = $state('');
  let barcode = $state('');
  let category = $state('');
  let priceStr = $state('');
  let costStr = $state('');
  let tax = $state(0);
  let unit = $state('pza');
  let track = $state(false);
  let stockStr = $state('');
  let minStr = $state('');
  let lotCode = $state('');
  let expires = $state('');
  let satProduct = $state('');
  let satUnit = $state('');
  let durStr = $state('');
  let errors = $state<Record<string, string>>({});

  // Reset the fields every time the modal opens (not on every keystroke).
  $effect(() => {
    if (!open) return;
    untrack(() => {
      const i = item;
      kind = i?.kind ?? initialKind;
      name = i?.name ?? '';
      sku = i?.sku ?? '';
      barcode = i?.barcode ?? '';
      category = i?.category ?? '';
      priceStr = i ? toPesos(i.price_cents) : '';
      costStr = i ? toPesos(i.cost_cents) : '';
      tax = i?.tax_rate ?? 0;
      unit = i?.unit ?? (initialKind === 'service' ? 'sesión' : 'pza');
      track = i?.track_stock ?? false;
      stockStr = '';
      minStr = i && i.min_stock ? String(i.min_stock) : '';
      lotCode = '';
      expires = '';
      satProduct = i?.sat_product_code ?? '';
      satUnit = i?.sat_unit_code ?? '';
      durStr = i?.duration_minutes ? String(i.duration_minutes) : '';
      errors = {};
      op.reset();
    });
  });

  const price = $derived(parsePesos(priceStr));
  const cost = $derived(costStr.trim() === '' ? 0 : parsePesos(costStr));
  const margin = $derived(Number.isNaN(price) || Number.isNaN(cost) ? null : marginPct(toCents(price), toCents(cost)));
  const isProduct = $derived(kind === 'product');
  const taxChips = [0, 8, 16];
  const taxIsCustom = $derived(!taxChips.includes(tax));

  function setKind(k: ItemKind) {
    if (k === kind) return;
    kind = k;
    if (k === 'service') track = false;
    if (unit === 'sesión' && k === 'product') unit = 'pza';
    if (unit === 'pza' && k === 'service') unit = 'sesión';
  }

  function validate(): CatalogInput | null {
    const e: Record<string, string> = {};
    if (tax == null || Number.isNaN(Number(tax))) tax = 0;
    if (!name.trim()) e.name = 'Escribe el nombre.';
    if (priceStr.trim() === '' || Number.isNaN(price) || price < 0) e.price = 'Escribe un precio válido (puede ser 0).';
    if (Number.isNaN(cost) || cost < 0) e.cost = 'El costo debe ser un monto válido.';
    if (!(tax >= 0 && tax <= 100)) e.tax = 'El IVA debe estar entre 0 y 100.';
    const stock = stockStr.trim() === '' ? 0 : parsePesos(stockStr);
    const min = minStr.trim() === '' ? 0 : parsePesos(minStr);
    if (isProduct && track) {
      if (Number.isNaN(stock) || stock < 0) e.stock = 'La existencia no puede ser negativa.';
      if (Number.isNaN(min) || min < 0) e.min = 'El mínimo no puede ser negativo.';
    }
    if (satProduct.trim() && !/^\d{8}$/.test(satProduct.trim())) e.satProduct = 'La clave del SAT tiene 8 dígitos.';
    if (satUnit.trim() && !/^[A-Za-z0-9]{2,3}$/.test(satUnit.trim())) e.satUnit = 'La unidad del SAT tiene 2 o 3 caracteres (E48, H87…).';
    const dur = durStr.trim() === '' ? null : Number(durStr);
    if (!isProduct && dur !== null && !(Number.isInteger(dur) && dur >= 5 && dur <= 480)) e.dur = 'La duración debe estar entre 5 y 480 minutos.';
    if (expires && expires < new Date().toISOString().slice(0, 10)) e.expires = 'Ese lote ya caducó.';
    errors = e;
    if (Object.keys(e).length) return null;
    const input: CatalogInput = {
      kind,
      name: name.trim(),
      sku: sku.trim(),
      barcode: barcode.trim(),
      category: category.trim(),
      price_cents: toCents(price),
      cost_cents: toCents(cost),
      tax_rate: tax,
      track_stock: isProduct && track,
      min_stock: isProduct && track ? min : 0,
      unit: unit.trim() || (isProduct ? 'pza' : 'sesión'),
      sat_product_code: satProduct.trim(),
      sat_unit_code: satUnit.trim().toUpperCase(),
      duration_minutes: isProduct ? null : dur
    };
    if (!item) {
      if (input.track_stock) {
        input.stock = stock;
        if (stock > 0 && lotCode.trim()) input.lot_code = lotCode.trim();
        if (stock > 0 && expires) input.expires_on = expires;
      }
    } else input.active = item.active;
    return input;
  }

  async function submit(ev: SubmitEvent) {
    ev.preventDefault();
    const input = validate();
    if (!input) return;
    let saved: CatalogItem | undefined;
    const editing = item;
    const ok = await op.run(async () => {
      saved = editing ? await api.pos.updateItem(editing.id, input) : await api.pos.createItem(input);
    });
    if (!ok || !saved) return;
    toast.show(editing ? 'Cambios guardados' : isProduct ? 'Producto agregado' : 'Servicio agregado');
    onsaved(saved);
  }

  /** Scanners type the code and press Enter: that must not submit the form. */
  function noSubmitOnEnter(e: KeyboardEvent) {
    if (e.key === 'Enter') e.preventDefault();
  }
</script>

<Modal {open} title={item ? 'Editar artículo' : 'Nuevo artículo'} {onclose}>
  <form id="{uid}-form" class="space-y-5" onsubmit={submit} novalidate>
    <div role="radiogroup" aria-label="Tipo de artículo" class="grid grid-cols-2 gap-2">
      {#each [['service', 'Servicio', 'Consulta, limpieza, sesión…'], ['product', 'Producto', 'Se vende por pieza y puede tener existencias']] as [k, label, hint]}
        <button
          type="button"
          role="radio"
          aria-checked={kind === k}
          class="rounded-xl border p-3 text-left transition focus-visible:outline-none {kind === k ? 'border-app-primary bg-app-primary/8 ring-2 ring-app-primary/20' : 'border-app-ink/15 hover:border-app-ink/30'}"
          onclick={() => setKind(k as ItemKind)}
        >
          <span class="block text-sm font-medium">{label}</span>
          <span class="mt-0.5 block text-xs text-app-muted">{hint}</span>
        </button>
      {/each}
    </div>

    <div>
      <label class="label" for="{uid}-name">Nombre</label>
      <input id="{uid}-name" class="field" bind:value={name} maxlength="120" autocomplete="off" aria-invalid={!!errors.name} aria-describedby={errors.name ? `${uid}-name-e` : undefined} placeholder={isProduct ? 'Ej. Gasas estériles 10×10' : 'Ej. Consulta general'} />
      {#if errors.name}<p id="{uid}-name-e" class="mt-1 text-xs text-app-danger" role="alert">{errors.name}</p>{/if}
    </div>

    <div class="grid gap-4 sm:grid-cols-2">
      <div>
        <label class="label" for="{uid}-sku">Código interno (SKU)</label>
        <input id="{uid}-sku" class="field" bind:value={sku} maxlength="40" autocomplete="off" placeholder="Opcional" />
      </div>
      <div>
        <label class="label" for="{uid}-bar">Código de barras</label>
        <input id="{uid}-bar" class="field font-mono" bind:value={barcode} maxlength="40" autocomplete="off" inputmode="numeric" placeholder="Escanéalo aquí" onkeydown={noSubmitOnEnter} />
      </div>
    </div>

    <div>
      <label class="label" for="{uid}-cat">Categoría</label>
      <input id="{uid}-cat" class="field" bind:value={category} list="{uid}-cats" maxlength="60" autocomplete="off" placeholder="Ej. Consultas, Material, Medicamentos" />
      <datalist id="{uid}-cats">{#each categories as c}<option value={c}></option>{/each}</datalist>
    </div>

    <div class="grid gap-4 sm:grid-cols-2">
      <div>
        <label class="label" for="{uid}-price">Precio de venta (MXN)</label>
        <input id="{uid}-price" class="field" bind:value={priceStr} inputmode="decimal" autocomplete="off" placeholder="0.00" aria-invalid={!!errors.price} aria-describedby="{uid}-price-h" />
        <p id="{uid}-price-h" class="hint" class:text-app-danger={!!errors.price} role={errors.price ? 'alert' : undefined}>{errors.price ?? 'Precio final al paciente, con IVA incluido.'}</p>
      </div>
      <div>
        <label class="label" for="{uid}-cost">Costo (opcional)</label>
        <input id="{uid}-cost" class="field" bind:value={costStr} inputmode="decimal" autocomplete="off" placeholder="0.00" aria-invalid={!!errors.cost} aria-describedby="{uid}-cost-h" />
        <p id="{uid}-cost-h" class="hint" class:text-app-danger={!!errors.cost} aria-live="polite">
          {#if errors.cost}{errors.cost}{:else if margin !== null}Margen: <strong class={margin < 0 ? 'text-app-danger' : 'text-app-accent'}>{pct(margin)}</strong> sobre el precio{:else}Con el costo calculamos tu margen.{/if}
        </p>
      </div>
    </div>

    <div class="grid gap-4 sm:grid-cols-2">
      <div>
        <span class="label" id="{uid}-tax-l">IVA</span>
        <div class="flex flex-wrap items-center gap-2" role="group" aria-labelledby="{uid}-tax-l">
          {#each taxChips as t}
            <button type="button" class="rounded-full px-3.5 py-2 text-sm font-medium ring-1 ring-inset transition {tax === t ? 'bg-app-ink text-app-surface ring-app-ink' : 'ring-app-ink/15 hover:bg-app-ink/5'}" aria-pressed={tax === t} onclick={() => (tax = t)}>{t} %</button>
          {/each}
          <label class="flex items-center gap-1.5 text-sm text-app-muted">
            <span class="sr-only">IVA personalizado</span>
            <input class="field !min-h-9 !w-20 !px-2.5 text-center {taxIsCustom ? 'border-app-primary' : ''}" type="number" min="0" max="100" step="0.5" bind:value={tax} aria-label="IVA personalizado en porcentaje" /> %
          </label>
        </div>
        {#if errors.tax}<p class="mt-1 text-xs text-app-danger" role="alert">{errors.tax}</p>{/if}
      </div>
      <div>
        <label class="label" for="{uid}-unit">Unidad</label>
        <input id="{uid}-unit" class="field" bind:value={unit} list="{uid}-units" maxlength="20" autocomplete="off" />
        <datalist id="{uid}-units">{#each UNITS as u}<option value={u}></option>{/each}</datalist>
      </div>
    </div>

    {#if !isProduct}
      <div>
        <label class="label" for="{uid}-dur">Duración en la agenda <span class="font-normal text-app-muted">(minutos, opcional)</span></label>
        <input id="{uid}-dur" class="field sm:max-w-40" bind:value={durStr} inputmode="numeric" maxlength="3" autocomplete="off" placeholder="Ej. 45" aria-invalid={!!errors.dur} aria-describedby="{uid}-dur-h" />
        <p id="{uid}-dur-h" class="hint" class:text-app-danger={!!errors.dur}>{errors.dur ?? 'Si la dejas vacía se usa el intervalo del profesional. Las citas y la reserva en línea reservan este tiempo.'}</p>
      </div>
    {/if}

    {#if isProduct}
      <div class="rounded-2xl border border-app-ink/10 bg-app-elevated p-4">
        <label class="flex cursor-pointer items-start gap-3">
          <input type="checkbox" role="switch" class="mt-1 h-5 w-5 accent-[rgb(var(--app-primary))]" bind:checked={track} />
          <span>
            <span class="block text-sm font-medium">Controlar existencias</span>
            <span class="block text-xs text-app-muted">Descuenta del inventario en cada venta y avisa cuando quede poco.</span>
          </span>
        </label>
        {#if track}
          <div class="mt-4 grid gap-4 sm:grid-cols-2">
            {#if !item}
              <div>
                <label class="label" for="{uid}-stock">Existencia inicial</label>
                <input id="{uid}-stock" class="field" bind:value={stockStr} inputmode="decimal" autocomplete="off" placeholder="0" aria-invalid={!!errors.stock} />
                {#if errors.stock}<p class="mt-1 text-xs text-app-danger" role="alert">{errors.stock}</p>{/if}
              </div>
              <div>
                <label class="label" for="{uid}-lot">Lote (opcional)</label>
                <input id="{uid}-lot" class="field" bind:value={lotCode} maxlength="40" autocomplete="off" placeholder="Ej. L2405" />
              </div>
              <div>
                <label class="label" for="{uid}-exp">Caducidad (opcional)</label>
                <input id="{uid}-exp" type="date" class="field" bind:value={expires} aria-invalid={!!errors.expires} />
                {#if errors.expires}<p class="mt-1 text-xs text-app-danger" role="alert">{errors.expires}</p>{/if}
              </div>
            {/if}
            <div>
              <label class="label" for="{uid}-min">Existencia mínima</label>
              <input id="{uid}-min" class="field" bind:value={minStr} inputmode="decimal" autocomplete="off" placeholder="0" aria-invalid={!!errors.min} aria-describedby="{uid}-min-h" />
              <p id="{uid}-min-h" class="hint" class:text-app-danger={!!errors.min}>{errors.min ?? 'Te avisamos cuando llegue a este número.'}</p>
            </div>
          </div>
          {#if item}<p class="hint mt-3">Para cambiar las existencias usa “Entrada”, “Salida” o “Contar” en Inventario, así queda historial.</p>{/if}
        {/if}
      </div>
    {/if}

    <details class="rounded-2xl border border-app-ink/10 p-4">
      <summary class="cursor-pointer text-sm font-medium">Datos fiscales (SAT) para facturar</summary>
      <div class="mt-3 grid gap-4 sm:grid-cols-2">
        <div>
          <label class="label" for="{uid}-satp">Clave de producto o servicio</label>
          <input id="{uid}-satp" class="field font-mono" bind:value={satProduct} inputmode="numeric" maxlength="8" autocomplete="off" placeholder={isProduct ? '01010101' : '85121800'} aria-invalid={!!errors.satProduct} />
          <p class="hint" class:text-app-danger={!!errors.satProduct}>{errors.satProduct ?? (isProduct ? 'Si la dejas vacía se usa 01010101 (no existe en el catálogo).' : 'Si la dejas vacía se usa 85121800 (servicios médicos).')}</p>
        </div>
        <div>
          <label class="label" for="{uid}-satu">Clave de unidad</label>
          <input id="{uid}-satu" class="field font-mono uppercase" bind:value={satUnit} maxlength="3" autocomplete="off" placeholder={isProduct ? 'H87' : 'E48'} aria-invalid={!!errors.satUnit} />
          <p class="hint" class:text-app-danger={!!errors.satUnit}>{errors.satUnit ?? (isProduct ? 'H87 = pieza.' : 'E48 = unidad de servicio.')}</p>
        </div>
      </div>
    </details>

    {#if kind === 'service'}
      <ConsumablesEditor serviceId={item?.id ?? null} {open} />
    {/if}

    <div aria-live="polite">
      {#if op.phase === 'error'}<p class="alert" role="alert"><Icon name="alert" size={18} />{op.message}</p>{/if}
    </div>
  </form>
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
    <button type="submit" form="{uid}-form" class="btn-primary" disabled={op.phase === 'loading'}>
      {#if op.phase === 'loading'}<span class="spin"></span>{/if}{item ? 'Guardar cambios' : 'Agregar'}
    </button>
  {/snippet}
</Modal>
