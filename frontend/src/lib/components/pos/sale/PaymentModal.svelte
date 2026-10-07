<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import Modal from '$lib/components/Modal.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { api, ApiError } from '$lib/api';
  import { moneyCents } from '$lib/format';
  import { toast } from '$lib/toast.svelte';
  import { PAY_METHODS, type PayMethod, type PointDevice, type PosSettings, type SalePaymentInput } from '$lib/types';
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
    onconfirm: (payments: SalePaymentInput[]) => void;
  }
  let { open, total, settings, busy, error, onclose, onconfirm }: Props = $props();

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
  let method = $state<PayMethod>('cash');
  let amount = $state<number | null>(null);
  let received = $state<number | null>(null);
  let reference = $state('');
  let formError = $state('');

  // Mercado Pago
  let devices = $state<PointDevice[]>([]);
  let devicesLoaded = $state(false);
  let deviceId = $state('');
  let charge = $state<LiveCharge | null>(null);
  let starting = $state(false);
  let mpError = $state('');
  let mpMissing = $state(false);
  let copied = $state(false);

  const paid = $derived(payments.reduce((a, p) => a + p.amount_cents, 0));
  const remaining = $derived(total - paid);
  const waiting = $derived(charge?.status === 'open');
  const methods = $derived(settings.methods.length ? settings.methods : (['cash'] as PayMethod[]));
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
    if (!methods.includes(method)) method = methods[0];
  });
  $effect(() => {
    if (open && method === 'mp_point' && !devicesLoaded) untrack(loadDevices);
  });

  async function loadDevices() {
    devicesLoaded = true;
    try {
      devices = await api.pos.pointDevices();
      if (!deviceId && devices.length) deviceId = devices[0].id;
    } catch (e) {
      devicesLoaded = false;
      handleMpError(e);
    }
  }

  function handleMpError(e: unknown) {
    if (e instanceof ApiError && e.code === 'MP_NOT_CONNECTED') mpMissing = true;
    else mpError = e instanceof Error ? e.message : 'No se pudo conectar con Mercado Pago.';
  }

  function addLine(line: SalePaymentInput) {
    payments.push({ ...line, id: ++seq });
  }

  function addManual() {
    formError = '';
    if (!amount || amount <= 0) return void (formError = 'Escribe el monto del pago.');
    if (amount > remaining) return void (formError = `El monto supera lo que falta (${moneyCents(remaining)}).`);
    if (method === 'cash') {
      if (received != null && received < amount) return void (formError = 'El efectivo recibido es menor al monto.');
      addLine({ method, amount_cents: amount, received_cents: received ?? amount });
    } else {
      addLine({ method, amount_cents: amount, reference: reference.trim() || undefined });
    }
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
    if (method === 'mp_point' && !deviceId) return void (formError = 'Elige la terminal.');
    starting = true;
    try {
      if (method === 'mp_point') {
        const r = await api.pos.pointCharge(deviceId, amount);
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

<Modal {open} title="Cobrar" {onclose} wide>
  <div class="grid gap-6 md:grid-cols-[1fr_1.1fr]">
    <!-- left: total + payments so far -->
    <div class="space-y-4">
      <div class="rounded-2xl bg-app-elevated p-4" aria-live="polite">
        <p class="section-title">Total a cobrar</p>
        <p class="display text-5xl tabular-nums">{moneyCents(total)}</p>
        <p class="mt-1 text-sm {remaining === 0 ? 'font-medium text-app-accent' : remaining < 0 ? 'font-medium text-app-danger' : 'text-app-muted'}">
          {#if remaining > 0}Falta <strong class="tabular-nums">{moneyCents(remaining)}</strong>
          {:else if remaining === 0}Pagos completos
          {:else}Los pagos exceden el total por {moneyCents(-remaining)}{/if}
        </p>
      </div>

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
        </div>

        {#if mpMissing}
          <div class="alert" role="alert">
            <Icon name="alert" size={18} />
            <span>Mercado Pago no está conectado. <a class="underline" href="/pos/ajustes">Conéctalo en Ajustes de cobros</a>.</span>
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
              <div>
                <label class="label" for="{uid}-dev">Terminal</label>
                {#if devicesLoaded && !devices.length && !mpMissing}
                  <p class="text-sm text-app-muted">No hay terminales vinculadas. Vincúlalas en <a class="underline" href="/pos/ajustes">Ajustes de cobros</a>.</p>
                {:else}
                  <select id="{uid}-dev" class="field" bind:value={deviceId} disabled={!devices.length}>
                    {#each devices as d (d.id)}<option value={d.id}>{d.id}{d.operating_mode ? ` · ${d.operating_mode}` : ''}</option>{/each}
                  </select>
                {/if}
              </div>
            {/if}
            <button type="button" class="btn-primary btn-lg min-h-12" disabled={starting || mpMissing || (method === 'mp_point' && !deviceId)} onclick={startCharge}>
              {#if starting}<span class="spin"></span>{/if}{method === 'mp_point' ? 'Enviar cobro a la terminal' : 'Generar liga de pago'}
            </button>
          {/if}
          {#if mpError}<p class="alert" role="alert"><Icon name="alert" size={18} />{mpError}</p>{/if}
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
            <button type="submit" class="btn-primary btn-lg min-h-12">Agregar pago{amount ? ' de ' + moneyCents(amount) : ''}</button>
          </form>
        {/if}
        {#if formError}<p class="alert" role="alert"><Icon name="alert" size={18} />{formError}</p>{/if}
      {:else if remaining === 0}
        <div class="grid place-items-center rounded-2xl bg-app-accent/10 px-4 py-10 text-center text-app-accent">
          <Icon name="check" size={28} />
          <p class="mt-2 font-medium">La cuenta está cubierta.</p>
          <p class="text-sm">Confirma el cobro para registrar la venta.</p>
        </div>
      {:else}
        <p class="alert" role="alert"><Icon name="alert" size={18} />Quita un pago: la suma excede el total de la cuenta.</p>
      {/if}
    </div>
  </div>

  {#if error}<p class="alert mt-4" role="alert"><Icon name="alert" size={18} />{error}</p>{/if}

  {#snippet footer()}
    <button type="button" class="btn-secondary min-h-11" onclick={onclose}>Volver a la cuenta</button>
    <button
      type="button"
      class="btn-primary min-h-11 px-6"
      disabled={busy || remaining !== 0 || waiting || payments.length === 0}
      onclick={() => onconfirm(payments.map(({ id: _id, ...p }) => p))}
    >
      {#if busy}<span class="spin"></span>{/if}Confirmar cobro
    </button>
  {/snippet}
</Modal>
