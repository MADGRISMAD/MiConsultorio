<script lang="ts">
  import { emailOk } from '$lib/format';
  import { untrack } from 'svelte';
  import { api } from '$lib/api';
  import { dateShort, moneyCents } from '$lib/format';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { Sale } from '$lib/types';
  import Modal from '$lib/components/Modal.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { CFDI_USES, GENERIC_RFC, RFC_RE, TAX_REGIMES, ZIP_RE } from './fiscal';

  interface Props {
    open: boolean;
    /** Sale to preselect (from the sales report or `?venta=`). */
    saleId?: string;
    onclose: () => void;
    onsaved: () => void;
  }
  let { open, saleId = '', onclose, onsaved }: Props = $props();

  let chosen = $state<Sale | null>(null);
  let q = $state('');
  let options = $state<Sale[]>([]);
  let listLoading = $state(false);
  let rfc = $state('');
  let legalName = $state('');
  let regime = $state('612');
  let zip = $state('');
  let cfdiUse = $state('G03');
  let email = $state('');
  let touched = $state(false);
  const op = new Op();

  // Fresh form each time the modal opens
  $effect(() => {
    if (!open) return;
    untrack(() => {
      chosen = null;
      q = '';
      rfc = legalName = zip = email = '';
      regime = '612';
      cfdiUse = 'G03';
      touched = false;
      op.reset();
    });
    if (saleId) {
      const id = saleId;
      api.pos.sale(id).then((s) => (chosen = s), (e) => op.fail(e instanceof Error ? e.message : 'No se pudo cargar la venta.'));
    }
  });

  // Sales that can still be invoiced, filtered by folio / patient
  $effect(() => {
    if (!open || chosen) return;
    const term = q.trim();
    const t = setTimeout(async () => {
      listLoading = true;
      try {
        options = await api.pos.sales({ invoiceable: true, q: term, limit: 20 });
      } catch {
        options = [];
      } finally {
        listLoading = false;
      }
    }, term ? 250 : 0);
    return () => clearTimeout(t);
  });

  const rfcNorm = $derived(rfc.trim().toUpperCase());
  const rfcOk = $derived(rfcNorm === GENERIC_RFC || RFC_RE.test(rfcNorm));
  const zipOk = $derived(ZIP_RE.test(zip.trim()));
  const mailOk = $derived(email.trim() === '' || emailOk(email));
  const valid = $derived(!!chosen && rfcOk && legalName.trim().length >= 2 && zipOk && mailOk);

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    touched = true;
    if (!valid || !chosen) return;
    const sale = chosen;
    const ok = await op.run(() =>
      api.pos.requestInvoice({
        sale_id: sale.id,
        rfc: rfcNorm,
        legal_name: legalName.trim(),
        tax_regime: regime,
        zip_code: zip.trim(),
        cfdi_use: cfdiUse,
        email: email.trim()
      })
    );
    if (ok) {
      toast.show('Solicitud de factura guardada');
      onsaved();
      onclose();
    }
  }
</script>

