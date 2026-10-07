<script lang="ts">
  import { ticketHtml } from '$lib/printer/ticket';
  import type { PosSettings, Sale } from '$lib/types';
  import Toggle from './Toggle.svelte';

  let { s = $bindable() }: { s: PosSettings } = $props();

  // A believable sale so the preview shows what a real ticket looks like
  const sample: Sale = {
    id: 'sample',
    folio: 128,
    customer_name: 'María Fernández',
    customer_curp: '',
    note: '',
    subtotal_cents: 140000,
    discount_cents: 10000,
    tax_cents: 4000,
    total_cents: 130000,
    status: 'paid',
    void_reason: '',
    voided_by: '',
    created_by: 'Dra. Ana López',
    created_at: new Date().toISOString(),
    lines: [
      { id: 'l1', item_id: null, kind: 'service', name: 'Consulta general', qty: 1, unit_price_cents: 80000, tax_rate: 0, discount_cents: 0, total_cents: 80000 },
      { id: 'l2', item_id: null, kind: 'service', name: 'Limpieza dental', qty: 1, unit_price_cents: 50000, tax_rate: 0, discount_cents: 10000, total_cents: 40000 },
      { id: 'l3', item_id: null, kind: 'product', name: 'Cepillo especial', qty: 2, unit_price_cents: 5000, tax_rate: 16, discount_cents: 0, total_cents: 10000 }
    ],
    payments: [{ method: 'cash', amount_cents: 130000, received_cents: 150000, change_cents: 20000, reference: '' }]
  };

  const html = $derived(ticketHtml(sample, $state.snapshot(s) as PosSettings));
</script>

<section class="card p-6">
  <h2 class="display text-2xl">Ticket</h2>
  <p class="mt-1 text-sm text-app-muted">Así se verá el ticket que entregas al paciente. La vista previa se actualiza mientras escribes.</p>
  <div class="mt-5 grid gap-6 lg:grid-cols-[1fr_20rem]">
    <div class="grid content-start gap-4">
      <div>
        <label class="label" for="tk-head">Encabezado</label>
        <textarea id="tk-head" class="field" rows="2" bind:value={s.ticket_header} maxlength="300" placeholder="Ej. Sucursal Centro · Horario L-V 9 a 19 h"></textarea>
      </div>
      <div>
        <label class="label" for="tk-foot">Pie de ticket</label>
        <textarea id="tk-foot" class="field" rows="2" bind:value={s.ticket_footer} maxlength="300" placeholder="Ej. Para facturar, solicítala el mismo día de tu consulta"></textarea>
      </div>
      <div class="grid gap-4 sm:grid-cols-2">
        <div>
          <label class="label" for="tk-width">Ancho de papel</label>
          <select id="tk-width" class="field" value={String(s.printer.width)} onchange={(e) => (s.printer.width = Number(e.currentTarget.value) as 58 | 80)}>
            <option value="58">58 mm (compacto)</option>
            <option value="80">80 mm (estándar)</option>
          </select>
        </div>
        <div>
          <label class="label" for="tk-copies">Copias</label>
          <select id="tk-copies" class="field" value={String(s.printer.copies)} onchange={(e) => (s.printer.copies = Number(e.currentTarget.value))}>
            {#each [1, 2, 3, 4, 5] as n}<option value={String(n)}>{n}</option>{/each}
          </select>
        </div>
      </div>
      <div class="divide-y divide-app-ink/10">
        <Toggle bind:checked={s.show_tax_line} label="Mostrar IVA incluido" hint="Agrega una línea con el IVA contenido en el total." />
        <Toggle bind:checked={s.printer.auto_print} label="Imprimir automáticamente al cobrar" hint="Si lo apagas, imprimes desde el detalle de la venta." />
        <Toggle bind:checked={s.printer.open_drawer} label="Abrir cajón al cobrar en efectivo" hint="Requiere impresora térmica con cajón conectado." />
        <Toggle bind:checked={s.printer.cut} label="Cortar papel al terminar" hint="Solo impresoras térmicas con cortador." />
      </div>
    </div>
    <div>
      <p class="section-title mb-2">Vista previa</p>
      <!-- the ticket is white paper in both themes, so the frame keeps its own background -->
      <iframe title="Vista previa del ticket" class="h-[26rem] w-full rounded-xl border border-app-ink/10 bg-white" srcdoc={html} sandbox=""></iframe>
    </div>
  </div>
</section>
