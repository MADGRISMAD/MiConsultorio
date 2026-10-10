<script lang="ts">
  import { untrack } from 'svelte';
  import OpError from '$lib/components/ui/OpError.svelte';
  import { filesApi } from '$lib/api/files';
  import { labApi } from '$lib/api/lab';
  import { Op } from '$lib/op.svelte';
  import type { Attachment } from '$lib/types/files';
  import type { Patient } from '$lib/types';
  import type { LabCatalog, LabFlag, LabOrder, LabResultInput, LabScan } from '$lib/types/lab';
  import Modal from '../Modal.svelte';
  import Icon from '../ui/Icon.svelte';
  import { catalogBound, computeFlag, FLAG_LABEL, FLAG_MARK, FLAG_TONE, parseNum, rangeText } from './labUtil';

  interface Props {
    open: boolean;
    patient: Patient;
    catalog: LabCatalog | null;
    files: Attachment[];
    /** Adds results to this order; without it a new order is created. */
    order: LabOrder | null;
    /** what the AI read from a report: fills the rows to review */
    seed?: LabScan | null;
    /** the scanned report: it goes to the patient's Archivos when the results are saved */
    scanFile?: File | null;
    onclose: () => void;
    onsaved: (o: LabOrder, warning?: string) => void;
  }
  let { open, patient, catalog, files, order, seed = null, scanFile = null, onclose, onsaved }: Props = $props();

  interface Row {
    key: number;
    panel: string;
    panelName: string;
    name: string;
    unit: string;
    kind: 'num' | 'text';
    value: string;
    low: string;
    high: string;
    /** the person typed a range: it prevails over the catalog */
    edited: boolean;
    hasCatalog: boolean;
    expected: string;
    textFlag: LabFlag;
    custom: boolean;
  }

  const op = new Op();
  let title = $state('');
  let labName = $state('');
  let notes = $state('');
  let attachment = $state('');
  let resultedOn = $state('');
  let complete = $state(false);
  let rows = $state<Row[]>([]);
  let panelPick = $state('');
  let customName = $state('');
  let seq = 0;

  const today = () => {
    const d = new Date();
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
  };
  const animal = $derived(patient.subject === 'animal');
  const species = $derived(String(patient.profile?.species ?? patient.profile?.especie ?? ''));
  const panels = $derived(
    (catalog?.panels ?? []).filter((p) => (animal ? p.audience === 'animal' && (!species || !p.species || p.species === species || !['Perro', 'Gato'].includes(species)) : p.audience === 'person'))
  );

  $effect(() => {
    if (!open) return;
    op.reset();
    title = order?.title ?? '';
    labName = order?.lab_name ?? '';
    notes = '';
    attachment = order?.attachment_id ?? '';
    resultedOn = today();
    complete = false;
    rows = [];
    panelPick = '';
    customName = '';
    const sc = seed;
    if (sc) untrack(() => applySeed(sc));
  });

  const norm = (s: string) => s.toLowerCase().normalize('NFD').replace(/[\u0300-\u036f]/g, '').replace(/[^a-z0-9]+/g, '');
  /** The rows the AI read: an analyte the catalog knows goes in its panel (with its unit and range if the report has none); the rest as manual rows. */
  function applySeed(sc: LabScan) {
    if (!order && sc.study) title = sc.study;
    if (!order && sc.lab_name) labName = sc.lab_name;
    if (sc.date) resultedOn = sc.date;
    const known = new Map<string, { p: LabCatalog['panels'][number]; a: LabCatalog['panels'][number]['analytes'][number] }>();
    for (const p of panels) for (const a of p.analytes) if (!known.has(norm(a.name))) known.set(norm(a.name), { p, a });
    for (const x of sc.results) {
      const hit = known.get(norm(x.analyte));
      const isNum = x.value_num != null;
      const hasRange = x.ref_low != null || x.ref_high != null;
      const b = hit && hit.a.kind === 'num' ? catalogBound(hit.a, patient.sex) : null;
      const textual = !isNum || hit?.a.kind === 'text';
      rows.push({
        key: ++seq,
        panel: hit ? hit.p.id : '',
        panelName: hit ? hit.p.name : x.section || 'Otros análisis',
        name: hit ? hit.a.name : x.analyte,
        unit: x.unit || hit?.a.unit || '',
        kind: textual ? 'text' : 'num',
        value: isNum ? String(x.value_num) : x.value_text,
        low: hasRange ? (x.ref_low != null ? String(x.ref_low) : '') : b?.low != null ? String(b.low) : '',
        high: hasRange ? (x.ref_high != null ? String(x.ref_high) : '') : b?.high != null ? String(b.high) : '',
        edited: hasRange,
        hasCatalog: !!b && (b.low != null || b.high != null),
        expected: hit?.a.expected ?? '',
        textFlag: textual ? (x.flag === 'anormal' ? 'anormal' : 'na') : 'na',
        custom: !hit
      });
    }
  }

  function addPanel(id: string) {
    const p = catalog?.panels.find((x) => x.id === id);
    if (!p) return;
    if (!title.trim() && !order) title = p.name;
    for (const a of p.analytes) {
      if (rows.some((r) => r.panel === p.id && r.name === a.name)) continue;
      const b = a.kind === 'num' ? catalogBound(a, patient.sex) : null;
      rows.push({
        key: ++seq,
        panel: p.id,
        panelName: p.name,
        name: a.name,
        unit: a.unit,
        kind: a.kind,
        value: '',
        low: b?.low != null ? String(b.low) : '',
        high: b?.high != null ? String(b.high) : '',
        edited: false,
        hasCatalog: b != null && (b.low != null || b.high != null),
        expected: a.expected ?? '',
        textFlag: 'na',
        custom: false
      });
    }
    panelPick = '';
  }
  function addCustom() {
    const name = customName.trim();
    if (!name) return;
    rows.push({ key: ++seq, panel: '', panelName: 'Otros análisis', name, unit: '', kind: 'num', value: '', low: '', high: '', edited: false, hasCatalog: false, expected: '', textFlag: 'na', custom: true });
    customName = '';
  }
  const remove = (key: number) => (rows = rows.filter((r) => r.key !== key));

  function flagOf(r: Row): LabFlag {
    if (r.kind === 'text') return r.value.trim() ? r.textFlag : 'na';
    return computeFlag(parseNum(r.value), parseNum(r.low), parseNum(r.high));
  }
  const filled = $derived(rows.filter((r) => r.value.trim() !== ''));
  const groups = $derived.by(() => {
    const m = new Map<string, Row[]>();
    for (const r of rows) m.set(r.panelName, [...(m.get(r.panelName) ?? []), r]);
    return [...m.entries()];
  });

  function buildResults(): LabResultInput[] | string {
    const out: LabResultInput[] = [];
    for (const r of filled) {
      const at = resultedOn ? `${resultedOn}T12:00:00` : undefined;
      const base = { panel: r.panel || r.panelName, analyte: r.name, unit: r.unit.trim(), resulted_at: at ? new Date(at).toISOString() : undefined };
      if (r.kind === 'text') {
        out.push({ ...base, value_text: r.value.trim(), flag: r.textFlag });
        continue;
      }
      const v = parseNum(r.value);
      if (v == null) return `El valor de «${r.name}» no es un número.`;
      const low = parseNum(r.low);
      const high = parseNum(r.high);
      if ((r.low.trim() && low == null) || (r.high.trim() && high == null)) return `El rango de «${r.name}» no es válido.`;
      if (low != null && high != null && low > high) return `El rango de «${r.name}» está invertido.`;
      out.push({ ...base, value_num: v, ref_low: low, ref_high: high, ref_source: low == null && high == null ? '' : r.edited || !r.hasCatalog ? 'laboratorio' : 'catalogo' });
    }
    return out;
  }

  async function save() {
    const results = buildResults();
    if (typeof results === 'string') return op.fail(results);
    if (!order && !title.trim()) return op.fail('Escribe el nombre del estudio.');
    if (results.length === 0) return op.fail('Captura al menos un resultado: para solicitar estudios usa «Nueva orden».');
    // what this order asks for: its panels and the single analytes added by hand (the printed sheet lists them)
    const requested = [...new Set(rows.filter((r) => !seed).map((r) => (r.panel ? r.panelName : r.name)))];
    let saved: LabOrder | undefined;
    let warning = '';
    const ok = await op.run(async () => {
      // the scanned report is an outside document: it is kept in Archivos (as «Laboratorio») and linked to the order
      let archived = '';
      if (scanFile) {
        try {
          const day = resultedOn ? new Date(`${resultedOn}T12:00:00`).toLocaleDateString('es-MX', { day: 'numeric', month: 'short', year: 'numeric' }) : '';
          const f = await filesApi.upload(patient.id, scanFile, { kind: 'lab', title: `Resultados: ${(order?.title || title).trim() || 'laboratorio'}${day ? ` · ${day}` : ''}`, note: 'Reporte del laboratorio escaneado', encounter_id: '' }, () => {});
          archived = f.id;
        } catch (e) {
          warning = `Los resultados se guardaron, pero el documento no se pudo archivar: ${e instanceof Error ? e.message : 'inténtalo de nuevo desde Archivos'}.`;
        }
      }
      if (order) {
        saved = await labApi.addResults(order.id, results, complete);
        if (archived && !order.attachment_id) saved = await labApi.updateOrder(order.id, { title: order.title, lab_name: order.lab_name, notes: order.notes, attachment_id: archived });
      } else {
        saved = await labApi.createOrder(patient.id, { title: title.trim(), lab_name: labName.trim(), notes: notes.trim(), attachment_id: archived || attachment, requested, results, complete });
      }
    });
    if (ok && saved) onsaved(saved, warning);
  }
