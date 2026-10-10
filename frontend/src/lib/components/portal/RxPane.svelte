<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onMount } from 'svelte';
  import { portalApi } from '$lib/api/portal';
  import { Op } from '$lib/op.svelte';
  import type { PortalPatient, PortalPrescription } from '$lib/types/portal';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import { fmtDate, t } from '$lib/i18n/index.svelte';
  import { printPortalReceta } from './print';

  let { patients, patientId, multi }: { patients: PortalPatient[]; patientId: string; multi: boolean } = $props();

  let list = $state<PortalPrescription[]>([]);
  let clinic = $state({ name: '', address: '', phone: '' });
  let byArea = $state(false);
  let loaded = $state(false);
  let openId = $state('');
  const loadOp = new Op();
  const printOp = new Op();

  onMount(() =>
    loadOp.run(async () => {
      const r = await portalApi.prescriptions();
      list = r.prescriptions;
      clinic = r.clinic;
      byArea = !!r.by_area;
      loaded = true;
    })
  );

  const shown = $derived(list.filter((r) => !patientId || r.patient_id === patientId));
  /** a clinic with several giros: the recetas are listed apart by area (the clinic's own order of first appearance) */
  const groups = $derived.by(() => {
    if (!byArea) return [{ key: '', label: '', items: shown }];
    const map = new Map<string, { key: string; label: string; items: PortalPrescription[] }>();
    for (const r of shown) {
      const key = r.area || '';
      if (!map.has(key)) map.set(key, { key, label: r.area_label || 'General', items: [] });
      map.get(key)!.items.push(r);
    }
    return [...map.values()];
  });
  const fmt = (iso: string) => fmtDate(iso.length === 10 ? `${iso}T12:00:00` : iso, { day: 'numeric', month: 'long', year: 'numeric' });
  const expired = (r: PortalPrescription) => !!r.valid_until && r.valid_until < new Date().toISOString().slice(0, 10);

  function print(r: PortalPrescription) {
    const animal = patients.find((p) => p.id === r.patient_id)?.subject === 'animal';
    return printOp.run(() => printPortalReceta(r, clinic, animal));
  }
</script>

{#if !loaded && loadOp.phase !== 'error'}
  <LoadingRows />
{:else if !loaded}
  <Alert>{loadOp.message}</Alert>
{:else if shown.length === 0}
  <div class="card-empty">{t('portal.rx.none')}</div>
{:else}
  <OpError op={printOp} class="mb-3" />
  {#each groups as g (g.key)}
  {#if g.label}<h3 class="section-title mb-2 mt-5 first:mt-0">{g.label}</h3>{/if}
  <ul class="grid gap-3">
    {#each g.items as r (r.id)}
      {@const isOpen = openId === r.id}
      <li class="card px-4 py-4 sm:px-5">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0">
            <p class="font-medium">{t('portal.rx.number', { kind: r.mode === 'instructions' ? t('portal.rx.instructions') : t('portal.rx.rx'), folio: String(r.folio).padStart(6, '0') })}</p>
            <p class="text-sm text-app-muted">{fmt(r.issued_at)} · {r.author_name}</p>
            {#if multi}<p class="mt-0.5 text-sm">Paciente: <strong>{r.patient_name}</strong></p>{/if}
          </div>
          <div class="flex flex-wrap items-center gap-2">
            {#if r.complementary}<span class="pill">Complementaria</span>{/if}
            {#if r.voided}<span class="pill pill-bad">{t('portal.rx.voided')}</span>
            {:else if r.superseded || expired(r)}<span class="pill pill-warn">{t('portal.rx.expired')}</span>
            {:else}<span class="pill pill-ok">{r.valid_until ? t('portal.rx.validUntil', { date: fmt(r.valid_until) }) : t('portal.rx.valid')}</span>{/if}
          </div>
        </div>
        {#if isOpen}
          <div class="mt-4 border-t border-app-ink/10 pt-4 text-sm" id="rx-{r.id}">
            {#if r.mode === 'instructions'}
              <p class="whitespace-pre-line">{r.instructions}</p>
            {:else}
              <ol class="grid list-decimal gap-3 pl-5">
                {#each r.items as it}
                  <li>
                    <p><strong>{it.medicine}</strong>{it.brand ? ` (${it.brand})` : ''}{it.presentation ? ` · ${it.presentation}` : ''}</p>
                    <p class="text-app-muted">
                      {[it.dose && t('portal.rx.dose', { v: it.dose }), it.route && t('portal.rx.route', { v: it.route }), it.frequency && t('portal.rx.frequency', { v: it.frequency }), it.duration && t('portal.rx.duration', { v: it.duration }), it.quantity && t('portal.rx.quantity', { v: it.quantity })].filter(Boolean).join(' · ')}
                    </p>
                    {#if it.notes}<p class="text-app-muted">{it.notes}</p>{/if}
                  </li>
                {/each}
              </ol>
              {#if r.instructions}<p class="mt-3 whitespace-pre-line"><strong class="mr-1">{t('portal.rx.general')}</strong>{r.instructions}</p>{/if}
            {/if}
            {#if r.next_visit}<p class="mt-3 text-app-muted">{t('portal.rx.nextVisit', { date: fmt(r.next_visit) })}</p>{/if}
            <p class="mt-3 text-xs text-app-muted">{t('portal.rx.license', { license: r.author_license })}{r.author_institution ? ` · ${r.author_institution}` : ''}{t('portal.rx.copy')}</p>
          </div>
        {/if}
        <div class="mt-3 flex flex-wrap gap-2">
          <button class="btn-secondary !min-h-9" aria-expanded={isOpen} aria-controls="rx-{r.id}" onclick={() => (openId = isOpen ? '' : r.id)}>
            <Icon name="eye" size={16} />{isOpen ? t('portal.rx.hide') : t('portal.rx.detail')}
          </button>
          <button class="btn-secondary !min-h-9" disabled={printOp.phase === 'loading'} onclick={() => print(r)}><Icon name="receipt" size={16} />{t('portal.rx.print')}</button>
        </div>
      </li>
    {/each}
  </ul>
  {/each}
{/if}
