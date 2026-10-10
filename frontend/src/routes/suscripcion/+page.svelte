<script lang="ts">
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onDestroy } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { api, ApiError } from '$lib/api';
  import { dateShort, moneyCents } from '$lib/format';
  import { contactEmail } from '$lib/landing/data';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { Billing, CheckoutRow, PlanOffer, Subscription } from '$lib/types';
  import Guard from '$lib/components/Guard.svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import PageHeader from '$lib/components/ui/PageHeader.svelte';
  import Pill from '$lib/components/ui/Pill.svelte';
  import StatePill from '$lib/components/ui/StatePill.svelte';

  let offers = $state<PlanOffer[]>([]);
  let billing = $state<Billing | null>(null);
  let checkouts = $state<CheckoutRow[]>([]);
  let subscription = $state<Subscription | null>(null);
  let online = $state(true);
  let sandbox = $state(false);
  let loading = $state(true);
  let error = $state('');
  let period = $state<'month' | 'year'>('month');
  let paying = $state('');
  let payError = $state('');

  async function load(quiet = false) {
    if (!quiet) loading = true;
    try {
      const r = await api.billing.overview();
      offers = r.offers;
      billing = r.billing;
      checkouts = r.checkouts;
      subscription = r.subscription;
      online = r.online;
      sandbox = r.sandbox;
      error = '';
    } catch (e) {
      if (!quiet) error = e instanceof Error ? e.message : 'No se pudo cargar tu suscripción.';
    } finally {
      loading = false;
    }
  }

  // ---- return from Mercado Pago (?pago=ok|pendiente|error) ----
  type Pago = 'ok' | 'pendiente' | 'error';
  let pago = $state<Pago | null>(null);
  let confirmed = $state(false);
  let polling = $state(false);
  let timer: ReturnType<typeof setInterval> | undefined;
  let returning = ''; // checkout de la suscripción al volver de Mercado Pago
  const POLL_MS = 3000;
  const POLL_MAX = 20; // ~60 s

  function stopPolling() {
    clearInterval(timer);
    timer = undefined;
    polling = false;
  }
  function startPolling() {
    stopPolling();
    polling = true;
    let tries = 0;
    timer = setInterval(async () => {
      tries++;
      // La de la suscripción le pregunta a Mercado Pago aunque su aviso no haya llegado.
      if (returning) await api.billing.checkoutStatus(returning).catch(() => null);
      await load(true);
      if ((returning ? checkouts.find((c) => c.id === returning) : checkouts[0])?.status === 'paid') {
        stopPolling();
        confirmed = true;
        await session.load();
        toast.show(returning ? '¡Listo! Tu suscripción está activa.' : '¡Pago recibido! Tu plan ya está activo.');
      } else if (tries >= POLL_MAX) stopPolling();
    }, POLL_MS);
  }
  onDestroy(stopPolling);

  let started = false;
  $effect(() => {
    if (started) return;
    started = true;
    const p = page.url.searchParams.get('pago');
    returning = page.url.searchParams.get('suscripcion') ?? '';
    if (returning) {
      pago = 'ok';
      void load().then(startPolling);
      void goto('/suscripcion', { replaceState: true, noScroll: true, keepFocus: true });
      return;
    }
    void load().then(() => {
      if (p === 'ok' || p === 'pendiente') {
        pago = p;
        if (checkouts[0]?.status === 'paid') {
          confirmed = true;
          void session.load();
        } else startPolling();
      } else if (p === 'error') pago = 'error';
    });
    if (p) void goto('/suscripcion', { replaceState: true, noScroll: true, keepFocus: true });
  });

  // ---- plans ----
  const monthly = (o: PlanOffer) => (period === 'year' ? Math.round(o.year_cents / 12) : o.month_cents);
  const saves = (o: PlanOffer) => o.month_cents > 0 && o.year_cents < o.month_cents * 12;
  const anySaves = $derived(offers.some(saves));
  const isCurrent = (o: PlanOffer) => billing?.plan === o.id;
  /** Suscribirse al plan actual tiene sentido si no está activo, o si se pagó a mano y aún no se cobra solo. */
  const subscribed = $derived(!!subscription?.active && !subscription.cancel_at_period_end);
  const canPayCurrent = $derived(!!billing && (billing.state !== 'active' || !subscribed));
  let cancelling = $state(false);
  async function cancelSub() {
    if (!confirm('¿Cancelar la suscripción? Ya no se cobrará, y conservas tu plan hasta el ' + dateShort(billing?.current_period_end ?? null) + '.')) return;
    cancelling = true;
    try {
      await api.billing.cancelSubscription();
      await load(true);
      toast.show('Suscripción cancelada. Conservas tu plan hasta el fin del periodo pagado.');
    } catch (e) {
      payError = e instanceof Error ? e.message : 'No se pudo cancelar.';
    }
    cancelling = false;
  }
  const quote = (o: PlanOffer) => `mailto:${contactEmail}?subject=${encodeURIComponent(`Cotización del plan ${o.name} de Caresia`)}`;

  async function pay(o: PlanOffer) {
    paying = o.id;
    payError = '';
    try {
      const r = await api.billing.checkout(o.id, period);
      window.location.assign(r.init_point);
      return; // leaving the page: keep the spinner
    } catch (e) {
      payError =
        e instanceof ApiError && e.status === 409
          ? e.message
          : e instanceof Error
            ? e.message
            : 'No se pudo iniciar el pago. Inténtalo de nuevo.';
    }
    paying = '';
  }

  const limit = (n: number | null, one: string, many: string) => (n === null ? `${many[0].toUpperCase()}${many.slice(1)} sin límite` : n === 1 ? `1 ${one}` : `Hasta ${n} ${many}`);
  const plain = (n: number | null, one: string, many: string) => (n === null ? `${many} sin límite` : n === 1 ? `1 ${one}` : `${n} ${many}`);

  /** What each card lists: the team, the clinic, and the benefits the plan brings (each thing once). */
  function features(o: PlanOffer): { text: string; on: boolean; icon?: 'sparkles' }[] {
    const team = [plain(o.max_doctors, 'especialista', 'especialistas'), plain(o.max_reception, 'recepcionista', 'recepcionistas')];
    if (o.max_cashiers !== 0) team.push(plain(o.max_cashiers, 'cajero', 'cajeros'));
    return [
      { text: `${team.join(' · ')}${o.max_doctors === null ? '' : ' (más el administrador)'}`, on: true },
      { text: `${plain(o.max_kinds, 'giro', 'giros')} · ${plain(o.max_branches, 'sucursal', 'sucursales')}`, on: true },
      { text: 'Agenda, reserva en línea y recordatorios por correo', on: true },
      { text: 'Expedientes, recetas y certificados de tu especialidad', on: true },
      { text: 'Portal del paciente y página pública del consultorio', on: true },
      { text: 'Encuesta de satisfacción, indicadores y alertas de interacciones', on: true },
      o.cobros
        ? { text: 'Cobros: punto de venta, caja, inventario y facturación', on: true }
        : { text: 'Sin sección de cobros (viene desde Crecimiento)', on: false },
      o.permissions
        ? { text: 'Permisos por rol y por persona', on: true }
        : { text: 'Permisos por rol: recepción agenda sin ver expedientes', on: true },
      o.magic_uses
        ? { text: `Asistente de IA: ${o.magic_uses.toLocaleString('es-MX')} usos al mes`, on: true, icon: 'sparkles' }
        : { text: 'Sin asistente de IA (viene desde Crecimiento)', on: false },
      { text: `${o.storage_gb} GB de archivos · soporte ${o.support === 'correo' ? 'por correo' : o.support}`, on: true }
    ];
  }
  const CK: Record<CheckoutRow['status'], { label: string; tone: 'warn' | 'ok' | 'bad' | 'muted' }> = {
    pending: { label: 'Pendiente', tone: 'warn' },
    paid: { label: 'Pagado', tone: 'ok' },
    failed: { label: 'Rechazado', tone: 'bad' },
    expired: { label: 'Vencido', tone: 'muted' }
  };
  const planName = (id: string) => offers.find((o) => o.id === id)?.name ?? id;
  const trialLeft = $derived(billing?.trial_days_left ?? null);
