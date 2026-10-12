<script lang="ts">
  import { Loader } from '$lib/loader.svelte';
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onMount } from 'svelte';
  import { dateTime } from '$lib/format';
  import { specialtyApi } from '$lib/api/specialty';
  import { Op } from '$lib/op.svelte';
  import { printPeriodontogram } from '$lib/print';
  import { ALL, cleanPerio, emptyPerio, emptyTooth, hasPerioData, LOWER, maxPd, pdTone, perioStats, SITES, UPPER } from '$lib/specialty/perio';
  import { toast } from '$lib/toast.svelte';
  import type { Patient } from '$lib/types';
  import type { PatientChart, PerioData, PerioTooth } from '$lib/types/specialty';
  import MetricChart, { type Series } from '../../specialty/MetricChart.svelte';
  import ViewedVersionBanner from '../../specialty/ViewedVersionBanner.svelte';
  import EmptyState from '../../ui/EmptyState.svelte';
  import Icon from '../../ui/Icon.svelte';

  let { patient, canWrite }: { patient: Patient; canWrite: boolean } = $props();

  const copy = <T,>(v: T): T => JSON.parse(JSON.stringify(v));
  const asData = (c: PatientChart): PerioData => ({ ...emptyPerio(), ...(c.data as unknown as PerioData) });

  const ld = new Loader('No se pudo cargar el periodontograma.');
  const saveOp = new Op();
  let history = $state<PatientChart[]>([]);
  let work = $state<PerioData>(emptyPerio());
  let base = $state('');
  let viewing = $state<string | null>(null);
  let selected = $state('16');
  let note = $state('');

  async function load() {
    await ld.run(async () => {
      const r = await specialtyApi.charts(patient.id, 'periodontogram');
      history = r.charts;
      // a new probing starts from an empty chart: depths are measured again every time
      work = emptyPerio();
      base = JSON.stringify(cleanPerio(work));
    });
  }
  onMount(load);

  const viewed = $derived(viewing ? history.find((h) => h.id === viewing) : undefined);
  const shown = $derived<PerioData>(viewed ? asData(viewed) : work);
  const readonly = $derived(!canWrite || !!viewed);
  const dirty = $derived(JSON.stringify(cleanPerio(work)) !== base);
  const stats = $derived(perioStats(shown));
  const previous = $derived(history[0] ? perioStats(asData(history[0])) : null);

  const tooth = $derived<PerioTooth>(shown.teeth[selected] ?? emptyTooth());
  /** the tooth being edited, created on first touch */
  function mut(): PerioTooth {
    if (!work.teeth[selected]) work.teeth[selected] = emptyTooth();
    return work.teeth[selected];
  }
  function setNum(arr: 'pd' | 'rec', i: number, raw: string) {
    const t = mut();
    const v = raw.trim() === '' ? null : Math.round(Number(raw));
    t[arr][i] = v == null || !Number.isFinite(v) || v < 0 || v > 15 ? null : v;
  }
  const step = (n: number) => (selected = ALL[Math.min(Math.max(ALL.indexOf(selected) + n, 0), ALL.length - 1)]);
  /** Enter on the last site goes to the next tooth: the six depths of a tooth are typed with the keyboard only. */
  function onKey(e: KeyboardEvent, last: boolean) {
    if (e.key === 'Enter' && last) {
      e.preventDefault();
      step(1);
      queueMicrotask(() => document.getElementById('pd-0')?.focus());
    }
  }

  async function save() {
    const data = cleanPerio(work);
    if (!hasPerioData(data)) return saveOp.fail('Anota al menos una medición antes de guardar.');
    if (await saveOp.run(() => specialtyApi.saveChart(patient.id, 'periodontogram', data, note.trim()))) {
      toast.show('Periodontograma guardado');
      note = '';
      await load();
    }
  }

  async function print() {
    const c = viewed ?? history[0];
    const data = viewed ? asData(viewed) : dirty ? cleanPerio(work) : c ? asData(c) : null;
    if (!data) return;
    await printPeriodontogram(patient, { data, note: viewed?.note ?? note, at: viewed?.created_at ?? c?.created_at ?? new Date().toISOString(), by: viewed?.created_by_name ?? c?.created_by_name ?? '' });
  }

  const series = (label: string, f: (s: ReturnType<typeof perioStats>) => number): Series[] => [
    { label, points: [...history].reverse().map((c) => ({ date: c.created_at.slice(0, 10), value: f(perioStats(asData(c))) })) }
  ];
  const toothClass = (k: string) => {
    const t = shown.teeth[k];
    if (t?.absent) return 'bg-app-ink/10 text-app-muted line-through';
    return pdTone(maxPd(t));
  };
  const bleeds = (k: string) => !!shown.teeth[k]?.bop.some(Boolean);
