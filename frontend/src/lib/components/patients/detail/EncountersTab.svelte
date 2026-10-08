<script lang="ts">
  import { consult } from '$lib/api/consult';
  import { moneyCents } from '$lib/format';
  import { session } from '$lib/session.svelte';
  import { ENCOUNTER_KINDS, type Encounter, type FieldDef, type Patient, type PatientSchema } from '$lib/types';
  import type { Charge } from '$lib/types/consult';
  import ConsultChargeModal from './ConsultChargeModal.svelte';
  import EmptyState from '../../ui/EmptyState.svelte';
  import Icon from '../../ui/Icon.svelte';
  import Pill from '../../ui/Pill.svelte';
  import { measureRows } from './util';

  interface Props {
    patient: Patient;
    schema: PatientSchema;
    encounters: Encounter[];
    canWrite: boolean;
    onnew: () => void;
    onaddendum: (e: Encounter) => void;
  }
  let { patient, schema, encounters, canWrite, onnew, onaddendum }: Props = $props();

  let charges = $state<Charge[]>([]);
  let chargeFor = $state<Encounter | null>(null);
  const chargeOf = (id: string) => charges.find((c) => c.encounter_id === id && c.status !== 'cancelled') ?? null;
  const CHARGE_LABEL: Record<string, string> = { draft: 'Borrador', sent: 'Enviada a caja', charged: 'Cobrada', cancelled: 'Cancelada' };

  async function loadCharges() {
    if (!canWrite) return;
    try {
      charges = await consult.list({ patient_id: patient.id });
    } catch {
      /* the pre-account is a convenience: the notes still show */
    }
  }
  $effect(() => {
    void encounters.length;
    void loadCharges();
  });

  const defs = $derived<FieldDef[]>((schema.measures_all ?? schema.measures)[patient.subject] ?? []);
  const byId = $derived(new Map(encounters.map((e) => [e.id, e])));
  const roots = $derived(
    [...encounters].filter((e) => !e.addendum_of || !byId.has(e.addendum_of)).sort((a, b) => b.occurred_at.localeCompare(a.occurred_at))
  );
  const addendaOf = (id: string) => encounters.filter((e) => e.addendum_of === id).sort((a, b) => a.occurred_at.localeCompare(b.occurred_at));

  const dt = (iso: string) => new Date(iso).toLocaleString('es-MX', { weekday: 'short', day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false });
  const late = (e: Encounter) => Math.abs(new Date(e.created_at).getTime() - new Date(e.occurred_at).getTime()) > 5 * 60_000;
</script>