</script>

<svelte:head><title>Suscripción y plan · Caresia</title></svelte:head>

<Guard title="Suscripción y plan" permissions={['adminUsers']} allowLocked>
  <PageHeader title="Suscripción y plan" subtitle="Tu plan, tus pagos y qué incluye cada opción." />

  {#if loading}
    <div class="card"><LoadingRows /></div>
  {:else if error}
    <Alert>{error} <button type="button" class="ml-2 underline" onclick={() => load()}>Reintentar</button></Alert>
  {:else if billing}
    <!-- Return from Mercado Pago -->
    {#if pago === 'error'}
      <p class="mb-4 flex items-start gap-2.5 rounded-xl bg-app-danger/10 px-3.5 py-3 text-sm text-app-danger" role="alert">
        <Icon name="alert" size={18} /><span><strong>El pago no se completó.</strong> No se hizo ningún cargo. Puedes intentarlo de nuevo cuando quieras.</span>
      </p>
    {:else if pago && confirmed}
      <p class="mb-4 flex items-start gap-2.5 rounded-xl bg-app-accent/12 px-3.5 py-3 text-sm text-app-accent" role="status">
        <Icon name="check" size={18} /><span><strong>¡Pago recibido!</strong> Tu plan {billing.plan_name} ya está activo. Gracias por confiar en Caresia.</span>
      </p>
    {:else if pago}
      <p class="mb-4 flex items-start gap-2.5 rounded-xl bg-app-primary/10 px-3.5 py-3 text-sm text-app-primary" role="status">
        {#if polling}<span class="spin mt-0.5"></span>{:else}<Icon name="clock" size={18} />{/if}
        <span>
          <strong>{pago === 'ok' ? 'Estamos confirmando tu pago.' : 'Tu pago está pendiente.'}</strong>
          {#if polling}Esta página se actualiza sola en cuanto Mercado Pago lo confirme.{:else}Si ya pagaste, puede tardar unos minutos en reflejarse; <button type="button" class="underline" onclick={() => { startPolling(); }}>volver a revisar</button>.{/if}
        </span>
      </p>
    {/if}

    <!-- Current plan -->
    <section class="card mb-6 flex flex-wrap items-center justify-between gap-4 p-5">
      <div>
        <p class="section-title">Plan actual</p>
        <p class="display mt-1 flex flex-wrap items-center gap-3 text-3xl">{billing.plan_name}<StatePill state={billing.state} /></p>
        <p class="mt-1 text-sm text-app-muted">
          {#if billing.state === 'trialing'}
            {trialLeft !== null ? `Te ${trialLeft === 1 ? 'queda 1 día' : `quedan ${trialLeft} días`} de prueba` : 'Estás en prueba'}{billing.trial_ends_at ? ` (termina el ${dateShort(billing.trial_ends_at)})` : ''}.
          {:else if billing.state === 'active' && subscription?.active && subscription.cancel_at_period_end}
            Suscripción cancelada: conservas tu plan hasta el {dateShort(billing.current_period_end)}.
          {:else if billing.state === 'active' && subscription?.active}
            Se cobra solo cada {subscription.period === 'year' ? 'año' : 'mes'}{subscription.amount_cents ? ` (${moneyCents(subscription.amount_cents)})` : ''} · próximo cobro el {dateShort(billing.current_period_end)}.
          {:else if billing.state === 'active'}
            Pagado hasta el {dateShort(billing.current_period_end)}.
          {:else if billing.state === 'trial_expired'}
            Tu prueba terminó{billing.trial_ends_at ? ` el ${dateShort(billing.trial_ends_at)}` : ''}. Elige un plan para seguir.
          {:else if billing.state === 'past_due'}
            El periodo pagado venció{billing.current_period_end ? ` el ${dateShort(billing.current_period_end)}` : ''}. Paga para reactivar.
          {:else}
            {billing.suspended_reason ? `Suspendido: ${billing.suspended_reason}.` : 'Tu cuenta está suspendida.'}
          {/if}
        </p>
      </div>
      <div class="flex flex-wrap items-center gap-3">
        {#if billing.cobros}<span class="badge"><Icon name="cash" size={14} />Incluye cobros</span>{/if}
        {#if subscribed}<button type="button" class="btn-secondary" disabled={cancelling} onclick={cancelSub}>{#if cancelling}<span class="spin"></span>{/if}Cancelar suscripción</button>{/if}
      </div>
    </section>

    {#if !online}
      <div class="card mb-6 p-6">
        <EmptyState icon="wallet" title="El pago en línea aún no está disponible" text="Por ahora te ayudamos a contratar o cambiar de plan directamente: escríbenos y lo activamos.">
          <a class="btn-primary" href="mailto:{contactEmail}?subject={encodeURIComponent('Quiero contratar un plan de Caresia')}"><Icon name="mail" size={18} />Contactar a Caresia</a>
        </EmptyState>
      </div>
    {/if}

    <!-- Period toggle -->
    <div class="mb-4 flex flex-wrap items-center gap-3">
      <div class="flex gap-1 rounded-full bg-app-ink/5 p-1" role="group" aria-label="Periodo de pago">
        {#each [['month', 'Mensual'], ['year', 'Anual']] as [id, label]}
          <button type="button" aria-pressed={period === id} class="rounded-full px-4 py-1.5 text-sm font-medium transition {period === id ? 'bg-app-panel text-app-ink shadow-sm' : 'text-app-muted hover:text-app-ink'}" onclick={() => (period = id as 'month' | 'year')}>{label}</button>
        {/each}
      </div>
      {#if anySaves}<span class="badge-soon">Anual: ahorras 2 meses</span>{/if}
      {#if sandbox}<span class="text-xs text-app-warning">Modo de pruebas: los pagos no son reales.</span>{/if}
    </div>

    {#if payError}<Alert class="mb-4">{payError}</Alert>{/if}

    <div class="grid gap-4 lg:grid-cols-3">
      {#each offers as o (o.id)}
        {@const featured = o.id === 'crecimiento'}
        {@const current = isCurrent(o)}
        <article class="card relative flex flex-col p-6 {featured ? 'ring-2 ring-app-primary' : ''}">
          {#if featured}<span class="badge absolute right-5 top-5">Recomendado</span>{/if}
          <h2 class="display text-3xl">{o.name}</h2>
          <p class="mt-1 min-h-[2.5rem] text-sm text-app-muted">{o.description}</p>
          <div class="mt-4">
            {#if o.online && o.month_cents > 0}
              <p class="display text-4xl">{moneyCents(monthly(o))}<span class="font-sans text-sm text-app-muted"> / mes</span></p>
              <p class="mt-0.5 min-h-5 text-xs text-app-muted">
                {#if period === 'year'}{moneyCents(o.year_cents)} al año{#if saves(o)} · <span class="font-medium text-app-accent">Ahorras 2 meses</span>{/if}{:else}Se cobra solo cada mes{/if} · IVA incluido
              </p>
            {:else}
              <p class="display text-4xl">A tu medida</p>
              <p class="mt-0.5 min-h-5 text-xs text-app-muted">Te cotizamos según tu consultorio</p>
            {/if}
          </div>
          <ul class="mt-5 grid flex-1 content-start gap-2 text-sm">
            {#each features(o) as f}
              <li class="flex gap-2 {f.on ? '' : 'text-app-muted'}"><Icon name={f.icon ?? (f.on ? 'check' : 'x')} size={16} class="mt-0.5 flex-none" />{f.text}</li>
            {/each}
          </ul>
          <div class="mt-6">
            {#if !o.online || o.month_cents <= 0}
              <a class="btn-secondary w-full" href={quote(o)}><Icon name="mail" size={18} />Cotizar</a>
            {:else if current && !canPayCurrent}
              <button type="button" class="btn-secondary w-full" disabled>Plan actual</button>
            {:else}
              <button type="button" class="{featured ? 'btn-primary' : 'btn-secondary'} w-full" disabled={!online || !!paying} onclick={() => pay(o)}>
                {#if paying === o.id}<span class="spin"></span>{/if}{current ? 'Activar cobro automático' : 'Suscribirme con Mercado Pago'}
              </button>
            {/if}
          </div>
        </article>
      {/each}
    </div>
    <p class="mt-3 text-xs text-app-muted">La suscripción se cobra sola a tu tarjeta en cada periodo; si estás en prueba, el primer cobro llega al terminar. Puedes cancelarla aquí cuando quieras y conservas tu plan hasta el fin del periodo pagado. Al cambiar de plan, la suscripción anterior se cancela sola.</p>
    <p class="mt-1 text-xs text-app-muted">Si tu consultorio ya tiene más personas de las que permite un plan, no se podrá cambiar a él y te lo indicaremos al intentar pagar.</p>

    <!-- History -->
    <section class="mt-8" aria-labelledby="hist-h">
      <h2 id="hist-h" class="display mb-3 text-3xl">Pagos recientes</h2>
      <div class="card overflow-hidden">
        {#if !checkouts.length}
          <EmptyState icon="receipt" title="Aún no hay pagos" text="Cuando pagues un plan, lo verás aquí." />
        {:else}
          <div class="overflow-x-auto">
            <table class="w-full min-w-[520px]">
              <thead class="border-b border-app-ink/10"><tr><th class="th">Fecha</th><th class="th">Plan</th><th class="th">Periodo</th><th class="th text-right">Monto</th><th class="th">Estado</th></tr></thead>
              <tbody class="divide-y divide-app-ink/10">
                {#each checkouts as c (c.id)}
                  <tr>
                    <td class="td whitespace-nowrap">{dateShort(c.paid_at ?? c.created_at)}</td>
                    <td class="td font-medium">{planName(c.plan)}</td>
                    <td class="td text-app-muted">{c.period === 'year' ? 'Anual' : 'Mensual'}{c.kind === 'subscription' ? ' · automático' : ''}</td>
                    <td class="td text-right font-medium">{moneyCents(c.amount_cents)}</td>
                    <td class="td">
                      <span class="flex flex-wrap items-center gap-2"><Pill tone={CK[c.status].tone}>{c.kind === 'subscription' && c.status === 'paid' ? 'Suscrito' : CK[c.status].label}</Pill>
                        {#if c.status === 'pending' && c.init_point}<a class="text-xs font-medium text-app-primary hover:underline" href={c.init_point}>{c.kind === 'subscription' ? 'Completar suscripción' : 'Completar pago'}</a>{/if}</span>
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </div>
    </section>
  {/if}
</Guard>
