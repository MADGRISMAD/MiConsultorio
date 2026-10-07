<script lang="ts">
  import { api, ApiError } from '$lib/api';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { PointDevice, ProviderStatus } from '$lib/types';
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
  const modeOp = new Op();
  let confirmOff = $state(false);
  let devices = $state<PointDevice[]>([]);
  let devLoading = $state(false);
  let devError = $state('');
  let switching = $state('');

  async function loadDevices() {
    devLoading = true;
    devError = '';
    try {
      devices = await api.pos.pointDevices();
    } catch (e) {
      devError = e instanceof Error ? e.message : 'No se pudieron cargar las terminales.';
    } finally {
      devLoading = false;
    }
  }
  $effect(() => {
    if (providers.point_connected) void loadDevices();
    else devices = [];
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
  async function toPdv(d: PointDevice) {
    switching = d.id;
    if (await modeOp.run(() => api.pos.pointMode(d.id, 'PDV'))) {
      toast.show('Terminal en modo PDV. Si no cambia sola, reiníciala.');
      await loadDevices();
    }
    switching = '';
  }
  const modeLabel = (m: string) => (m === 'PDV' ? 'Modo PDV' : m === 'STANDALONE' ? 'Modo independiente' : m || 'Desconocido');
</script>

<section class="card p-6">
  <div class="flex flex-wrap items-center justify-between gap-2">
    <h2 class="display text-2xl">Mercado Pago (cobros con tarjeta)</h2>
    {#if providers.point_connected}<Pill tone="ok">Conectada</Pill>{:else if providers.point_available}<Pill>Sin conectar</Pill>{:else}<Pill tone="warn">No disponible</Pill>{/if}
  </div>
  <p class="mt-1 text-sm text-app-muted">Cobra con una terminal Point o manda ligas de pago; el dinero llega a tu cuenta de Mercado Pago.</p>
  {#if providers.sandbox}<p class="mt-2 text-xs text-app-warning">Modo de pruebas: los cobros no son reales.</p>{/if}

  {#if !providers.point_available}
    <p class="mt-4 flex items-start gap-2.5 rounded-xl bg-app-warning/10 px-3.5 py-3 text-sm text-app-warning">
      <Icon name="info" size={18} />
      <span>Esta función aún no está activada en el servidor. Quien administra la instalación debe configurar <code class="font-mono text-[13px]">MP_CLIENT_ID</code>, <code class="font-mono text-[13px]">MP_CLIENT_SECRET</code> y <code class="font-mono text-[13px]">API_PUBLIC_URL</code>.</span>
    </p>
  {:else if !providers.point_connected}
    <div class="mt-4">
      <p class="text-sm">Conecta tu cuenta para autorizar a Caresia a crear cobros en tu nombre. Te llevaremos a Mercado Pago y regresarás aquí.</p>
      {#if connectOp.phase === 'error'}<p class="alert mt-3" role="alert"><Icon name="alert" size={18} />{connectOp.message}</p>{/if}
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
        <h3 class="text-sm font-semibold">Terminales Point</h3>
        <button type="button" class="icon-btn" aria-label="Actualizar terminales" onclick={loadDevices}><Icon name="refresh" size={18} /></button>
      </div>
      {#if devLoading && !devices.length}
        <div class="mt-2 h-12 animate-pulse rounded-xl bg-app-ink/8" role="status" aria-label="Cargando"></div>
      {:else if devError}
        <p class="alert mt-2" role="alert"><Icon name="alert" size={18} />{devError}</p>
      {:else if !devices.length}
        <p class="mt-2 text-sm text-app-muted">No encontramos terminales vinculadas a tu cuenta. Vincula tu Point desde la app de Mercado Pago y actualiza.</p>
      {:else}
        <ul class="mt-2 divide-y divide-app-ink/10 rounded-xl border border-app-ink/10">
          {#each devices as d (d.id)}
            <li class="flex flex-wrap items-center justify-between gap-3 px-4 py-3 text-sm">
              <div class="min-w-0">
                <p class="truncate font-mono text-[13px]">{d.id}</p>
                <p class="mt-0.5"><Pill tone={d.operating_mode === 'PDV' ? 'ok' : 'warn'}>{modeLabel(d.operating_mode)}</Pill></p>
              </div>
              {#if d.operating_mode !== 'PDV'}
                <button type="button" class="btn-secondary" disabled={!!switching} onclick={() => toPdv(d)}>
                  {#if switching === d.id}<span class="spin"></span>{/if}Cambiar a modo PDV
                </button>
              {/if}
            </li>
          {/each}
        </ul>
        {#if modeOp.phase === 'error'}<p class="alert mt-2" role="alert"><Icon name="alert" size={18} />{modeOp.message}</p>{/if}
        <p class="hint">En modo PDV la terminal recibe el monto desde Caresia y solo falta que el paciente pase su tarjeta. En modo independiente se captura el monto a mano en la terminal y Caresia no puede enviarle cobros.</p>
      {/if}
    </div>
  {/if}
</section>

<ConfirmModal open={confirmOff} title="Desconectar Mercado Pago" op={disconnectOp} confirmLabel="Desconectar" onconfirm={disconnect} onclose={() => (confirmOff = false)}>
  Ya no podrás cobrar con terminal Point ni ligas de pago hasta volver a conectar tu cuenta. Las ventas ya registradas no cambian.
</ConfirmModal>
