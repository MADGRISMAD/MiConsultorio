<script lang="ts">
  import Alert from '$lib/components/ui/Alert.svelte';
  import { api, ApiError } from '$lib/api';
  import { Op } from '$lib/op.svelte';
  import { printReceta } from '$lib/print';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import { CLINIC_KINDS } from '$lib/types';
  import { rxApi } from '$lib/api/rx';
  import type { Patient, PatientSchema, Prescription, RxControl } from '$lib/types';
  import type { CatalogMed, DoseResult, Icd10, RxCheck, RxItemInput } from '$lib/types/rx';
  import { allergyMatches, patientAllergies } from '../../rx/allergy';
  import Autocomplete from '../../rx/Autocomplete.svelte';
  import DoseCalculator from '../../rx/DoseCalculator.svelte';
  import Modal from '../../Modal.svelte';
  import Icon from '../../ui/Icon.svelte';

  interface Props {
    open: boolean;
    patient: Patient;
    schema: PatientSchema;
    encounterId?: string;
    diagnosis?: string;
    /** the next consultation suggested in the note (YYYY-MM-DD) */
    nextVisit?: string;
    onclose: () => void;
    oncreated: (rx: Prescription) => void;
  }
  let { open, patient, schema, encounterId, diagnosis: diagnosisSeed = '', nextVisit: nextVisitSeed = '', onclose, oncreated }: Props = $props();

  const instr = $derived(schema.rx_mode === 'instructions');
  const CONTROLS: RxControl[] = ['No', 'Antibiótico', 'Fracción III', 'Fracción I o II'];
  const FREQ = ['Cada 4 horas', 'Cada 6 horas', 'Cada 8 horas', 'Cada 12 horas', 'Cada 24 horas', 'Una vez al día', 'Dosis única', 'Solo si hay dolor o molestia'];
  const DUR = ['3 días', '5 días', '7 días', '10 días', '14 días', 'Tratamiento continuo'];

  const blank = (): RxItemInput => ({ medicine: '', brand: '', presentation: '', dose: '', route: schema.routes?.[0] ?? '', frequency: '', duration: '', quantity: '', notes: '', control: 'No' });
  let diagnosis = $state('');
  let items = $state<RxItemInput[]>([blank()]);
  // catalog entry behind each item (null when typed by hand)
  let picked = $state<(CatalogMed | null)[]>([null]);
  let calcFor = $state<number | null>(null);
  let weight = $state('');
  let weightNote = $state('');
  // confirmations asked by the server: allergy match or dose above the reference maximum
  let pending = $state<{ kind: 'allergy' | 'dose' | 'interaction'; message: string; lines: string[] } | null>(null);
  let reasonText = $state('');
  let confirmed = $state<{ allergy?: string; dose?: string; interaction?: string }>({});
  // alerts while writing: allergies, interactions with what the patient already takes, and that medication itself
  let live = $state<RxCheck | null>(null);
  let checkSeq = 0;
  $effect(() => {
    const names = items.map((i) => i.medicine.trim()).filter(Boolean);
    const mine = ++checkSeq;
    if (!open || instr || patient.subject === 'animal') return void (live = null);
    const t = setTimeout(() => {
      rxApi.check(patient.id, items.filter((i) => i.medicine.trim()).map((i) => ({ medicine: i.medicine.trim(), brand: i.brand })))
        .then((r) => mine === checkSeq && (live = r))
        .catch(() => mine === checkSeq && (live = null));
    }, names.length ? 450 : 0);
    return () => clearTimeout(t);
  });
  let disclaimer = $state('');
  const subject = $derived(patient.subject === 'animal' ? 'animal' : 'person');
  const species = $derived(typeof patient.profile?.species === 'string' ? (patient.profile.species as string) : '');
  const allergies = $derived(patientAllergies(patient.profile));
  const alertsOf = (it: RxItemInput) => allergyMatches(allergies, it.medicine, it.brand);
  const anyRetained = $derived(items.some((i) => i.control === 'Antibiótico' || i.control === 'Fracción III'));
  let instructions = $state('');
  let nextVisit = $state('');
  let validDays = $state(30);
  // the patient's valid recetas: a new one replaces those of the same area unless it is a complement
  let earlier = $state<Prescription[]>([]);
  let complementary = $state(false);
  let area = $state('');
  const clinicKinds = $derived(schema.kinds ?? []);
  const myAreas = $derived(session.user?.role === 'admin' ? [] : (session.user?.areas ?? []));
  const areaPool = $derived(myAreas.length ? clinicKinds.filter((k) => myAreas.includes(k)) : clinicKinds);
  const effectiveArea = $derived(area || areaPool[0] || '');
  const sameArea = $derived(earlier.filter((r) => !r.voided_at && (r.area ?? '') === effectiveArea));
  const kindLabel = (k: string) => CLINIC_KINDS[k as keyof typeof CLINIC_KINDS]?.label ?? k;
  async function loadEarlier() {
    try {
      earlier = await api.patients.prescriptions(patient.id);
    } catch {
      earlier = [];
    }
  }
  let error = $state('');
  let needCedula = $state(false);
  let created = $state<Prescription | null>(null);
  let printing = $state(false);
  const op = new Op();
  let wasOpen = false;

  $effect(() => {
    if (open && !wasOpen) {
      diagnosis = diagnosisSeed;
      items = [blank()];
      picked = [null];
      calcFor = null;
      pending = null;
      confirmed = {};
      reasonText = weight = weightNote = '';
      loadWeight();
      complementary = false;
      area = '';
      earlier = [];
      loadEarlier();
      instructions = nextVisit = error = '';
      if (nextVisitSeed >= today) nextVisit = nextVisitSeed;
      validDays = 30;
      needCedula = false;
      created = null;
      op.reset();
    }
    wasOpen = open;
  });

  const today = new Date().toISOString().slice(0, 10);

  // Weight of the latest consultation that recorded one; the prescriber can change it.
  async function loadWeight() {
    try {
      const list = await api.patients.encounters(patient.id);
      // opened from the recetas tab (no note picked): start from the latest note's diagnosis and suggested next visit
      const latest = [...list].filter((e) => !e.hidden && !e.addendum_of).sort((a, b) => b.occurred_at.localeCompare(a.occurred_at))[0];
      if (latest) {
        if (!diagnosis.trim() && latest.assessment?.trim()) diagnosis = latest.assessment.trim();
        if (!nextVisit && latest.next_visit && latest.next_visit >= today) nextVisit = latest.next_visit;
      }
      const last = [...list]
        .filter((e) => !e.hidden && Number(e.measures?.weight_kg) > 0)
        .sort((a, b) => b.occurred_at.localeCompare(a.occurred_at))[0];
      if (last && !weight) {
        weight = String(last.measures.weight_kg);
        weightNote = `Peso de la consulta del ${new Date(last.occurred_at).toLocaleDateString('es-MX', { day: 'numeric', month: 'short', year: 'numeric' })}. Confírmalo antes de calcular dosis.`;
      }
    } catch {
      // optional help; the prescriber can type the weight
    }
  }

  const searchMeds = async (q: string) => {
    const r = await rxApi.medications(q, subject, species);
    disclaimer = r.disclaimer;
    return r.medications;
  };
  const searchDx = async (q: string) => (await rxApi.diagnoses(q, 15, subject, species)).diagnoses;

  function pickMed(n: number, m: CatalogMed) {
    const it = items[n];
    it.medicine = m.name;
    it.brand = m.brand ?? '';
    it.presentation = m.presentations?.[0] ?? '';
    if ((schema.routes ?? []).includes(m.route)) it.route = m.route;
    it.control = m.control;
    it.catalog_id = m.id;
    delete it.dose_mg;
    delete it.doses_per_day;
    picked[n] = m;
    calcFor = null;
  }
  function typedMed(n: number) {
    delete items[n].catalog_id;
    picked[n] = null;
    if (calcFor === n) calcFor = null;
  }
  function applyDose(n: number, r: DoseResult) {
    const it = items[n];
    it.dose = r.dose;
    it.frequency = r.frequency;
    if (r.presentation) it.presentation = r.presentation;
    it.dose_mg = r.dose_mg;
    it.doses_per_day = r.doses_per_day;
    calcFor = null;
    toast.show('Dosis agregada: revísala antes de crear la receta');
  }
  const addItem = () => {
    items = [...items, blank()];
    picked = [...picked, null];
  };

  function submit(ev: SubmitEvent) {
    ev.preventDefault();
    pending = null;
    return send();
  }

  // The server revalidates allergies and doses; a warning comes back as `pending` for the prescriber to confirm.
  async function send() {
    error = '';
    needCedula = false;
    if (!instr) {
      const bad = items.findIndex((i) => !i.medicine.trim() || !i.dose.trim() || !i.frequency.trim() || !i.duration.trim());
      if (bad >= 0) return (error = `Medicamento ${bad + 1}: indica denominación genérica, dosis, frecuencia y duración.`);
      if (items.some((i) => i.control === 'Fracción I o II')) return (error = 'Los medicamentos de Fracción I o II requieren receta especial con código de barras de COFEPRIS; Caresia no puede emitirla. Cambia o quita ese medicamento.');
    } else if (!instructions.trim()) return (error = 'Escribe las indicaciones.');
    if (!(validDays >= 1 && validDays <= 30)) return (error = 'La vigencia debe ser de 1 a 30 días.');
    let rx: Prescription | undefined;
    const ok = await op.run(async () => {
      try {
        const w = parseFloat(weight.replace(',', '.'));
        const res = await rxApi.createPrescription(patient.id, {
          ...(encounterId ? { encounter_id: encounterId } : {}),
          diagnosis: diagnosis.trim(),
          items: instr ? [] : items.map((i) => ({ ...i, medicine: i.medicine.trim() })),
          instructions: instructions.trim(),
          ...(nextVisit ? { next_visit: nextVisit } : {}),
          valid_days: validDays,
          ...(areaPool.length > 1 ? { area: effectiveArea } : {}),
          ...(complementary && sameArea.length ? { complementary: true } : {}),
          ...(!instr && w > 0 ? { weight_kg: w } : {}),
          ...(confirmed.allergy ? { allergy_override_reason: confirmed.allergy } : {}),
          ...(confirmed.dose ? { dose_override_reason: confirmed.dose } : {}),
          ...(confirmed.interaction ? { interaction_override_reason: confirmed.interaction } : {})
        });
        if (res.kind === 'created') rx = res.prescription;
        else {
          reasonText = '';
          pending =
            res.kind === 'allergy'
              ? { kind: 'allergy', message: res.message, lines: res.conflicts.map((c) => c.message) }
              : res.kind === 'interaction'
                ? { kind: 'interaction', message: res.message, lines: res.interactions.map((c) => `${c.drug} + ${c.with} (${c.with_source}): ${c.message}`) }
                : { kind: 'dose', message: res.message, lines: res.warnings.map((c) => c.message) };
        }
      } catch (e) {
        if (e instanceof ApiError && e.code === 'CEDULA_REQUIRED') needCedula = true;
        throw e;
      }
    });
    if (ok && rx) {
      created = rx;
      oncreated(rx);
      toast.show(instr ? 'Hoja de indicaciones creada' : 'Receta creada');
    }
  }

  function confirmPending() {
    if (!pending) return;
    if (!reasonText.trim()) return (error = 'Escribe el motivo para continuar.');
    confirmed = { ...confirmed, [pending.kind]: reasonText.trim() };
    pending = null;
    return send();
  }

  async function print() {
    if (!created) return;
    printing = true;
    try {
      await printReceta(created.id);
    } catch (e) {
      toast.show(e instanceof Error ? e.message : 'No se pudo imprimir.', 'error');
    } finally {
      printing = false;
    }
  }
  const remove = (n: number) => {
    if (items.length < 2) return;
    items = items.filter((_, i) => i !== n);
    picked = picked.filter((_, i) => i !== n);
    calcFor = null;
  };
  const title = $derived(created ? (instr ? 'Hoja de indicaciones lista' : 'Receta lista') : instr ? 'Nueva hoja de indicaciones' : 'Nueva receta');
