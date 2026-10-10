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

  interface Props {
    slugOverride?: string;
    onstate?: (v: { enabled: boolean }) => void;
    goto?: (tab: string) => void;
  }
  let { slugOverride = '', onstate, goto }: Props = $props();

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

  const mySlug = $derived(slugOverride || s?.slug || '');
  const link = $derived(mySlug ? `${publicOrigin()}/${mySlug}/portal` : '');
  $effect(() => {
    if (s) onstate?.({ enabled: s.enabled });
  });

  async function save(e: SubmitEvent) {
    e.preventDefault();
    if (!s) return;
    const body = { enabled: s.enabled, welcome: s.welcome };
    if (await saveOp.run(async () => { s = await portalApi.saveSettings(body); })) toast.show('Ajustes del portal guardados');
  }
</script>

<section aria-labelledby="pt-set">
  <h3 id="pt-set" class="sr-only">Portal del paciente</h3>
  {#if loadError}
    <Alert>{loadError}</Alert>
  {:else if !s}
    <LoadingRows />
  {:else}
    <form onsubmit={save} class="grid gap-4">
      <p class="text-sm text-app-muted">
        Tus pacientes (o sus tutores) entran con un código que les llega al correo registrado en su expediente y ven sus citas, recetas y carnet de vacunación. No ven notas clínicas.
      </p>
      <label class="flex cursor-pointer items-start gap-3 rounded-2xl p-4 ring-1 ring-app-ink/12 {s.enabled ? 'bg-app-accent/8' : ''}">
        <input type="checkbox" class="mt-0.5 h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={s.enabled} disabled={!mySlug} />
        <span>
          <span class="font-medium">Activar el portal del paciente</span>
          <span class="block text-sm text-app-muted">{#if !mySlug}Necesita tu dirección en línea.{:else if s.enabled}Activo: <a class="break-all font-medium text-app-primary underline" href={link} target="_blank" rel="noopener">{link}</a>{:else}Apagado: los pacientes no pueden entrar.{/if}</span>
        </span>
      </label>
      {#if !mySlug}
        <p class="note">Primero escribe tu dirección en línea. <button type="button" class="font-medium underline" onclick={() => goto?.('reservas')}>Ir a «Reservas»</button></p>
      {/if}
      <div>
        <label class="label" for="pt-welcome">Mensaje de bienvenida (opcional)</label>
        <textarea id="pt-welcome" class="field" rows="3" maxlength="600" bind:value={s.welcome}></textarea>
      </div>
      <OpError op={saveOp} />
      <div class="save-sticky"><button class="btn-primary" disabled={saveOp.phase === 'loading'}>Guardar</button></div>
    </form>
  {/if}
</section>
