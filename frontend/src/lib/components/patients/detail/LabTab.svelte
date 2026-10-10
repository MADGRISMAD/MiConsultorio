<script lang="ts">
  import { Loader } from '$lib/loader.svelte';
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onMount } from 'svelte';
  import { filesApi } from '$lib/api/files';
  import { labApi } from '$lib/api/lab';
  import { Op } from '$lib/op.svelte';
  import { printLabOrder, printLabReport } from '$lib/print';
  import { session } from '$lib/session.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { Patient, PatientSchema } from '$lib/types';
  import type { Attachment } from '$lib/types/files';
  import type { LabCatalog, LabOrder, LabResult, LabScan, LabStatus, LabTrends } from '$lib/types/lab';
  import ConfirmModal from '../../ConfirmModal.svelte';
  import LabCaptureModal from '../../lab/LabCaptureModal.svelte';
  import LabCorrectModal from '../../lab/LabCorrectModal.svelte';
  import LabScanModal from '../../lab/LabScanModal.svelte';
  import LabTrendChart from '../../lab/LabTrendChart.svelte';
  import { dateFmt, FLAG_LABEL, FLAG_MARK, FLAG_TONE, rangeText } from '../../lab/labUtil';
  import Modal from '../../Modal.svelte';
  import EmptyState from '../../ui/EmptyState.svelte';
  import Icon from '../../ui/Icon.svelte';
  import Pill from '../../ui/Pill.svelte';

  let { patient, canWrite }: { patient: Patient; schema: PatientSchema | null; canWrite: boolean; isAdmin: boolean } = $props();

  let orders = $state<LabOrder[]>([]);
  let catalog = $state<LabCatalog | null>(null);
  let files = $state<Attachment[]>([]);
  const ld = new Loader('No se pudo cargar el laboratorio.');
  let view = $state<'ordenes' | 'tendencias'>('ordenes');
  let showHistory = $state(false);
  // each order opens and closes: the newest starts open, the rest closed (a record is not always needed on screen)
  let openOrders = $state<Record<string, boolean>>({});
  const isOpen = (o: LabOrder, i: number) => openOrders[o.id] ?? i === 0;
  const toggle = (o: LabOrder, i: number) => (openOrders[o.id] = !isOpen(o, i));
  const setAll = (v: boolean) => (openOrders = Object.fromEntries(orders.map((o) => [o.id, v])));
  const current = (o: LabOrder) => o.results.filter((r) => !r.superseded_by);
  const outOfRange = (o: LabOrder) => current(o).filter((r) => r.flag !== 'normal' && r.flag !== 'na').length;

  const STATUS: Record<LabStatus, { label: string; tone: 'info' | 'ok' | 'warn' | 'bad' | 'muted' }> = {
    solicitado: { label: 'Solicitado', tone: 'info' },
    parcial: { label: 'Parcial', tone: 'warn' },
    completo: { label: 'Completo', tone: 'ok' },
    cancelado: { label: 'Cancelado', tone: 'muted' }
  };

  async function load() {
    await ld.run(async () => {
        orders = await labApi.orders(patient.id);
    });
  }
  onMount(() => {
    load();
    labApi.catalog().then((c) => (catalog = c)).catch(() => {});
    filesApi.list(patient.id).then((r) => (files = [...r.files].sort((a, b) => Number(b.kind === 'lab') - Number(a.kind === 'lab')))).catch(() => {});
  });

  const replace = (o: LabOrder) => {
    orders = orders.some((x) => x.id === o.id) ? orders.map((x) => (x.id === o.id ? o : x)) : [o, ...orders];
    trendsLoaded = false;
  };
  const fileOf = (id: string | null) => files.find((f) => f.id === id);
  const historyCount = $derived(orders.reduce((n, o) => n + o.results.filter((r) => r.superseded_by).length, 0));

  // ---- capture ----
  let capture = $state<{ order: LabOrder | null; seed?: LabScan | null } | null>(null);
  // ---- scan a report with AI: the reading opens the capture window to review it ----
  let scanning = $state<{ order: LabOrder | null } | null>(null);
  function scanned(sc: LabScan) {
    const order = scanning?.order ?? null;
    scanning = null;
    capture = { order, seed: sc };
  }
  function saved(o: LabOrder) {
    capture = null;
    replace(o);
    toast.show('Resultados guardados');
  }

  // ---- correction ----
  let correcting = $state<{ order: LabOrder; result: LabResult } | null>(null);
  function corrected(o: LabOrder) {
    correcting = null;
    replace(o);
    toast.show('Corrección guardada; el valor anterior queda en el historial');
  }

  // ---- status / cancel ----
  const statusOp = new Op();
  async function markComplete(o: LabOrder) {
    if (await statusOp.run(async () => replace(await labApi.setStatus(o.id, 'completo')))) toast.show('Orden completa');
    else toast.show(statusOp.message, 'error');
  }
  let cancelling = $state<LabOrder | null>(null);
  let cancelReason = $state('');
  const cancelOp = new Op();
  async function confirmCancel() {
    const o = cancelling;
    if (!o) return;
    if (!cancelReason.trim()) return cancelOp.fail('Escribe el motivo de la cancelación.');
    if (await cancelOp.run(async () => replace(await labApi.setStatus(o.id, 'cancelado', cancelReason.trim())))) {
      cancelling = null;
      toast.show('Orden cancelada');
    }
  }

  // ---- edit data ----
  let editing = $state<LabOrder | null>(null);
  let form = $state({ title: '', lab_name: '', notes: '', attachment_id: '' });
  const editOp = new Op();
  function openEdit(o: LabOrder) {
    form = { title: o.title, lab_name: o.lab_name, notes: o.notes, attachment_id: o.attachment_id ?? '' };
    editOp.reset();
    editing = o;
  }
  async function saveEdit() {
    const o = editing;
    if (!o) return;
    if (!form.title.trim()) return editOp.fail('Escribe el nombre del estudio.');
    if (await editOp.run(async () => replace(await labApi.updateOrder(o.id, { ...form, title: form.title.trim() })))) {
      editing = null;
      toast.show('Orden actualizada');
    }
  }

  // ---- trends ----
  let trends = $state<LabTrends | null>(null);
  let trendsLoaded = false;
  let analyte = $state('');
  let trendError = $state('');
  async function loadTrends(name = analyte) {
    try {
      const t = await labApi.trends(patient.id, name);
      trends = t;
      if (!name && t.analytes.length) {
        analyte = t.analytes[0].analyte;
        trends = await labApi.trends(patient.id, analyte);
      }
      trendsLoaded = true;
      trendError = '';
    } catch (e) {
      trendError = e instanceof Error ? e.message : 'No se pudieron cargar las tendencias.';
    }
  }
  $effect(() => {
    if (view === 'tendencias' && !trendsLoaded) loadTrends(analyte);
  });
  const trendUnit = $derived(trends?.points.length ? (trends.points[trends.points.length - 1].unit ?? '') : '');

  // ---- print ----
  let printing = $state(false);
  async function print() {
    printing = true;
    try {
      await printLabReport(patient, orders, catalog?.notice ?? '');
    } catch (e) {
      toast.show(e instanceof Error ? e.message : 'No se pudo imprimir.', 'error');
    } finally {
      printing = false;
    }
  }

  async function printOrder(o: LabOrder) {
    const pro = session.user?.professional;
    try {
      await printLabOrder(patient, o, catalog, { name: session.user?.name ?? '', title: pro?.title ?? '', cedula: pro?.cedula ?? '', institution: pro?.institution ?? '' });
    } catch (e) {
      toast.show(e instanceof Error ? e.message : 'No se pudo imprimir.', 'error');
    }
  }

  function groups(o: LabOrder): [string, LabResult[]][] {
    const m = new Map<string, LabResult[]>();
    for (const r of o.results) {
      if (r.superseded_by && !showHistory) continue;
      const k = r.panel || 'Otros análisis';
      m.set(k, [...(m.get(k) ?? []), r]);
    }
    return [...m.entries()].map(([k, v]) => [catalog?.panels.find((p) => p.id === k)?.name ?? k, v]);
  }
