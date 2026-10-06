<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { Op } from '$lib/op.svelte';
  import { emptyExpedient, type Expedient, type ExpedientInput } from '$lib/types';
  import ConfirmModal from './ConfirmModal.svelte';
  import ExpedientForm from './ExpedientForm.svelte';
  import ExpedientView from './ExpedientView.svelte';
  import Modal from './Modal.svelte';
  import Notice from './Notice.svelte';
  import SearchBar from './SearchBar.svelte';

  let { admin = false }: { admin?: boolean } = $props();

  let items = $state<Expedient[]>([]);
  let loading = $state(true);
  let loadError = $state('');
  let search = $state<'none' | 'loading' | 'notFound'>('none');

  async function load() {
    try {
      items = await api.expedients();
      loadError = '';
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'No se pudieron cargar los expedientes.';
    } finally {
      loading = false;
    }
  }
  onMount(load);

  async function searchByCurp(text: string) {
    search = 'loading';
    try {
      items = [await api.expedient(text)];
      search = 'none';
    } catch {
      search = 'notFound';
    }
  }
  async function clearSearch() {
    search = 'none';
    await load();
  }

  let viewing = $state<Expedient | null>(null);

  // ----- create / edit -----
  let formOpen = $state(false);
  let editingCurp = $state<string | null>(null);
  let form = $state<ExpedientInput>(emptyExpedient());
  const formOp = new Op();

  function openCreate() {
    editingCurp = null;
    form = emptyExpedient();
    formOp.reset();
    formOpen = true;
  }
  function openEdit(e: Expedient) {
    editingCurp = e.CURP;
    const { id: _id, age: _age, ...rest } = e;
    form = { ...rest };
    formOp.reset();
    formOpen = true;
  }
  async function submitForm(ev: SubmitEvent) {
    ev.preventDefault();
    const curp = editingCurp;
    const ok = await formOp.run(
      () => (curp ? api.updateExpedient(curp, form) : api.createExpedient(form)),
      curp ? 'Expediente actualizado exitosamente' : 'Expediente creado exitosamente'
    );
    if (!ok) return;
    await clearSearch();
    setTimeout(() => {
      formOpen = false;
      formOp.reset();
    }, 800);
  }

  // ----- delete -----
  let deleting = $state<Expedient | null>(null);
  const deleteOp = new Op();
  async function confirmDelete() {
    const target = deleting;
    if (!target) return;
    if (await deleteOp.run(() => api.deleteExpedient(target.CURP))) {
      deleting = null;
      deleteOp.reset();
      await clearSearch();
    }
  }
</script>

<div class="mx-auto max-w-6xl px-4 py-8">
  <div class="flex items-center justify-between gap-4">
    <h1 class="font-display text-4xl">{admin ? 'Administrar historiales' : 'Historiales'}</h1>
    {#if admin}<button type="button" class="btn-primary" onclick={openCreate}>Nuevo expediente</button>{/if}
  </div>

  <div class="py-6"><SearchBar onsearch={searchByCurp} onclear={clearSearch} /></div>

  {#if search === 'loading'}
    <div class="mx-auto max-w-md py-8 text-center"><Notice kind="loading" message="Cargando..." /></div>
  {:else if search === 'notFound'}
    <div class="mx-auto max-w-md py-8 text-center">
      <img class="mx-auto w-20" src="/notFound.png" alt="" />
      <p class="py-4">No hay un expediente con esa CURP</p>
      <button type="button" class="btn-secondary" onclick={clearSearch}>Ver todos</button>
    </div>
  {:else}
    <div class="overflow-x-auto rounded-2xl bg-white shadow-sm ring-1 ring-ink/10">
      {#if loading}
        <p class="p-8 text-center text-ink-soft">Cargando...</p>
      {:else if loadError}
        <p class="p-8 text-center text-red-600" role="alert">{loadError}</p>
      {:else if items.length === 0}
        <p class="p-8 text-center text-ink-soft">No hay expedientes registrados.</p>
      {:else}
        <table class="w-full min-w-max">
          <thead class="border-b border-ink/10 bg-paper">
            <tr>
              <th class="th">Nombre completo</th>
              <th class="th">CURP</th>
              <th class="th">Fecha de nacimiento</th>
              <th class="th">Sexo</th>
              <th class="th"><span class="sr-only">Acciones</span></th>
            </tr>
          </thead>
          <tbody class="divide-y divide-ink/5">
            {#each items as e (e.id)}
              <tr>
                <td class="td whitespace-nowrap">{e.names} {e.last_names}</td>
                <td class="td whitespace-nowrap font-mono text-xs">{e.CURP}</td>
                <td class="td whitespace-nowrap">{e.date_of_birth}</td>
                <td class="td whitespace-nowrap">{e.sex}</td>
                <td class="td">
                  <div class="flex justify-end gap-2">
                    <button type="button" class="icon-btn" title="Ver expediente" aria-label="Ver expediente de {e.names} {e.last_names}" onclick={() => (viewing = e)}><img src="/watch.png" alt="" class="h-5 w-5" /></button>
                    {#if admin}
                      <button type="button" class="icon-btn" title="Eliminar" aria-label="Eliminar expediente de {e.names} {e.last_names}" onclick={() => { deleteOp.reset(); deleting = e; }}><img src="/delete.png" alt="" class="h-5 w-5" /></button>
                      <button type="button" class="icon-btn" title="Editar" aria-label="Editar expediente de {e.names} {e.last_names}" onclick={() => openEdit(e)}><img src="/edit.png" alt="" class="h-5 w-5" /></button>
                    {/if}
                  </div>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </div>
  {/if}
</div>

<Modal open={viewing !== null} title="Historial médico" wide onclose={() => (viewing = null)}>
  {#if viewing}<ExpedientView expedient={viewing} />{/if}
  {#snippet footer()}
    {#if viewing}<a class="btn-secondary" href="/admin/navegar-historiales/{encodeURIComponent(viewing.CURP)}">Abrir en su página</a>{/if}
    <button type="button" class="btn-secondary" onclick={() => (viewing = null)}>Cerrar</button>
  {/snippet}
</Modal>

<Modal open={formOpen} title={editingCurp ? 'Editar expediente' : 'Nuevo expediente'} wide onclose={() => (formOpen = false)}>
  <form id="expedient-form" onsubmit={submitForm}>
    <ExpedientForm bind:data={form} />
  </form>
  {#if formOp.phase !== 'idle'}
    <div class="mt-4"><Notice kind={formOp.phase} message={formOp.phase === 'loading' ? 'Cargando...' : formOp.message} /></div>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={() => (formOpen = false)}>Cancelar</button>
    <button type="button" class="btn-secondary" onclick={() => (form = emptyExpedient())}>Limpiar</button>
    <button type="submit" form="expedient-form" class="btn-primary" disabled={formOp.phase === 'loading'}>Aceptar</button>
  {/snippet}
</Modal>

<ConfirmModal
  open={deleting !== null}
  title="¿Seguro que quieres eliminar este expediente?"
  op={deleteOp}
  onconfirm={confirmDelete}
  onclose={() => (deleting = null)}
>
  <p class="text-ink-soft">Esto eliminará todo el historial de {deleting?.names} {deleting?.last_names} ({deleting?.CURP}).</p>
</ConfirmModal>
