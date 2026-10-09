
<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import { arcoApi } from '$lib/api/arco';
  import { Op } from '$lib/op.svelte';
  import type { ArcoNewInput, ArcoRequest } from '$lib/types/arco';
  import Modal from '$lib/components/Modal.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { KINDS, KIND_LABEL } from './labels';

  let { open, onclose, oncreated }: { open: boolean; onclose: () => void; oncreated: (r: ArcoRequest) => void } = $props();

  const today = () => new Date().toLocaleDateString('en-CA');
  const blank = (): ArcoNewInput => ({ kind: 'acceso', requester_name: '', requester_email: '', requester_phone: '', description: '', patient_id: '', received_on: today() });
  let form = $state<ArcoNewInput>(blank());
  const op = new Op();

  $effect(() => {
    if (open) {
      form = blank();
      op.reset();
    }
  });

  async function save(e: SubmitEvent) {
    e.preventDefault();
    if (!form.requester_email.trim() && !form.requester_phone.trim()) {
      op.fail('Registra un correo o un teléfono para comunicar la respuesta.');
      return;
    }
    let created: ArcoRequest | undefined;
    if (await op.run(async () => (created = await arcoApi.create($state.snapshot(form) as ArcoNewInput)))) {
      if (created) oncreated(created);
    }
  }
</script>

<Modal {open} title="Registrar solicitud ARCO" {onclose}>
  <form id="arco-new-form" class="grid gap-4 sm:grid-cols-2" onsubmit={save}>
    <p class="sm:col-span-2 text-sm text-app-muted">Para solicitudes que llegaron en persona, por escrito o por teléfono. Los plazos corren desde la fecha en que se recibió.</p>
    <div>
      <label class="label" for="an-kind">Tipo de solicitud</label>
      <select id="an-kind" class="field" bind:value={form.kind}>
        {#each KINDS as k (k)}<option value={k}>{KIND_LABEL[k]}</option>{/each}
      </select>
    </div>
    <div>
      <label class="label" for="an-date">Fecha de recepción</label>
      <input id="an-date" class="field" type="date" max={today()} bind:value={form.received_on} required />
    </div>
    <div class="sm:col-span-2">
      <label class="label" for="an-name">Nombre de quien solicita</label>
      <input id="an-name" class="field" maxlength="120" bind:value={form.requester_name} required minlength="3" autocomplete="off" />
    </div>
    <div>
      <label class="label" for="an-email">Correo</label>
      <input id="an-email" class="field" type="email" bind:value={form.requester_email} autocomplete="off" />
    </div>
    <div>
      <label class="label" for="an-phone">Teléfono</label>
      <input id="an-phone" class="field" type="tel" bind:value={form.requester_phone} autocomplete="off" />
    </div>
    <div class="sm:col-span-2">
      <label class="label" for="an-desc">Qué solicita</label>
      <textarea id="an-desc" class="field" rows="4" maxlength="3000" bind:value={form.description} required minlength="5"></textarea>
    </div>
    <OpError op={op} class="sm:col-span-2" />
  </form>
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
    <button type="submit" form="arco-new-form" class="btn-primary" disabled={op.phase === 'loading'}>
      {#if op.phase === 'loading'}<span class="spin"></span>{/if}Registrar
    </button>
  {/snippet}
</Modal>
