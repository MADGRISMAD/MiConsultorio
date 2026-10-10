<script lang="ts">
  import SectionTabs from '$lib/components/nav/SectionTabs.svelte';
  import { REPORT_TABS } from '$lib/components/nav/tabs';
  import Guard from '$lib/components/Guard.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import PageHeader from '$lib/components/ui/PageHeader.svelte';
  import OperationsPanel from '$lib/components/reports/OperationsPanel.svelte';
  import PatientsPanel from '$lib/components/reports/PatientsPanel.svelte';
  import { session } from '$lib/session.svelte';

  let tab = $state<'ops' | 'patients'>('ops');
</script>

<svelte:head><title>Reportes · Caresia</title></svelte:head>

<Guard title="Reportes" permissions={['adminUsers', 'navHistorials']}>
  <SectionTabs tabs={REPORT_TABS} label="Reportes" />
  <PageHeader title="Reportes" subtitle="Cómo va la operación del consultorio y quiénes son tus pacientes.">
    {#snippet actions()}
      {#if session.has('posReports') && session.cobros}
        <a href="/pos/reportes" class="btn-secondary"><Icon name="receipt" size={18} />Reportes de ventas</a>
      {/if}
    {/snippet}
  </PageHeader>

  {#if session.has('adminUsers')}
    <div class="mb-6 flex gap-1 overflow-x-auto rounded-full bg-app-ink/5 p-1 sm:inline-flex" role="tablist" aria-label="Tipo de reporte">
      <button type="button" role="tab" id="tab-ops" aria-selected={tab === 'ops'} aria-controls="panel-reports" class="whitespace-nowrap rounded-full px-4 py-1.5 text-sm font-medium transition {tab === 'ops' ? 'bg-app-panel text-app-ink shadow-sm' : 'text-app-muted hover:text-app-ink'}" onclick={() => (tab = 'ops')}>Operación</button>
      <button type="button" role="tab" id="tab-patients" aria-selected={tab === 'patients'} aria-controls="panel-reports" class="whitespace-nowrap rounded-full px-4 py-1.5 text-sm font-medium transition {tab === 'patients' ? 'bg-app-panel text-app-ink shadow-sm' : 'text-app-muted hover:text-app-ink'}" onclick={() => (tab = 'patients')}>Pacientes</button>
    </div>
  {/if}

  <div id="panel-reports" role="tabpanel" aria-labelledby={tab === 'ops' ? 'tab-ops' : 'tab-patients'}>
    {#if tab === 'patients' && session.has('adminUsers')}
      <PatientsPanel />
    {:else}
      <OperationsPanel />
    {/if}
  </div>
</Guard>
