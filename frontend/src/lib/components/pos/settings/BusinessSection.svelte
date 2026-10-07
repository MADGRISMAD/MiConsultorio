<script lang="ts">
  import type { PosSettings } from '$lib/types';
  import { TAX_REGIMES } from '../billing/fiscal';

  let { s = $bindable() }: { s: PosSettings } = $props();
</script>

<section class="card p-6">
  <h2 class="display text-2xl">Datos del negocio y fiscales</h2>
  <p class="mt-1 text-sm text-app-muted">Aparecen en el ticket y se usan como tus datos fiscales.</p>
  <div class="mt-5 grid gap-4 sm:grid-cols-2">
    <div>
      <label class="label" for="biz-name">Nombre comercial</label>
      <input id="biz-name" class="field" bind:value={s.business_name} maxlength="80" />
    </div>
    <div>
      <label class="label" for="biz-legal">Razón social</label>
      <input id="biz-legal" class="field" bind:value={s.legal_name} maxlength="120" />
    </div>
    <div>
      <label class="label" for="biz-rfc">RFC</label>
      <input id="biz-rfc" class="field uppercase" bind:value={s.rfc} maxlength="13" autocomplete="off" />
    </div>
    <div>
      <label class="label" for="biz-reg">Régimen fiscal</label>
      <select id="biz-reg" class="field" bind:value={s.tax_regime}>
        <option value="">Sin especificar</option>
        {#each TAX_REGIMES as r}<option value={r.id}>{r.id} · {r.name}</option>{/each}
      </select>
    </div>
    <div class="sm:col-span-2">
      <label class="label" for="biz-addr">Domicilio fiscal</label>
      <input id="biz-addr" class="field" bind:value={s.tax_address} maxlength="200" autocomplete="street-address" />
    </div>
    <div>
      <label class="label" for="biz-zip">Código postal</label>
      <input id="biz-zip" class="field" bind:value={s.zip_code} inputmode="numeric" maxlength="5" autocomplete="postal-code" />
    </div>
    <div>
      <label class="label" for="biz-phone">Teléfono</label>
      <input id="biz-phone" class="field" type="tel" bind:value={s.phone} maxlength="30" autocomplete="tel" />
    </div>
  </div>
</section>
