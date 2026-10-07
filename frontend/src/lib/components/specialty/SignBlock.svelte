<script lang="ts">
  import type { SignatureInput, SignerRole } from '$lib/types/specialty';
  import SignaturePad from './SignaturePad.svelte';

  interface Props {
    animal?: boolean;
    patientName?: string;
    sig: SignatureInput;
  }
  let { animal = false, patientName = '', sig = $bindable() }: Props = $props();

  const roles = $derived<{ v: SignerRole; label: string }[]>(animal ? [{ v: 'propietario', label: 'Propietario' }, { v: 'tutor', label: 'Tutor o responsable' }] : [{ v: 'paciente', label: 'Paciente' }, { v: 'tutor', label: 'Tutor o responsable' }]);
  const uid = $props.id();

  function pickRole(r: SignerRole) {
    sig.signer_role = r;
  }
</script>

<div class="space-y-4">
  <div class="grid gap-4 sm:grid-cols-2">
    <div>
      <label class="label" for="{uid}-role">Firma como</label>
      <select id="{uid}-role" class="field" value={sig.signer_role} onchange={(e) => pickRole(e.currentTarget.value as SignerRole)}>
        {#each roles as r}<option value={r.v}>{r.label}</option>{/each}
      </select>
    </div>
    <div>
      <label class="label" for="{uid}-name">Nombre completo de quien firma</label>
      <input id="{uid}-name" class="field" autocomplete="off" maxlength="150" bind:value={sig.signer_name} placeholder={sig.signer_role === 'paciente' ? patientName : ''} />
    </div>
  </div>
  <SignaturePad bind:value={sig.signature_png} />
  <details>
    <summary class="cursor-pointer text-sm text-app-muted">Agregar testigos (opcional)</summary>
    <div class="mt-2 grid gap-4 sm:grid-cols-2">
      <div><label class="label" for="{uid}-w1">Testigo 1</label><input id="{uid}-w1" class="field" maxlength="150" bind:value={sig.witness1} /></div>
      <div><label class="label" for="{uid}-w2">Testigo 2</label><input id="{uid}-w2" class="field" maxlength="150" bind:value={sig.witness2} /></div>
    </div>
  </details>
</div>
