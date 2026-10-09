<script lang="ts">
  import { moneyCents } from '$lib/format';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { Professional } from '$lib/types/pos2';
  import type { Cart, CartLine, SalePerson } from './cart.svelte';
  import { lineGross } from './cart.svelte';
  import MoneyInput from './MoneyInput.svelte';
  import { qtyText } from './money';

  interface Props {
    cart: Cart;
    canEditPrice: boolean;
    /** Patients to suggest; empty when the person may not browse expedients. */
    people: SalePerson[];
    /** Who can be credited with the sale (commissions); empty hides the selector. */
    professionals?: Professional[];
    showTax: boolean;
    busy: boolean;
    onfree: () => void;
    oncheckout: () => void;
  }
  let { cart, canEditPrice, people, professionals = [], showTax, busy, onfree, oncheckout }: Props = $props();

  let editing = $state<string | null>(null);
  let sugOpen = $state(false);
  let sugIndex = $state(0);
  const uid = $props.id();

  const isPiece = (l: CartLine) => !l.unit || l.unit.toLowerCase().startsWith('pz');

  function setQty(l: CartLine, raw: number) {
    let n = isPiece(l) ? Math.round(raw) : Math.round(raw * 1000) / 1000;
    if (!Number.isFinite(n) || n <= 0) return;
    const max = cart.maxQty(l);
    if (n > max) {
      n = max;
      toast.show(max > 0 ? `Solo quedan ${qtyText(max)} de «${l.name}».` : `«${l.name}» está agotado.`, 'error');
    }
    if (n > 0) l.qty = n;
  }
  function step(l: CartLine, dir: 1 | -1) {
    if (dir < 0 && l.qty <= 1) return cart.remove(l.key);
    setQty(l, dir < 0 ? Math.max(l.qty - 1, 0.001) : l.qty + 1);
  }

  const suggestions = $derived.by(() => {
    const t = cart.customer.trim().toLowerCase();
    if (t.length < 2 || !people.length) return [];
    return people.filter((p) => p.name.toLowerCase().includes(t) && p.name.toLowerCase() !== t).slice(0, 6);
  });
  function choose(p: SalePerson) {
    cart.customer = p.name;
    cart.curp = p.curp;
    cart.patientId = p.id ?? '';
    sugOpen = false;
  }
  function sugKey(e: KeyboardEvent) {
    if (!sugOpen || !suggestions.length) return;
    if (e.key === 'ArrowDown') (e.preventDefault(), (sugIndex = (sugIndex + 1) % suggestions.length));
    else if (e.key === 'ArrowUp') (e.preventDefault(), (sugIndex = (sugIndex - 1 + suggestions.length) % suggestions.length));
    else if (e.key === 'Enter') (e.preventDefault(), choose(suggestions[sugIndex]));
    else if (e.key === 'Escape') sugOpen = false;
  }
</script>

