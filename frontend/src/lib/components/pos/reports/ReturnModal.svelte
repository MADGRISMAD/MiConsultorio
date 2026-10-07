<script lang="ts">
  import { api } from '$lib/api';
  import { pos2 } from '$lib/api/pos2';
  import { moneyCents } from '$lib/format';
  import { Op } from '$lib/op.svelte';
  import { printHtml, returnTicketHtml } from '$lib/printer/ticket';
  import { toast } from '$lib/toast.svelte';
  import { PAY_METHODS, type PayMethod } from '$lib/types';
  import type { ReturnInfo, ReturnResult } from '$lib/types/pos2';
  import Modal from '$lib/components/Modal.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';

  interface Props {
    saleId: string | null;
    onclose: () => void;
    /** A return was recorded: refresh the lists. */
    ondone: () => void;
  }
  let { saleId, onclose, ondone }: Props = $props();

  let info = $state<ReturnInfo | null>(null);
  let loading = $state(false);
  let loadError = $state('');
  let qtys = $state<Record<string, string>>({});
  let restock = $state<Record<string, boolean>>({});
  let reason = $state('');
  let payWith = $state('auto');
  let done = $state<ReturnResult | null>(null);
  const op = new Op();
  const printOp = new Op();

  $effect(() => {
    if (!saleId) return;
    const id = saleId;
    info = null;
    done = null;
    loadError = '';
    reason = '';
    payWith = 'auto';
    qtys = {};
    restock = {};
    op.reset();
    printOp.reset();
    loading = true;
    pos2.returnInfo(id).then(
      (r) => {
        if (id !== saleId) return;
        info = r;
        for (const l of r.lines) restock[l.sale_item_id] = l.tracks_stock;
      },
      (e) => (loadError = e instanceof Error ? e.message : 'No se pudo cargar la venta.')
    ).finally(() => (loading = false));
  });

  const label = (m: string) => PAY_METHODS[m as PayMethod]?.label.replace(/ \(.*\)/, '') ?? m;
  const num = (v: string | undefined) => {
    const n = Number((v ?? '').replace(',', '.'));
    return Number.isFinite(n) && n > 0 ? n : 0;
  };

  /** What the customer gets back for the quantities typed (the server computes the exact figure). */
  const estimate = $derived.by(() => {
    let total = 0;
    for (const l of info?.lines ?? []) {
      const q = Math.min(num(qtys[l.sale_item_id]), l.returnable_qty);
      if (q <= 0) continue;
      total += q >= l.returnable_qty - 0.0005 ? l.returnable_cents : Math.min(l.returnable_cents, Math.round(l.unit_cents * q));
    }
    return total;
  });

  /** The same split the server proposes: the most recent payment first. */
  const split = $derived.by(() => {
    const out: { method: string; amount_cents: number }[] = [];
    if (!info || estimate <= 0) return out;
    if (payWith !== 'auto') return [{ method: payWith, amount_cents: estimate }];
    let left = estimate;
    for (const m of info.refundable) {
      if (left <= 0) break;
      const take = Math.min(left, m.amount_cents);
      if (take > 0) out.push({ method: m.method, amount_cents: take });
      left -= take;
    }
    return out;
  });
  const methodsForAll = $derived((info?.refundable ?? []).filter((m) => m.amount_cents >= estimate && estimate > 0));
  const hasCash = $derived(split.some((s) => s.method === 'cash'));
  const hasMp = $derived(split.some((s) => s.method === 'mp_point' || s.method === 'mp_link'));

  function setAll(l: { sale_item_id: string; returnable_qty: number }) {
    qtys[l.sale_item_id] = String(l.returnable_qty);
  }

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    const i = info;
    if (!i) return;
    const lines = i.lines
      .filter((l) => num(qtys[l.sale_item_id]) > 0)
      .map((l) => ({ sale_item_id: l.sale_item_id, qty: Math.min(num(qtys[l.sale_item_id]), l.returnable_qty), restock: l.tracks_stock ? !!restock[l.sale_item_id] : undefined }));
    if (lines.length === 0) return op.fail('Escribe cuántas piezas o servicios regresan.');
    if (reason.trim().length < 3) return op.fail('Escribe el motivo de la devolución.');
    const body = { reason: reason.trim(), lines, refunds: payWith === 'auto' ? undefined : split };
    if (await op.run(async () => void (done = await pos2.createReturn(i.sale.id, body)))) {
      toast.show(`Devolución #${done!.folio} registrada`);
      ondone();
    }
  }

  async function printNote() {
    const r = done;
    if (!r) return;
    await printOp.run(async () => printHtml(returnTicketHtml(r, (await api.pos.settings()).settings)));
  }
