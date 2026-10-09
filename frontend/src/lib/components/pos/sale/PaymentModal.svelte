<script lang="ts">
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onDestroy, untrack } from 'svelte';
  import Modal from '$lib/components/Modal.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { api, ApiError } from '$lib/api';
  import { moneyCents } from '$lib/format';
  import { toast } from '$lib/toast.svelte';
  import { PAY_METHODS, type PayMethod, type PointState, type PosSettings, type SalePaymentInput } from '$lib/types';
  import MoneyInput from './MoneyInput.svelte';
  import QrCode from './QrCode.svelte';

  interface Props {
    open: boolean;
    total: number;
    settings: PosSettings;
    busy: boolean;
    /** Error from the last attempt to register the sale. */
    error: string;
    onclose: () => void;
    onconfirm: (payments: SalePaymentInput[], onAccount: boolean) => void;
    /** Offer "cobrar a abonos": what the payments do not cover stays as a balance. */
    allowCredit?: boolean;
    /** Why credit is not available right now (shown instead of the switch). */
    creditBlocked?: string;
    /** Registering an abono on an open sale: paying less than the total is the point. */
    partial?: boolean;
    title?: string;
    totalLabel?: string;
  }
  let { open, total, settings, busy, error, onclose, onconfirm, allowCredit = false, creditBlocked = '', partial = false, title = 'Cobrar', totalLabel = 'Total a cobrar' }: Props = $props();

  let onAccount = $state(false);
  const flexible = $derived(partial || onAccount);

  interface PayLine extends SalePaymentInput {
    id: number;
  }
  interface LiveCharge {
    id: string;
    kind: 'point' | 'link';
    amount: number;
    status: 'open' | 'approved' | 'canceled' | 'error';
    url?: string;
  }

  let seq = 0;
  let payments = $state<PayLine[]>([]);
  // 'mixed' is not a payment method of its own: it adds a card payment (own terminal) and a cash payment in one step
  let method = $state<PayMethod | 'mixed'>('cash');
  let cardPart = $state<number | null>(null);
  let amount = $state<number | null>(null);
  let received = $state<number | null>(null);
  let reference = $state('');
  let formError = $state('');

  // Mercado Pago
  let pointState = $state<PointState | null>(null);
  let pointLoaded = $state(false);
  let charge = $state<LiveCharge | null>(null);
  let starting = $state(false);
  let mpError = $state('');
  let mpMissing = $state(false);
  let copied = $state(false);

  const paid = $derived(payments.reduce((a, p) => a + p.amount_cents, 0));
  const remaining = $derived(total - paid);
  const waiting = $derived(charge?.status === 'open');
  const methods = $derived(settings.methods.length ? settings.methods : (['cash'] as PayMethod[]));
  /** mixed payment needs cash and the clinic's own card terminal */
  const canMix = $derived(methods.includes('cash') && methods.includes('card'));
  const cashPart = $derived(cardPart != null && cardPart > 0 && cardPart < remaining ? remaining - cardPart : null);
  const mixedChange = $derived(cashPart != null && received != null ? Math.max(0, received - cashPart) : 0);
  const change = $derived(method === 'cash' && received != null && amount != null ? Math.max(0, received - amount) : 0);
  const totalChange = $derived(payments.reduce((a, p) => a + Math.max(0, (p.received_cents ?? p.amount_cents) - p.amount_cents), 0));

  /** Quick "received" amounts: the exact one and the next round bills above it. */
  const quick = $derived.by(() => {
    const base = amount ?? 0;
    if (base <= 0) return [] as number[];
    const out = new Set<number>();
    for (const d of [5000, 10000, 20000, 50000, 100000]) {
      const v = Math.ceil(base / d) * d;
      if (v > base) out.add(v);
    }
    return [...out].sort((a, b) => a - b).slice(0, 4);
  });

  function resetForm() {
    amount = remaining > 0 ? remaining : null;
    cardPart = null;
    received = null;
    reference = '';
    formError = '';
    mpError = '';
    mpMissing = false;
  }

  // Fresh form whenever the dialog opens, the method changes or a payment is added/removed
  $effect(() => {
    open;
    method;
    payments.length;
    untrack(resetForm);
  });
  $effect(() => {
    if (method !== 'mixed' && !methods.includes(method)) method = methods[0];
    if (method === 'mixed' && !canMix) method = methods[0];
  });
  $effect(() => {
    if (open && method === 'mp_point' && !pointLoaded) untrack(loadPoint);
  });

  /** Is the account connected and the terminal ready? Same check MiTiendita runs before charging. */
  async function loadPoint() {
    pointLoaded = true;
    try {
      pointState = await api.pos.pointStatus();
      mpMissing = !pointState.ok && ['not_connected', 'no_terminal', 'terminal_missing', 'token_revoked'].includes(pointState.code);
    } catch (e) {
      pointLoaded = false;
      handleMpError(e);
    }
  }

  const NEEDS_SETTINGS = ['MP_NOT_CONNECTED', 'not_connected', 'no_terminal', 'terminal_missing', 'token_revoked'];
  function handleMpError(e: unknown) {
    if (e instanceof ApiError && NEEDS_SETTINGS.includes(e.code)) {
      mpMissing = true;
      mpError = e.message;
    } else mpError = e instanceof Error ? e.message : 'No se pudo conectar con Mercado Pago.';
  }

  function addLine(line: SalePaymentInput) {
    payments.push({ ...line, id: ++seq });
  }

  function addManual() {
    formError = '';
    if (method === 'mixed') return;
    if (!amount || amount <= 0) return void (formError = 'Escribe el monto del pago.');
    if (amount > remaining) return void (formError = `El monto supera lo que falta (${moneyCents(remaining)}).`);
    if (method === 'cash') {
      if (received != null && received < amount) return void (formError = 'El efectivo recibido es menor al monto.');
      addLine({ method, amount_cents: amount, received_cents: received ?? amount });
    } else {
      addLine({ method, amount_cents: amount, reference: reference.trim() || undefined });
    }
    // A single payment that covers the whole account closes the sale right away: one tap instead of two.
    if (remaining === 0 && !busy) onconfirm(payments.map(({ id: _id, ...p }) => p), onAccount);
  }

  /** Mixed payment: part on the own card terminal (with the voucher reference), the rest in cash with change. */
  function addMixed() {
    formError = '';
    if (cardPart == null || cardPart <= 0) return void (formError = 'Escribe cuánto se paga con tarjeta.');
    if (cardPart >= remaining) return void (formError = `La parte con tarjeta debe ser menor a lo que falta (${moneyCents(remaining)}); si todo es con tarjeta, usa Tarjeta.`);
    const cash = remaining - cardPart;
    if (received != null && received < cash) return void (formError = 'El efectivo recibido es menor a lo que falta en efectivo.');
    addLine({ method: 'card', amount_cents: cardPart, reference: reference.trim() || undefined });
    addLine({ method: 'cash', amount_cents: cash, received_cents: received ?? cash });
    if (remaining === 0 && !busy) onconfirm(payments.map(({ id: _id, ...p }) => p), onAccount);
  }

  // ---- Mercado Pago charges ----
  let timer: ReturnType<typeof setInterval> | undefined;
  let polling = false;

  function stopPoll() {
    clearInterval(timer);
    timer = undefined;
  }
  function startPoll() {
    stopPoll();
    timer = setInterval(tick, 2000);
  }
  async function tick() {
    const current = charge;
    if (!current || current.status !== 'open' || polling) return;
    polling = true;
    try {
      const c = await api.pos.charge(current.id);
      if (!charge || charge.id !== c.id) return;
      if (c.status === 'approved') {
        stopPoll();
        addLine({ method: current.kind === 'point' ? 'mp_point' : 'mp_link', amount_cents: c.amount_cents || current.amount, intent_id: c.id });
        charge = null;
        toast.show('Pago aprobado');
      } else if (c.status !== 'open') {
        stopPoll();
        charge.status = c.status;
      }
    } catch {
      /* transient network error: keep polling */
    } finally {
      polling = false;
    }
  }
  onDestroy(stopPoll);

  async function startCharge() {
    formError = '';
    mpError = '';
    if (!amount || amount <= 0) return void (formError = 'Escribe el monto a cobrar.');
    if (amount > remaining) return void (formError = `El monto supera lo que falta (${moneyCents(remaining)}).`);
    starting = true;
    try {
      if (method === 'mp_point') {
        const r = await api.pos.pointCharge(amount);
        charge = { id: r.id, kind: 'point', amount, status: 'open' };
      } else {
        const r = await api.pos.payLink(amount, `Cobro ${settings.business_name || 'Caresia'}`);
        charge = { id: r.id, kind: 'link', amount, status: 'open', url: r.url };
      }
      startPoll();
    } catch (e) {
      handleMpError(e);
    } finally {
      starting = false;
    }
  }

  async function cancelCharge() {
    const c = charge;
    stopPoll();
    charge = null;
    if (!c) return;
    try {
      await api.pos.cancelCharge(c.id);
    } catch {
      /* already closed on the provider side */
    }
  }

  async function copyUrl() {
    if (!charge?.url) return;
    try {
      await navigator.clipboard.writeText(charge.url);
      copied = true;
      setTimeout(() => (copied = false), 2000);
    } catch {
      toast.show('No se pudo copiar. Selecciona la liga y cópiala manualmente.', 'error');
    }
  }

  const label = (m: PayMethod) => PAY_METHODS[m]?.label ?? m;
  const uid = $props.id();
