<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import { emptyExpedient, type Expedient, type ExpedientInput } from '$lib/types';
  import ConfirmModal from './ConfirmModal.svelte';
  import ExpedientForm from './ExpedientForm.svelte';
  import ExpedientView from './ExpedientView.svelte';
  import Modal from './Modal.svelte';
  import SearchBar from './SearchBar.svelte';
  import EmptyState from './ui/EmptyState.svelte';
  import Icon from './ui/Icon.svelte';
  import LoadingRows from './ui/LoadingRows.svelte';
  import PageHeader from './ui/PageHeader.svelte';

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
    if (!(await formOp.run(() => (curp ? api.updateExpedient(curp, form) : api.createExpedient(form))))) return;
    formOpen = false;
    toast.show(curp ? 'Expediente actualizado' : 'Expediente creado');
    await clearSearch();
  }

  // ----- delete -----
  let deleting = $state<Expedient | null>(null);
  const deleteOp = new Op();
  async function confirmDelete() {
    const target = deleting;
    if (!target) return;
    if (await deleteOp.run(() => api.deleteExpedient(target.CURP))) {
      deleting = null;
      toast.show('Expediente eliminado');
      await clearSearch();
    }
  }

  const initials = (e: Expedient) => (e.names[0] + (e.last_names[0] ?? '')).toUpperCase();
</script>

<PageHeader title={admin ? 'Administrar historiales' : 'Historiales'} subtitle={admin ? 'Crea, edita y elimina expedientes clínicos.' : 'Consulta los expedientes de tus pacientes.'}>
  {#snippet actions()}
    {#if admin}<button type="button" class="btn-primary" onclick={openCreate}><Icon name="plus" size={18} stroke={2.2} />Nuevo expediente</button>{/if}
  {/snippet}
</PageHeader>

<div class="mb-4"><SearchBar onsearch={searchByCurp} onclear={clearSearch} /></div>

<div class="card overflow-hidden">
  {#if search === 'loading' || loading}
    <LoadingRows />
  {:else if search === 'notFound'}
    <EmptyState icon="search" title="No hay un expediente con esa CURP" text="Revisa que esté escrita completa (18 caracteres).">
      <button type="button" class="btn-secondary" onclick={clearSearch}>Ver todos</button>
    </EmptyState>
  {:else if loadError}
    <p class="alert m-5" role="alert"><Icon name="alert" size={18} />{loadError}</p>
  {:else if items.length === 0}
    <EmptyState icon="folder" title="Aún no hay expedientes" text={admin ? 'Crea el primer expediente clínico.' : 'Cuando se registren pacientes aparecerán aquí.'}>
      {#if admin}<button type="button" class="btn-primary" onclick={openCreate}><Icon name="plus" size={18} stroke={2.2} />Nuevo expediente</button>{/if}
    </EmptyState>
  {:else}
    <div class="overflow-x-auto">
      <table class="w-full min-w-[40rem]">
        <thead class="border-b border-app-ink/10 bg-app-elevated">
          <tr>
            <th class="th">Paciente</th>
            <th class="th">CURP</th>
            <th class="th">Nacimiento</th>
            <th class="th">Sexo</th>
            <th class="th"><span class="sr-only">Acciones</span></th>
          </tr>
        </thead>
        <tbody class="divide-y divide-app-ink/8">
          {#each items as e (e.id)}
            <tr class="transition hover:bg-app-ink/[0.03]">
              <td class="td">
                <span class="flex items-center gap-3">
                  <span class="grid h-9 w-9 flex-none place-items-center rounded-full bg-app-primary/15 text-xs font-semibold text-app-primary">{initials(e)}</span>
                  <span class="font-semibold">{e.names} {e.last_names}</span>
                </span>
              </td>
              <td class="td whitespace-nowrap font-mono text-xs">{e.CURP}</td>
              <td class="td whitespace-nowrap">{e.date_of_birth} <span class="text-app-muted">· {e.age} años</span></td>
              <td class="td">{e.sex}</td>
              <td class="td">
                <div class="flex justify-end gap-1">
                  <button type="button" class="icon-btn" title="Ver expediente" aria-label="Ver expediente de {e.names} {e.last_names}" onclick={() => (viewing = e)}><Icon name="eye" size={19} /></button>
                  {#if admin}
                    <button type="button" class="icon-btn" title="Editar" aria-label="Editar expediente de {e.names} {e.last_names}" onclick={() => openEdit(e)}><Icon name="edit" size={19} /></button>
                    <button type="button" class="icon-btn danger" title="Eliminar" aria-label="Eliminar expediente de {e.names} {e.last_names}" onclick={() => { deleteOp.reset(); deleting = e; }}><Icon name="trash" size={19} /></button>
                  {/if}
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<Modal open={viewing !== null} title="Historial médico" wide onclose={() => (viewing = null)}>
  {#if viewing}<ExpedientView expedient={viewing} />{/if}
  {#snippet footer()}
    {#if viewing}<a class="btn-secondary" href="/admin/navegar-historiales/{encodeURIComponent(viewing.CURP)}">Abrir en su página</a>{/if}
    <button type="button" class="btn-primary" onclick={() => (viewing = null)}>Cerrar</button>
  {/snippet}
</Modal>

<Modal open={formOpen} title={editingCurp ? 'Editar expediente' : 'Nuevo expediente'} wide onclose={() => (formOpen = false)}>
  <form id="expedient-form" onsubmit={submitForm}>
    <ExpedientForm bind:data={form} />
  </form>
  {#if formOp.phase === 'error'}
    <p class="alert mt-4" role="alert"><Icon name="alert" size={18} />{formOp.message}</p>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-ghost mr-auto" onclick={() => (form = emptyExpedient())}>Limpiar</button>
    <button type="button" class="btn-secondary" onclick={() => (formOpen = false)}>Cancelar</button>
    <button type="submit" form="expedient-form" class="btn-primary" disabled={formOp.phase === 'loading'}>
      {#if formOp.phase === 'loading'}<span class="spin"></span>{/if}Guardar
    </button>
  {/snippet}
</Modal>

<ConfirmModal open={deleting !== null} title="¿Eliminar este expediente?" confirmLabel="Eliminar" op={deleteOp} onconfirm={confirmDelete} onclose={() => (deleting = null)}>
  <p>Se eliminará todo el historial de <strong class="text-app-ink">{deleting?.names} {deleting?.last_names}</strong> ({deleting?.CURP}). Esta acción no se puede deshacer.</p>
</ConfirmModal>
