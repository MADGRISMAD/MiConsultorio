<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { printer, printTest, supports, type Transport } from '$lib/printer/connection.svelte';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { PosSettings } from '$lib/types';
  import Icon from '$lib/components/ui/Icon.svelte';
  import Pill from '$lib/components/ui/Pill.svelte';

  let { s = $bindable() }: { s: PosSettings } = $props();

  type Kind = PosSettings['printer']['kind'];
  const KINDS: { id: Kind; label: string; about: string }[] = [
    { id: 'browser', label: 'Impresora normal (navegador)', about: 'Usa el cuadro de impresión del navegador: sirve con cualquier impresora, de oficina o térmica instalada en el equipo, e incluso AirPrint.' },
    { id: 'usb', label: 'Térmica USB', about: 'Conexión directa por cable USB, sin cuadro de impresión.' },
    { id: 'bluetooth', label: 'Térmica Bluetooth', about: 'Impresión directa desde celular, tableta o computadora con Bluetooth.' },
    { id: 'serial', label: 'Térmica por puerto serie', about: 'Para impresoras con cable serie o adaptador USB-serie.' }
  ];
  const BAUDS = [9600, 19200, 38400, 57600, 115200];

  const thermal = $derived(s.printer.kind !== 'browser');
  const transport = $derived((thermal ? s.printer.kind : null) as Transport | null);
  const supported = $derived(transport ? supports(transport) : true);
  const isIos = typeof navigator !== 'undefined' && /iPad|iPhone|iPod/.test(navigator.userAgent);
  const info = $derived(KINDS.find((k) => k.id === s.printer.kind));
  const sameTransport = $derived(!!transport && printer.transport === transport);
  const connected = $derived(sameTransport && printer.connected);

  const pairOp = new Op();
  const testOp = new Op();
  let baud = $state(printer.baud);

  // The browser remembers permission: pick the printer back up silently
  $effect(() => {
    if (thermal && printer.transport && !printer.connected && !printer.busy) void printer.reconnect();
  });

  async function pair() {
    if (!transport) return;
    if (await pairOp.run(() => printer.pair(transport, baud))) toast.show('Impresora conectada');
  }
  async function test() {
    if (await testOp.run(() => printTest($state.snapshot(s) as PosSettings))) toast.show('Prueba enviada');
  }
</script>

<section class="card p-6">
  <h2 class="display text-2xl">Impresora de tickets</h2>
  <p class="mt-1 text-sm text-app-muted">Elige cómo se imprime en esta computadora. La opción se guarda para el negocio; la conexión a la impresora se hace en cada navegador o dispositivo.</p>

  <div class="mt-4 grid gap-2 sm:grid-cols-2" role="radiogroup" aria-label="Tipo de impresora">
    {#each KINDS as k (k.id)}
      <label class="flex cursor-pointer items-center gap-3 rounded-xl border px-4 py-3 text-sm font-medium transition has-[:focus-visible]:outline has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-app-primary {s.printer.kind === k.id ? 'border-app-primary bg-app-primary/8 text-app-primary' : 'border-app-ink/15 hover:border-app-ink/30'}">
        <input type="radio" class="sr-only" name="printer-kind" value={k.id} bind:group={s.printer.kind} />
        <span class="grid h-4 w-4 shrink-0 place-items-center rounded-full border-2 {s.printer.kind === k.id ? 'border-app-primary' : 'border-app-ink/30'}">{#if s.printer.kind === k.id}<span class="h-2 w-2 rounded-full bg-app-primary"></span>{/if}</span>
        {k.label}
      </label>
    {/each}
  </div>
  {#if info}<p class="mt-3 text-sm text-app-muted">{info.about}</p>{/if}

  {#if thermal}
    {#if isIos}
      <p class="mt-4 flex items-start gap-2.5 note">
        <Icon name="alert" size={18} /><span>Safari en iPhone y iPad no permite conectar impresoras térmicas. Usa la opción “Impresora normal (navegador)” con AirPrint, o abre Caresia desde una computadora o Android con Chrome.</span>
      </p>
    {:else if !supported}
      <p class="mt-4 flex items-start gap-2.5 note">
        <Icon name="alert" size={18} /><span>Este navegador no admite {transport === 'usb' ? 'WebUSB' : transport === 'bluetooth' ? 'Web Bluetooth' : 'Web Serial'}. Abre Caresia con Chrome o Edge (en una página https o localhost), o elige “Impresora normal (navegador)”.</span>
      </p>
    {/if}

    <div class="mt-4 rounded-xl bg-app-elevated p-4">
      <div class="flex flex-wrap items-center gap-2">
        {#if connected}<Pill tone="ok">Conectada</Pill>{:else}<Pill tone="warn">Sin conexión</Pill>{/if}
        {#if sameTransport && printer.name}<span class="text-sm">{printer.name}</span>{/if}
        {#if printer.busy}<span class="spin text-app-muted"></span>{/if}
      </div>
      {#if s.printer.kind === 'serial'}
        <div class="mt-3 max-w-xs">
          <label class="label" for="pr-baud">Velocidad (baudios)</label>
          <select id="pr-baud" class="field" bind:value={baud}>{#each BAUDS as b}<option value={b}>{b}</option>{/each}</select>
          <p class="hint">Usa 115200 si tu impresora lo admite: manda el ticket unas 12 veces más rápido que 9600. Si imprime símbolos raros, baja la velocidad (suele ser 9600 en impresoras antiguas).</p>
        </div>
      {/if}
      <div class="mt-4 flex flex-wrap gap-2">
        <button type="button" class="btn-primary" disabled={!supported || isIos || printer.busy} onclick={pair}>{connected ? 'Elegir otra impresora' : 'Conectar impresora'}</button>
        {#if sameTransport && !connected}<button type="button" class="btn-secondary" disabled={printer.busy} onclick={() => printer.reconnect()}>Reconectar</button>{/if}
        {#if sameTransport}<button type="button" class="btn-ghost" disabled={printer.busy} onclick={() => printer.disconnect()}>Desconectar</button>{/if}
        <button type="button" class="btn-secondary" disabled={testOp.phase === 'loading' || (!connected && !!transport)} onclick={test}>
          {#if testOp.phase === 'loading'}<span class="spin"></span>{/if}Imprimir prueba
        </button>
      </div>
      {#if printer.error || pairOp.phase === 'error' || testOp.phase === 'error'}
        <Alert class="mt-3">{printer.error || pairOp.message || testOp.message}</Alert>
      {/if}
    </div>

    <ul class="mt-4 grid list-disc gap-1.5 pl-5 text-sm text-app-muted">
      <li>Enciende la impresora y revisa que tenga papel antes de conectar.</li>
      <li>Elige el mismo ancho de papel que tu rollo (58 o 80 mm) en la sección Ticket.</li>
      <li>Bluetooth: si tu equipo lo pide, vincula primero la impresora desde los ajustes de Bluetooth del dispositivo.</li>
      <li>Funciona en Chrome o Edge, con Caresia abierto en https. Cierra otros programas que usen la impresora.</li>
      <li>Si el navegador pierde la conexión, usa “Reconectar”; no hace falta elegirla de nuevo.</li>
    </ul>
  {:else}
    <div class="mt-4 flex flex-wrap items-center gap-2">
      <button type="button" class="btn-secondary" disabled={testOp.phase === 'loading'} onclick={test}>
        {#if testOp.phase === 'loading'}<span class="spin"></span>{/if}Imprimir prueba
      </button>
      <OpError op={testOp} class="flex-1" />
    </div>
  {/if}
</section>
