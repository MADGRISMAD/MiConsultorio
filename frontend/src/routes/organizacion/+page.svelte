<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onMount } from 'svelte';
  import { enterBranch, orgApi } from '$lib/api/org';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import { CLINIC_KINDS } from '$lib/types';
  import type { OrgBranch, OrgOverview } from '$lib/types/org';
  import ConfirmModal from '$lib/components/ConfirmModal.svelte';
  import Guard from '$lib/components/Guard.svelte';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import PageHeader from '$lib/components/ui/PageHeader.svelte';
  import Pill from '$lib/components/ui/Pill.svelte';
  import NewBranchModal from '$lib/components/org/NewBranchModal.svelte';
  import OrgReports from '$lib/components/org/OrgReports.svelte';

  let ov = $state<OrgOverview | null>(null);
  let branch = $state('');
  let newOpen = $state(false);
  let toggle = $state<{ b: OrgBranch; off: boolean } | null>(null);
  const load = new Op();
  const act = new Op();
  const toggleOp = new Op();

  const full = $derived(!!ov && (ov.active_branches ?? 0) >= ov.branch_limit);

  async function refresh() {
    await load.run(async () => (ov = await orgApi.overview()));
  }

  async function enter(b: OrgBranch) {
    await act.run(() => enterBranch(b.id));
  }

  async function confirmToggle() {
    const t = toggle;
    if (!t) return;
    if (await toggleOp.run(() => (t.off ? orgApi.suspend(t.b.id) : orgApi.reactivate(t.b.id)))) {
      toast.show(t.off ? `${t.b.name} quedó de baja. Sus datos se conservan.` : `${t.b.name} está activa de nuevo.`);
      toggle = null;
      await refresh();
    }
  }

  function created(b: OrgBranch) {
    newOpen = false;
    toast.show(`${b.name} ya está lista. Puedes entrar desde la lista.`);
    refresh();
  }

  onMount(refresh);
</script>

<svelte:head><title>Sucursales · Caresia</title></svelte:head>

<Guard title="Sucursales" permissions={['adminUsers']} branches>
  <PageHeader title="Sucursales" subtitle="Todas tus sucursales, con sus reportes juntos. Cada una conserva sus pacientes, agenda, inventario y caja por separado.">
    {#snippet actions()}
      {#if ov?.can_create}
        <button type="button" class="btn-primary" onclick={() => (newOpen = true)} disabled={full}><Icon name="plus" size={18} />Nueva sucursal</button>
      {/if}
    {/snippet}
  </PageHeader>

  {#if load.phase === 'error'}
    <Alert>{load.message}</Alert>
  {:else if !ov}
    <div class="card"><LoadingRows /></div>
  {:else if !ov.organization}
    <div class="card mx-auto max-w-xl">
      {#if ov.can_create}
        <EmptyState icon="building" title="Aún solo tienes un consultorio" text="Crea una sucursal para manejar varios consultorios con el mismo usuario y ver sus reportes consolidados. Tu plan {ov.plan_name} permite {ov.branch_limit} {ov.branch_limit === 1 ? 'sucursal' : 'sucursales'}, contando la matriz.">
          {#if ov.branch_limit > 1}
            <button type="button" class="btn-primary" onclick={() => (newOpen = true)}><Icon name="plus" size={18} />Crear mi primera sucursal</button>
          {:else}
            <p class="text-sm text-app-muted">Las sucursales vienen con el plan Pro.</p>
          {/if}
        </EmptyState>
      {:else}
        <EmptyState icon="lock" title="Solo para el dueño" text="Las sucursales las administra el dueño de la organización. Si es tu caso, entra con la cuenta con la que registraste el consultorio." />
      {/if}
    </div>
  {:else}
    <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
      <h2 class="display text-2xl">{ov.organization.name}</h2>
      <p class="text-sm text-app-muted" role="status">{ov.active_branches} de {ov.branch_limit} sucursales en servicio · plan {ov.plan_name}</p>
    </div>
    <OpError op={act} class="mb-3" />
    <ul class="mb-8 grid gap-3 sm:grid-cols-2 xl:grid-cols-3" aria-label="Sucursales">
      {#each ov.branches as b (b.id)}
        <li class="card flex flex-col gap-3 p-4 {b.suspended ? 'opacity-70' : ''}">
          <div class="flex items-start justify-between gap-2">
            <div class="min-w-0">
              <p class="truncate font-semibold">{b.name}</p>
              <p class="text-xs text-app-muted">{CLINIC_KINDS[b.kind as keyof typeof CLINIC_KINDS]?.label ?? b.kind}{b.address ? ` · ${b.address}` : ''}</p>
            </div>
            <div class="flex flex-none flex-wrap justify-end gap-1">
              {#if b.is_matrix}<Pill tone="info">Matriz</Pill>{/if}
              {#if b.current}<Pill tone="ok">Estás aquí</Pill>{/if}
              {#if b.suspended}<Pill tone="bad">De baja</Pill>{/if}
            </div>
          </div>
          <p class="text-xs text-app-muted">{b.people} {b.people === 1 ? 'persona' : 'personas'} en el equipo</p>
          <div class="mt-auto flex flex-wrap gap-2">
            {#if !b.suspended && !b.current}
              <button type="button" class="btn-primary" disabled={act.phase === 'loading'} onclick={() => enter(b)}><Icon name="arrow-right" size={18} />Entrar</button>
            {/if}
            {#if !b.is_matrix && !b.current}
              {#if b.suspended}
                <button type="button" class="btn-secondary" onclick={() => (toggle = { b, off: false })}><Icon name="refresh" size={18} />Reactivar</button>
              {:else}
                <button type="button" class="btn-secondary" onclick={() => (toggle = { b, off: true })}><Icon name="ban" size={18} />Dar de baja</button>
              {/if}
            {/if}
          </div>
        </li>
      {/each}
    </ul>

    <div class="mb-5 flex flex-wrap items-end justify-between gap-3">
      <h2 class="display text-3xl">Reportes consolidados</h2>
      <div>
        <label class="label" for="org-branch">Sucursal</label>
        <select id="org-branch" class="field w-auto min-w-[14rem]" bind:value={branch}>
          <option value="">Todas las sucursales</option>
          {#each ov.branches as b (b.id)}<option value={b.id}>{b.name}{b.suspended ? ' (de baja)' : ''}</option>{/each}
        </select>
      </div>
    </div>
    <OrgReports {branch} />
  {/if}

  <NewBranchModal open={newOpen} first={!!ov && !ov.organization} onclose={() => (newOpen = false)} oncreated={created} />

  <ConfirmModal
    open={!!toggle}
    title={toggle?.off ? 'Dar de baja la sucursal' : 'Reactivar la sucursal'}
    op={toggleOp}
    confirmLabel={toggle?.off ? 'Dar de baja' : 'Reactivar'}
    onconfirm={confirmToggle}
    onclose={() => {
      toggle = null;
      toggleOp.reset();
    }}
  >
    {#if toggle?.off}
      <p><strong>{toggle.b.name}</strong> dejará de operar: su equipo no podrá entrar ni se recibirán reservas en línea. Sus pacientes, expedientes y ventas <strong>no se borran</strong>, y puedes reactivarla cuando quieras (mientras tu plan tenga lugar). Seguirá apareciendo en los reportes.</p>
    {:else if toggle}
      <p>Reactivar <strong>{toggle.b.name}</strong> la deja operando de nuevo. Cuenta contra el límite de sucursales de tu plan.</p>
    {/if}
  </ConfirmModal>
</Guard>
