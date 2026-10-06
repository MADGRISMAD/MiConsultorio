<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import { PERMISSIONS, type User } from '$lib/types';
  import ConfirmModal from './ConfirmModal.svelte';
  import Modal from './Modal.svelte';
  import Icon from './ui/Icon.svelte';
  import LoadingRows from './ui/LoadingRows.svelte';
  import PageHeader from './ui/PageHeader.svelte';

  const columns = [
    [PERMISSIONS.adminUsers, 'Usuarios'],
    [PERMISSIONS.adminAppointments, 'Admin. citas'],
    [PERMISSIONS.adminHistorials, 'Admin. historiales'],
    [PERMISSIONS.navHistorials, 'Ver historiales'],
    [PERMISSIONS.navAppointments, 'Ver citas']
  ] as const;

  let users = $state<User[]>([]);
  let perms = $state<Record<string, string[]>>({});
  let loading = $state(true);
  let loadError = $state('');

  async function load() {
    try {
      users = await api.users();
      perms = Object.fromEntries(users.map((u) => [u.username, [...u.permissions]]));
      loadError = '';
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'No se pudieron cargar los usuarios.';
    } finally {
      loading = false;
    }
  }
  onMount(load);

  function toggle(username: string, permission: string, on: boolean) {
    const current = perms[username] ?? [];
    perms[username] = on ? [...new Set([...current, permission])] : current.filter((p) => p !== permission);
  }

  const changed = $derived(
    users.filter((u) => {
      const now = perms[u.username] ?? [];
      return now.length !== u.permissions.length || now.some((p) => !u.permissions.includes(p));
    })
  );
  const anyUserAdmin = $derived(Object.values(perms).some((p) => p.includes(PERMISSIONS.adminUsers)));

  // ----- apply permission changes -----
  let applyOpen = $state(false);
  const applyOp = new Op();
  async function apply() {
    if (!anyUserAdmin) {
      applyOp.fail('Por lo menos un usuario debe tener permisos de administración de usuarios.');
      return;
    }
    const payload = Object.fromEntries(changed.map((u) => [u.username, perms[u.username]]));
    if (await applyOp.run(() => api.updatePermissions(payload))) {
      applyOpen = false;
      toast.show('Permisos actualizados');
      await load();
      await session.load(); // our own permissions may have changed
    }
  }

  // ----- delete -----
  let deleting = $state<string | null>(null);
  const deleteOp = new Op();
  async function confirmDelete() {
    const target = deleting;
    if (!target) return;
    if (target === session.user?.username) return deleteOp.fail('No puedes eliminar tu propio usuario.');
    if (perms[target]?.includes(PERMISSIONS.adminUsers)) return deleteOp.fail('No puedes eliminar un usuario con permisos de administración de usuarios. Quítaselos primero.');
    if (await deleteOp.run(() => api.deleteUser(target))) {
      deleting = null;
      toast.show('Usuario eliminado');
      await load();
    }
  }

  // ----- create -----
  let createOpen = $state(false);
  let newUser = $state({ username: '', password: '', permissions: [] as string[] });
  const createOp = new Op();
  function openCreate() {
    newUser = { username: '', password: '', permissions: [] };
    createOp.reset();
    createOpen = true;
  }
  async function submitCreate(e: SubmitEvent) {
    e.preventDefault();
    if (await createOp.run(() => api.createUser(newUser))) {
      createOpen = false;
      toast.show('Usuario creado');
      await load();
    }
  }

  // ----- edit password -----
  let editing = $state<string | null>(null);
  let newPassword = $state('');
  const editOp = new Op();
  async function submitEdit(e: SubmitEvent) {
    e.preventDefault();
    const target = editing;
    if (!target) return;
    if (await editOp.run(() => api.updateUserPassword(target, newPassword))) {
      editing = null;
      toast.show('Contraseña actualizada');
    }
  }
</script>

