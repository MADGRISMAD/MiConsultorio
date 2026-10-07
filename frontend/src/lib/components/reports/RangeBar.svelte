<script lang="ts">
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

<div class="flex flex-wrap items-end gap-3">
  <div class="flex gap-1 overflow-x-auto rounded-full bg-app-ink/5 p-1" role="group" aria-label="Periodo">
    {#each RANGE_PRESETS as p}
      <button type="button" aria-pressed={preset === p.id} class="whitespace-nowrap rounded-full px-3.5 py-1.5 text-sm font-medium transition {preset === p.id ? 'bg-app-panel text-app-ink shadow-sm' : 'text-app-muted hover:text-app-ink'}" onclick={() => pick(p.id)}>{p.label}</button>
    {/each}
  </div>
  {#if preset === 'custom'}
    <div class="flex items-end gap-2">
      <div><label class="label" for="rpt-from">Desde</label><input id="rpt-from" type="date" class="field" bind:value={from} max={to} /></div>
      <div><label class="label" for="rpt-to">Hasta</label><input id="rpt-to" type="date" class="field" bind:value={to} min={from} /></div>
    </div>
  {/if}
</div>
