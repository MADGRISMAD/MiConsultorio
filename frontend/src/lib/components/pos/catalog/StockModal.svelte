<script lang="ts">
  import { api } from '$lib/api';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { CatalogItem } from '$lib/types';
  import Modal from '$lib/components/Modal.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { parsePesos, qty } from './helpers';

  export type StockMode = 'in' | 'out' | 'count';

  interface Props {
    open: boolean;
    item: CatalogItem | null;
    mode: StockMode;
    onsaved: () => void;
    onclose: () => void;
  }
  let { open, item, mode, onsaved, onclose }: Props = $props();

  const uid = $props.id();
  const op = new Op();
  let amount = $state('');
  let note = $state('');
  let error = $state('');

  $effect(() => {
    if (open) {
      amount = mode === 'count' && item ? String(item.stock) : '';
      note = '';
      error = '';
      op.reset();
    }
  });

  const copy = $derived(
    {
      in: { title: 'Entrada de mercancía', label: 'Cantidad que entra', cta: 'Registrar entrada', hint: 'Compra o reposición. Se suma a la existencia actual.' },
      out: { title: 'Salida o merma', label: 'Cantidad que sale', cta: 'Registrar salida', hint: 'Caducado, dañado, uso interno o pérdida. Se resta de la existencia.' },
      count: { title: 'Contar existencias', label: 'Cantidad contada', cta: 'Guardar conteo', hint: 'Escribe lo que hay físicamente; ajustamos la existencia a ese número.' }
    }[mode]
  );
  const value = $derived(parsePesos(amount));
  const after = $derived(!item || Number.isNaN(value) ? null : mode === 'in' ? item.stock + value : mode === 'out' ? item.stock - value : value);

  async function submit(ev: SubmitEvent) {
    ev.preventDefault();
    if (!item) return;
    if (amount.trim() === '' || Number.isNaN(value) || value < 0 || (mode !== 'count' && value === 0)) {
      error = mode === 'count' ? 'Escribe la cantidad contada (0 o más).' : 'Escribe una cantidad mayor a 0.';
      return;
    }
    error = '';
    const it = item;
    const n = note.trim();
    const body =
      mode === 'in' ? { delta: value, reason: 'purchase' as const, note: n } : mode === 'out' ? { delta: -value, reason: 'loss' as const, note: n } : { set_to: value, reason: 'adjustment' as const, note: n };
    if (!(await op.run(() => api.pos.adjustStock(it.id, body)))) return;
    toast.show(`Existencia de “${it.name}” actualizada`);
    onsaved();
  }
</script>

<Modal {open} title={copy.title} {onclose}>
  {#if item}
    <form id="{uid}-f" class="space-y-4" onsubmit={submit} novalidate>
      <p class="text-sm"><strong>{item.name}</strong><span class="block text-app-muted">Existencia actual: {qty(item.stock)} {item.unit}</span></p>
      <div>
        <label class="label" for="{uid}-q">{copy.label} ({item.unit})</label>
        <!-- svelte-ignore a11y_autofocus -->
        <input id="{uid}-q" class="field" inputmode="decimal" autocomplete="off" bind:value={amount} aria-invalid={!!error} autofocus />
        <p class="hint" class:text-app-danger={!!error} role={error ? 'alert' : undefined}>{error || copy.hint}</p>
      </div>
      {#if after !== null}
        <p class="text-sm text-app-muted" aria-live="polite">Quedará en <strong class={after < 0 ? 'text-app-danger' : 'text-app-ink'}>{qty(after)} {item.unit}</strong></p>
      {/if}
      <div>
        <label class="label" for="{uid}-n">Nota (opcional)</label>
        <input id="{uid}-n" class="field" bind:value={note} maxlength="200" autocomplete="off" placeholder={mode === 'in' ? 'Ej. Factura 1234, proveedor X' : mode === 'out' ? 'Ej. Caducado' : 'Ej. Conteo mensual'} />
      </div>
      {#if op.phase === 'error'}<p class="alert" role="alert"><Icon name="alert" size={18} />{op.message}</p>{/if}
    </form>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
    <button type="submit" form="{uid}-f" class="btn-primary" disabled={op.phase === 'loading'}>{#if op.phase === 'loading'}<span class="spin"></span>{/if}{copy.cta}</button>
  {/snippet}
</Modal>
