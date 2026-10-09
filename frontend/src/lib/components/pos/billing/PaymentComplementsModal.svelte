<script lang="ts">
  import Alert from '$lib/components/ui/Alert.svelte';
  import { consult } from '$lib/api/consult';
  import { dateShort, moneyCents } from '$lib/format';
  import { toast } from '$lib/toast.svelte';
  import { PAY_METHODS, type InvoiceRequest, type PayMethod } from '$lib/types';
  import type { PaymentComplements } from '$lib/types/consult';
  import Modal from '$lib/components/Modal.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import Pill from '$lib/components/ui/Pill.svelte';

  interface Props {
    invoice: InvoiceRequest | null;
    onclose: () => void;
  }
  let { invoice, onclose }: Props = $props();

  let data = $state<PaymentComplements | null>(null);
  let loading = $state(false);
  let error = $state('');
  let issuing = $state('');
  let seq = 0;

  async function load(id: string) {
    const my = ++seq;
    loading = true;
    error = '';
    try {
      const r = await consult.complements(id);
      if (my === seq) data = r;
    } catch (e) {
      if (my === seq) error = e instanceof Error ? e.message : 'No se pudieron cargar los abonos.';
    } finally {
      if (my === seq) loading = false;
    }
  }
  $effect(() => {
    if (invoice) {
      data = null;
      void load(invoice.id);
    }
  });

  async function issue(paymentId: string) {
    if (!invoice) return;
    issuing = paymentId;
    try {
      const r = await consult.issueComplement(invoice.id, paymentId);
      toast.show(r.emailed ? `Complemento ${r.installment} timbrado y enviado por correo` : `Complemento ${r.installment} timbrado`);
      for (const w of r.warnings) toast.show(w, 'error');
      await load(invoice.id);
    } catch (e) {
      toast.show(e instanceof Error ? e.message : 'No se pudo timbrar el complemento.', 'error');
    } finally {
      issuing = '';
    }
  }
  const method = (m: string) => PAY_METHODS[m as PayMethod]?.label ?? m;
  const pending = $derived(data?.payments.filter((p) => !p.complement_id).length ?? 0);
</script>

<Modal open={!!invoice} title={invoice ? `Complementos de pago · venta #${invoice.folio}` : ''} {onclose} wide>
  <p class="mb-4 flex items-start gap-2.5 rounded-xl bg-app-primary/10 px-3.5 py-3 text-sm text-app-primary">
    <Icon name="info" size={18} />
    <span>Esta venta se facturó a abonos (PPD, forma de pago 99). El SAT pide un <strong>complemento de pago</strong> por cada abono recibido, ligado al folio fiscal de la factura.</span>
  </p>
  {#if loading && !data}
    <p class="text-sm text-app-muted" role="status">Cargando…</p>
  {:else if error}
    <Alert>{error}</Alert>
  {:else if data}
    {#if data.payments.length === 0}
      <p class="text-sm text-app-muted">Esta venta no tiene abonos.</p>
    {:else}
      <p class="mb-3 text-sm text-app-muted">{pending ? `${pending} ${pending === 1 ? 'abono sin complemento' : 'abonos sin complemento'}.` : 'Todos los abonos tienen su complemento.'}</p>
      <div class="overflow-x-auto">
        <table class="w-full min-w-[720px]">
          <thead class="border-b border-app-ink/10">
            <tr>
              <th class="th">Parc.</th><th class="th">Fecha</th><th class="th">Forma</th><th class="th text-right">Saldo anterior</th><th class="th text-right">Pagado</th><th class="th text-right">Insoluto</th><th class="th">Complemento</th><th class="th"><span class="sr-only">Acciones</span></th>
            </tr>
          </thead>
          <tbody class="divide-y divide-app-ink/10">
            {#each data.payments as p (p.payment_id)}
              <tr>
                <td class="td font-medium">{p.installment}</td>
                <td class="td whitespace-nowrap text-app-muted">{dateShort(p.paid_at)}</td>
                <td class="td">{method(p.method)}</td>
                <td class="td text-right tabular-nums">{moneyCents(p.previous_cents)}</td>
                <td class="td text-right font-medium tabular-nums">{moneyCents(p.amount_cents)}</td>
                <td class="td text-right tabular-nums">{moneyCents(p.balance_cents)}</td>
                <td class="td">
                  {#if p.complement_id}
                    <Pill tone={p.state === 'stamped' ? 'ok' : 'warn'}>{p.state === 'stamped' ? 'Timbrado' : 'Timbrando…'}</Pill>
                    {#if p.complement_uuid}<span class="mt-1 block break-all font-mono text-[11px] text-app-muted">{p.complement_uuid}</span>{/if}
                  {:else if p.problem}
                    <span class="text-xs text-app-muted">{p.problem}</span>
                  {:else}
                    <Pill tone="warn">Pendiente</Pill>
                  {/if}
                </td>
                <td class="td">
                  <div class="flex justify-end gap-1">
                    {#if p.can_issue}
                      <button type="button" class="btn-secondary" disabled={!!issuing} onclick={() => issue(p.payment_id)}>{#if issuing === p.payment_id}<span class="spin"></span>{/if}Timbrar complemento</button>
                    {/if}
                    {#if invoice && p.complement_id && p.state === 'stamped'}
                      <a class="btn-ghost" href={consult.complementFileUrl(invoice.id, p.complement_id, 'xml')} download>XML</a>
                      <a class="btn-ghost" href={consult.complementFileUrl(invoice.id, p.complement_id, 'pdf')} download>PDF</a>
                    {/if}
                  </div>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>Cerrar</button>
  {/snippet}
</Modal>
