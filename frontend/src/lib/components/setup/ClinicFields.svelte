<script lang="ts">
  let { name = $bindable(), phone = $bindable(), address = $bindable() }: { name: string; phone: string; address: string } = $props();
</script>

<div class="grid gap-4 sm:grid-cols-2">
  <div class="sm:col-span-2">
    <label class="label" for="cf-name">Nombre del consultorio</label>
    <input id="cf-name" class="field" bind:value={name} required minlength="2" maxlength="120" autocomplete="organization" />
  </div>
  <div>
    <label class="label" for="cf-phone">Teléfono <span class="font-normal text-app-muted">(opcional)</span></label>
    <input id="cf-phone" class="field" type="tel" bind:value={phone} maxlength="30" autocomplete="tel" placeholder="55 1234 5678" />
  </div>
  <div>
    <label class="label" for="cf-address">Dirección <span class="font-normal text-app-muted">(opcional)</span></label>
    <input id="cf-address" class="field" bind:value={address} maxlength="250" autocomplete="street-address" placeholder="Calle, colonia, ciudad" />
  </div>
</div>
