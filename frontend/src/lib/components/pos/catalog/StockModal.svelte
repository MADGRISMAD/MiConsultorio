<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import { api } from '$lib/api';
  import { pos2 } from '$lib/api/pos2';
  import type { StockLot } from '$lib/types/pos2';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { CatalogItem } from '$lib/types';
  import Modal from '$lib/components/Modal.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { dayLabel } from './expiry';
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
  let lotCode = $state('');
  let expires = $state('');
  /** exits: '' follows FEFO, otherwise the lot to take from */
  let lotId = $state('');
  let lots = $state<StockLot[]>([]);

  $effect(() => {
    if (open) {
      amount = mode === 'count' && item ? String(item.stock) : '';
      note = '';
      error = '';
      lotCode = '';
      expires = '';
      lotId = '';
      lots = [];
      op.reset();
      if (item) {
        pos2
          .lots(item.id)
          .then((l) => (lots = l))
          .catch(() => {
            /* the lot list is a convenience */
          });
      }
    }
  });
  const pickedLot = $derived(lots.find((l) => l.id === lotId));

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
    if (mode === 'in' && expires && expires < new Date().toISOString().slice(0, 10)) {
      error = 'Ese lote ya caducó; no se puede dar entrada.';
      return;
    }
    if (mode === 'out' && pickedLot && value > pickedLot.qty) {
      error = `El lote ${pickedLot.lot_code} solo tiene ${qty(pickedLot.qty)}.`;
      return;
    }
    error = '';
    const it = item;
    const n = note.trim();
    const body =
      mode === 'in'
        ? { delta: value, reason: 'purchase' as const, note: n, lot_code: lotCode.trim() || undefined, expires_on: expires || undefined }
        : mode === 'out'
          ? { delta: -value, reason: 'loss' as const, note: n, lot_id: lotId || undefined }
          : { set_to: value, reason: 'adjustment' as const, note: n };
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
      {#if mode === 'in'}
        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label class="label" for="{uid}-lot">Lote (opcional)</label>
            <input id="{uid}-lot" class="field" bind:value={lotCode} maxlength="40" autocomplete="off" list="{uid}-lots" placeholder="Ej. L2405" />
            <datalist id="{uid}-lots">{#each lots as l (l.id)}<option value={l.lot_code}></option>{/each}</datalist>
          </div>
          <div>
            <label class="label" for="{uid}-exp">Caducidad (opcional)</label>
            <input id="{uid}-exp" type="date" class="field" bind:value={expires} />
          </div>
        </div>
        <p class="hint -mt-2">Si el lote y la caducidad ya existen, la cantidad se suma a ese lote.</p>
      {:else if mode === 'out' && lots.length}
        <div>
          <label class="label" for="{uid}-from">Tomar de</label>
          <select id="{uid}-from" class="field" bind:value={lotId}>
            <option value="">El que caduca primero (recomendado)</option>
            {#each lots.filter((l) => l.qty > 0) as l (l.id)}
              <option value={l.id}>{l.lot_code}{l.expires_on ? ` · caduca ${dayLabel(l.expires_on)}${l.expired ? ' (caducado)' : ''}` : ''} · {qty(l.qty)} {item?.unit ?? ''}</option>
            {/each}
          </select>
        </div>
      {/if}
      {#if lots.length}
        <details class="text-sm">
          <summary class="cursor-pointer text-app-muted">Lotes actuales ({lots.length})</summary>
          <ul class="mt-2 divide-y divide-app-ink/10 rounded-xl ring-1 ring-inset ring-app-ink/10">
            {#each lots as l (l.id)}
              <li class="flex items-center justify-between gap-3 px-3 py-2">
                <span class="min-w-0 truncate">{l.lot_code}</span>
                <span class="flex shrink-0 items-center gap-2 text-xs text-app-muted">
                  {#if l.expires_on}<span class="pill {l.expired ? 'pill-bad' : ''}">{l.expired ? 'Caducó' : 'Caduca'} {dayLabel(l.expires_on)}</span>{/if}
                  <span class="tabular-nums">{qty(l.qty)} {item?.unit ?? ''}</span>
                </span>
              </li>
            {/each}
          </ul>
        </details>
      {/if}
      <div>
        <label class="label" for="{uid}-n">Nota (opcional)</label>
        <input id="{uid}-n" class="field" bind:value={note} maxlength="200" autocomplete="off" placeholder={mode === 'in' ? 'Ej. Factura 1234, proveedor X' : mode === 'out' ? 'Ej. Caducado' : 'Ej. Conteo mensual'} />
      </div>
      <OpError op={op} />
    </form>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
    <button type="submit" form="{uid}-f" class="btn-primary" disabled={op.phase === 'loading'}>{#if op.phase === 'loading'}<span class="spin"></span>{/if}{copy.cta}</button>
  {/snippet}
</Modal>
