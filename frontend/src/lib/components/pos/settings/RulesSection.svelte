<script lang="ts">
  import type { PosSettings } from '$lib/types';
  import Toggle from './Toggle.svelte';

  let { s = $bindable() }: { s: PosSettings } = $props();
</script>

<section class="card p-6">
  <h2 class="display text-2xl">Reglas de venta</h2>
  <div class="mt-5 grid gap-4 sm:grid-cols-2">
    <div>
      <label class="label" for="rl-cur">Moneda</label>
      <select id="rl-cur" class="field" bind:value={s.currency}>
        <option value="MXN">Peso mexicano (MXN)</option>
        <option value="USD">Dólar (USD)</option>
      </select>
    </div>
    <div>
      <label class="label" for="rl-tax">IVA predeterminado (%)</label>
      <input id="rl-tax" class="field" type="number" min="0" max="100" step="0.5" bind:value={s.default_tax_rate} />
      <p class="hint">Se aplica a conceptos nuevos. Los servicios médicos suelen ir en 0%.</p>
    </div>
  </div>
  <div class="mt-3 divide-y divide-app-ink/10">
    <Toggle bind:checked={s.require_open_cash} label="Exigir caja abierta para cobrar" hint="Nadie cobra hasta que se abra la caja del día." />
    <Toggle bind:checked={s.allow_negative_stock} label="Permitir vender sin existencias" hint="El inventario puede quedar en negativo; útil si no capturas todas tus compras." />
    <Toggle bind:checked={s.allow_discounts} label="Permitir descuentos" />
  </div>
  {#if s.allow_discounts}
    <div class="mt-3 max-w-xs">
      <label class="label" for="rl-disc">Descuento máximo (%)</label>
      <input id="rl-disc" class="field" type="number" min="0" max="100" step="1" bind:value={s.max_discount_pct} />
    </div>
  {/if}
</section>
