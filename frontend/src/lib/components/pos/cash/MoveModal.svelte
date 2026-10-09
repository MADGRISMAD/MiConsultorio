<script lang="ts">
  import Alert from '$lib/components/ui/Alert.svelte';
  import Modal from '$lib/components/Modal.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { Op } from '$lib/op.svelte';
  import { api } from '$lib/api';
  import { toast } from '$lib/toast.svelte';
  import MoneyInput from '../sale/MoneyInput.svelte';

  interface Props {
    open: boolean;
    kind: 'in' | 'out';
    onclose: () => void;
    ondone: () => void;
  }
  let { open, kind, onclose, ondone }: Props = $props();

  let amount = $state<number | null>(null);
  let concept = $state('');
  let error = $state('');
  const op = new Op();

  $effect(() => {
    if (open) {
      amount = null;
      concept = '';
      error = '';
      op.reset();
    }
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    error = '';
    if (!amount || amount <= 0) return void (error = 'Escribe un monto mayor a cero.');
    if (!concept.trim()) return void (error = 'Escribe el concepto del movimiento.');
    const ok = await op.run(() => api.pos.cashMove({ kind, amount_cents: amount!, concept: concept.trim() }));
    if (ok) {
      toast.show(kind === 'in' ? 'Entrada registrada' : 'Salida registrada');
      ondone();
    }
  }
</script>

<Modal {open} title={kind === 'in' ? 'Entrada de efectivo' : 'Salida de efectivo'} {onclose}>
  <form id="cash-move" onsubmit={submit} class="space-y-4">
    <p class="text-sm text-app-muted">
      {kind === 'in' ? 'Registra dinero que entra a la caja fuera de una venta (por ejemplo, más fondo para cambio).' : 'Registra dinero que sale de la caja (por ejemplo, un pago a proveedor o un retiro).'}
    </p>
    <MoneyInput id="move-amount" label="Monto" bind:cents={amount} autofocus />
    <div>
      <label class="label" for="move-concept">Concepto</label>
      <input id="move-concept" class="field" bind:value={concept} maxlength="120" autocomplete="off" placeholder={kind === 'in' ? 'Ej. Fondo adicional' : 'Ej. Pago de agua purificada'} />
    </div>
    {#if error || op.phase === 'error'}<Alert>{error || op.message}</Alert>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
    <button type="submit" form="cash-move" class="btn-primary" disabled={op.phase === 'loading'}>
      {#if op.phase === 'loading'}<span class="spin"></span>{/if}Registrar {kind === 'in' ? 'entrada' : 'salida'}
    </button>
  {/snippet}
</Modal>