</script>

<Modal open={!!saleId} title={done ? `Devolución #${done.folio}` : info ? `Devolución de la venta #${info.sale.folio}` : 'Devolución'} {onclose} wide>
  {#if loading}
    <div class="space-y-3" role="status" aria-label="Cargando">{#each [0, 1, 2] as i}<div class="h-10 animate-pulse rounded-lg bg-app-ink/8"></div>{/each}</div>
  {:else if loadError}
    <p class="alert" role="alert"><Icon name="alert" size={18} />{loadError}</p>
  {:else if done}
    <div class="grid gap-4">
      <p class="rounded-xl bg-app-success/10 px-3.5 py-3 text-sm text-app-success"><strong>Devolución registrada por {moneyCents(done.total_cents)}.</strong></p>
      <ul class="grid gap-1.5 text-sm">
        {#each done.lines as l}
          <li class="flex justify-between gap-3"><span>{l.qty} × {l.name}{#if l.restocked}<span class="text-app-muted"> · regresó al inventario</span>{/if}</span><span class="font-medium">{moneyCents(l.amount_cents)}</span></li>
        {/each}
      </ul>
      <div>
        <p class="section-title mb-1.5">Entrega al cliente</p>
        <ul class="grid gap-1 text-sm">
          {#each done.refunds as r}
            <li class="flex justify-between"><span>{label(r.method)}{#if r.method === 'cash'}<span class="text-app-muted"> · sale de la caja</span>{:else if r.method === 'mp_point' || r.method === 'mp_link'}<span class="text-app-muted"> · devuelto por Mercado Pago</span>{/if}</span><span class="font-medium">{moneyCents(r.amount_cents)}</span></li>
          {/each}
        </ul>
      </div>
      {#if done.credit_note_pending}
        <p class="rounded-xl bg-app-warning/10 px-3.5 py-3 text-sm"><Icon name="alert" size={16} class="mr-1 inline" />Esta venta tiene factura emitida: falta emitir la <strong>nota de crédito (CFDI de egreso)</strong> con tu proveedor o contador. Caresia todavía no la genera.</p>
      {/if}
      {#if printOp.phase === 'error'}<p class="alert" role="alert"><Icon name="alert" size={18} />{printOp.message}</p>{/if}
    </div>
  {:else if info}
    {#if !info.can_return}
      <p class="alert" role="alert"><Icon name="alert" size={18} />{info.sale.status === 'open' ? 'La venta todavía tiene saldo por cobrar: cóbrala completa o cancélala.' : 'Esta venta ya no admite devoluciones.'}</p>
    {:else if info.lines.every((l) => l.returnable_qty <= 0)}
      <p class="alert" role="alert"><Icon name="info" size={18} />Todo lo de esta venta ya fue devuelto.</p>
    {:else}
      <form id="return-form" onsubmit={submit} class="grid gap-5">
        <div class="overflow-x-auto rounded-xl border border-app-ink/10">
          <table class="w-full min-w-[34rem]">
            <thead class="border-b border-app-ink/10"><tr><th class="th">Concepto</th><th class="th text-right">Se puede devolver</th><th class="th text-right">Regresa</th><th class="th">Inventario</th></tr></thead>
            <tbody class="divide-y divide-app-ink/10">
              {#each info.lines as l (l.sale_item_id)}
                <tr class={l.returnable_qty <= 0 ? 'opacity-50' : ''}>
                  <td class="td">{l.name}{#if l.returned_qty > 0}<span class="block text-xs text-app-muted">Ya se devolvió {l.returned_qty}</span>{/if}</td>
                  <td class="td text-right">{l.returnable_qty} <span class="text-xs text-app-muted">· {moneyCents(l.returnable_cents)}</span></td>
                  <td class="td text-right">
                    {#if l.returnable_qty > 0}
                      <div class="flex items-center justify-end gap-1.5">
                        <input class="field w-20 text-right" inputmode="decimal" aria-label="Cantidad a devolver de {l.name}" placeholder="0" bind:value={qtys[l.sale_item_id]} />
                        <button type="button" class="btn-ghost px-2 text-xs" onclick={() => setAll(l)}>Todo</button>
                      </div>
                    {:else}—{/if}
                  </td>
                  <td class="td">
                    {#if l.returnable_qty > 0 && l.tracks_stock}
                      <label class="flex items-center gap-2 text-sm"><input type="checkbox" bind:checked={restock[l.sale_item_id]} />Reponer</label>
                    {:else}<span class="text-xs text-app-muted">{l.kind === 'service' ? 'Servicio' : 'No lleva inventario'}</span>{/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>

        <div>
          <label class="label" for="ret-reason">Motivo</label>
          <input id="ret-reason" class="field" bind:value={reason} maxlength="200" autocomplete="off" placeholder="Ej. Producto dañado, el paciente no lo necesitó" />
        </div>

        {#if estimate > 0}
          <div class="rounded-xl bg-app-elevated p-4">
            <p class="flex items-baseline justify-between text-base font-semibold"><span>A devolver</span><span>{moneyCents(estimate)}</span></p>
            <div class="mt-2 grid gap-2 text-sm">
              {#if methodsForAll.length > 1}
                <label class="flex flex-wrap items-center gap-2"><span class="text-app-muted">Devolver por</span>
                  <select class="field w-auto" bind:value={payWith}>
                    <option value="auto">Como pagó (automático)</option>
                    {#each methodsForAll as m}<option value={m.method}>{label(m.method)}</option>{/each}
                  </select>
                </label>
              {/if}
              <ul class="grid gap-1">
                {#each split as s}<li class="flex justify-between"><span>{label(s.method)}</span><span class="font-medium">{moneyCents(s.amount_cents)}</span></li>{/each}
              </ul>
              {#if hasCash}<p class="text-xs text-app-muted">El efectivo sale de la caja abierta.</p>{/if}
              {#if hasMp}<p class="text-xs text-app-muted">El dinero regresa a la tarjeta del cliente a través de Mercado Pago.</p>{/if}
              {#if info.sale.invoiced}<p class="text-xs text-app-warning">La venta tiene factura emitida: después tendrás que emitir una nota de crédito.</p>{/if}
            </div>
          </div>
        {/if}

        {#if op.phase === 'error'}<p class="alert" role="alert"><Icon name="alert" size={18} />{op.message}</p>{/if}
      </form>
    {/if}
    {#if info.returns.length > 0}
      <div class="mt-5">
        <p class="section-title mb-1.5">Devoluciones anteriores</p>
        <ul class="grid gap-1 text-sm">
          {#each info.returns as r}<li class="flex justify-between gap-3"><span>#{r.folio} · {r.reason}</span><span class="font-medium">{moneyCents(r.total_cents)}</span></li>{/each}
        </ul>
      </div>
    {/if}
  {/if}
  {#snippet footer()}
    {#if done}
      <button type="button" class="btn-secondary" disabled={printOp.phase === 'loading'} onclick={printNote}><Icon name="receipt" size={16} />Imprimir nota</button>
      <button type="button" class="btn-primary" onclick={onclose}>Listo</button>
    {:else}
      <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
      {#if info?.can_return && estimate > 0}
        <button type="submit" form="return-form" class="btn-primary" disabled={op.phase === 'loading'}>{#if op.phase === 'loading'}<span class="spin"></span>{/if}Registrar devolución</button>
      {/if}
    {/if}
  {/snippet}
</Modal>
