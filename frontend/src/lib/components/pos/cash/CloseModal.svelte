<script lang="ts">
  import Alert from '$lib/components/ui/Alert.svelte';
  import Modal from '$lib/components/Modal.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { Op } from '$lib/op.svelte';
  import { api } from '$lib/api';
  import { moneyCents } from '$lib/format';
  import { toast } from '$lib/toast.svelte';
  import { printHtml } from '$lib/printer/ticket';
  import type { CashSession } from '$lib/types';
  import MoneyInput from '../sale/MoneyInput.svelte';
  import CashBreakdown from './CashBreakdown.svelte';
  import { corteHtml } from './corte';

  interface Props {
    open: boolean;
    session: CashSession;
    businessName: string;
    paperWidth: 58 | 80;
    onclose: () => void;
    /** Called as soon as the register is closed; the dialog then shows the corte. */
    onclosed: (s: CashSession) => void;
  }
  let { open, session, businessName, paperWidth, onclose, onclosed }: Props = $props();

  let counted = $state<number | null>(null);
  let note = $state('');
  let error = $state('');
  let closed = $state<CashSession | null>(null);
  const op = new Op();

  const diff = $derived(counted == null ? null : counted - session.expected_cents);

  $effect(() => {
    if (open && !closed) {
      counted = null;
      note = '';
      error = '';
      op.reset();
    }
    if (!open) closed = null;
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    error = '';
    if (counted == null) return void (error = 'Escribe el efectivo que contaste (puede ser cero).');
    let result: CashSession | undefined;
    const ok = await op.run(async () => {
      result = await api.pos.closeCash(counted!, note.trim());
    });
    if (ok && result) {
      closed = result;
      toast.show('Caja cerrada');
      onclosed(result);
    }
  }

  async function print() {
    if (!closed) return;
    try {
      await printHtml(corteHtml(closed, businessName, paperWidth));
    } catch (e) {
      toast.show(`No se pudo imprimir el corte${e instanceof Error && e.message ? ': ' + e.message : ''}`, 'error');
    }
  }
</script>

<Modal {open} title={closed ? 'Corte de caja' : 'Cerrar caja'} {onclose} wide={!!closed}>
  {#if closed}
    <CashBreakdown session={closed} />
  {:else}
    <form id="cash-close" onsubmit={submit} class="space-y-4">
      <div class="rounded-2xl bg-app-primary/10 p-4">
        <p class="section-title text-app-primary">Efectivo esperado en caja</p>
        <p class="display text-4xl tabular-nums">{moneyCents(session.expected_cents)}</p>
      </div>
      <MoneyInput id="close-counted" label="Efectivo contado" bind:cents={counted} autofocus />
      <div
        class="flex items-baseline justify-between rounded-xl px-4 py-3 {diff == null ? 'bg-app-elevated text-app-muted' : diff === 0 ? 'bg-app-accent/12 text-app-accent' : diff > 0 ? 'bg-app-warning/12 text-app-warning' : 'bg-app-danger/10 text-app-danger'}"
        aria-live="polite"
      >
        <span class="text-sm font-medium">{diff == null ? 'Diferencia' : diff === 0 ? 'La caja cuadra' : diff > 0 ? 'Sobrante' : 'Faltante'}</span>
        <span class="display text-3xl tabular-nums">{diff == null ? '—' : moneyCents(Math.abs(diff))}</span>
      </div>
      <div>
        <label class="label" for="close-note">Nota (opcional)</label>
        <input id="close-note" class="field" bind:value={note} maxlength="200" autocomplete="off" placeholder="Ej. Faltante por cambio mal entregado" />
      </div>
      {#if error || op.phase === 'error'}<Alert>{error || op.message}</Alert>{/if}
    </form>
  {/if}
  {#snippet footer()}
    {#if closed}
      <button type="button" class="btn-secondary" onclick={print}><Icon name="receipt" size={16} />Imprimir corte</button>
      <button type="button" class="btn-primary" onclick={onclose}>Listo</button>
    {:else}
      <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
      <button type="submit" form="cash-close" class="btn-primary" disabled={op.phase === 'loading'}>
        {#if op.phase === 'loading'}<span class="spin"></span>{/if}Cerrar caja
      </button>
    {/if}
  {/snippet}
</Modal>
