<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { api } from '$lib/api';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { PointState, PointTerminal, ProviderStatus } from '$lib/types';
  import ConfirmModal from '$lib/components/ConfirmModal.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import Pill from '$lib/components/ui/Pill.svelte';

  interface Props {
    providers: ProviderStatus;
    /** Reload provider status after connecting or disconnecting. */
    onchanged: () => void;
  }
  let { providers, onchanged }: Props = $props();

  const connectOp = new Op();
  const disconnectOp = new Op();
  const registerOp = new Op();
  let confirmOff = $state(false);
  let terminals = $state<PointTerminal[]>([]);
  let ready = $state<PointState | null>(null);
  let loading = $state(false);
  let loadError = $state('');
  let choosing = $state('');

  async function load() {
    loading = true;
    loadError = '';
    try {
      [terminals, ready] = await Promise.all([api.pos.pointTerminals(), api.pos.pointStatus()]);
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'No se pudieron cargar las terminales.';
    } finally {
      loading = false;
    }
  }
  $effect(() => {
    if (providers.point_connected) void load();
    else {
      terminals = [];
      ready = null;
    }
  });

  async function connect() {
    let url = '';
    if (await connectOp.run(async () => (url = await api.pos.pointConnectUrl()))) window.location.assign(url);
  }
  async function disconnect() {
    if (await disconnectOp.run(() => api.pos.pointDisconnect())) {
      confirmOff = false;
      toast.show('Cuenta de Mercado Pago desconectada');
      onchanged();
    }
  }
  async function use(t: PointTerminal) {
    choosing = t.id;
    if (await registerOp.run(() => api.pos.pointRegister(t.id))) {
      toast.show('Terminal registrada y en modo PDV');
      await load();
    }
    choosing = '';
  }
  const modeLabel = (m: string) => (m === 'PDV' ? 'Modo PDV' : m === 'STANDALONE' ? 'Modo independiente' : m || 'Modo desconocido');
</script>

<section class="card p-6">
  <div class="flex flex-wrap items-center justify-between gap-2">
    <h2 class="display text-2xl">Mercado Pago (cobros con tarjeta)</h2>
    {#if providers.point_connected}<Pill tone="ok">Conectada</Pill>{:else if providers.point_available}<Pill>Sin conectar</Pill>{:else}<Pill tone="warn">No disponible</Pill>{/if}
  </div>
  <p class="mt-1 text-sm text-app-muted">Cobra con una terminal Point o manda ligas de pago; el dinero llega a tu cuenta de Mercado Pago.</p>
  {#if providers.sandbox}<p class="mt-2 text-xs text-app-warning">Modo de pruebas: los cobros no son reales.</p>{/if}

  {#if !providers.point_available}
    <p class="mt-4 flex items-start gap-2.5 note">
      <Icon name="info" size={18} />
      <span>Esta función aún no está activada en el servidor. Quien administra la instalación debe configurar <code class="font-mono text-[13px]">MP_CLIENT_ID</code>, <code class="font-mono text-[13px]">MP_CLIENT_SECRET</code> y <code class="font-mono text-[13px]">API_PUBLIC_URL</code>.</span>
    </p>
  {:else if !providers.point_connected}
    <div class="mt-4">
      <p class="text-sm">Conecta tu cuenta para autorizar a Caresia a crear cobros en tu nombre. Te llevaremos a Mercado Pago y regresarás aquí.</p>
      <OpError op={connectOp} class="mt-3" />
      <button type="button" class="btn-primary mt-4" disabled={connectOp.phase === 'loading'} onclick={connect}>
        {#if connectOp.phase === 'loading'}<span class="spin"></span>{/if}Conectar mi cuenta de Mercado Pago
      </button>
    </div>
  {:else}
    <div class="mt-4 flex flex-wrap items-center justify-between gap-3 rounded-xl bg-app-elevated px-4 py-3 text-sm">
      <p>Cuenta conectada{#if providers.point_account}: <span class="font-mono text-[13px]">{providers.point_account}</span>{/if}</p>
      <button type="button" class="btn-ghost text-app-danger" onclick={() => { disconnectOp.reset(); confirmOff = true; }}>Desconectar</button>
    </div>

    <div class="mt-5">
      <div class="flex items-center justify-between gap-2">
        <h3 class="text-sm font-semibold">Terminal Point</h3>
        <button type="button" class="icon-btn" aria-label="Actualizar terminales" onclick={load}><Icon name="refresh" size={18} /></button>
      </div>
      {#if ready}
        <p class="mt-2 flex items-start gap-2 text-sm {ready.ok ? 'text-app-accent' : 'text-app-muted'}">
          <Icon name={ready.ok ? 'check' : 'info'} size={18} class="mt-px flex-none" />
          <span>{ready.ok ? `Lista para cobrar${ready.terminal_label ? ` en ${ready.terminal_label}` : ''}.` : ready.message}</span>
        </p>
      {/if}
      {#if loading && !terminals.length}
        <div class="mt-2 h-12 animate-pulse rounded-xl bg-app-ink/8" role="status" aria-label="Cargando"></div>
      {:else if loadError}
        <Alert class="mt-2">{loadError}</Alert>
      {:else if !terminals.length}
        <p class="mt-2 text-sm text-app-muted">No encontramos terminales vinculadas a tu cuenta. Vincula tu Point desde la app de Mercado Pago y actualiza.</p>
      {:else}
        <ul class="mt-2 divide-y divide-app-ink/10 rounded-xl border border-app-ink/10">
          {#each terminals as t (t.id)}
            <li class="flex flex-wrap items-center justify-between gap-3 px-4 py-3 text-sm">
              <div class="min-w-0">
                <p class="truncate font-medium">{t.label}</p>
                <p class="truncate font-mono text-[12px] text-app-muted">{t.id}</p>
                <p class="mt-1 flex flex-wrap gap-1.5">
                  {#if t.registered}<Pill tone="ok">En uso</Pill>{/if}
                  <Pill tone={t.operating_mode === 'PDV' ? 'ok' : 'warn'}>{modeLabel(t.operating_mode)}</Pill>
                </p>
              </div>
              {#if !t.registered}
                <button type="button" class="btn-secondary" disabled={!!choosing} onclick={() => use(t)}>
                  {#if choosing === t.id}<span class="spin"></span>{/if}Usar esta terminal
                </button>
              {/if}
            </li>
          {/each}
        </ul>
        <OpError op={registerOp} class="mt-2" />
        <p class="hint">Al elegirla, la terminal pasa a modo PDV: recibe el monto desde Caresia y solo falta que el paciente pase su tarjeta. Cancelar una venta pagada con la terminal devuelve el dinero en Mercado Pago.</p>
      {/if}
    </div>
  {/if}
</section>

<ConfirmModal open={confirmOff} title="Desconectar Mercado Pago" op={disconnectOp} confirmLabel="Desconectar" onconfirm={disconnect} onclose={() => (confirmOff = false)}>
  Ya no podrás cobrar con terminal Point ni ligas de pago hasta volver a conectar tu cuenta. Las ventas ya registradas no cambian.
</ConfirmModal>