</script>

{#if ld.loading}
  <div class="card h-48 animate-pulse"></div>
{:else if ld.error}
  <Alert>{ld.error}</Alert>
{:else}
  <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
    <div role="group" aria-label="Vista de laboratorio" class="inline-flex rounded-full bg-app-ink/6 p-1">
      {#each [['ordenes', 'Órdenes y resultados'], ['tendencias', 'Tendencias']] as [k, label] (k)}
        <button type="button" aria-pressed={view === k} class="min-h-9 rounded-full px-4 text-sm font-medium transition {view === k ? 'bg-app-panel text-app-ink shadow-app' : 'text-app-muted hover:text-app-ink'}" onclick={() => (view = k as typeof view)}>{label}</button>
      {/each}
    </div>
    <div class="flex flex-wrap gap-2">
      <button type="button" class="btn-secondary" disabled={printing || orders.length === 0} onclick={print}><Icon name="receipt" size={18} />Imprimir resultados</button>
      {#if canWrite && !patient.archived_at}
        {#if session.magic}<button type="button" class="btn-secondary" onclick={() => (scanning = { order: null })}><Icon name="sparkles" size={18} />Escanear resultados</button>{/if}
        <button type="button" class="btn-primary" onclick={() => (capture = { order: null })}><Icon name="plus" size={18} />Nueva orden o captura</button>
      {/if}
    </div>
  </div>

  {#if view === 'ordenes'}
    {#if orders.length === 0}
      <div class="card"><EmptyState icon="droplet" title="Sin estudios de laboratorio" text="Registra una orden y captura sus resultados por panel: el sistema marca los valores fuera de rango y arma las tendencias.">
        {#if canWrite && !patient.archived_at}<button type="button" class="btn-primary" onclick={() => (capture = { order: null })}><Icon name="plus" size={18} />Nueva orden o captura</button>{/if}
      </EmptyState></div>
    {:else}
      {#if historyCount > 0}
        <label class="mb-3 flex cursor-pointer items-center gap-2 text-sm text-app-muted">
          <input type="checkbox" class="h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={showHistory} />
          Mostrar historial de correcciones ({historyCount})
        </label>
      {/if}
      {#if orders.length > 1}
        <div class="mb-3 flex gap-3 text-sm">
          <button type="button" class="font-medium text-app-primary underline underline-offset-4" onclick={() => setAll(true)}>Expandir todo</button>
          <button type="button" class="font-medium text-app-primary underline underline-offset-4" onclick={() => setAll(false)}>Contraer todo</button>
        </div>
      {/if}
      <ul class="grid grid-cols-[minmax(0,1fr)] gap-4">
        {#each orders as o, i (o.id)}
          {@const open = isOpen(o, i)}
          {@const att = fileOf(o.attachment_id)}
          <li class="card p-4 sm:p-5 {o.status === 'cancelado' ? 'opacity-80' : ''}">
            <div class="flex flex-wrap items-start justify-between gap-3">
              <button type="button" class="flex min-w-0 flex-1 items-start gap-2 rounded-lg text-left" aria-expanded={open} aria-controls="lab-o-{o.id}" onclick={() => toggle(o, i)}>
                <Icon name="chevron-down" size={18} class="mt-1.5 shrink-0 text-app-muted transition {open ? '' : '-rotate-90'}" />
                <span class="min-w-0">
                  <span class="block break-words text-lg font-medium">{o.title}</span>
                  <span class="mt-0.5 block text-sm text-app-muted">
                    {dateFmt(o.ordered_at)}{o.lab_name ? ` · ${o.lab_name}` : ''}{o.ordered_by_name ? ` · ${o.ordered_by_name}` : ''}
                  </span>
                  {#if !open}
                    <span class="mt-1 block text-sm text-app-muted">
                      {#if current(o).length}{current(o).length} resultado{current(o).length === 1 ? '' : 's'}{#if outOfRange(o)} · <strong class="font-semibold text-app-warning">{outOfRange(o)} fuera de rango</strong>{/if}{:else if o.requested.length}{o.requested.length} estudio{o.requested.length === 1 ? '' : 's'} solicitado{o.requested.length === 1 ? '' : 's'}{:else}Sin resultados{/if}
                    </span>
                  {/if}
                </span>
              </button>
              <Pill tone={STATUS[o.status].tone}>{STATUS[o.status].label}</Pill>
            </div>

            {#if open}
            <div id="lab-o-{o.id}">
              {#if o.notes}<p class="mt-2 text-sm">{o.notes}</p>{/if}
              {#if o.requested.length}<p class="mt-2 text-sm text-app-muted"><span class="font-medium text-app-ink">Estudios solicitados:</span> {o.requested.join(' · ')}</p>{/if}
              {#if o.attachment_id}
                <p class="mt-1 text-sm">
                  <a class="inline-flex items-center gap-1.5 font-medium text-app-primary underline" href={filesApi.url(o.attachment_id)} target="_blank" rel="noopener"><Icon name="file" size={14} />{att ? att.title || att.original_name : 'Ver archivo del resultado'}</a>
                </p>
              {/if}
              {#if o.status === 'cancelado'}<p class="mt-1 text-sm text-app-muted">Cancelada el {o.cancelled_at ? dateFmt(o.cancelled_at) : ''} por {o.cancelled_by}: {o.cancel_reason}</p>{/if}
            {#each groups(o) as [panel, rows] (panel)}
              <div class="mt-4 overflow-x-auto">
                <table class="w-full min-w-[30rem] text-left text-sm">
                  <caption class="section-title pb-1 text-left">{panel}</caption>
                  <thead><tr><th class="th !px-2">Análisis</th><th class="th !px-2">Resultado</th><th class="th !px-2">Referencia</th><th class="th !px-2">Indicador</th>{#if canWrite}<th class="th !px-2"><span class="sr-only">Acciones</span></th>{/if}</tr></thead>
                  <tbody>
                    {#each rows as r (r.id)}
                      <tr class="border-t border-app-ink/10 {r.superseded_by ? 'text-app-muted' : ''}">
                        <td class="td !px-2 !py-2 {r.superseded_by ? 'line-through' : ''}">{r.analyte}</td>
                        <td class="td !px-2 !py-2 font-medium {r.superseded_by ? 'line-through' : ''}">{r.value_num ?? r.value_text} <span class="font-normal text-app-muted">{r.unit}</span></td>
                        <td class="td !px-2 !py-2">{rangeText(r.ref_low, r.ref_high) || '—'}{#if r.ref_source}<span class="block text-xs text-app-muted">{r.ref_source === 'catalogo' ? 'general' : 'del laboratorio'}</span>{/if}</td>
                        <td class="td !px-2 !py-2">
                          {#if r.superseded_by}<span class="text-xs">Reemplazado</span>
                          {:else if r.flag !== 'na'}<Pill tone={FLAG_TONE[r.flag]}>{FLAG_MARK[r.flag]} {FLAG_LABEL[r.flag]}</Pill>
                          {:else}<span class="text-xs text-app-muted">—</span>{/if}
                          {#if r.supersedes_id}<span class="block text-xs text-app-muted">Corrección: {r.notes}</span>{/if}
                        </td>
                        {#if canWrite}
                          <td class="td !px-2 !py-2 text-right">
                            {#if !r.superseded_by && o.status !== 'cancelado' && !patient.archived_at}<button type="button" class="btn-ghost !min-h-8 !px-3" onclick={() => (correcting = { order: o, result: r })}>Corregir</button>{/if}
                          </td>
                        {/if}
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            {:else}
              {#if o.status !== 'cancelado'}<p class="mt-3 text-sm text-app-muted">Aún no hay resultados capturados.</p>{/if}
            {/each}

            {#if canWrite && o.status !== 'cancelado' && !patient.archived_at}
              <div class="mt-4 flex flex-wrap gap-2">
                <button type="button" class="btn-secondary" onclick={() => (capture = { order: o })}><Icon name="plus" size={16} />Capturar resultados</button>
                {#if session.magic}<button type="button" class="btn-secondary" onclick={() => (scanning = { order: o })}><Icon name="sparkles" size={16} />Escanear resultados</button>{/if}
                {#if o.status !== 'completo' && o.results.length > 0}<button type="button" class="btn-secondary" disabled={statusOp.phase === 'loading'} onclick={() => markComplete(o)}><Icon name="check" size={16} />Marcar completa</button>{/if}
                <button type="button" class="btn-ghost" onclick={() => openEdit(o)}><Icon name="edit" size={16} />Editar datos</button>
                <button type="button" class="btn-ghost" onclick={() => { cancelReason = ''; cancelOp.reset(); cancelling = o; }}><Icon name="ban" size={16} />Cancelar orden</button>
              </div>
            {/if}
            <div class="mt-3 flex flex-wrap gap-2 border-t border-app-ink/8 pt-3">
              <button type="button" class="btn-ghost" onclick={() => printOrder(o)}><Icon name="receipt" size={16} />Imprimir orden para el laboratorio</button>
            </div>
            </div>
            {/if}
          </li>
        {/each}
      </ul>
      <p class="mt-4 text-xs text-app-muted">Los resultados no se editan ni se borran: un error se corrige con una nueva captura y el valor anterior queda en el historial. Las órdenes se cancelan con un motivo.</p>
    {/if}
  {:else}
    <div class="card p-4 sm:p-5">
      {#if trendError}
        <Alert>{trendError}</Alert>
      {:else if !trends}
        <div class="h-48 animate-pulse"></div>
      {:else if trends.analytes.length === 0}
        <EmptyState icon="chart" title="Aún no hay valores numéricos" text="Cuando captures resultados con valor numérico podrás ver cómo evolucionan en el tiempo." />
      {:else}
        <div class="mb-4 max-w-sm">
          <label class="label" for="lab-trend">Análisis</label>
          <select id="lab-trend" class="field" bind:value={analyte} onchange={() => loadTrends(analyte)}>
            {#each trends.analytes as a (a.analyte)}<option value={a.analyte}>{a.analyte} ({a.count})</option>{/each}
          </select>
        </div>
        {#if trends.points.length > 0}
          <LabTrendChart points={trends.points} {analyte} unit={trendUnit} />
        {/if}
      {/if}
    </div>
  {/if}
{/if}

<LabCaptureModal open={!!capture} {patient} {catalog} {files} order={capture?.order ?? null} seed={capture?.seed ?? null} onclose={() => (capture = null)} onsaved={saved} />
<LabScanModal open={!!scanning} {patient} onclose={() => (scanning = null)} onread={scanned} />
<LabCorrectModal target={correcting} onclose={() => (correcting = null)} onsaved={corrected} />

<ConfirmModal open={!!cancelling} title="Cancelar orden de laboratorio" op={cancelOp} onconfirm={confirmCancel} onclose={() => (cancelling = null)} confirmLabel="Cancelar orden">
  <p>La orden <strong class="text-app-ink">no se borra</strong>: queda como cancelada con el motivo y sus resultados dejan de contar en las tendencias.</p>
  <label class="label mt-4" for="lab-cancel">Motivo</label>
  <input id="lab-cancel" class="field" bind:value={cancelReason} autocomplete="off" maxlength="300" placeholder="Ej. Se solicitó en el paciente equivocado" />
</ConfirmModal>

<Modal open={!!editing} title="Editar datos de la orden" onclose={() => (editing = null)}>
  <form id="lab-edit" onsubmit={(e) => { e.preventDefault(); saveEdit(); }} class="grid gap-3">
    <div>
      <label class="label" for="le-title">Estudio</label>
      <input id="le-title" class="field" bind:value={form.title} maxlength="160" autocomplete="off" />
    </div>
    <div>
      <label class="label" for="le-lab">Laboratorio</label>
      <input id="le-lab" class="field" bind:value={form.lab_name} maxlength="120" autocomplete="off" />
    </div>
    <div>
      <label class="label" for="le-att">Archivo del resultado</label>
      <select id="le-att" class="field" bind:value={form.attachment_id}>
        <option value="">Sin archivo</option>
        {#each files as f (f.id)}<option value={f.id}>{f.title || f.original_name}</option>{/each}
      </select>
    </div>
    <div>
      <label class="label" for="le-notes">Notas</label>
      <input id="le-notes" class="field" bind:value={form.notes} maxlength="1000" autocomplete="off" />
    </div>
    <OpError op={editOp} />
  </form>
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={() => (editing = null)}>Cancelar</button>
    <button type="submit" form="lab-edit" class="btn-primary" disabled={editOp.phase === 'loading'}>Guardar</button>
  {/snippet}
</Modal>
