<script lang="ts">
  import { portalApi } from '$lib/api/portal';
  import { Op } from '$lib/op.svelte';
  import type { PortalPatient, PortalVaccination } from '$lib/types/portal';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import { printPortalCarnet } from './print';
  import { fmtDate, t } from '$lib/i18n/index.svelte';

  let {
    patients,
    patientId,
    clinicName,
    list,
    loaded,
    error
  }: { patients: PortalPatient[]; patientId: string; clinicName: string; list: PortalVaccination[]; loaded: boolean; error: string } = $props();

  const printOp = new Op();
  const KINDS = ['vaccine', 'deworming_internal', 'deworming_external', 'other'];
  const kindLabel = (k: string) => (KINDS.includes(k) ? t(`portal.vac.${k}`) : k);
  const fmt = (iso: string) => fmtDate(`${iso}T12:00:00`);
  const today = new Date().toISOString().slice(0, 10);

  const shown = $derived(list.filter((v) => !patientId || v.patient_id === patientId));
  // one card per patient
  const groups = $derived(
    patients
      .filter((p) => !patientId || p.id === patientId)
      .map((p) => ({ patient: p, rows: shown.filter((v) => v.patient_id === p.id) }))
      .filter((g) => g.rows.length > 0)
  );
</script>

{#if !loaded && !error}
  <LoadingRows />
{:else if error}
  <p class="alert" role="alert"><Icon name="alert" size={18} />{error}</p>
{:else if groups.length === 0}
  <div class="card px-5 py-8 text-center text-sm text-app-muted">{t('portal.vac.none')}</div>
{:else}
  {#if printOp.phase === 'error'}<p class="alert mb-3" role="alert"><Icon name="alert" size={18} />{printOp.message}</p>{/if}
  <div class="grid gap-6">
    {#each groups as g (g.patient.id)}
      <section class="card overflow-hidden" aria-labelledby="vc-{g.patient.id}">
        <div class="flex flex-wrap items-center justify-between gap-3 px-4 pt-4 sm:px-5">
          <h2 id="vc-{g.patient.id}" class="font-medium">{g.patient.name}{g.patient.species ? ` · ${g.patient.species}` : ''}</h2>
          <button class="btn-secondary !min-h-9" disabled={printOp.phase === 'loading'} onclick={() => printOp.run(() => printPortalCarnet(g.patient.name, clinicName, g.rows))}>
            <Icon name="receipt" size={16} />{t('portal.vac.print')}
          </button>
        </div>
        <ul class="mt-3 divide-y divide-app-ink/10">
          {#each g.rows as v (v.id)}
            <li class="flex flex-wrap items-start justify-between gap-2 px-4 py-3 text-sm sm:px-5">
              <div class="min-w-0">
                <p class="font-medium">{v.name}</p>
                <p class="text-app-muted">{kindLabel(v.kind)} · {t('portal.vac.applied', { date: fmt(v.applied_on) })}{v.dose ? ` · ${v.dose}` : ''}{v.lot ? ` · ${t('portal.vac.lot', { lot: v.lot })}` : ''}</p>
              </div>
              {#if v.next_due}
                <span class="pill {v.next_due < today ? 'pill-warn' : 'pill-info'}">{t('portal.vac.next', { date: fmt(v.next_due) })}</span>
              {/if}
            </li>
          {/each}
        </ul>
      </section>
    {/each}
  </div>
{/if}
