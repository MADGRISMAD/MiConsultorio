<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import { orgApi } from '$lib/api/org';
  import { Op } from '$lib/op.svelte';
  import { CLINIC_KIND_KEYS, CLINIC_KINDS } from '$lib/types';
  import type { OrgBranch } from '$lib/types/org';
  import Icon from '$lib/components/ui/Icon.svelte';
  import Modal from '$lib/components/Modal.svelte';

  interface Props {
    open: boolean;
    first?: boolean;
    onclose: () => void;
    oncreated: (b: OrgBranch) => void;
  }
  let { open, first = false, onclose, oncreated }: Props = $props();

  let form = $state({ name: '', kind: 'GENERAL_MEDICAL', phone_number: '', address: '' });
  const op = new Op();
  const uid = $props.id();

  $effect(() => {
    if (open) {
      form = { name: '', kind: 'GENERAL_MEDICAL', phone_number: '', address: '' };
      op.reset();
    }
  });

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    let created: OrgBranch | undefined;
    if (await op.run(async () => (created = await orgApi.createBranch(form)))) oncreated(created!);
  }
</script>

<Modal {open} title={first ? 'Crear tu primera sucursal' : 'Nueva sucursal'} {onclose}>
  <form id="{uid}-form" class="grid gap-4" onsubmit={submit}>
    <p class="text-sm text-app-muted">
      Cada sucursal es un consultorio independiente: sus pacientes, agenda, inventario y caja no se mezclan con los de las demás. Tú podrás entrar a todas con tu mismo usuario.
    </p>
    <div>
      <label class="label" for="{uid}-name">Nombre de la sucursal</label>
      <input id="{uid}-name" class="field" bind:value={form.name} required minlength="2" maxlength="120" autocomplete="off" placeholder="Ej. Sucursal Norte" />
    </div>
    <div>
      <label class="label" for="{uid}-kind">Giro</label>
      <select id="{uid}-kind" class="field" bind:value={form.kind}>
        {#each CLINIC_KIND_KEYS as k}<option value={k}>{CLINIC_KINDS[k].label}</option>{/each}
      </select>
    </div>
    <div class="grid gap-4 sm:grid-cols-2">
      <div>
        <label class="label" for="{uid}-phone">Teléfono (opcional)</label>
        <input id="{uid}-phone" class="field" type="tel" bind:value={form.phone_number} maxlength="30" autocomplete="off" />
      </div>
      <div>
        <label class="label" for="{uid}-address">Dirección (opcional)</label>
        <input id="{uid}-address" class="field" bind:value={form.address} maxlength="250" autocomplete="off" />
      </div>
    </div>
    <OpError op={op} />
  </form>
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
    <button type="submit" form="{uid}-form" class="btn-primary" disabled={op.phase === 'loading'}>
      {#if op.phase === 'loading'}<span class="spin"></span>{/if}Crear sucursal
    </button>
  {/snippet}
</Modal>
