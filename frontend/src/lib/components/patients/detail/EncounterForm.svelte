<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { api } from '$lib/api';
  import { consult } from '$lib/api/consult';
  import { specialtyApi } from '$lib/api/specialty';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { ChargeDraftLine } from '$lib/types/consult';
  import { ENCOUNTER_KINDS, type Encounter, type EncounterKind, type FieldValues, type Patient, type PatientSchema } from '$lib/types';
  import DynamicFields from '../../DynamicFields.svelte';
  import Modal from '../../Modal.svelte';
  import Icon from '../../ui/Icon.svelte';
  import ConsultChargeEditor from './ConsultChargeEditor.svelte';
  import { toItems } from './consultLines';
  import FollowUpField from './FollowUpField.svelte';

  interface Props {
    open: boolean;
    patient: Patient;
    schema: PatientSchema;
    prefill?: { appointment_id?: string; reason?: string };
    onclose: () => void;
    onsaved: (e: Encounter, thenRx: boolean) => void;
  }
  let { open, patient, schema, prefill, onclose, onsaved }: Props = $props();

  const pad = (n: number) => String(n).padStart(2, '0');
  const local = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
  const CIE = /^[A-TV-Z]\d{2}(\.[0-9A-Z]{1,4})?$/;

  /** wording of the exam box for clinics that work with a single giro */
  const examLabel = $derived.by(() => {
    const k = schema.kinds ?? [];
    if (k.length && k.every((x) => x === 'DENTAL')) return 'Exploración clínica (dientes, encía, tejidos blandos)';
    if (k.length && k.every((x) => x === 'PSYCHOLOGY')) return 'Observación clínica (apariencia, ánimo, discurso)';
    if (k.length && k.every((x) => ['PHYSIOTHERAPY', 'CHIROPRACTIC', 'ORTHOPEDICS'].includes(x))) return 'Exploración física (movilidad, fuerza, dolor)';
    if (k.length && k.every((x) => x === 'NUTRITION')) return 'Evaluación (dieta, antropometría)';
    return 'Exploración';
  });
  const examLabelShown = $derived(patient.subject === 'animal' ? 'Objetivo: exploración física' : examLabel);
  const measureDefs = $derived(schema.measures[patient.subject] ?? []);
  const kinds = $derived(((patient.subject === 'animal' ? schema.encounter_kinds_animal : undefined) ?? (schema.encounter_kinds?.length ? schema.encounter_kinds : (Object.keys(ENCOUNTER_KINDS) as EncounterKind[]))).filter((k) => k !== 'adenda'));

  let kind = $state<EncounterKind>('consulta');
  let when = $state(local(new Date()));
  let minWhen = $state(local(new Date()));
  let maxWhen = $state(local(new Date()));
  let whenTouched = $state(false);
  let reason = $state('');
  let subjective = $state('');
  let measures = $state<FieldValues>({});
  let exam = $state('');
  let assessment = $state('');
  let codes = $state<string[]>([]);
  let codeDraft = $state('');
  let codeError = $state('');
  let plan = $state('');
  let notes = $state('');
  let isPrivate = $state(false);
  let followDate = $state('');
  let followTime = $state('');
  let error = $state('');
  /** services and supplies of this consultation, saved as a pre-account once the note exists */
  let chargeLines = $state<ChargeDraftLine[]>([]);
  let sendToCash = $state(true);
  const cobros = $derived(session.cobros);
  const op = new Op();
  let wasOpen = false;

  $effect(() => {
    if (open && !wasOpen) {
      const now = new Date();
      kind = 'consulta';
      when = local(now);
      maxWhen = local(now);
      minWhen = local(new Date(now.getTime() - 7 * 864e5));
      whenTouched = false;
      reason = prefill?.reason ?? '';
      subjective = exam = assessment = plan = notes = '';
      measures = {};
      codes = [];
      codeDraft = codeError = error = '';
      isPrivate = false;
      followDate = followTime = '';
      chargeLines = [];
      sendToCash = true;
      op.reset();
    }
    wasOpen = open;
  });

  function addCode() {
    const parts = codeDraft.split(/[\s,;]+/).map((c) => c.trim().toUpperCase()).filter(Boolean);
    codeError = '';
    for (const c of parts) {
      if (!CIE.test(c)) {
        codeError = `"${c}" no es un código CIE-10 válido (ejemplo: J06.9).`;
        return false;
      }
      if (!codes.includes(c)) codes = [...codes, c];
    }
    codeDraft = '';
    return true;
  }
  function codeKey(ev: KeyboardEvent) {
    if (ev.key === 'Enter' || ev.key === ',') {
      ev.preventDefault();
      addCode();
    } else if (ev.key === 'Backspace' && !codeDraft && codes.length) codes = codes.slice(0, -1);
  }

  const cleanMeasures = () => Object.fromEntries(Object.entries(measures).filter(([, v]) => v !== '' && v != null && !(Array.isArray(v) && !v.length)));

  /** The note is already saved (and permanent): a problem here never undoes it. */
  async function saveCharge(enc: Encounter) {
    try {
      const c = await consult.create({
        patient_id: patient.id,
        encounter_id: enc.id,
        ...(prefill?.appointment_id ? { appointment_id: prefill.appointment_id } : {}),
        items: toItems(chargeLines),
        send: cobros && sendToCash
      });
      for (const w of c.warnings ?? []) toast.show(w, 'error');
      toast.show(cobros && sendToCash ? 'Pre-cuenta enviada a caja' : 'Pre-cuenta guardada');
    } catch (e) {
      toast.show(`La nota quedó guardada, pero la pre-cuenta no: ${e instanceof Error ? e.message : 'inténtalo de nuevo'} Agrégala desde la nota.`, 'error');
    }
  }

  async function saveFollowUp() {
    try {
      const a = await specialtyApi.followUp(patient.id, { date: followDate, start_hour: followTime || undefined, reason: reason.trim() });
      toast.show(`Cita de seguimiento agendada el ${a.date} a las ${a.startHour} (por confirmar)`);
    } catch (e) {
      toast.show(`La nota quedó guardada, pero la cita de seguimiento no: ${e instanceof Error ? e.message : 'inténtalo de nuevo'}`, 'error');
    }
  }

  async function save(thenRx: boolean) {
    error = '';
    if (codeDraft && !addCode()) return;
    const m = cleanMeasures();
    if (![reason, subjective, exam, assessment, plan, notes].some((s) => s.trim()) && !Object.keys(m).length) {
      error = 'Escribe al menos el motivo, lo que cuenta el paciente o algún otro apartado.';
      return;
    }
    if (chargeLines.some((l) => !(l.qty > 0))) {
      error = 'Revisa las cantidades de la pre-cuenta.';
      return;
    }
    let saved: Encounter | undefined;
    const ok = await op.run(async () => {
      saved = await api.patients.createEncounter(patient.id, {
        kind,
        ...(whenTouched ? { occurred_at: new Date(when).toISOString() } : {}),
        ...(prefill?.appointment_id ? { appointment_id: prefill.appointment_id } : {}),
        reason: reason.trim(),
        subjective: subjective.trim(),
        measures: m,
        exam: exam.trim(),
        assessment: assessment.trim(),
        diagnosis_codes: codes,
        plan: plan.trim(),
        notes: notes.trim(),
        private: isPrivate,
        ...(followDate ? { next_visit: followDate } : {})
      });
    });
    if (ok && saved) {
      toast.show('Nota guardada en la bitácora');
      if (chargeLines.length) await saveCharge(saved);
      if (followDate) await saveFollowUp();
      onsaved(saved, thenRx);
    }
  }