{#snippet body(e: Encounter)}
  {#if e.hidden}
    <p class="flex items-center gap-2 rounded-xl bg-app-ink/5 px-4 py-3 text-sm text-app-muted"><Icon name="lock" size={16} />Nota privada de {e.author_name}</p>
  {:else}
    {@const rows = measureRows(defs, e.measures)}
    <div class="space-y-3.5">
      {#each [['Motivo', e.reason], ['Lo que cuenta el paciente', e.subjective]] as [label, text]}
        {#if text}<div><p class="section-title">{label}</p><p class="mt-1 whitespace-pre-line break-words">{text}</p></div>{/if}
      {/each}
      {#if rows.length}
        <div>
          <p class="section-title">Signos vitales y mediciones</p>
          <dl class="mt-1.5 flex flex-wrap gap-2">
            {#each rows as r}<div class="rounded-xl bg-app-ink/5 px-3 py-1.5 text-sm"><dt class="inline text-app-muted">{r.label}: </dt><dd class="inline font-medium">{r.value}</dd></div>{/each}
          </dl>
        </div>
      {/if}
      {#if e.exam}<div><p class="section-title">Exploración</p><p class="mt-1 whitespace-pre-line break-words">{e.exam}</p></div>{/if}
      {#if e.assessment || e.diagnosis_codes?.length}
        <div>
          <p class="section-title">Diagnóstico</p>
          {#if e.assessment}<p class="mt-1 whitespace-pre-line break-words">{e.assessment}</p>{/if}
          {#if e.diagnosis_codes?.length}<p class="mt-1.5 flex flex-wrap gap-1.5">{#each e.diagnosis_codes as c}<span class="badge font-mono">{c}</span>{/each}</p>{/if}
        </div>
      {/if}
      {#if e.plan}<div><p class="section-title">Plan e indicaciones</p><p class="mt-1 whitespace-pre-line break-words">{e.plan}</p></div>{/if}
      {#if e.notes}<div><p class="section-title">Notas</p><p class="mt-1 whitespace-pre-line break-words">{e.notes}</p></div>{/if}
    </div>
  {/if}
{/snippet}

{#snippet head(e: Encounter)}
  <div class="flex flex-wrap items-center gap-2">
    <Pill tone={e.kind === 'adenda' ? 'warn' : 'info'}>{ENCOUNTER_KINDS[e.kind] ?? e.kind}</Pill>
    {#if e.private}<Pill tone="muted"><Icon name="lock" size={12} />Privada</Pill>{/if}
    <span class="text-sm font-medium first-letter:uppercase">{dt(e.occurred_at)}</span>
    {#if late(e)}<span class="text-xs text-app-muted">· registrada el {dt(e.created_at)}</span>{/if}
  </div>
  <p class="mt-1 text-sm text-app-muted">
    {e.author_name}{e.author_role ? ` · ${e.author_role}` : ''}{e.author_license ? ` · Cédula ${e.author_license}` : ''}
  </p>
{/snippet}

<div class="mb-4 flex flex-wrap items-center justify-between gap-3">
  <p class="max-w-xl text-sm text-app-muted">La bitácora es un registro permanente: las notas no se editan ni se borran, se corrigen con una adenda.</p>
  {#if canWrite}<button type="button" class="btn-primary" onclick={onnew}><Icon name="plus" size={18} />Nueva nota</button>{/if}
</div>

{#if roots.length === 0}
  <div class="card">
    <EmptyState icon="folder" title="Aún no hay notas" text="Registra la primera consulta para empezar la bitácora de {patient.names}.">
      {#if canWrite}<button type="button" class="btn-primary" onclick={onnew}><Icon name="plus" size={18} />Nueva nota</button>{/if}
    </EmptyState>
  </div>
{:else}
  <ol class="space-y-4">
    {#each roots as e (e.id)}
      <li class="card p-5 sm:p-6">
        {@render head(e)}
        <div class="mt-4">{@render body(e)}</div>
        {#each addendaOf(e.id) as a (a.id)}
          <div class="mt-5 border-l-4 border-app-warning/60 pl-4">
            {@render head(a)}
            {#if a.reason && !a.hidden}<p class="mt-2 text-sm font-medium">Adenda: {a.reason}</p>{/if}
            <div class="mt-2">{@render body({ ...a, reason: '' })}</div>
          </div>
        {/each}
        {#if canWrite && !e.hidden}
          {@const ch = chargeOf(e.id)}
          <div class="mt-4 flex flex-wrap items-center gap-x-4 gap-y-1 border-t border-app-ink/8 pt-3">
            <button type="button" class="btn-ghost -ml-3" onclick={() => onaddendum(e)}><Icon name="edit" size={16} />Agregar adenda</button>
            <button type="button" class="btn-ghost" onclick={() => (chargeFor = e)}>
              <Icon name="receipt" size={16} />{ch ? 'Pre-cuenta' : 'Agregar pre-cuenta'}
            </button>
            {#if ch}
              <span class="text-sm text-app-muted">{CHARGE_LABEL[ch.status]}{session.cobros ? ` · ${moneyCents(ch.total_cents)}` : ''} · {ch.item_count} {ch.item_count === 1 ? 'concepto' : 'conceptos'}</span>
            {/if}
          </div>
        {/if}
      </li>
    {/each}
  </ol>
{/if}

<ConsultChargeModal
  open={!!chargeFor}
  patientId={patient.id}
  encounterId={chargeFor?.id}
  charge={chargeFor ? chargeOf(chargeFor.id) : null}
  onclose={() => (chargeFor = null)}
  onchanged={loadCharges}
/>