</script>

<Modal {open} {title} {onclose} wide>
  {#if created}
    <div class="flex flex-col items-center py-6 text-center">
      <span class="grid h-14 w-14 place-items-center rounded-2xl bg-app-accent/14 text-app-accent"><Icon name="check" size={26} /></span>
      <p class="display mt-4 text-2xl">Folio {String(created.folio).padStart(6, '0')}</p>
      <p class="mt-1 max-w-sm text-sm text-app-muted">Imprímela y fírmala de forma autógrafa: Caresia no firma por ti.</p>
    </div>
  {:else}
    <form id="rx-form" class="space-y-5" onsubmit={submit}>
      {#if instr}
        <p class="rounded-xl bg-app-primary/8 px-4 py-3 text-sm text-app-muted">Es una <strong class="text-app-ink">hoja de indicaciones</strong>, no una receta de medicamentos.</p>
      {/if}
      <div>
        <label class="label" for="rx-dx">Diagnóstico</label>
        <Autocomplete id="rx-dx" bind:value={diagnosis} search={searchDx} title={(d: Icd10) => (d.code ? `${d.code} · ${d.name}` : d.name)} onpick={(d: Icd10) => (diagnosis = d.code ? `${d.name} (${d.code})` : d.name)} placeholder={subject === 'animal' ? 'Escribe o busca un diagnóstico veterinario' : 'Escribe o busca en CIE-10 (código o nombre)'} minChars={2} describedby="rx-dx-h" />
        <p id="rx-dx-h" class="hint">Catálogo CIE-10 parcial: si no aparece, escribe el diagnóstico libremente.</p>
      </div>

      {#if instr}
        <div>
          <label class="label" for="rx-ins">Indicaciones</label>
          <textarea id="rx-ins" class="field min-h-52" rows="9" bind:value={instructions}></textarea>
        </div>
      {:else}
        <datalist id="rx-freq">{#each FREQ as f}<option value={f}></option>{/each}</datalist>
        <datalist id="rx-dur">{#each DUR as f}<option value={f}></option>{/each}</datalist>
        {#if allergies.length}
          <div class="flex items-start gap-2.5 rounded-xl bg-app-warning/15 px-3.5 py-3 text-sm" role="status">
            <Icon name="alert" size={18} />
            <span><strong>Alergias registradas:</strong> {allergies.join(', ')}.</span>
          </div>
        {/if}
        <div class="max-w-xs">
          <label class="label" for="rx-weight">Peso del paciente (kg)</label>
          <input id="rx-weight" class="field" inputmode="decimal" bind:value={weight} autocomplete="off" aria-describedby="rx-weight-h" />
          <p id="rx-weight-h" class="hint">{weightNote || 'Opcional. Sirve para calcular dosis por peso y comparar con el máximo de referencia.'}</p>
        </div>
        <div class="space-y-3">
          <p class="section-title">Medicamentos</p>
          <p class="hint">Escribe siempre la <strong>denominación genérica</strong> (sustancia activa). La marca es opcional.</p>
          {#each items as it, n (n)}
            <fieldset class="rounded-2xl border border-app-ink/10 bg-app-elevated/40 p-4">
              <legend class="flex items-center gap-2 px-1 text-sm font-medium">Medicamento {n + 1}</legend>
              <div class="grid gap-3 sm:grid-cols-2">
                <div class="sm:col-span-2">
                  <label class="label" for="rx-med-{n}">Denominación genérica *</label>
                  <Autocomplete
                    id="rx-med-{n}"
                    bind:value={it.medicine}
                    search={searchMeds}
                    title={(m: CatalogMed) => m.name}
                    detail={(m: CatalogMed) => `${m.source === 'clinic' ? 'Propio de la clínica · ' : ''}${m.category}${m.control !== 'No' ? ` · ${m.control}` : ''}${m.species?.length ? ` · ${m.species.join(', ')}` : ''}`}
                    onpick={(m: CatalogMed) => pickMed(n, m)}
                    oninput={() => typedMed(n)}
                    placeholder="Escribe para buscar, ej. Paracetamol"
                    required
                  />
                  {#each alertsOf(it) as a}
                    <Alert class="mt-2"><span>Alergia registrada: «{a.allergy}»{a.family ? ` (familia ${a.family})` : ''}. Al crear la receta tendrás que indicar el motivo para continuar.</span></Alert>
                  {/each}
                  {#if picked[n]}
                    {@const m = picked[n]}
                    <p class="hint">
                      Referencia: {m.typical_dose}{m.notes ? ` · ${m.notes}` : ''}
                      {#if m.mg_per_kg}
                        · <button type="button" class="underline" onclick={() => (calcFor = calcFor === n ? null : n)}>{calcFor === n ? 'Ocultar calculadora' : 'Calcular dosis por peso'}</button>
                      {/if}
                    </p>
                  {/if}
                </div>
                <div><label class="label" for="rx-brand-{n}">Marca (opcional)</label><input id="rx-brand-{n}" class="field" bind:value={it.brand} autocomplete="off" /></div>
                <div><label class="label" for="rx-pres-{n}">Presentación</label><input id="rx-pres-{n}" class="field" bind:value={it.presentation} autocomplete="off" placeholder="Tabletas 500 mg" /></div>
                <div><label class="label" for="rx-dose-{n}">Dosis *</label><input id="rx-dose-{n}" class="field" bind:value={it.dose} autocomplete="off" placeholder="1 tableta" /></div>
                <div>
                  <label class="label" for="rx-route-{n}">Vía</label>
                  <select id="rx-route-{n}" class="field" bind:value={it.route}>{#each schema.routes ?? [] as r}<option value={r}>{r}</option>{/each}</select>
                </div>
                <div><label class="label" for="rx-freq-{n}">Frecuencia *</label><input id="rx-freq-{n}" class="field" list="rx-freq" bind:value={it.frequency} autocomplete="off" /></div>
                <div><label class="label" for="rx-dur-{n}">Duración *</label><input id="rx-dur-{n}" class="field" list="rx-dur" bind:value={it.duration} autocomplete="off" /></div>
                <div><label class="label" for="rx-qty-{n}">Cantidad a surtir</label><input id="rx-qty-{n}" class="field" bind:value={it.quantity} autocomplete="off" placeholder="1 caja con 20" /></div>
                <div>
                  <label class="label" for="rx-ctl-{n}">Control</label>
                  <select id="rx-ctl-{n}" class="field" bind:value={it.control} aria-describedby="rx-ctl-h-{n}">{#each CONTROLS as c}<option value={c}>{c}</option>{/each}</select>
                </div>
                {#if picked[n] && calcFor === n}
                  <div class="sm:col-span-2"><DoseCalculator med={picked[n]} {weight} idPrefix="rx-calc-{n}" onapply={(r) => applyDose(n, r)} /></div>
                {/if}
                <p id="rx-ctl-h-{n}" class="hint sm:col-span-2 {it.control === 'Fracción I o II' ? '!text-app-danger font-medium' : ''}">
                  {#if it.control === 'Fracción I o II'}
                    Caresia no puede emitir esta receta: los medicamentos de Fracción I o II (estupefacientes y psicotrópicos) requieren la receta especial con código de barras de COFEPRIS. Elige otro control o quita el medicamento.
                  {:else}
                    Antibiótico y Fracción III: la farmacia retiene la receta. Fracción I o II (estupefacientes y psicotrópicos) requiere la receta especial con código de barras de COFEPRIS, que Caresia no puede emitir.
                  {/if}
                </p>
                <div class="sm:col-span-2"><label class="label" for="rx-notes-{n}">Notas</label><input id="rx-notes-{n}" class="field" bind:value={it.notes} autocomplete="off" placeholder="Tomar con alimentos…" /></div>
              </div>
              {#if items.length > 1}<button type="button" class="btn-ghost mt-3 -ml-3 text-app-danger" onclick={() => remove(n)}><Icon name="trash" size={16} />Quitar medicamento</button>{/if}
            </fieldset>
          {/each}
          <button type="button" class="btn-secondary" onclick={addItem}><Icon name="plus" size={16} />Agregar medicamento</button>
          {#if disclaimer}<p class="hint">{disclaimer}</p>{/if}
          {#if anyRetained}
            <p class="rounded-xl bg-app-primary/8 px-4 py-3 text-sm" role="status"><strong>Receta retenida:</strong> hay antibióticos o medicamentos de Fracción III; la farmacia conservará la receta y no se podrá surtir de nuevo.</p>
          {/if}
        </div>
        <div>
          <label class="label" for="rx-ins">Indicaciones generales</label>
          <textarea id="rx-ins" class="field min-h-24" rows="3" bind:value={instructions} placeholder="Reposo, hidratación, datos de alarma…"></textarea>
        </div>
      {/if}

      {#if areaPool.length > 1}
        <div>
          <label class="label" for="rx-area">Área que emite {instr ? 'la hoja' : 'la receta'}</label>
          <select id="rx-area" class="field" bind:value={area}>
            {#each areaPool as k (k)}<option value={k}>{kindLabel(k)}</option>{/each}
          </select>
          <p class="hint">Las recetas de otra área no se ven afectadas.</p>
        </div>
      {/if}
      {#if sameArea.length}
        <div class="rounded-xl border border-app-ink/12 bg-app-elevated px-4 py-3 text-sm">
          <label class="flex cursor-pointer items-start gap-3">
            <input type="checkbox" class="mt-0.5 h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={complementary} />
            <span><strong>Es complementaria</strong> a la vigente</span>
          </label>
          <p class="mt-1.5 text-app-muted">
            {#if complementary}
              La{sameArea.length > 1 ? 's' : ''} {sameArea.length > 1 ? 'recetas' : 'receta'} {sameArea.map((r) => '#' + String(r.folio).padStart(6, '0')).join(', ')} seguirá{sameArea.length > 1 ? 'n' : ''} vigente{sameArea.length > 1 ? 's' : ''}.
            {:else}
              Al crear esta, {sameArea.length > 1 ? 'las recetas' : 'la receta'} {sameArea.map((r) => '#' + String(r.folio).padStart(6, '0')).join(', ')} de esta área {sameArea.length > 1 ? 'dejarán' : 'dejará'} de ser válida{sameArea.length > 1 ? 's' : ''}. Márcala como complementaria si solo añade a la anterior.
            {/if}
          </p>
        </div>
      {/if}

      <div class="grid gap-4 sm:grid-cols-2">
        <div><label class="label" for="rx-next">Próxima cita</label><input id="rx-next" type="date" class="field" min={today} bind:value={nextVisit} /></div>
        <div>
          <label class="label" for="rx-valid">Vigencia (días)</label>
          <input id="rx-valid" type="number" class="field" min="1" max="30" bind:value={validDays} />
          <p class="hint">Máximo 30 días.</p>
        </div>
      </div>

      {#if live && !pending && (live.interactions.length || live.chronic.length || live.allergies.length)}
        <section class="space-y-2 rounded-xl border border-app-ink/12 bg-app-elevated p-3.5 text-sm" aria-label="Alertas de seguridad de la receta">
          {#each live.allergies as a}<p class="flex items-start gap-2 text-app-danger"><Icon name="alert" size={16} /><span><strong>Alergia:</strong> {a.message}</span></p>{/each}
          {#each live.interactions as h}
            <p class="flex items-start gap-2 {h.severity === 'grave' ? 'text-app-danger' : 'text-app-warning'}"><Icon name="alert" size={16} /><span><strong>{h.severity === 'grave' ? 'Interacción grave' : 'Interacción moderada'}:</strong> {h.drug} + {h.with} ({h.with_source}). {h.message}</span></p>
          {/each}
          {#if live.chronic.length}<p class="text-app-muted"><strong class="text-app-ink">Ya toma:</strong> {live.chronic.map((m) => m.name + (m.dose ? ` ${m.dose}` : '')).join(' · ')}</p>{/if}
        </section>
      {/if}
      {#if pending}
        <div class="space-y-3 rounded-xl border border-app-danger/40 bg-app-danger/8 p-4" role="alertdialog" aria-labelledby="rx-pend-t">
          <p id="rx-pend-t" class="flex items-center gap-2 font-medium text-app-danger"><Icon name="alert" size={18} />{pending.kind === 'allergy' ? 'Posible alergia del paciente' : pending.kind === 'interaction' ? 'Interacción grave entre medicamentos' : 'Dosis por encima del máximo de referencia'}</p>
          <ul class="list-disc space-y-1 pl-5 text-sm">{#each pending.lines as l}<li>{l}</li>{/each}</ul>
          <div>
            <label class="label" for="rx-reason">Motivo para continuar</label>
            <textarea id="rx-reason" class="field" rows="2" maxlength="300" bind:value={reasonText} placeholder={pending.kind === 'allergy' ? 'Ej. Tolera el medicamento, documentado' : pending.kind === 'interaction' ? 'Ej. INR controlado; beneficio mayor al riesgo' : 'Ej. Dosis validada para este paciente'}></textarea>
            <p class="hint">Se guarda en la receta y en la bitácora de auditoría.</p>
          </div>
          <div class="flex flex-wrap gap-2">
            <button type="button" class="btn-primary" onclick={confirmPending}>Continuar con este motivo</button>
            <button type="button" class="btn-secondary" onclick={() => (pending = null)}>Volver a editar</button>
          </div>
        </div>
      {/if}
      {#if needCedula}
        <div class="alert" role="alert">
          <Icon name="alert" size={18} />
          <span>Para emitir recetas necesitas registrar tu cédula profesional y la institución que expidió tu título. <a href="/cuenta" class="underline">Registra tu cédula</a></span>
        </div>
      {:else if error}
        <Alert>{error}</Alert>
      {:else if op.phase === 'error'}
        <Alert>{op.message}</Alert>
      {/if}
    </form>
  {/if}
  {#snippet footer()}
    {#if created}
      <button type="button" class="btn-secondary" onclick={onclose}>Cerrar</button>
      <button type="button" class="btn-primary" disabled={printing} onclick={print}><Icon name="receipt" size={16} />Imprimir</button>
    {:else}
      <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
      <button type="submit" form="rx-form" class="btn-primary" disabled={op.phase === 'loading'}>{#if op.phase === 'loading'}<span class="spin"></span>{/if}{instr ? 'Crear hoja' : 'Crear receta'}</button>
    {/if}
  {/snippet}
</Modal>
