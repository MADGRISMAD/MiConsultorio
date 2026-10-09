<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import Modal from '$lib/components/Modal.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { Op } from '$lib/op.svelte';
  import { api } from '$lib/api';
  import { toast } from '$lib/toast.svelte';
  import type { CashSession } from '$lib/types';
  import MoneyInput from '../sale/MoneyInput.svelte';

  interface Props {
    open: boolean;
    onclose: () => void;
    onopened: (s: CashSession) => void;
  }
  let { open, onclose, onopened }: Props = $props();

  let amount = $state<number | null>(null);
  const op = new Op();

  $effect(() => {
    if (open) {
      amount = null;
      op.reset();
    }
  });

  async function submit(e?: SubmitEvent) {
    e?.preventDefault();
    let session: CashSession | undefined;
    const ok = await op.run(async () => {
      session = await api.pos.openCash(amount ?? 0);
    });
    if (ok && session) {
      toast.show('Caja abierta');
      onopened(session);
    }
  }
</script>

<Modal {open} title="Abrir caja" {onclose}>
  <form id="open-cash" onsubmit={submit} class="space-y-4">
    <p class="text-sm text-app-muted">Cuenta el efectivo con el que inicia el turno (fondo para dar cambio). Si empiezas sin fondo, déjalo en cero.</p>
    <MoneyInput id="open-cash-amount" label="Fondo inicial" bind:cents={amount} autofocus />
    <OpError op={op} />
  </form>
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
    <button type="submit" form="open-cash" class="btn-primary" disabled={op.phase === 'loading'}>
      {#if op.phase === 'loading'}<span class="spin"></span>{/if}Abrir caja
    </button>
  {/snippet}
</Modal>
