<script lang="ts">
  import { publicOrigin } from '$lib/site';
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onMount } from 'svelte';
  import { portalApi } from '$lib/api/portal';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { PortalSettings } from '$lib/types/portal';
  import Icon from '../ui/Icon.svelte';
  import LoadingRows from '../ui/LoadingRows.svelte';

  let s = $state<PortalSettings | null>(null);
  let loadError = $state('');
  const saveOp = new Op();

  onMount(async () => {
    try {
      s = await portalApi.settings();
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'No se pudieron cargar los ajustes.';
    }
  });

  const link = $derived(s?.slug ? `${publicOrigin()}/portal/${s.slug}` : '');

  async function save(e: SubmitEvent) {
    e.preventDefault();
    if (!s) return;
    const body = { enabled: s.enabled, welcome: s.welcome };
    if (await saveOp.run(async () => { s = await portalApi.saveSettings(body); })) toast.show('Ajustes del portal guardados');
  }
</script>

<section aria-labelledby="pt-set">
  <h3 id="pt-set" class="section-title mb-3">Portal del paciente</h3>
  {#if loadError}
    <Alert>{loadError}</Alert>
  {:else if !s}
    <LoadingRows />
  {:else}
    <form onsubmit={save} class="grid gap-4">
      <p class="text-sm text-app-muted">
        Tus pacientes (o sus tutores) entran con un código que les llega al correo registrado en su expediente y ven sus citas, recetas y carnet de vacunación. No ven notas clínicas.
      </p>
      <label class="flex cursor-pointer items-center gap-3 text-sm font-medium">
        <input type="checkbox" class="h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={s.enabled} />
        Activar el portal del paciente
      </label>
      {#if !s.slug}
        <p class="note">Para activarlo, primero define la dirección de reserva en línea en los ajustes de agenda: el portal usa la misma dirección.</p>
      {:else if link}
        <p class="text-sm">Dirección del portal: <a class="break-all font-medium text-app-primary underline" href={link} target="_blank" rel="noopener">{link}</a></p>
      {/if}
      <div>
        <label class="label" for="pt-welcome">Mensaje de bienvenida (opcional)</label>
        <textarea id="pt-welcome" class="field" rows="3" maxlength="600" bind:value={s.welcome}></textarea>
      </div>
      <OpError op={saveOp} />
      <div><button class="btn-primary" disabled={saveOp.phase === 'loading'}>Guardar</button></div>
    </form>
  {/if}
</section>