<Modal {open} title="Solicitud de factura" {onclose}>
  <form id="invoice-form" class="grid gap-4" onsubmit={submit} novalidate>
    <div>
      <p class="label">Venta a facturar</p>
      {#if chosen}
        <div class="flex items-center justify-between gap-3 rounded-xl bg-app-elevated px-3.5 py-3 text-sm">
          <div class="min-w-0">
            <p class="font-medium">Venta #{chosen.folio} · {moneyCents(chosen.total_cents)}</p>
            <p class="truncate text-app-muted">{dateShort(chosen.created_at)}{chosen.customer_name ? ` · ${chosen.customer_name}` : ''}</p>
          </div>
          {#if !saleId}<button type="button" class="btn-ghost" onclick={() => (chosen = null)}>Cambiar</button>{/if}
        </div>
      {:else}
        <div class="relative">
          <span class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-app-muted"><Icon name="search" size={16} /></span>
          <input class="field pl-9" placeholder="Buscar por folio o paciente" bind:value={q} aria-label="Buscar venta" />
        </div>
        <ul class="mt-2 max-h-48 divide-y divide-app-ink/10 overflow-y-auto rounded-xl border border-app-ink/10" aria-busy={listLoading}>
          {#each options as s (s.id)}
            <li>
              <button type="button" class="flex w-full items-center justify-between gap-3 px-3.5 py-2.5 text-left text-sm hover:bg-app-elevated" onclick={() => (chosen = s)}>
                <span class="min-w-0"><span class="font-medium">#{s.folio}</span> <span class="text-app-muted">{dateShort(s.created_at)}{s.customer_name ? ` · ${s.customer_name}` : ''}</span></span>
                <span class="shrink-0 font-medium">{moneyCents(s.total_cents)}</span>
              </button>
            </li>
          {:else}
            <li class="px-3.5 py-4 text-center text-sm text-app-muted">{listLoading ? 'Buscando…' : 'No hay ventas pagadas pendientes de factura.'}</li>
          {/each}
        </ul>
        {#if touched && !chosen}<p class="mt-1 text-xs text-app-danger">Elige una venta.</p>{/if}
      {/if}
    </div>

    <div class="grid gap-4 sm:grid-cols-2">
      <div>
        <label class="label" for="inv-rfc">RFC</label>
        <input id="inv-rfc" class="field uppercase" bind:value={rfc} maxlength="13" autocomplete="off" aria-invalid={touched && !rfcOk} placeholder="XAXX010101000" />
        {#if touched && !rfcOk}<p class="mt-1 text-xs text-app-danger">El RFC no tiene un formato válido.</p>{:else}<p class="hint">Para público en general: {GENERIC_RFC}</p>{/if}
      </div>
      <div>
        <label class="label" for="inv-zip">Código postal fiscal</label>
        <input id="inv-zip" class="field" bind:value={zip} inputmode="numeric" maxlength="5" autocomplete="postal-code" aria-invalid={touched && !zipOk} />
        {#if touched && !zipOk}<p class="mt-1 text-xs text-app-danger">Son 5 dígitos.</p>{/if}
      </div>
    </div>
    <div>
      <label class="label" for="inv-name">Razón social</label>
      <input id="inv-name" class="field" bind:value={legalName} aria-invalid={touched && legalName.trim().length < 2} />
      <p class="hint">Tal como aparece en la constancia de situación fiscal, sin el régimen societario si no lo incluye.</p>
    </div>
    <div class="grid gap-4 sm:grid-cols-2">
      <div>
        <label class="label" for="inv-reg">Régimen fiscal</label>
        <select id="inv-reg" class="field" bind:value={regime}>
          {#each TAX_REGIMES as r}<option value={r.id}>{r.id} · {r.name}</option>{/each}
        </select>
      </div>
      <div>
        <label class="label" for="inv-use">Uso de CFDI</label>
        <select id="inv-use" class="field" bind:value={cfdiUse}>
          {#each CFDI_USES as u}<option value={u.id}>{u.id} · {u.name}</option>{/each}
        </select>
      </div>
    </div>
    <div>
      <label class="label" for="inv-mail">Correo <span class="font-normal text-app-muted">(opcional)</span></label>
      <input id="inv-mail" type="email" class="field" bind:value={email} autocomplete="email" aria-invalid={touched && !emailOk} />
      {#if touched && !mailOk}<p class="mt-1 text-xs text-app-danger">Revisa el correo.</p>{:else}<p class="hint">Donde se enviará la factura.</p>{/if}
    </div>
    {#if op.phase === 'error'}<p class="alert" role="alert"><Icon name="alert" size={18} />{op.message}</p>{/if}
  </form>
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
    <button type="submit" form="invoice-form" class="btn-primary" disabled={op.phase === 'loading'}>
      {#if op.phase === 'loading'}<span class="spin"></span>{/if}Guardar solicitud
    </button>
  {/snippet}
</Modal>
