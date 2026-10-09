<script lang="ts">
  import { loadIssuer, printPrivacyNotice, privacyNoticeHtml } from '$lib/print';
  import { Op } from '$lib/op.svelte';
  import type { Issuer, Patient } from '$lib/types';
  import Modal from '../Modal.svelte';
  import Icon from '../ui/Icon.svelte';

  interface Props {
    open: boolean;
    /** the patient the notice is about (omit it for a new registration: the name is left blank) */
    patient?: Patient | null;
    /** animals get the owner's wording */
    animal?: boolean;
    onclose: () => void;
  }
  let { open, patient = null, animal = false, onclose }: Props = $props();

  let issuer = $state<Issuer | null>(null);
  let error = $state('');
  const printOp = new Op();

  $effect(() => {
    if (!open || issuer) return;
    error = '';
    loadIssuer().then(
      (i) => (issuer = i),
      (e) => (error = e instanceof Error ? e.message : 'No se pudo cargar el aviso de privacidad.')
    );
  });

  const html = $derived.by(() => {
    if (!issuer) return '';
    // a new registration has no patient yet: the wording follows the kind of patient being registered
    return privacyNoticeHtml(patient, animal && !patient ? { ...issuer, kind: 'VETERINARY' } : issuer);
  });
</script>

<Modal {open} title="Aviso de privacidad" {onclose} wide>
  {#if error}
    <p class="alert" role="alert"><Icon name="alert" size={18} />{error}</p>
  {:else if !issuer}
    <div class="h-64 animate-pulse rounded-xl bg-app-ink/8" role="status" aria-label="Cargando"></div>
  {:else}
    <p class="mb-3 text-sm text-app-muted">Es el aviso de tu consultorio, con tus datos de contacto y de ARCO. Léeselo o entrégaselo al {animal ? 'propietario' : 'paciente'} antes de registrar su consentimiento.</p>
    <iframe title="Aviso de privacidad" srcdoc={html} class="h-[60vh] w-full rounded-xl border border-app-ink/15 bg-white"></iframe>
    {#if printOp.phase === 'error'}<p class="alert mt-3" role="alert"><Icon name="alert" size={18} />{printOp.message}</p>{/if}
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>Cerrar</button>
    <button type="button" class="btn-primary" disabled={!issuer || printOp.phase === 'loading'} onclick={() => printOp.run(() => printPrivacyNotice(patient, animal && !patient && issuer ? { ...issuer, kind: 'VETERINARY' } : (issuer ?? undefined)))}>
      <Icon name="receipt" size={18} />Imprimir
    </button>
  {/snippet}
</Modal>
