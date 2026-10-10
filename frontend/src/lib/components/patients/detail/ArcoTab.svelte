<script lang="ts">
  import { Loader } from '$lib/loader.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onMount } from 'svelte';
  import { arcoApi } from '$lib/api/arco';
  import { fullName } from '$lib/format';
  import { toast } from '$lib/toast.svelte';
  import type { Patient } from '$lib/types';
  import type { ArcoRequest } from '$lib/types/arco';
  import ArcoDetail from '../../arco/ArcoDetail.svelte';
  import ArcoNewModal from '../../arco/ArcoNewModal.svelte';
  import { KIND_LABEL, STATUS_LABEL, arcoDone, deadlinePill, deadlineText, fmtDay, statusPill } from '../../arco/labels';
  import EmptyState from '../../ui/EmptyState.svelte';
  import Icon from '../../ui/Icon.svelte';

  let { patient, onchange = () => {} }: { patient: Patient; onchange?: (rows: ArcoRequest[]) => void } = $props();

  let rows = $state<ArcoRequest[]>([]);
  const ld = new Loader('No se pudieron cargar las solicitudes ARCO.');
  let selected = $state<string | null>(null);
  let newOpen = $state(false);

  async function load() {
    await ld.run(async () => {
      rows = (await arcoApi.list({ patient: patient.id, status: '' })).requests;
      onchange(rows);
    });
  }
  onMount(load);

  function created(r: ArcoRequest) {
    newOpen = false;
    toast.show(`Solicitud ${r.folio} registrada`);
    void load();
    selected = r.id;
  }
  const who = $derived({ id: patient.id, name: fullName(patient), email: patient.email, phone: patient.phone });
</script>

{#if ld.loading}
  <div class="card h-40 animate-pulse"></div>
{:else if ld.error}
  <Alert>{ld.error}</Alert>
{:else}
  <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
    <p class="max-w-xl text-sm text-app-muted">
      Las solicitudes de acceso, rectificación, cancelación, oposición o revocación que {patient.names} hizo sobre sus datos personales. Cuando una se atiende y se ejecuta queda marcada como realizada.
    </p>
    <button type="button" class="btn-secondary" onclick={() => (newOpen = true)}><Icon name="plus" size={18} />Registrar solicitud</button>
  </div>

  {#if rows.length === 0}
    <div class="card"><EmptyState icon="shield" title="Sin solicitudes ARCO" text="Este paciente no ha hecho ninguna solicitud sobre sus datos." /></div>
  {:else}
    <ul class="space-y-3">
      {#each rows as r (r.id)}
        <li>
          <button type="button" class="card flex w-full flex-col gap-2 p-4 text-left transition hover:border-app-ink/25 sm:flex-row sm:items-center sm:justify-between" onclick={() => (selected = r.id)}>
            <span class="min-w-0">
              <span class="flex flex-wrap items-center gap-2">
                <span class="font-mono text-sm font-semibold">{r.folio}</span>
                <span class="pill pill-info">{KIND_LABEL[r.kind]}</span>
                {#if arcoDone(r)}<span class="pill pill-ok"><Icon name="check" size={13} />Solicitud ARCO realizada</span>{:else}<span class="pill {statusPill(r.status)}">{STATUS_LABEL[r.status]}</span>{/if}
              </span>
              <span class="mt-1 block text-sm text-app-muted">Recibida {fmtDay(r.received_at)}{r.executed_at ? ` · ejecutada ${fmtDay(r.executed_at)}` : r.answered_at ? ` · respondida ${fmtDay(r.answered_at)}` : ''}</span>
            </span>
            {#if deadlineText(r)}<span class="pill {deadlinePill(r.deadline_state)} self-start sm:self-center"><Icon name="clock" size={13} />{deadlineText(r)}</span>{/if}
          </button>
        </li>
      {/each}
    </ul>
  {/if}
{/if}

<ArcoNewModal open={newOpen} onclose={() => (newOpen = false)} oncreated={created} patient={who} />
<ArcoDetail id={selected} onclose={() => (selected = null)} onchanged={load} />
