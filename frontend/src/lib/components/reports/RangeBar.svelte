<script lang="ts">
  import Segmented from '$lib/components/ui/Segmented.svelte';
  import { RANGE_PRESETS, rangeFor, type RangePreset } from './range';

  interface Props {
    from: string;
    to: string;
    preset?: RangePreset;
  }
  let { from = $bindable(), to = $bindable(), preset = $bindable('30d') }: Props = $props();

  function pick(p: RangePreset) {
    preset = p;
    if (p !== 'custom') ({ from, to } = rangeFor(p));
  }
</script>

<div class="flex min-w-0 max-w-full flex-wrap items-end gap-3">
  <Segmented scroll size="sm" label="Periodo" value={preset} options={RANGE_PRESETS.map((p) => ({ value: p.id, label: p.label }))} onselect={pick} />
  {#if preset === 'custom'}
    <div class="flex items-end gap-2">
      <div><label class="label" for="rpt-from">Desde</label><input id="rpt-from" type="date" class="field" bind:value={from} max={to} /></div>
      <div><label class="label" for="rpt-to">Hasta</label><input id="rpt-to" type="date" class="field" bind:value={to} min={from} /></div>
    </div>
  {/if}
</div>
