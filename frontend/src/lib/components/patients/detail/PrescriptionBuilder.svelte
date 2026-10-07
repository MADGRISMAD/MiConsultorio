<script lang="ts">
  import { api, ApiError } from '$lib/api';
  import { Op } from '$lib/op.svelte';
  import { printReceta } from '$lib/print';
  import { toast } from '$lib/toast.svelte';
  import type { Patient, PatientSchema, Prescription, RxControl, RxItem } from '$lib/types';
  import Modal from '../../Modal.svelte';
  import Icon from '../../ui/Icon.svelte';

  interface Props {
    open: boolean;
    patient: Patient;
    schema: PatientSchema;
    encounterId?: string;
    diagnosis?: string;
    onclose: () => void;
    oncreated: (rx: Prescription) => void;
  }
  let { open, patient, schema, encounterId, diagnosis: diagnosisSeed = '', onclose, oncreated }: Props = $props();

  const instr = $derived(schema.rx_mode === 'instructions');
  const CONTROLS: RxControl[] = ['No', 'Antibiótico', 'Fracción III', 'Fracción I o II'];
  const FREQ = ['Cada 4 horas', 'Cada 6 horas', 'Cada 8 horas', 'Cada 12 horas', 'Cada 24 horas', 'Una vez al día', 'Dosis única', 'Solo si hay dolor o molestia'];
  const DUR = ['3 días', '5 días', '7 días', '10 días', '14 días', 'Tratamiento continuo'];

  const blank = (): RxItem => ({ medicine: '', brand: '', presentation: '', dose: '', route: schema.routes?.[0] ?? '', frequency: '', duration: '', quantity: '', notes: '', control: 'No' });
  let diagnosis = $state('');
  let items = $state<RxItem[]>([blank()]);
  let instructions = $state('');
  let nextVisit = $state('');
  let validDays = $state(30);
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
      instructions = nextVisit = error = '';
      validDays = 30;
      needCedula = false;
      created = null;
      op.reset();
    }
    wasOpen = open;
  });

  const today = new Date().toISOString().slice(0, 10);

  async function submit(ev: SubmitEvent) {
    ev.preventDefault();
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
        rx = await api.patients.createPrescription(patient.id, {
          ...(encounterId ? { encounter_id: encounterId } : {}),
          diagnosis: diagnosis.trim(),
          items: instr ? [] : items.map((i) => ({ ...i, medicine: i.medicine.trim() })),
          instructions: instructions.trim(),
          ...(nextVisit ? { next_visit: nextVisit } : {}),
          valid_days: validDays
        });
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
  const remove = (n: number) => (items = items.length > 1 ? items.filter((_, i) => i !== n) : items);
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
        <input id="rx-dx" class="field" bind:value={diagnosis} autocomplete="off" />
      </div>

      {#if instr}
        <div>
          <label class="label" for="rx-ins">Indicaciones</label>
          <textarea id="rx-ins" class="field min-h-52" rows="9" bind:value={instructions}></textarea>
        </div>
      {:else}
        <datalist id="rx-freq">{#each FREQ as f}<option value={f}></option>{/each}</datalist>
        <datalist id="rx-dur">{#each DUR as f}<option value={f}></option>{/each}</datalist>
        <div class="space-y-3">
          <p class="section-title">Medicamentos</p>
          <p class="hint">Escribe siempre la <strong>denominación genérica</strong> (sustancia activa). La marca es opcional.</p>
          {#each items as it, n (n)}
            <fieldset class="rounded-2xl border border-app-ink/10 bg-app-elevated/40 p-4">
              <legend class="flex items-center gap-2 px-1 text-sm font-medium">Medicamento {n + 1}</legend>
              <div class="grid gap-3 sm:grid-cols-2">
                <div class="sm:col-span-2">
                  <label class="label" for="rx-med-{n}">Denominación genérica *</label>
                  <input id="rx-med-{n}" class="field" bind:value={it.medicine} autocomplete="off" placeholder="Ej. Paracetamol" />
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
          <button type="button" class="btn-secondary" onclick={() => (items = [...items, blank()])}><Icon name="plus" size={16} />Agregar medicamento</button>
        </div>
        <div>
          <label class="label" for="rx-ins">Indicaciones generales</label>
          <textarea id="rx-ins" class="field min-h-24" rows="3" bind:value={instructions} placeholder="Reposo, hidratación, datos de alarma…"></textarea>
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

      {#if needCedula}
        <div class="alert" role="alert">
          <Icon name="alert" size={18} />
          <span>Para emitir recetas necesitas registrar tu cédula profesional y la institución que expidió tu título. <a href="/cuenta" class="underline">Registra tu cédula</a></span>
        </div>
      {:else if error}
        <p class="alert" role="alert"><Icon name="alert" size={18} />{error}</p>
      {:else if op.phase === 'error'}
        <p class="alert" role="alert"><Icon name="alert" size={18} />{op.message}</p>
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