</script>

<Modal {open} {title} {onclose} wide>
  <div class="grid gap-6 md:grid-cols-[1fr_1.1fr]">
    <!-- left: total + payments so far -->
    <div class="space-y-4">
      <div class="rounded-2xl bg-app-elevated p-4" aria-live="polite">
        <p class="section-title">{totalLabel}</p>
        <p class="display text-5xl tabular-nums">{moneyCents(total)}</p>
        <p class="mt-1 text-sm {remaining === 0 ? 'font-medium text-app-accent' : remaining < 0 ? 'font-medium text-app-danger' : 'text-app-muted'}">
          {#if remaining > 0 && flexible}{partial ? 'Quedará un saldo de' : 'Quedará a abonos'} <strong class="tabular-nums">{moneyCents(remaining)}</strong>
          {:else if remaining > 0}Falta <strong class="tabular-nums">{moneyCents(remaining)}</strong>
          {:else if remaining === 0}Pagos completos
          {:else}Los pagos exceden el total por {moneyCents(-remaining)}{/if}
        </p>
      </div>

      {#if allowCredit}
        {#if creditBlocked}
          <p class="text-xs text-app-muted">{creditBlocked}</p>
        {:else}
          <label class="flex min-h-11 cursor-pointer items-start gap-3 rounded-xl p-3 ring-1 ring-inset ring-app-ink/15 has-[:checked]:bg-app-primary/10 has-[:checked]:ring-app-primary">
            <input type="checkbox" class="mt-1 h-4 w-4" bind:checked={onAccount} disabled={waiting} />
            <span class="text-sm"><span class="block font-medium">Cobrar a abonos</span><span class="block text-xs text-app-muted">Recibe un anticipo (puede ser $0) y deja el resto como saldo por cobrar.</span></span>
          </label>
        {/if}
      {/if}

      {#if payments.length}
        <ul class="divide-y divide-app-ink/10 rounded-2xl ring-1 ring-inset ring-app-ink/10" aria-label="Pagos registrados">
          {#each payments as p (p.id)}
            <li class="flex items-center justify-between gap-3 px-3.5 py-2.5">
              <div class="min-w-0 text-sm">
                <p class="font-medium">{label(p.method)}</p>
                <p class="truncate text-xs text-app-muted">
                  {#if p.method === 'cash' && (p.received_cents ?? 0) > p.amount_cents}Recibido {moneyCents(p.received_cents!)} · cambio {moneyCents(p.received_cents! - p.amount_cents)}
                  {:else if p.intent_id}Aprobado en Mercado Pago
                  {:else if p.reference}Ref. {p.reference}{/if}
                </p>
              </div>
              <div class="flex items-center gap-1">
                <span class="font-medium tabular-nums">{moneyCents(p.amount_cents)}</span>
                {#if !p.intent_id}
                  <button type="button" class="icon-btn danger h-11 w-11" aria-label="Quitar pago de {label(p.method)}" onclick={() => (payments = payments.filter((x) => x.id !== p.id))}><Icon name="x" size={16} /></button>
                {:else}
                  <span class="grid h-11 w-11 place-items-center text-app-accent" title="Cobro ya realizado"><Icon name="check" size={16} /></span>
                {/if}
              </div>
            </li>
          {/each}
        </ul>
        {#if totalChange > 0}
          <p class="text-sm">Cambio a entregar: <strong class="tabular-nums">{moneyCents(totalChange)}</strong></p>
        {/if}
      {:else}
        <p class="text-sm text-app-muted">Agrega uno o varios pagos para dividir la cuenta (por ejemplo, parte en efectivo y parte con tarjeta).</p>
      {/if}
    </div>

    <!-- right: add a payment -->
    <div class="space-y-4">
      {#if remaining > 0}
        <div role="radiogroup" aria-label="Método de pago" class="grid grid-cols-2 gap-2">
          {#each methods as m (m)}
            <button
              type="button"
              role="radio"
              aria-checked={method === m}
              disabled={waiting}
              class="min-h-14 rounded-xl px-3 py-2 text-left text-sm transition disabled:opacity-50 {method === m ? 'bg-app-primary/10 ring-2 ring-app-primary' : 'ring-1 ring-inset ring-app-ink/15 hover:bg-app-elevated'}"
              onclick={() => (method = m)}
            >
              <span class="block font-medium leading-tight">{label(m).replace(/ \(.*\)/, '')}</span>
              <span class="block text-xs text-app-muted">{PAY_METHODS[m]?.hint}</span>
            </button>
          {/each}
          {#if canMix}
            <button
              type="button"
              role="radio"
              aria-checked={method === 'mixed'}
              disabled={waiting}
              class="min-h-14 rounded-xl px-3 py-2 text-left text-sm transition disabled:opacity-50 {method === 'mixed' ? 'bg-app-primary/10 ring-2 ring-app-primary' : 'ring-1 ring-inset ring-app-ink/15 hover:bg-app-elevated'}"
              onclick={() => (method = 'mixed')}
            >
              <span class="block font-medium leading-tight">Mixto</span>
              <span class="block text-xs text-app-muted">Parte con tarjeta y el resto en efectivo</span>
            </button>
          {/if}
        </div>

        {#if mpMissing}
          <div class="alert" role="alert">
            <Icon name="alert" size={18} />
            <span>Mercado Pago no está conectado. <a class="underline" href="/ajustes?s=terminal">Conéctalo en Ajustes</a>.</span>
          </div>
        {/if}

        {#if method === 'mp_point' || method === 'mp_link'}
          {#if charge}
            <div class="rounded-2xl border border-app-ink/10 p-4 text-center" aria-live="polite">
              {#if charge.status === 'open'}
                <span class="spin mx-auto mb-3 block h-6 w-6"></span>
                <p class="font-medium">{charge.kind === 'point' ? 'Pídele al paciente que pase su tarjeta en la terminal' : 'Esperando el pago del paciente'}</p>
                <p class="display mt-1 text-3xl tabular-nums">{moneyCents(charge.amount)}</p>
                {#if charge.kind === 'link' && charge.url}
                  <div class="mt-4 flex flex-col items-center gap-3">
                    <QrCode value={charge.url} />
                    <p class="text-xs text-app-muted">El paciente escanea el código o abre la liga desde su celular.</p>
                    <div class="flex w-full items-center gap-2">
                      <input class="field text-xs" readonly value={charge.url} aria-label="Liga de pago" onfocus={(e) => e.currentTarget.select()} />
                      <button type="button" class="btn-secondary shrink-0" onclick={copyUrl}>{copied ? 'Copiada' : 'Copiar'}</button>
                    </div>
                  </div>
                {/if}
                <button type="button" class="btn-secondary mt-4 min-h-11" onclick={cancelCharge}>Cancelar cobro</button>
              {:else}
                <p class="alert justify-center" role="alert">
                  {charge.status === 'canceled' ? 'El cobro fue cancelado.' : 'El cobro no se pudo completar.'}
                </p>
                <button type="button" class="btn-primary mt-3 min-h-11" onclick={() => (charge = null)}>Intentar de nuevo</button>
              {/if}
            </div>
          {:else}
            <MoneyInput id="{uid}-amt" label="Monto" bind:cents={amount} onenter={startCharge} />
            {#if method === 'mp_point'}
              {#if pointState?.ok}
                <p class="flex items-center gap-2 text-sm text-app-muted"><Icon name="check" size={16} class="text-app-accent" />Terminal lista{pointState.terminal_label ? `: ${pointState.terminal_label}` : ''}</p>
              {:else if pointState}
                <Alert>{pointState.message} <a class="underline" href="/ajustes?s=terminal">Ir a Ajustes</a></Alert>
              {/if}
            {/if}
            <button type="button" class="btn-primary btn-lg min-h-12" disabled={starting || mpMissing || (method === 'mp_point' && !!pointState && !pointState.ok)} onclick={startCharge}>
              {#if starting}<span class="spin"></span>{/if}{method === 'mp_point' ? 'Enviar cobro a la terminal' : 'Generar liga de pago'}
            </button>
          {/if}
          {#if mpError}<Alert>{mpError}</Alert>{/if}
        {:else if method === 'mixed'}
          <form
            class="space-y-4"
            onsubmit={(e) => {
              e.preventDefault();
              addMixed();
            }}
          >
            <MoneyInput id="{uid}-card" label="Parte con tarjeta (terminal propia)" bind:cents={cardPart} />
            <div>
              <label class="label" for="{uid}-mref">Referencia del voucher (opcional)</label>
              <input id="{uid}-mref" class="field" bind:value={reference} maxlength="80" autocomplete="off" placeholder="Últimos 4 dígitos o folio del voucher" />
            </div>
            <div class="flex items-baseline justify-between rounded-xl bg-app-elevated px-4 py-3" aria-live="polite">
              <span class="text-sm text-app-muted">Resto en efectivo</span>
              <span class="display text-3xl tabular-nums">{cashPart != null ? moneyCents(cashPart) : '—'}</span>
            </div>
            {#if cashPart != null}
              <div>
                <MoneyInput id="{uid}-mrec" label="Efectivo recibido" bind:cents={received} placeholder={String(cashPart / 100)} />
                <div class="mt-2 flex flex-wrap gap-2">
                  <button type="button" class="btn-secondary min-h-11" onclick={() => (received = cashPart)}>Exacto</button>
                  {#each [5000, 10000, 20000, 50000, 100000].map((d) => Math.ceil(cashPart / d) * d).filter((v, i, a) => v > cashPart && a.indexOf(v) === i).slice(0, 3) as v (v)}
                    <button type="button" class="btn-secondary min-h-11" onclick={() => (received = v)}>{moneyCents(v).replace(/\.00$/, '')}</button>
                  {/each}
                </div>
              </div>
              <div class="flex items-baseline justify-between rounded-xl bg-app-elevated px-4 py-3" aria-live="polite">
                <span class="text-sm text-app-muted">Cambio</span>
                <span class="display text-3xl tabular-nums {received != null && received < cashPart ? 'text-app-danger' : ''}">
                  {received != null && received < cashPart ? 'Falta ' + moneyCents(cashPart - received) : moneyCents(mixedChange)}
                </span>
              </div>
            {/if}
            <p class="hint">Cobra {cardPart ? moneyCents(cardPart) : 'la parte de tarjeta'} en tu terminal y confirma aquí cuando salga aprobado.</p>
            <button type="submit" class="btn-primary btn-lg min-h-12">Cobrar {moneyCents(remaining)} (mixto)</button>
          </form>
        {:else}
          <form
            class="space-y-4"
            onsubmit={(e) => {
              e.preventDefault();
              addManual();
            }}
          >
            <MoneyInput id="{uid}-amt" label="Monto" bind:cents={amount} />
            {#if method === 'cash'}
              <div>
                <MoneyInput id="{uid}-rec" label="Recibido" bind:cents={received} placeholder={amount ? String(amount / 100) : '0.00'} />
                <div class="mt-2 flex flex-wrap gap-2">
                  <button type="button" class="btn-secondary min-h-11" onclick={() => (received = amount)}>Exacto</button>
                  {#each quick as v (v)}
                    <button type="button" class="btn-secondary min-h-11" onclick={() => (received = v)}>{moneyCents(v).replace(/\.00$/, '')}</button>
                  {/each}
                </div>
              </div>
              <div class="flex items-baseline justify-between rounded-xl bg-app-elevated px-4 py-3" aria-live="polite">
                <span class="text-sm text-app-muted">Cambio</span>
                <span class="display text-3xl tabular-nums {received != null && amount != null && received < amount ? 'text-app-danger' : ''}">
                  {received != null && amount != null && received < amount ? 'Falta ' + moneyCents(amount - received) : moneyCents(change)}
                </span>
              </div>
            {:else}
              <div>
                <label class="label" for="{uid}-ref">Referencia (opcional)</label>
                <input id="{uid}-ref" class="field" bind:value={reference} maxlength="80" autocomplete="off" placeholder={method === 'card' ? 'Últimos 4 dígitos o folio del voucher' : method === 'transfer' ? 'Clave de rastreo' : 'Referencia'} />
              </div>
            {/if}
            <button type="submit" class="btn-primary btn-lg min-h-12">{amount && amount === remaining ? 'Cobrar ' + moneyCents(amount) : 'Agregar pago' + (amount ? ' de ' + moneyCents(amount) : '')}</button>
          </form>
        {/if}
        {#if formError}<Alert>{formError}</Alert>{/if}
      {:else if remaining === 0}
        <div class="grid place-items-center rounded-2xl bg-app-accent/10 px-4 py-10 text-center text-app-accent">
          <Icon name="check" size={28} />
          <p class="mt-2 font-medium">La cuenta está cubierta.</p>
          <p class="text-sm">Confirma el cobro para registrar la venta.</p>
        </div>
      {:else}
        <Alert>Quita un pago: la suma excede el total de la cuenta.</Alert>
      {/if}
    </div>
  </div>

  {#if error}<Alert class="mt-4">{error}</Alert>{/if}

  {#snippet footer()}
    <button type="button" class="btn-secondary min-h-11" onclick={onclose}>Volver a la cuenta</button>
    <button
      type="button"
      class="btn-primary min-h-11 px-6"
      disabled={busy || waiting || remaining < 0 || (partial ? payments.length === 0 : onAccount ? false : remaining !== 0 || payments.length === 0)}
      onclick={() => onconfirm(payments.map(({ id: _id, ...p }) => p), onAccount)}
    >
      {#if busy}<span class="spin"></span>{/if}{partial ? 'Registrar abono' : onAccount && remaining > 0 ? 'Confirmar a abonos' : 'Confirmar cobro'}
    </button>
  {/snippet}
</Modal>
