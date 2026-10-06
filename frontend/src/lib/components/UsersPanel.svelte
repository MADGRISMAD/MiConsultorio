<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import { PERMISSIONS, type User } from '$lib/types';
  import ConfirmModal from './ConfirmModal.svelte';
  import Modal from './Modal.svelte';
  import Notice from './Notice.svelte';

  const columns = [
    [PERMISSIONS.adminUsers, 'Admin. Usuarios'],
    [PERMISSIONS.adminAppointments, 'Admin. Citas'],
    [PERMISSIONS.adminHistorials, 'Admin. Historiales'],
    [PERMISSIONS.navHistorials, 'Nav. Historiales'],
    [PERMISSIONS.navAppointments, 'Nav. Citas']
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
      applyOp.fail('Error, por lo menos un usuario debe tener permisos de admin. de usuarios.');
      return;
    }
    const payload = Object.fromEntries(changed.map((u) => [u.username, perms[u.username]]));
    if (await applyOp.run(() => api.updatePermissions(payload), 'Cambios realizados exitosamente')) {
      await load();
      await session.load(); // our own permissions may have changed
      setTimeout(() => {
        applyOpen = false;
        applyOp.reset();
      }, 1200);
    }
  }

  // ----- delete -----
  let deleting = $state<string | null>(null);
  const deleteOp = new Op();
  async function confirmDelete() {
    const target = deleting;
    if (!target) return;
    if (target === session.user?.username) return deleteOp.fail('Error, no puedes eliminar tu propio usuario.');
    if (perms[target]?.includes(PERMISSIONS.adminUsers)) return deleteOp.fail('Error, no puedes eliminar un usuario con permisos de admin. de usuarios.');
    if (await deleteOp.run(() => api.deleteUser(target))) {
      deleting = null;
      deleteOp.reset();
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
    if (await createOp.run(() => api.createUser(newUser), 'Usuario creado exitosamente')) {
      await load();
      setTimeout(() => {
        createOpen = false;
        createOp.reset();
      }, 800);
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
    if (await editOp.run(() => api.updateUserPassword(target, newPassword), 'Usuario modificado exitosamente')) {
      setTimeout(() => {
        editing = null;
        editOp.reset();
      }, 800);
    }
  }
</script>

<div class="mx-auto max-w-6xl px-4 py-8">
  <div class="flex items-center justify-between gap-4">
    <h1 class="font-display text-4xl">Administrar usuarios</h1>
    <button type="button" class="btn-primary" onclick={openCreate}>Agregar usuario</button>
  </div>

  <div class="mt-6 overflow-x-auto rounded-2xl bg-white shadow-sm ring-1 ring-ink/10">
    {#if loading}
      <p class="p-8 text-center text-ink-soft">Cargando...</p>
    {:else if loadError}
      <p class="p-8 text-center text-red-600" role="alert">{loadError}</p>
    {:else}
      <table class="w-full min-w-max">
        <thead class="border-b border-ink/10 bg-paper">
          <tr>
            <th class="th">Nombre de usuario</th>
            {#each columns as [, label]}<th class="th text-center">{label}</th>{/each}
            <th class="th"><span class="sr-only">Acciones</span></th>
          </tr>
        </thead>
        <tbody class="divide-y divide-ink/5">
          {#each users as u (u.username)}
            <tr>
              <td class="td">
                <span class="flex items-center gap-3">
                  <img class="h-10 w-10 rounded-full" src="/undefined_user.png" alt="" />
                  {u.username}
                </span>
              </td>
              {#each columns as [perm, label]}
                <td class="td text-center">
                  <input
                    type="checkbox"
                    class="h-4 w-4 rounded border-ink/30 text-signal focus:ring-signal"
                    aria-label="{label} de {u.username}"
                    checked={perms[u.username]?.includes(perm)}
                    onchange={(e) => toggle(u.username, perm, e.currentTarget.checked)}
                  />
                </td>
              {/each}
              <td class="td">
                <div class="flex justify-end gap-2">
                  <button type="button" class="icon-btn" title="Eliminar" aria-label="Eliminar a {u.username}" onclick={() => { deleteOp.reset(); deleting = u.username; }}><img src="/delete.png" alt="" class="h-5 w-5" /></button>
                  <button type="button" class="icon-btn" title="Cambiar contraseña" aria-label="Cambiar contraseña de {u.username}" onclick={() => { newPassword = ''; editOp.reset(); editing = u.username; }}><img src="/edit.png" alt="" class="h-5 w-5" /></button>
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>

  <button type="button" class="btn-primary mt-4" disabled={changed.length === 0} onclick={() => { applyOp.reset(); applyOpen = true; }}>Aplicar cambios</button>
  {#if changed.length > 0}<span class="ml-3 text-sm text-ink-soft">{changed.length} usuario(s) con cambios sin guardar</span>{/if}
</div>

<ConfirmModal open={applyOpen} title="¿Seguro que quieres aplicar los cambios?" op={applyOp} confirmLabel="Sí" onconfirm={apply} onclose={() => (applyOpen = false)}>
  <p class="text-ink-soft">Esto actualizará los permisos de los usuarios.</p>
</ConfirmModal>

<ConfirmModal open={deleting !== null} title="¿Seguro que quieres eliminar al usuario {deleting}?" op={deleteOp} onconfirm={confirmDelete} onclose={() => (deleting = null)}>
  <p class="text-ink-soft">Esto eliminará toda la información del usuario.</p>
</ConfirmModal>

<Modal open={createOpen} title="Crear usuario" onclose={() => (createOpen = false)}>
  <form id="create-user-form" class="space-y-3 text-left" onsubmit={submitCreate}>
    <label class="block">
      <span class="mb-1 block text-xs font-semibold text-ink-soft">Nombre de usuario *</span>
      <input class="field" bind:value={newUser.username} required maxlength="64" autocomplete="off" />
    </label>
    <label class="block">
      <span class="mb-1 block text-xs font-semibold text-ink-soft">Contraseña * (mínimo 8 caracteres)</span>
      <input class="field" type="password" bind:value={newUser.password} required minlength="8" maxlength="72" autocomplete="new-password" />
    </label>
    <fieldset>
      <legend class="mb-1 text-xs font-semibold text-ink-soft">Permisos</legend>
      <div class="grid gap-2 sm:grid-cols-2">
        {#each columns as [perm, label]}
          <label class="flex items-center gap-2 text-sm">
            <input type="checkbox" class="h-4 w-4 rounded border-ink/30 text-signal focus:ring-signal" bind:group={newUser.permissions} value={perm} />
            {label}
          </label>
        {/each}
      </div>
    </fieldset>
  </form>
  {#if createOp.phase !== 'idle'}
    <div class="mt-4"><Notice kind={createOp.phase} message={createOp.phase === 'loading' ? 'Cargando...' : createOp.message} /></div>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={() => (createOpen = false)}>Cancelar</button>
    <button type="button" class="btn-secondary" onclick={() => (newUser = { username: '', password: '', permissions: [] })}>Limpiar</button>
    <button type="submit" form="create-user-form" class="btn-primary" disabled={createOp.phase === 'loading'}>Aceptar</button>
  {/snippet}
</Modal>

<Modal open={editing !== null} title="Editar usuario {editing}" onclose={() => (editing = null)}>
  <form id="edit-user-form" class="text-left" onsubmit={submitEdit}>
    <label class="block">
      <span class="mb-1 block text-xs font-semibold text-ink-soft">Nueva contraseña * (mínimo 8 caracteres)</span>
      <input class="field" type="password" bind:value={newPassword} required minlength="8" maxlength="72" autocomplete="new-password" />
    </label>
  </form>
  {#if editOp.phase !== 'idle'}
    <div class="mt-4"><Notice kind={editOp.phase} message={editOp.phase === 'loading' ? 'Cargando...' : editOp.message} /></div>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={() => (editing = null)}>Cancelar</button>
    <button type="submit" form="edit-user-form" class="btn-primary" disabled={editOp.phase === 'loading'}>Aceptar</button>
  {/snippet}
</Modal>
