<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import { api } from '$lib/api';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { Encounter } from '$lib/types';
  import Modal from '../../Modal.svelte';
  import Icon from '../../ui/Icon.svelte';

  interface Props {
    target: Encounter | null;
    onclose: () => void;
    onsaved: () => void;
  }
  let { target, onclose, onsaved }: Props = $props();
  let reason = $state('');
  let text = $state('');
  const op = new Op();
  $effect(() => {
    if (target) {
      reason = text = '';
      op.reset();
    }
  });
  async function submit(ev: SubmitEvent) {
    ev.preventDefault();
    const t = target;
    if (!t) return;
    if (!reason.trim() || !text.trim()) return op.fail('Indica qué se corrige y escribe el texto de la adenda.');
    if (await op.run(() => api.patients.addendum(t.id, { reason: reason.trim(), text: text.trim() }))) {
      toast.show('Adenda agregada');
      onsaved();
    }
  }
</script>

<Modal open={!!target} title="Agregar adenda" {onclose}>
  <form id="add-form" class="space-y-4" onsubmit={submit}>
    <p class="text-sm text-app-muted">Las notas no se editan ni se borran; si hay un error se agrega una adenda.</p>
    <div>
      <label class="label" for="add-reason">¿Qué se corrige o aclara?</label>
      <input id="add-reason" class="field" bind:value={reason} autocomplete="off" placeholder="Ej. Dosis mal registrada" />
    </div>
    <div>
      <label class="label" for="add-text">Texto de la adenda</label>
      <textarea id="add-text" class="field min-h-32" rows="5" bind:value={text}></textarea>
    </div>
    <OpError op={op} />
  </form>
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
    <button type="submit" form="add-form" class="btn-primary" disabled={op.phase === 'loading'}>{#if op.phase === 'loading'}<span class="spin"></span>{/if}Guardar adenda</button>
  {/snippet}
</Modal>