</script>

{#if ld.loading}
  <div class="card h-40 animate-pulse"></div>
{:else if ld.error}
  <Alert>{ld.error}</Alert>
{:else}
  {#if viewed}<ViewedVersionBanner {viewed} back={() => (viewing = null)} backLabel="Volver a una medición nueva" />{/if}

  <div class="card px-4 py-4 sm:px-5">
    <p class="text-sm text-app-muted">Dientes permanentes, seis sitios por diente (distal, medio y mesial por vestibular y por lingual o palatino). Verde hasta 3 mm, amarillo 4 mm y rojo desde 5 mm: bolsa periodontal. {canWrite && !viewed ? 'Escribe las profundidades y pulsa Enter en el último sitio para pasar al siguiente diente.' : ''}</p>

    {#each [{ name: 'Arcada superior', list: UPPER }, { name: 'Arcada inferior', list: LOWER }] as arch}
      <div class="mt-4">
        <p class="section-title">{arch.name}</p>
        <div class="mt-1 grid grid-cols-8 gap-1 sm:grid-cols-[repeat(16,minmax(0,1fr))]">
          {#each arch.list as k (k)}
            <button type="button" onclick={() => (selected = k)} aria-pressed={selected === k} aria-label="Diente {k}"
              class="relative min-h-10 rounded-lg border text-xs font-semibold {toothClass(k)} {selected === k ? 'border-app-primary ring-2 ring-app-primary' : 'border-app-ink/15'}">
              {k}
              {#if bleeds(k)}<span class="absolute right-0.5 top-0.5 h-1.5 w-1.5 rounded-full bg-app-danger" title="Sangrado"></span>{/if}
              {#if shown.teeth[k]?.plaque}<span class="absolute bottom-0.5 right-0.5 h-1.5 w-1.5 rounded-full bg-app-warning" title="Placa"></span>{/if}
            </button>
          {/each}
        </div>
      </div>
    {/each}
    <p class="mt-2 text-xs text-app-muted"><span class="mr-1 inline-block h-2 w-2 rounded-full bg-app-danger"></span>sangrado <span class="ml-3 mr-1 inline-block h-2 w-2 rounded-full bg-app-warning"></span>placa</p>
  </div>

  <div class="card mt-4 px-4 py-4 sm:px-5" aria-labelledby="perio-tooth">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <h3 id="perio-tooth" class="display text-xl">Diente {selected}</h3>
      <div class="flex gap-2">
        <button type="button" class="btn-secondary" onclick={() => step(-1)} aria-label="Diente anterior"><Icon name="arrow-left" size={16} /></button>
        <button type="button" class="btn-secondary" onclick={() => step(1)} aria-label="Diente siguiente"><Icon name="arrow-right" size={16} /></button>
      </div>
    </div>

    {#if tooth.absent}
      <p class="mt-3 text-sm text-app-muted">Diente ausente.{#if !readonly} <button type="button" class="text-app-primary underline" onclick={() => (mut().absent = false)}>Quitar la marca</button>{/if}</p>
    {:else}
      {#each [{ title: 'Vestibular', off: 0 }, { title: 'Lingual / palatino', off: 3 }] as side, si}
        <div class="mt-4">
          <p class="section-title">{side.title}</p>
          <div class="mt-1 grid grid-cols-[6.5rem_repeat(3,minmax(0,1fr))] items-center gap-x-2 gap-y-2 text-sm">
            <span></span>{#each SITES as s}<span class="text-center text-xs text-app-muted">{s}</span>{/each}
            <span class="text-app-muted">Profundidad (mm)</span>
            {#each [0, 1, 2] as j}
              {@const i = side.off + j}
              <input id={i === 0 ? 'pd-0' : undefined} class="field text-center {pdTone(tooth.pd[i])}" inputmode="numeric" maxlength="2" disabled={readonly} value={tooth.pd[i] ?? ''} aria-label="{side.title}, {SITES[j]}: profundidad al sondaje" oninput={(e) => setNum('pd', i, e.currentTarget.value)} onkeydown={(e) => onKey(e, si === 1 && j === 2)} />
            {/each}
            <span class="text-app-muted">Recesión (mm)</span>
            {#each [0, 1, 2] as j}
              {@const i = side.off + j}
              <input class="field text-center" inputmode="numeric" maxlength="2" disabled={readonly} value={tooth.rec[i] ?? ''} aria-label="{side.title}, {SITES[j]}: recesión" oninput={(e) => setNum('rec', i, e.currentTarget.value)} />
            {/each}
            <span class="text-app-muted">Sangrado</span>
            {#each [0, 1, 2] as j}
              {@const i = side.off + j}
              <label class="flex justify-center"><input type="checkbox" class="h-5 w-5" disabled={readonly} checked={tooth.bop[i]} aria-label="{side.title}, {SITES[j]}: sangrado al sondaje" onchange={(e) => (mut().bop[i] = e.currentTarget.checked)} /></label>
            {/each}
          </div>
        </div>
      {/each}

      <div class="mt-4 grid gap-3 sm:grid-cols-3">
        <label class="block"><span class="label">Movilidad</span>
          <select class="field" disabled={readonly} value={tooth.mob} onchange={(e) => (mut().mob = Number(e.currentTarget.value))}>
            <option value={0}>0 · sin movilidad</option><option value={1}>1 · leve</option><option value={2}>2 · moderada</option><option value={3}>3 · severa</option>
          </select></label>
        <label class="block"><span class="label">Furca</span>
          <select class="field" disabled={readonly} value={tooth.furc} onchange={(e) => (mut().furc = Number(e.currentTarget.value))}>
            <option value={0}>0 · sin afectación</option><option value={1}>1 · incipiente</option><option value={2}>2 · parcial</option><option value={3}>3 · completa</option>
          </select></label>
        <div class="flex flex-col justify-end gap-2 text-sm">
          <label class="flex items-center gap-2"><input type="checkbox" class="h-4 w-4" disabled={readonly} checked={tooth.plaque} onchange={(e) => (mut().plaque = e.currentTarget.checked)} />Con placa</label>
          {#if !readonly}<button type="button" class="text-left text-app-primary underline" onclick={() => (mut().absent = true)}>Marcar diente ausente</button>{/if}
        </div>
      </div>
    {/if}
  </div>

  <div class="card mt-4 px-4 py-4 sm:px-5" aria-labelledby="perio-stats">
    <h3 id="perio-stats" class="display text-xl">Resumen</h3>
    <dl class="mt-3 grid grid-cols-2 gap-3 text-sm sm:grid-cols-4">
      <div><dt class="text-xs text-app-muted">Sitios con sangrado</dt><dd class="text-lg font-semibold">{stats.bleedingPct}%</dd></div>
      <div><dt class="text-xs text-app-muted">Dientes con placa</dt><dd class="text-lg font-semibold">{stats.plaquePct}%</dd></div>
      <div><dt class="text-xs text-app-muted">Bolsas de 5 mm o más</dt><dd class="text-lg font-semibold">{stats.pockets5}</dd></div>
      <div><dt class="text-xs text-app-muted">Profundidad promedio</dt><dd class="text-lg font-semibold">{stats.meanPd} mm</dd></div>
      <div><dt class="text-xs text-app-muted">Pérdida de inserción promedio</dt><dd class="text-lg font-semibold">{stats.meanCal} mm</dd></div>
      <div><dt class="text-xs text-app-muted">Bolsas de 4 mm</dt><dd class="text-lg font-semibold">{stats.pockets4}</dd></div>
      <div><dt class="text-xs text-app-muted">Dientes presentes</dt><dd class="text-lg font-semibold">{stats.teeth}</dd></div>
      <div><dt class="text-xs text-app-muted">Sitios medidos</dt><dd class="text-lg font-semibold">{stats.sites}</dd></div>
    </dl>
    {#if !viewed && previous && stats.sites}
      <p class="mt-3 text-sm text-app-muted">Contra la medición anterior: sangrado {stats.bleedingPct - previous.bleedingPct >= 0 ? '+' : ''}{Math.round((stats.bleedingPct - previous.bleedingPct) * 10) / 10} puntos, bolsas de 5 mm o más {stats.pockets5 - previous.pockets5 >= 0 ? '+' : ''}{stats.pockets5 - previous.pockets5}.</p>
    {/if}

    {#if canWrite && !viewed}
      <label class="label mt-4" for="perio-note">Nota de esta medición (opcional)</label>
      <input id="perio-note" class="field" bind:value={note} maxlength="300" placeholder="Por ejemplo: antes del raspado y alisado" />
      <OpError op={saveOp} class="mt-3" />
      <div class="mt-3 flex flex-wrap gap-2">
        <button type="button" class="btn-primary" disabled={!dirty || saveOp.phase === 'loading'} onclick={save}><Icon name="check" size={18} />Guardar medición</button>
        {#if history.length}<button type="button" class="btn-secondary" onclick={print}><Icon name="receipt" size={18} />Imprimir la última</button>{/if}
      </div>
    {:else if history.length}
      <button type="button" class="btn-secondary mt-3" onclick={print}><Icon name="receipt" size={18} />Imprimir</button>
    {/if}
  </div>

  {#if history.length > 1}
    <div class="mt-4 grid gap-4 md:grid-cols-2">
      <MetricChart title="Sitios con sangrado" unit="%" series={series('Sangrado', (s) => s.bleedingPct)} />
      <MetricChart title="Bolsas de 5 mm o más" unit="" decimals={0} series={series('Bolsas', (s) => s.pockets5)} />
    </div>
  {/if}

  <h3 class="display mb-2 mt-6 text-xl">Mediciones guardadas</h3>
  {#if history.length === 0}
    <div class="card"><EmptyState icon="info" title="Sin mediciones" text="Aquí quedan las mediciones por fecha para comparar cómo evolucionan las encías." /></div>
  {:else}
    <ul class="card divide-y divide-app-ink/8">
      {#each history as h (h.id)}
        {@const s = perioStats(asData(h))}
        <li class="flex flex-wrap items-center justify-between gap-2 px-4 py-3 text-sm">
          <span class="min-w-0"><span class="font-medium">{dateTime(h.created_at)}</span><span class="block text-xs text-app-muted">{h.created_by_name} · sangrado {s.bleedingPct}% · {s.pockets5} bolsas ≥5 mm{h.note ? ` · ${h.note}` : ''}</span></span>
          <button type="button" class="btn-secondary" onclick={() => (viewing = viewing === h.id ? null : h.id)}>{viewing === h.id ? 'Cerrar' : 'Ver'}</button>
        </li>
      {/each}
    </ul>
  {/if}
{/if}
