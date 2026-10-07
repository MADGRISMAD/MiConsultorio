<script lang="ts">
  import { onMount } from 'svelte';
  import { portalApi } from '$lib/api/portal';
  import { Op } from '$lib/op.svelte';
  import type { PortalPatient, PortalPrescription } from '$lib/types/portal';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import { printPortalReceta } from './print';

  let { patients, patientId, multi }: { patients: PortalPatient[]; patientId: string; multi: boolean } = $props();

  let list = $state<PortalPrescription[]>([]);
  let clinic = $state({ name: '', address: '', phone: '' });
  let loaded = $state(false);
  let openId = $state('');
  const loadOp = new Op();
  const printOp = new Op();

  onMount(() =>
    loadOp.run(async () => {
      const r = await portalApi.prescriptions();
      list = r.prescriptions;
      clinic = r.clinic;
      loaded = true;
    })
  );

  const shown = $derived(list.filter((r) => !patientId || r.patient_id === patientId));
  const fmt = (iso: string) => new Date(iso.length === 10 ? `${iso}T12:00:00` : iso).toLocaleDateString('es-MX', { day: 'numeric', month: 'long', year: 'numeric' });
  const expired = (r: PortalPrescription) => !!r.valid_until && r.valid_until < new Date().toISOString().slice(0, 10);

  function print(r: PortalPrescription) {
    const animal = patients.find((p) => p.id === r.patient_id)?.subject === 'animal';
    return printOp.run(() => printPortalReceta(r, clinic, animal));
  }
</script>

{#if !loaded && loadOp.phase !== 'error'}
  <LoadingRows />
{:else if !loaded}
  <p class="alert" role="alert"><Icon name="alert" size={18} />{loadOp.message}</p>
{:else if shown.length === 0}
  <div class="card px-5 py-8 text-center text-sm text-app-muted">No hay recetas para mostrar.</div>
{:else}
  {#if printOp.phase === 'error'}<p class="alert mb-3" role="alert"><Icon name="alert" size={18} />{printOp.message}</p>{/if}
  <ul class="grid gap-3">
    {#each shown as r (r.id)}
      {@const isOpen = openId === r.id}
      <li class="card px-4 py-4 sm:px-5">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0">
            <p class="font-medium">{r.mode === 'instructions' ? 'Indicaciones' : 'Receta'} n.º {String(r.folio).padStart(6, '0')}</p>
            <p class="text-sm text-app-muted">{fmt(r.issued_at)} · {r.author_name}</p>
            {#if multi}<p class="mt-0.5 text-sm">Paciente: <strong>{r.patient_name}</strong></p>{/if}
          </div>
          <div class="flex flex-wrap items-center gap-2">
            {#if r.voided}<span class="pill pill-bad">Cancelada</span>
            {:else if expired(r)}<span class="pill pill-warn">Vencida</span>
            {:else}<span class="pill pill-ok">Vigente{r.valid_until ? ` hasta ${fmt(r.valid_until)}` : ''}</span>{/if}
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
                      {[it.dose && `Dosis: ${it.dose}`, it.route && `Vía: ${it.route}`, it.frequency && `Frecuencia: ${it.frequency}`, it.duration && `Duración: ${it.duration}`, it.quantity && `Cantidad: ${it.quantity}`].filter(Boolean).join(' · ')}
                    </p>
                    {#if it.notes}<p class="text-app-muted">{it.notes}</p>{/if}
                  </li>
                {/each}
              </ol>
              {#if r.instructions}<p class="mt-3 whitespace-pre-line"><strong class="mr-1">Indicaciones generales:</strong>{r.instructions}</p>{/if}
            {/if}
            {#if r.next_visit}<p class="mt-3 text-app-muted">Próxima cita sugerida: {fmt(r.next_visit)}</p>{/if}
            <p class="mt-3 text-xs text-app-muted">Cédula profesional {r.author_license}{r.author_institution ? ` · ${r.author_institution}` : ''}. Esta es una copia informativa; la receta firmada es la que te entregó el consultorio.</p>
          </div>
        {/if}
        <div class="mt-3 flex flex-wrap gap-2">
          <button class="btn-secondary !min-h-9" aria-expanded={isOpen} aria-controls="rx-{r.id}" onclick={() => (openId = isOpen ? '' : r.id)}>
            <Icon name="eye" size={16} />{isOpen ? 'Ocultar' : 'Ver detalle'}
          </button>
          <button class="btn-secondary !min-h-9" disabled={printOp.phase === 'loading'} onclick={() => print(r)}><Icon name="receipt" size={16} />Imprimir</button>
        </div>
      </li>
    {/each}
  </ul>
{/if}
