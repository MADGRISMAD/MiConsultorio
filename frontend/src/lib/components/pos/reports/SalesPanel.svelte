<script lang="ts">
  import Segmented from '$lib/components/ui/Segmented.svelte';
  import PageHeader from '$lib/components/ui/PageHeader.svelte';
  import SalesList from './SalesList.svelte';
  import { PRESETS, presetRange, type Preset } from './range';

  /** Ventas: whoever charges finds a sale, reprints it, invoices it or registers a return, without opening the reports. */
  let preset = $state<Preset>('today');
  let from = $state(presetRange('today').from);
  let to = $state(presetRange('today').to);
  let reload = $state(0);

  function pick(p: Preset) {
    preset = p;
    if (p !== 'custom') ({ from, to } = presetRange(p));
  }
</script>

<PageHeader title="Ventas" subtitle="Busca una venta para reimprimir su ticket, facturarla o registrar una devolución." />

<div class="flex flex-wrap items-end gap-3">
  <Segmented scroll size="sm" label="Periodo" value={preset} options={PRESETS.map((p) => ({ value: p.id, label: p.label }))} onselect={pick} />
  {#if preset === 'custom'}
    <div class="flex items-end gap-2">
      <div><label class="label" for="ventas-from">Desde</label><input id="ventas-from" type="date" class="field" bind:value={from} max={to} /></div>
      <div><label class="label" for="ventas-to">Hasta</label><input id="ventas-to" type="date" class="field" bind:value={to} min={from} /></div>
    </div>
  {/if}
</div>

<SalesList {from} {to} {reload} heading={false} onchanged={() => reload++} />