</script>

<Modal {open} title={order ? `Capturar resultados · ${order.title}` : 'Registrar resultados del paciente'} {onclose} wide>
  <form id="lab-capture" onsubmit={(e) => { e.preventDefault(); save(); }} class="grid gap-4">
    {#if seed}
      <p class="flex items-start gap-2 rounded-xl bg-app-primary/10 px-3.5 py-2.5 text-sm" role="status"><Icon name="sparkles" size={18} class="mt-0.5 shrink-0 text-app-primary" /><span>La IA leyó {seed.results.length} resultados del reporte. <strong>Revisa cada valor contra el documento</strong>, corrige lo que haga falta y guarda.</span></p>
    {/if}
    {#if catalog}
      <p class="rounded-xl bg-app-warning/10 px-3.5 py-2.5 text-xs text-app-ink">{catalog.notice} Si tu laboratorio imprime su propio rango, escríbelo en la fila: ese rango siempre prevalece.</p>
    {/if}

    {#if !order}
      <div class="grid gap-3 sm:grid-cols-2">
        <div class="sm:col-span-2">
          <label class="label" for="lab-title">Estudio</label>
          <input id="lab-title" class="field" bind:value={title} maxlength="160" placeholder="Ej. Química sanguínea de control" autocomplete="off" />
        </div>
        <div>
          <label class="label" for="lab-lab">Laboratorio (opcional)</label>
          <input id="lab-lab" class="field" bind:value={labName} maxlength="120" autocomplete="off" />
        </div>
        <div>
          <label class="label" for="lab-att">Archivo del resultado (opcional)</label>
          <select id="lab-att" class="field" bind:value={attachment}>
            <option value="">Sin archivo</option>
            {#each files as f (f.id)}<option value={f.id}>{f.title || f.original_name}</option>{/each}
          </select>
        </div>
        <div class="sm:col-span-2">
          <label class="label" for="lab-notes">Indicaciones o notas (opcional)</label>
          <input id="lab-notes" class="field" bind:value={notes} maxlength="1000" autocomplete="off" />
        </div>
      </div>
    {/if}

    <div class="grid items-end gap-3 sm:grid-cols-[1fr_auto_10rem]">
      <div>
        <label class="label" for="lab-panel">Agregar un panel</label>
        <div class="flex gap-2">
          <select id="lab-panel" class="field" bind:value={panelPick}>
            <option value="">Elige un panel…</option>
            {#each panels as p (p.id)}<option value={p.id}>{p.name}</option>{/each}
          </select>
          <button type="button" class="btn-secondary shrink-0" disabled={!panelPick} onclick={() => addPanel(panelPick)}><Icon name="plus" size={16} />Agregar</button>
        </div>
      </div>
      <div class="hidden sm:block"></div>
      <div>
        <label class="label" for="lab-date">Fecha del resultado</label>
        <input id="lab-date" type="date" class="field" bind:value={resultedOn} max={today()} />
      </div>
    </div>

    {#if rows.length === 0}
      <p class="rounded-xl border border-dashed border-app-ink/15 px-4 py-6 text-center text-sm text-app-muted">Elige un panel para capturar sus análisis, o agrega uno manualmente. Solo se guardan las filas con valor.</p>
    {/if}

    {#each groups as [name, list] (name)}
      <fieldset class="m-0 min-w-0 rounded-2xl border border-app-ink/10 p-3">
        <legend class="section-title px-1">{name}</legend>
        <div class="hidden gap-2 px-1 pb-1 text-[11px] text-app-muted sm:grid sm:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)_5rem_5rem_5rem_6rem_2rem]">
          <span>Análisis</span><span>Valor</span><span>Unidad</span><span>Ref. mínima</span><span>Ref. máxima</span><span>Bandera</span><span></span>
        </div>
        {#each list as r (r.key)}
          {@const flag = flagOf(r)}
          <div class="grid grid-cols-2 items-center gap-2 border-t border-app-ink/8 py-2 first:border-t-0 sm:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)_5rem_5rem_5rem_6rem_2rem]">
            <div class="col-span-2 text-sm font-medium sm:col-span-1">{r.name}{#if r.expected}<span class="block text-xs font-normal text-app-muted">Esperado: {r.expected}</span>{/if}</div>
            {#if r.kind === 'text'}
              <div class="flex gap-1.5">
                <input class="field" aria-label="Resultado de {r.name}" bind:value={r.value} maxlength="200" placeholder={r.expected || 'Resultado'} />
              </div>
              <input class="field" aria-label="Unidad de {r.name}" bind:value={r.unit} maxlength="30" />
              <div class="col-span-2 sm:col-span-3">
                <select class="field" aria-label="Valoración de {r.name}" bind:value={r.textFlag}>
                  <option value="na">Sin valoración</option>
                  <option value="normal">Normal</option>
                  <option value="anormal">Alterado</option>
                </select>
              </div>
            {:else}
              <input class="field" inputmode="decimal" aria-label="Valor de {r.name}" bind:value={r.value} placeholder="Valor" />
              <input class="field" aria-label="Unidad de {r.name}" bind:value={r.unit} maxlength="30" />
              <input class="field" inputmode="decimal" aria-label="Rango mínimo de {r.name}" bind:value={r.low} oninput={() => (r.edited = true)} placeholder="mín." />
              <input class="field" inputmode="decimal" aria-label="Rango máximo de {r.name}" bind:value={r.high} oninput={() => (r.edited = true)} placeholder="máx." />
            {/if}
            <div class="min-h-6 text-sm" aria-live="polite">
              {#if r.value.trim()}<span class="pill pill-{FLAG_TONE[flag] === 'muted' ? 'info' : FLAG_TONE[flag]} !bg-transparent">{FLAG_MARK[flag]} {FLAG_LABEL[flag]}</span>{/if}
            </div>
            <button type="button" class="icon-btn danger justify-self-end" aria-label="Quitar {r.name}" onclick={() => remove(r.key)}><Icon name="x" size={16} /></button>
            {#if r.kind === 'num' && (r.low || r.high) && r.value.trim()}
              <p class="col-span-2 text-xs text-app-muted sm:col-span-7">Referencia {r.edited || !r.hasCatalog ? 'del laboratorio' : 'general (catálogo)'}: {rangeText(parseNum(r.low), parseNum(r.high))} {r.unit}</p>
            {/if}
          </div>
        {/each}
      </fieldset>
    {/each}

    <div class="flex flex-wrap items-end gap-2">
      <div class="min-w-48 flex-1">
        <label class="label" for="lab-custom">Análisis manual</label>
        <input id="lab-custom" class="field" bind:value={customName} maxlength="120" placeholder="Nombre del análisis" autocomplete="off" onkeydown={(e) => { if (e.key === 'Enter') { e.preventDefault(); addCustom(); } }} />
      </div>
      <button type="button" class="btn-secondary" disabled={!customName.trim()} onclick={addCustom}><Icon name="plus" size={16} />Agregar fila</button>
    </div>

    <label class="flex cursor-pointer items-center gap-3 text-sm font-medium">
      <input type="checkbox" class="h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={complete} />
      Con esto la orden queda completa
    </label>

    <OpError op={op} />
  </form>
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
    <button type="submit" form="lab-capture" class="btn-primary" disabled={op.phase === 'loading'}>
      {#if op.phase === 'loading'}<span class="spin"></span>{/if}{order ? 'Guardar resultados' : 'Guardar resultados'}
    </button>
  {/snippet}
</Modal>