<PageHeader title="Usuarios y permisos" subtitle="Decide qué puede ver y hacer cada persona de tu equipo.">
  {#snippet actions()}
    <button type="button" class="btn-primary" onclick={openCreate}><Icon name="plus" size={18} stroke={2.2} />Agregar usuario</button>
  {/snippet}
</PageHeader>

<div class="card overflow-hidden">
  {#if loading}
    <LoadingRows />
  {:else if loadError}
    <p class="alert m-5" role="alert"><Icon name="alert" size={18} />{loadError}</p>
  {:else}
    <div class="overflow-x-auto">
      <table class="w-full min-w-[46rem]">
        <thead class="border-b border-app-ink/10 bg-app-elevated">
          <tr>
            <th class="th">Usuario</th>
            {#each columns as [, label]}<th class="th text-center">{label}</th>{/each}
            <th class="th"><span class="sr-only">Acciones</span></th>
          </tr>
        </thead>
        <tbody class="divide-y divide-app-ink/8">
          {#each users as u (u.username)}
            <tr class="transition hover:bg-app-ink/[0.03]">
              <td class="td">
                <span class="flex items-center gap-3">
                  <span class="grid h-9 w-9 flex-none place-items-center rounded-full bg-app-primary/15 text-sm font-semibold text-app-primary">{u.username.slice(0, 1).toUpperCase()}</span>
                  <span class="font-semibold">{u.username}</span>
                  {#if u.username === session.user?.username}<span class="badge">Tú</span>{/if}
                </span>
              </td>
              {#each columns as [perm, label]}
                <td class="td text-center">
                  <input
                    type="checkbox"
                    class="h-[1.15rem] w-[1.15rem] cursor-pointer accent-[rgb(var(--app-primary))]"
                    aria-label="{label} de {u.username}"
                    checked={perms[u.username]?.includes(perm)}
                    onchange={(e) => toggle(u.username, perm, e.currentTarget.checked)}
                  />
                </td>
              {/each}
              <td class="td">
                <div class="flex justify-end gap-1">
                  <button type="button" class="icon-btn" title="Cambiar contraseña" aria-label="Cambiar contraseña de {u.username}" onclick={() => { newPassword = ''; editOp.reset(); editing = u.username; }}><Icon name="lock" size={19} /></button>
                  <button type="button" class="icon-btn danger" title="Eliminar" aria-label="Eliminar a {u.username}" onclick={() => { deleteOp.reset(); deleting = u.username; }}><Icon name="trash" size={19} /></button>
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
    <div class="flex flex-wrap items-center justify-between gap-3 border-t border-app-ink/10 bg-app-elevated px-4 py-3">
      <span class="text-sm text-app-muted">
        {#if changed.length > 0}{changed.length} usuario(s) con cambios sin guardar{:else}Marca o desmarca permisos y guarda los cambios.{/if}
      </span>
      <button type="button" class="btn-primary" disabled={changed.length === 0} onclick={() => { applyOp.reset(); applyOpen = true; }}>Aplicar cambios</button>
    </div>
  {/if}
</div>

<ConfirmModal open={applyOpen} title="¿Aplicar los cambios?" confirmLabel="Aplicar" op={applyOp} onconfirm={apply} onclose={() => (applyOpen = false)}>
  <p>Se actualizarán los permisos de {changed.length} usuario(s). Los cambios surten efecto de inmediato.</p>
</ConfirmModal>

<ConfirmModal open={deleting !== null} title="¿Eliminar a {deleting}?" confirmLabel="Eliminar" op={deleteOp} onconfirm={confirmDelete} onclose={() => (deleting = null)}>
  <p>Se eliminará el usuario y perderá el acceso de inmediato.</p>
</ConfirmModal>

<Modal open={createOpen} title="Agregar usuario" onclose={() => (createOpen = false)}>
  <form id="create-user-form" class="space-y-4 text-left" onsubmit={submitCreate}>
    <div>
      <label class="label" for="nu-name">Nombre de usuario</label>
      <input id="nu-name" class="field" bind:value={newUser.username} required maxlength="64" autocomplete="off" />
    </div>
    <div>
      <label class="label" for="nu-pass">Contraseña</label>
      <input id="nu-pass" class="field" type="password" bind:value={newUser.password} required minlength="8" maxlength="72" autocomplete="new-password" />
      <p class="hint">Mínimo 8 caracteres.</p>
    </div>
    <fieldset>
      <legend class="label">Permisos</legend>
      <div class="grid gap-2 sm:grid-cols-2">
        {#each columns as [perm, label]}
          <label class="flex cursor-pointer items-center gap-2.5 rounded-lg bg-app-ink/5 px-3 py-2 text-sm font-semibold">
            <input type="checkbox" class="h-4 w-4 accent-[rgb(var(--app-primary))]" bind:group={newUser.permissions} value={perm} />
            {label}
          </label>
        {/each}
      </div>
    </fieldset>
  </form>
  {#if createOp.phase === 'error'}
    <p class="alert mt-4" role="alert"><Icon name="alert" size={18} />{createOp.message}</p>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={() => (createOpen = false)}>Cancelar</button>
    <button type="submit" form="create-user-form" class="btn-primary" disabled={createOp.phase === 'loading'}>
      {#if createOp.phase === 'loading'}<span class="spin"></span>{/if}Crear usuario
    </button>
  {/snippet}
</Modal>

<Modal open={editing !== null} title="Cambiar contraseña de {editing}" onclose={() => (editing = null)}>
  <form id="edit-user-form" class="text-left" onsubmit={submitEdit}>
    <label class="label" for="eu-pass">Nueva contraseña</label>
    <input id="eu-pass" class="field" type="password" bind:value={newPassword} required minlength="8" maxlength="72" autocomplete="new-password" />
    <p class="hint">Mínimo 8 caracteres.</p>
  </form>
  {#if editOp.phase === 'error'}
    <p class="alert mt-4" role="alert"><Icon name="alert" size={18} />{editOp.message}</p>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={() => (editing = null)}>Cancelar</button>
    <button type="submit" form="edit-user-form" class="btn-primary" disabled={editOp.phase === 'loading'}>
      {#if editOp.phase === 'loading'}<span class="spin"></span>{/if}Guardar
    </button>
  {/snippet}
</Modal>
