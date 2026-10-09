<script lang="ts">
  import { fileToLogo } from '$lib/image';
  import ClinicLogo from '$lib/components/ui/ClinicLogo.svelte';

  let {
    name = $bindable(),
    phone = $bindable(),
    address = $bindable(),
    image = $bindable('')
  }: { name: string; phone: string; address: string; image?: string } = $props();

  let imageError = $state('');
  async function pick(e: Event) {
    const input = e.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    input.value = '';
    if (!file) return;
    imageError = '';
    try {
      image = await fileToLogo(file);
    } catch (err) {
      imageError = err instanceof Error ? err.message : 'No se pudo usar la imagen.';
    }
  }
</script>

<div class="grid gap-4 sm:grid-cols-2">
  <div class="flex flex-wrap items-center gap-4 sm:col-span-2">
    <ClinicLogo src={image} size={72} {name} />
    <div class="min-w-0">
      <p class="label !mb-1">Logo o foto del consultorio <span class="font-normal text-app-muted">(opcional)</span></p>
      <div class="flex flex-wrap gap-2">
        <label class="btn-secondary cursor-pointer">
          <input type="file" accept="image/png,image/jpeg,image/webp" class="sr-only" onchange={pick} />{image ? 'Cambiar foto' : 'Elegir foto'}
        </label>
        {#if image}<button type="button" class="btn-ghost" onclick={() => (image = '')}>Usar el logo de Caresia</button>{/if}
      </div>
      <p class="hint !mt-1">{image ? 'Se imprime en tus recetas, planes y expedientes.' : 'Si no eliges una, se usa el logo de Caresia. Aparece en tus recetas, planes y expedientes.'}</p>
      {#if imageError}<p class="alert mt-2" role="alert">{imageError}</p>{/if}
    </div>
  </div>
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