</script>

<Modal {open} title="Nueva nota en la bitácora" {onclose} wide>
  <form id="enc-form" class="space-y-5" onsubmit={(ev) => { ev.preventDefault(); save(false); }}>
    <div class="grid gap-4 sm:grid-cols-2">
      <div>
        <label class="label" for="enc-kind">Tipo</label>
        <select id="enc-kind" class="field" bind:value={kind}>
          {#each kinds as k}<option value={k}>{ENCOUNTER_KINDS[k]}</option>{/each}
        </select>
      </div>
      <div>
        <label class="label" for="enc-when">Fecha y hora</label>
        <input id="enc-when" type="datetime-local" class="field" bind:value={when} min={minWhen} max={maxWhen} oninput={() => (whenTouched = true)} />
        <p class="hint">Puedes registrar hasta 7 días atrás.</p>
      </div>
    </div>

    <div>
      <label class="label" for="enc-reason">Motivo de consulta</label>
      <input id="enc-reason" class="field" bind:value={reason} autocomplete="off" />
    </div>
    <div>
      <label class="label" for="enc-subj">{patient.subject === 'animal' ? 'Subjetivo: lo que cuenta el propietario' : 'Lo que cuenta el paciente'}</label>
      <textarea id="enc-subj" class="field min-h-40" rows="7" bind:value={subjective} placeholder="Su relato con sus propias palabras: cómo empezó, cómo se siente, qué ha notado…"></textarea>
      <p class="hint">Es la conversación con {patient.subject === 'animal' ? 'el propietario' : 'el paciente'}; escríbela con el detalle que necesites.</p>
    </div>

    {#if measureDefs.length}
      <div>
        <p class="section-title mb-3">Signos vitales y mediciones</p>
        <DynamicFields fields={measureDefs} bind:values={measures} id="enc-m" headings={false} />
      </div>
    {/if}

    <div>
      <label class="label" for="enc-exam">{examLabelShown}</label>
      <textarea id="enc-exam" class="field min-h-24" rows="3" bind:value={exam}></textarea>
    </div>
    <div>
      <label class="label" for="enc-ass">{patient.subject === 'animal' ? 'Impresión diagnóstica' : 'Diagnóstico o impresión clínica'}</label>
      <textarea id="enc-ass" class="field min-h-20" rows="2" bind:value={assessment}></textarea>
    </div>
    <div>
      <label class="label" for="enc-cie">Códigos CIE-10</label>
      <div class="flex flex-wrap items-center gap-2 rounded-xl border border-app-ink/15 bg-app-panel p-2 focus-within:border-app-primary focus-within:ring-4 focus-within:ring-app-primary/15">
        {#each codes as c}
          <span class="badge font-mono">{c}<button type="button" class="ml-0.5 grid place-items-center" aria-label="Quitar {c}" onclick={() => (codes = codes.filter((x) => x !== c))}><Icon name="x" size={12} /></button></span>
        {/each}
        <input id="enc-cie" class="min-w-24 flex-1 bg-transparent px-1.5 py-1 font-mono text-sm uppercase outline-none" bind:value={codeDraft} onkeydown={codeKey} onblur={() => codeDraft && addCode()} placeholder={codes.length ? '' : 'J06.9'} autocomplete="off" autocapitalize="characters" aria-describedby="enc-cie-h" />
      </div>
      <p id="enc-cie-h" class="hint {codeError ? 'text-app-danger' : ''}" role={codeError ? 'alert' : undefined}>{codeError || 'Escribe un código y pulsa Enter o coma. Opcional.'}</p>
    </div>
    <div>
      <label class="label" for="enc-plan">Plan e indicaciones</label>
      <textarea id="enc-plan" class="field min-h-24" rows="3" bind:value={plan}></textarea>
    </div>
    <div>
      <label class="label" for="enc-notes">Notas</label>
      <textarea id="enc-notes" class="field min-h-20" rows="2" bind:value={notes}></textarea>
    </div>

    <fieldset class="rounded-2xl border border-app-ink/10 p-4">
      <legend class="section-title px-1">{cobros ? 'Servicios e insumos de esta consulta' : 'Lo realizado en esta consulta'}</legend>
      <ConsultChargeEditor bind:lines={chargeLines} {cobros} uid="enc-form-charge" />
      {#if cobros && chargeLines.length}
        <label class="mt-4 flex cursor-pointer items-center gap-2 text-sm">
          <input type="checkbox" class="h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={sendToCash} />
          Enviar a caja al guardar la nota
        </label>
      {/if}
    </fieldset>

    <FollowUpField bind:date={followDate} bind:time={followTime} id="enc-follow" />

    <label class="flex cursor-pointer items-start gap-3 rounded-2xl border border-app-ink/10 bg-app-elevated/60 p-4">
      <input type="checkbox" class="mt-1 h-5 w-5 accent-[rgb(var(--app-primary))]" bind:checked={isPrivate} />
      <span>
        <span class="block text-sm font-medium">Nota privada (solo yo puedo leerla)</span>
        <span class="hint block">Útil para notas de psicoterapia. El resto del equipo verá que existe una nota, pero no su contenido.</span>
      </span>
    </label>

    {#if error}<Alert>{error}</Alert>{/if}
    <OpError op={op} />
    <p class="flex items-start gap-2 text-sm text-app-muted"><Icon name="lock" size={16} class="mt-0.5 shrink-0" />Al guardar, la nota ya no se puede editar ni borrar (NOM-004). Si hay un error, se agrega una adenda.</p>
  </form>
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
    {#if schema.rx_mode === 'medication'}
      <button type="button" class="btn-secondary" disabled={op.phase === 'loading'} onclick={() => save(true)}>Guardar y hacer receta</button>
    {:else}
      <button type="button" class="btn-secondary" disabled={op.phase === 'loading'} onclick={() => save(true)}>Guardar y hacer indicaciones</button>
    {/if}
    <button type="submit" form="enc-form" class="btn-primary" disabled={op.phase === 'loading'}>
      {#if op.phase === 'loading'}<span class="spin"></span>{/if}Guardar en la bitácora
    </button>
  {/snippet}
</Modal>
