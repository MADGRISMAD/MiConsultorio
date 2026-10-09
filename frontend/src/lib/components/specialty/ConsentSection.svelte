<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { dateTime as dt } from '$lib/format';
  import { untrack } from 'svelte';
  import { specialtyApi } from '$lib/api/specialty';
  import { Op } from '$lib/op.svelte';
  import { printSignedConsent } from '$lib/print';
  import { CONSENT_LABEL } from '$lib/print/specialty';
  import { toast } from '$lib/toast.svelte';
  import type { Patient } from '$lib/types';
  import type { Consent, ConsentKind, SignatureInput } from '$lib/types/specialty';
  import Modal from '../Modal.svelte';
  import EmptyState from '../ui/EmptyState.svelte';
  import Icon from '../ui/Icon.svelte';
  import { CONSENT_KINDS, consentText } from './consentTexts';
  import SignBlock from './SignBlock.svelte';
  import { session } from '$lib/session.svelte';

  /** bump `version` to reload the list (e.g. after a plan was signed) */
  let { patient, canWrite, version = 0 }: { patient: Patient; canWrite: boolean; version?: number } = $props();

  const animal = $derived(patient.subject === 'animal');
  const patientName = $derived(`${patient.names} ${patient.last_names}`.trim());
  let consents = $state<Consent[]>([]);
  let loading = $state(true);
  let error = $state('');

  async function load() {
    try {
      consents = await specialtyApi.consents(patient.id);
      error = '';
    } catch (e) {
      error = e instanceof Error ? e.message : 'No se pudieron cargar los consentimientos.';
    } finally {
      loading = false;
    }
  }
  $effect(() => {
    void version;
    untrack(load);
  });

  let open = $state(false);
  let kind = $state<ConsentKind>('procedimiento');
  let text = $state('');
  let sig = $state<SignatureInput>({ signer_name: '', signer_role: 'paciente', signature_png: '', witness1: '', witness2: '' });
  const op = new Op();
  const kinds = $derived(CONSENT_KINDS.filter((k) => k.v !== 'plan_tratamiento' && (animal || k.v !== 'animal')));

  function fill(k: ConsentKind) {
    kind = k;
    text = consentText(k, { patient: patientName, clinic: session.clinic?.name ?? 'el consultorio', animal });
  }
  function start() {
    sig = { signer_name: '', signer_role: animal ? 'propietario' : 'paciente', signature_png: '', witness1: '', witness2: '' };
    fill(animal ? 'animal' : 'procedimiento');
    op.reset();
    open = true;
  }
  async function submit() {
    if (!text.trim()) return op.fail('El texto del consentimiento no puede estar vacío.');
    if (!sig.signer_name.trim()) return op.fail('Escribe el nombre de quien firma.');
    if (!sig.signature_png) return op.fail('Falta la firma.');
    if (await op.run(() => specialtyApi.signConsent(patient.id, { kind, text_snapshot: text.trim(), ...sig }))) {
      open = false;
      toast.show('Consentimiento firmado y guardado');
      load();
    }
  }
  let busy = $state('');
  async function print(c: Consent) {
    busy = c.id;
    try {
      await printSignedConsent(patient, c.id);
    } catch (e) {
      toast.show(e instanceof Error ? e.message : 'No se pudo imprimir.', 'error');
    } finally {
      busy = '';
    }
  }
</script>

<section class="mt-8" aria-labelledby="consents-h">
  <div class="mb-2 flex flex-wrap items-center justify-between gap-3">
    <h3 id="consents-h" class="display text-xl">Consentimientos firmados</h3>
    {#if canWrite}<button type="button" class="btn-secondary" onclick={start}><Icon name="edit" size={18} />Firmar consentimiento</button>{/if}
  </div>
  {#if loading}
    <div class="card h-20 animate-pulse"></div>
  {:else if error}
    <Alert>{error}</Alert>
  {:else if consents.length === 0}
    <div class="card"><EmptyState icon="shield" title="Sin consentimientos" text="Los consentimientos firmados en pantalla se conservan con el texto exacto y la firma." /></div>
  {:else}
    <ul class="space-y-2">
      {#each consents as c (c.id)}
        <li class="card flex flex-wrap items-center justify-between gap-2 p-3 sm:px-5">
          <div class="min-w-0">
            <p class="text-sm font-medium">{CONSENT_LABEL[c.kind] ?? c.kind}</p>
            <p class="truncate text-xs text-app-muted">{dt(c.signed_at)} · firmó {c.signer_name} ({c.signer_role}) · registró {c.registered_by_name}</p>
          </div>
          <button type="button" class="btn-ghost" disabled={busy === c.id} onclick={() => print(c)}><Icon name="receipt" size={16} />Imprimir</button>
        </li>
      {/each}
    </ul>
  {/if}
</section>

<Modal {open} title="Consentimiento informado" onclose={() => (open = false)} wide>
  <div class="space-y-4">
    <div>
      <label class="label" for="c-kind">Tipo</label>
      <select id="c-kind" class="field" value={kind} onchange={(e) => fill(e.currentTarget.value as ConsentKind)}>{#each kinds as k}<option value={k.v}>{k.label}</option>{/each}</select>
    </div>
    <div>
      <label class="label" for="c-text">Texto que se muestra y se firma</label>
      <textarea id="c-text" class="field" rows="9" maxlength="20000" bind:value={text}></textarea>
      <p class="hint">Queda guardado tal cual lo ves aquí. Revísalo con quien firma antes de continuar.</p>
    </div>
    <SignBlock bind:sig {animal} {patientName} />
    <OpError op={op} />
  </div>
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={() => (open = false)}>Cancelar</button>
    <button type="button" class="btn-primary" disabled={op.phase === 'loading'} onclick={submit}>{#if op.phase === 'loading'}<span class="spin"></span>{/if}Firmar y guardar</button>
  {/snippet}
</Modal>