<section id="cuenta" aria-label="Cuenta" class="card flex min-h-0 flex-col overflow-hidden">
  <div class="flex items-center justify-between gap-2 px-4 pb-2 pt-4">
    <h2 class="display text-2xl">Cuenta</h2>
    <div class="flex items-center gap-1">
      <button type="button" class="btn-ghost min-h-11" onclick={onfree}><Icon name="plus" size={16} />Concepto libre</button>
      {#if !cart.empty}
        <button type="button" class="icon-btn danger h-11 w-11" aria-label="Vaciar cuenta" title="Vaciar cuenta" onclick={() => cart.clear()}><Icon name="trash" size={18} /></button>
      {/if}
    </div>
  </div>

  <div class="min-h-0 flex-1 overflow-y-auto px-4">
    {#if cart.empty}
      <div class="grid place-items-center px-4 py-10 text-center text-sm text-app-muted">
        <span class="mb-3 grid h-12 w-12 place-items-center rounded-2xl bg-app-primary/10 text-app-primary"><Icon name="receipt" size={22} /></span>
        La cuenta está vacía.<br />Toca un concepto del catálogo o escanea un código.
      </div>
    {:else}
      <ul class="divide-y divide-app-ink/10">
        {#each cart.lines as l (l.key)}
          {@const gross = lineGross(l)}
          <li class="py-3">
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <p class="text-[15px] font-medium leading-snug">{l.name}</p>
                <p class="text-xs text-app-muted tabular-nums">
                  {moneyCents(l.price_cents)}{isPiece(l) ? '' : ` / ${l.unit}`}
                  {#if l.price_cents !== l.base_price_cents}<span class="pill pill-warn ml-1">Precio editado</span>{/if}
                </p>
              </div>
              <div class="text-right">
                <p class="font-medium tabular-nums">{moneyCents(gross - (cart.allowDiscounts ? l.discount_cents : 0))}</p>
                {#if cart.allowDiscounts && l.discount_cents > 0}
                  <p class="text-xs text-app-accent tabular-nums">−{moneyCents(l.discount_cents)}</p>
                {/if}
              </div>
            </div>
            <div class="mt-2 flex items-center gap-2">
              <div class="inline-flex items-center rounded-full ring-1 ring-inset ring-app-ink/15">
                <button type="button" class="icon-btn h-11 w-11" aria-label={l.qty <= 1 ? `Quitar ${l.name}` : `Quitar una unidad de ${l.name}`} onclick={() => step(l, -1)}>
                  {#if l.qty <= 1}<Icon name="trash" size={16} />{:else}<span aria-hidden="true" class="text-lg leading-none">−</span>{/if}
                </button>
                <input
                  class="h-11 w-14 bg-transparent text-center text-[15px] font-medium tabular-nums focus:outline-none"
                  inputmode={isPiece(l) ? 'numeric' : 'decimal'}
                  aria-label="Cantidad de {l.name}"
                  value={qtyText(l.qty)}
                  onchange={(e) => {
                    const n = Number(e.currentTarget.value.replace(',', '.'));
                    if (n > 0) setQty(l, n);
                    e.currentTarget.value = qtyText(l.qty);
                  }}
                  onfocus={(e) => e.currentTarget.select()}
                />
                <button type="button" class="icon-btn h-11 w-11" aria-label="Agregar una unidad de {l.name}" onclick={() => step(l, 1)}><Icon name="plus" size={16} /></button>
              </div>
              {#if canEditPrice || cart.allowDiscounts}
                <button
                  type="button"
                  class="btn-ghost min-h-11 px-3"
                  aria-expanded={editing === l.key}
                  aria-controls="{uid}-{l.key}"
                  onclick={() => (editing = editing === l.key ? null : l.key)}
                >
                  <Icon name="tag" size={16} />Ajustar
                </button>
              {/if}
              <button type="button" class="icon-btn danger ml-auto h-11 w-11" aria-label="Eliminar {l.name}" onclick={() => cart.remove(l.key)}><Icon name="x" size={16} /></button>
            </div>
            {#if editing === l.key}
              <div id="{uid}-{l.key}" class="mt-2 grid gap-3 rounded-xl bg-app-elevated p-3 sm:grid-cols-2">
                {#if canEditPrice}
                  <MoneyInput
                    id="{uid}-p-{l.key}"
                    label="Precio unitario"
                    bind:cents={() => l.price_cents, (v) => (l.price_cents = v ?? 0)}
                  />
                {/if}
                {#if cart.allowDiscounts}
                  <MoneyInput
                    id="{uid}-d-{l.key}"
                    label="Descuento del concepto"
                    bind:cents={() => l.discount_cents || null, (v) => (l.discount_cents = Math.min(v ?? 0, gross))}
                  />
                {/if}
              </div>
            {/if}
          </li>
        {/each}
      </ul>
    {/if}

    <div class="mt-3 space-y-3 border-t border-app-ink/10 py-4">
      <div class="relative">
        <label class="label" for="{uid}-cust">Paciente (opcional)</label>
        <input
          id="{uid}-cust"
          class="field"
          bind:value={cart.customer}
          maxlength="120"
          autocomplete="off"
          placeholder="Nombre de quien paga"
          role={people.length ? 'combobox' : undefined}
          aria-expanded={people.length ? sugOpen && suggestions.length > 0 : undefined}
          aria-controls={people.length ? `${uid}-sug` : undefined}
          aria-autocomplete={people.length ? 'list' : undefined}
          oninput={() => {
            cart.curp = '';
            cart.patientId = '';
            sugOpen = true;
            sugIndex = 0;
          }}
          onfocus={() => (sugOpen = true)}
          onblur={() => setTimeout(() => (sugOpen = false), 120)}
          onkeydown={sugKey}
        />
        {#if sugOpen && suggestions.length}
          <ul id="{uid}-sug" role="listbox" aria-label="Pacientes" class="absolute inset-x-0 top-full z-20 mt-1 overflow-hidden rounded-xl border border-app-ink/10 bg-app-panel shadow-app">
            {#each suggestions as p, n (p.id ?? p.name)}
              <li role="option" aria-selected={n === sugIndex}>
                <button type="button" class="flex min-h-11 w-full items-center px-3.5 text-left text-sm hover:bg-app-elevated {n === sugIndex ? 'bg-app-elevated' : ''}" onmousedown={(e) => (e.preventDefault(), choose(p))}>{p.name}</button>
              </li>
            {/each}
          </ul>
        {/if}
      </div>
      {#if professionals.length}
        <div>
          <label class="label" for="{uid}-pro">Profesional (opcional)</label>
          <select id="{uid}-pro" class="field" bind:value={cart.professionalId}>
            <option value="">Sin profesional</option>
            {#each professionals as pr (pr.id)}<option value={pr.id}>{pr.name}</option>{/each}
          </select>
        </div>
      {/if}
      {#if cart.origin}
        <p class="flex items-center gap-2 rounded-xl bg-app-primary/10 px-3 py-2 text-xs text-app-primary" data-testid="cart-origin">
          <Icon name="info" size={14} />{cart.origin}
        </p>
      {/if}
      <div>
        <label class="label" for="{uid}-note">Nota (opcional)</label>
        <input id="{uid}-note" class="field" bind:value={cart.note} maxlength="200" autocomplete="off" placeholder="Ej. Pago de tratamiento" />
      </div>
    </div>
  </div>

  <div class="border-t border-app-ink/10 bg-app-elevated/50 px-4 py-4">
    {#if cart.allowDiscounts && !cart.empty}
      <div class="mb-3 flex flex-wrap items-end gap-2">
        <div class="min-w-0 flex-1">
          <span class="label" id="{uid}-dl">Descuento general</span>
          {#if cart.discMode === 'amount'}
            <MoneyInput bind:cents={cart.discAmount} placeholder="0.00" />
          {:else}
            <div class="relative">
              <input
                class="field pr-8 text-right tabular-nums"
                inputmode="decimal"
                placeholder="0"
                aria-labelledby="{uid}-dl"
                value={cart.discPct ?? ''}
                oninput={(e) => {
                  const n = Number(e.currentTarget.value.replace(',', '.'));
                  cart.discPct = e.currentTarget.value.trim() === '' || !Number.isFinite(n) || n < 0 ? null : Math.min(n, 100);
                }}
              />
              <span class="pointer-events-none absolute inset-y-0 right-3.5 grid place-items-center text-app-muted" aria-hidden="true">%</span>
            </div>
          {/if}
        </div>
        <div class="inline-flex rounded-full bg-app-ink/5 p-1" role="group" aria-label="Tipo de descuento">
          {#each [['amount', '$'], ['pct', '%']] as [m, label] (m)}
            <button
              type="button"
              class="min-h-10 min-w-11 rounded-full text-sm font-medium {cart.discMode === m ? 'bg-app-panel shadow-sm' : 'text-app-muted'}"
              aria-pressed={cart.discMode === m}
              onclick={() => (cart.discMode = m as 'amount' | 'pct')}>{label}</button
            >
          {/each}
        </div>
      </div>
    {/if}

    <dl class="space-y-1 text-sm" aria-live="polite">
      <div class="flex justify-between"><dt class="text-app-muted">Subtotal</dt><dd class="tabular-nums">{moneyCents(cart.gross)}</dd></div>
      {#if cart.discountTotal > 0}
        <div class="flex justify-between"><dt class="text-app-muted">Descuento</dt><dd class="tabular-nums text-app-accent">−{moneyCents(cart.discountTotal)}</dd></div>
      {/if}
      {#if showTax && cart.tax > 0}
        <div class="flex justify-between"><dt class="text-app-muted">IVA incluido</dt><dd class="tabular-nums">{moneyCents(cart.tax)}</dd></div>
      {/if}
      <div class="flex items-baseline justify-between pt-1">
        <dt class="font-medium">Total</dt>
        <dd class="display text-4xl tabular-nums" data-testid="cart-total">{moneyCents(cart.total)}</dd>
      </div>
    </dl>
    {#if cart.problem && !cart.empty}
      <p class="mt-2 text-xs font-medium text-app-danger" role="alert">{cart.problem}</p>
    {/if}
    <button type="button" class="btn-primary btn-lg mt-3 min-h-14 text-lg" disabled={!!cart.problem || busy} onclick={oncheckout}>
      Cobrar {cart.empty ? '' : moneyCents(cart.total)}
    </button>
  </div>
</section>
