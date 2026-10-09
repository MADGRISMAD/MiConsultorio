<script lang="ts">
  import Alert from '$lib/components/ui/Alert.svelte';
  import { loadIssuer, privacyNoticeHtml } from '$lib/print';
  import type { Issuer, Patient } from '$lib/types';
  import Modal from '../Modal.svelte';
  import Icon from '../ui/Icon.svelte';

  interface Props {
    open: boolean;
    /** the patient the notice is about (omit it for a new registration) */
    patient?: Patient | null;
    /** animals get the owner's wording */
    animal?: boolean;
    onclose: () => void;
  }
  let { open, patient = null, animal = false, onclose }: Props = $props();

  let issuer = $state<Issuer | null>(null);
  let error = $state('');

  $effect(() => {
    if (!open || issuer) return;
    error = '';
    loadIssuer().then(
      (i) => (issuer = i),
      (e) => (error = e instanceof Error ? e.message : 'No se pudo cargar el aviso de privacidad.')
    );
  });

  // plain readable text: no consent box, no signature, nothing to print
  const html = $derived(issuer ? privacyNoticeHtml(patient, animal && !patient ? { ...issuer, kind: 'VETERINARY' } : issuer, { reading: true }) : '');
</script>

<Modal {open} title="Aviso de privacidad" {onclose} wide>
  {#if error}
    <Alert>{error}</Alert>
  {:else if !issuer}
    <div class="h-64 animate-pulse rounded-xl bg-app-ink/8" role="status" aria-label="Cargando"></div>
  {:else}
    <iframe title="Aviso de privacidad" srcdoc={html} class="h-[65vh] w-full rounded-xl border border-app-ink/15 bg-white"></iframe>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-primary" onclick={onclose}>Cerrar</button>
  {/snippet}
</Modal>
