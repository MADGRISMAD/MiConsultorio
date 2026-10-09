<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import { labApi } from '$lib/api/lab';
  import { Op } from '$lib/op.svelte';
  import type { LabOrder, LabResult } from '$lib/types/lab';
  import Modal from '../Modal.svelte';
  import Icon from '../ui/Icon.svelte';
  import { computeFlag, FLAG_LABEL, parseNum } from './labUtil';

  interface Props {
    target: { order: LabOrder; result: LabResult } | null;
    onclose: () => void;
    onsaved: (o: LabOrder) => void;
  }
  let { target, onclose, onsaved }: Props = $props();

  const op = new Op();
  let value = $state('');
  let low = $state('');
  let high = $state('');
  let unit = $state('');
  let reason = $state('');

  $effect(() => {
    const r = target?.result;
    if (!r) return;
    op.reset();
    value = r.value_num != null ? String(r.value_num) : r.value_text;
    low = r.ref_low != null ? String(r.ref_low) : '';
    high = r.ref_high != null ? String(r.ref_high) : '';
    unit = r.unit;
    reason = '';
  });

  const numeric = $derived(target?.result.value_num != null);
  const flag = $derived(numeric ? computeFlag(parseNum(value), parseNum(low), parseNum(high)) : 'na');

  async function save() {
    if (!target) return;
    const { order, result: r } = target;
    if (!reason.trim()) return op.fail('Escribe el motivo de la corrección.');
    if (!value.trim()) return op.fail('Captura el valor correcto.');
    const base = { panel: r.panel, analyte: r.analyte, unit: unit.trim(), resulted_at: r.resulted_at, supersedes_id: r.id, notes: reason.trim() };
    let input;
    if (numeric) {
      const v = parseNum(value);
      const l = parseNum(low);
      const h = parseNum(high);
      if (v == null) return op.fail('El valor debe ser un número.');
      if ((low.trim() && l == null) || (high.trim() && h == null)) return op.fail('El rango no es válido.');
      const same = l === r.ref_low && h === r.ref_high;
      input = { ...base, value_num: v, ref_low: l, ref_high: h, ref_source: l == null && h == null ? ('' as const) : same && r.ref_source ? r.ref_source : ('laboratorio' as const) };
    } else {
      input = { ...base, value_text: value.trim(), flag: r.flag };
    }
    let saved: LabOrder | undefined;
    if (await op.run(async () => (saved = await labApi.addResults(order.id, [input])))) if (saved) onsaved(saved);
  }
</script>

<Modal open={!!target} title="Corregir resultado" {onclose}>
  {#if target}
    <form id="lab-correct" onsubmit={(e) => { e.preventDefault(); save(); }} class="grid gap-4">
      <p class="text-sm text-app-muted">
        El resultado anterior no se borra: queda en el historial como reemplazado y este nuevo valor pasa a ser el vigente.
      </p>
      <p class="text-sm"><strong>{target.result.analyte}</strong> · valor anterior: {target.result.value_num ?? target.result.value_text} {target.result.unit}</p>
      <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <div class="col-span-2">
          <label class="label" for="cr-value">Valor correcto</label>
          <input id="cr-value" class="field" bind:value inputmode={numeric ? 'decimal' : 'text'} autocomplete="off" />
        </div>
        <div>
          <label class="label" for="cr-unit">Unidad</label>
          <input id="cr-unit" class="field" bind:value={unit} maxlength="30" />
        </div>
        {#if numeric}
          <div class="flex items-end pb-2 text-sm" aria-live="polite">{value.trim() ? FLAG_LABEL[flag] : ''}</div>
          <div>
            <label class="label" for="cr-low">Ref. mínima</label>
            <input id="cr-low" class="field" bind:value={low} inputmode="decimal" />
          </div>
          <div>
            <label class="label" for="cr-high">Ref. máxima</label>
            <input id="cr-high" class="field" bind:value={high} inputmode="decimal" />
          </div>
        {/if}
      </div>
      <div>
        <label class="label" for="cr-reason">Motivo de la corrección</label>
        <input id="cr-reason" class="field" bind:value={reason} maxlength="500" placeholder="Ej. Error de captura; el reporte dice 105" autocomplete="off" />
      </div>
      <OpError op={op} />
    </form>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
    <button type="submit" form="lab-correct" class="btn-primary" disabled={op.phase === 'loading'}>
      {#if op.phase === 'loading'}<span class="spin"></span>{/if}Guardar corrección
    </button>
  {/snippet}
</Modal>
