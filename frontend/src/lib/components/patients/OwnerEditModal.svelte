<script lang="ts">
  import { ownersApi } from '$lib/api/owners';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { OwnerListItem, OwnerRef } from '$lib/types/owners';
  import Modal from '../Modal.svelte';
  import Icon from '../ui/Icon.svelte';
  import OwnerPicker from './OwnerPicker.svelte';

  interface Props {
    owner: OwnerRef | null;
    onclose: () => void;
    /** saved or merged: reload the list */
    onchanged: () => void;
  }
  let { owner, onclose, onchanged }: Props = $props();

  let name = $state('');
  let phone = $state('');
  let email = $state('');
  let merging = $state(false);
  let target = $state<OwnerListItem | null>(null);
  const op = new Op();
  const mergeOp = new Op();

  $effect(() => {
    if (!owner) return;
    name = owner.name;
    phone = owner.phone;
    email = owner.email;
    merging = false;
    target = null;
    op.reset();
    mergeOp.reset();
  });

  async function save(e: SubmitEvent) {
    e.preventDefault();
    const o = owner;
    if (!o) return;
    if (!name.trim()) return op.fail('Escribe el nombre del propietario.');
    if (email.trim() && !/^\S+@\S+\.\S+$/.test(email.trim())) return op.fail('Revisa el correo electrónico.');
    if (await op.run(async () => void (await ownersApi.update(o.id, { name: name.trim(), phone: phone.trim(), email: email.trim() })))) {
      toast.show('Datos del propietario actualizados en todas sus mascotas');
      onchanged();
      onclose();
    }
  }

  async function merge() {
    const o = owner;
    const t = target;
    if (!o || !t) return;
    let moved = 0;
    if (await mergeOp.run(async () => void (moved = (await ownersApi.merge(o.id, t.id)).moved))) {
      toast.show(`Se unieron los propietarios: ${moved === 1 ? '1 mascota pasó' : `${moved} mascotas pasaron`} a ${t.name}`);
      onchanged();
      onclose();
    }
  }
</script>

<Modal open={!!owner} title="Datos del propietario" {onclose}>
  <form id="owner-form" onsubmit={save} class="grid gap-4">
    <p class="text-sm text-app-muted">Lo que cambies aquí se actualiza en <strong class="text-app-ink">todas sus mascotas</strong> (expedientes, recetas y recordatorios).</p>
    <div>
      <label class="label" for="own-name">Nombre</label>
      <input id="own-name" class="field" bind:value={name} maxlength="160" autocomplete="off" />
    </div>
    <div class="grid gap-4 sm:grid-cols-2">
      <div>
        <label class="label" for="own-phone">Teléfono</label>
        <input id="own-phone" class="field" type="tel" bind:value={phone} maxlength="30" autocomplete="off" />
      </div>
      <div>
        <label class="label" for="own-email">Correo electrónico</label>
        <input id="own-email" class="field" type="email" bind:value={email} maxlength="160" autocomplete="off" />
      </div>
    </div>
    {#if op.phase === 'error'}<p class="alert" role="alert"><Icon name="alert" size={18} />{op.message}</p>{/if}
  </form>

  <div class="mt-6 border-t border-app-ink/10 pt-4">
    {#if !merging}
      <button type="button" class="btn-ghost -ml-3 text-sm" onclick={() => (merging = true)}>¿Este propietario está registrado dos veces? Unirlo con otro</button>
    {:else}
      <div class="grid gap-3 rounded-xl bg-app-elevated p-4">
        <p class="text-sm">Elige el registro con el que se queda. Todas las mascotas de <strong>{owner?.name}</strong> pasarán a ese propietario y este registro desaparece.</p>
        <OwnerPicker selected={target} onpick={(o) => (target = o)} onclear={() => (target = null)} id="own-merge-search" />
        {#if mergeOp.phase === 'error'}<p class="alert" role="alert"><Icon name="alert" size={18} />{mergeOp.message}</p>{/if}
        <div class="flex flex-wrap justify-end gap-2">
          <button type="button" class="btn-secondary" onclick={() => (merging = false)}>No unir</button>
          <button type="button" class="btn-danger" disabled={!target || target.id === owner?.id || mergeOp.phase === 'loading'} onclick={merge}>Unir propietarios</button>
        </div>
      </div>
    {/if}
  </div>

  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
    <button type="submit" form="owner-form" class="btn-primary" disabled={op.phase === 'loading'}>{#if op.phase === 'loading'}<span class="spin"></span>{/if}Guardar</button>
  {/snippet}
</Modal>
