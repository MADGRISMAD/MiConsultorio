<script lang="ts">
  import { Loader } from '$lib/loader.svelte';
  import OpError from '$lib/components/ui/OpError.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import { dateTime as fmtDate } from '$lib/format';
  import { onMount } from 'svelte';
  import { growthApi } from '$lib/api/lab';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { GrowthImportPreview, GrowthImportRecord } from '$lib/types/lab';
  import Icon from '../ui/Icon.svelte';
  import LoadingRows from '../ui/LoadingRows.svelte';
  import GrowthTableDetail from './GrowthTableDetail.svelte';

  let imports = $state<GrowthImportRecord[] | null>(null);
  let viewing = $state<GrowthImportRecord | null>(null);
  const clean = (s: string) => s.replace(/\s*https?:\/\/\S+/g, '').trim(); // sin direcciones web
  const ld = new Loader('No se pudieron cargar las tablas.');
  let standard = $state('');
  let source = $state('');
  let fileName = $state('');
  let csv = $state('');
  let preview = $state<GrowthImportPreview | null>(null);
  let fileInput = $state<HTMLInputElement>();
  const previewOp = new Op();
  const saveOp = new Op();

  const INDICATOR: Record<string, string> = {
    weight_for_age: 'Peso para la edad',
    length_height_for_age: 'Talla/longitud para la edad',
    bmi_for_age: 'IMC para la edad',
    head_circumference_for_age: 'Perímetro cefálico para la edad'
  };

  async function load() {
    await ld.run(async () => {
      imports = await growthApi.imports();
    });
  }
  onMount(load);

  async function pick(e: Event) {
    const f = (e.currentTarget as HTMLInputElement).files?.[0];
    preview = null;
    if (!f) return;
    if (f.size > 6 << 20) {
      csv = '';
      fileName = '';
      previewOp.fail('El archivo es demasiado grande (máximo 6 MB).');
      return;
    }
    fileName = f.name;
    csv = await f.text();
  }
  const body = () => ({ standard: standard.trim(), source_name: source.trim(), file_name: fileName, csv });

  async function check(e: SubmitEvent) {
    e.preventDefault();
    if (!standard.trim()) return previewOp.fail('Escribe el nombre del estándar (por ejemplo OMS o CDC).');
    if (!source.trim()) return previewOp.fail('Escribe de dónde obtuviste las tablas (nombre y dirección).');
    if (!csv) return previewOp.fail('Elige el archivo CSV.');
    preview = null;
    await previewOp.run(async () => (preview = await growthApi.preview(body())));
  }
  async function confirm() {
    if (!preview?.valid) return;
    if (await saveOp.run(() => growthApi.confirm(body()))) {
      toast.show(`Tablas «${standard.trim()}» cargadas`);
      preview = null;
      csv = '';
      fileName = '';
      if (fileInput) fileInput.value = '';
      await load();
    }
  }
  const sexLabel = (s: string) => (s === 'M' ? 'Hombres / niños' : 'Mujeres / niñas');
</script>

<section aria-labelledby="gr-set">
  <h3 id="gr-set" class="section-title mb-3">Tablas de crecimiento (OMS / CDC)</h3>
  <div class="grid max-w-3xl grid-cols-[minmax(0,1fr)] gap-4">
    <p class="text-sm text-app-muted">
      Caresia ya trae cargadas las tablas de la <strong>OMS</strong> (0 a 5 años) y del <strong>CDC</strong> (0 a 20 años), así que no necesitas subir nada para ver percentiles y valores Z en la pestaña Crecimiento. Carga un archivo solo si quieres actualizar o reemplazar una tabla: se guarda como una versión nueva de ese estándar (no se sobrescribe nada), tiene prioridad sobre la que viene incluida en lo que cubra, y queda en la bitácora con su fuente. El formato y los enlaces de descarga están en <code>docs/CRECIMIENTO.md</code>.
    </p>

    <form onsubmit={check} class="grid gap-3 rounded-2xl border border-app-ink/10 p-4">
      <div class="grid gap-3 sm:grid-cols-2">
        <div>
          <label class="label" for="gr-standard">Estándar</label>
          <input id="gr-standard" class="field" bind:value={standard} maxlength="40" placeholder="OMS" autocomplete="off" oninput={() => (preview = null)} />
        </div>
        <div>
          <label class="label" for="gr-file">Archivo CSV</label>
          <input id="gr-file" bind:this={fileInput} type="file" accept=".csv,text/csv" class="field" onchange={pick} />
        </div>
        <div class="sm:col-span-2">
          <label class="label" for="gr-source">Fuente de las tablas</label>
          <input id="gr-source" class="field" bind:value={source} maxlength="200" placeholder="Ej. OMS, Patrones de crecimiento infantil 2006 (who.int/tools/child-growth-standards)" autocomplete="off" oninput={() => (preview = null)} />
        </div>
      </div>
      <OpError op={previewOp} />
      <div><button class="btn-secondary" disabled={previewOp.phase === 'loading'}>{#if previewOp.phase === 'loading'}<span class="spin"></span>{/if}Revisar archivo</button></div>
    </form>

    {#if preview}
      <div class="rounded-2xl border border-app-ink/10 p-4" aria-live="polite">
        {#if preview.valid}
          <p class="mb-2 flex items-center gap-2 font-medium"><Icon name="check" size={18} />El archivo es válido: {preview.rows} filas, se guardará como «{preview.standard}» versión {preview.next_version}.</p>
          <p class="mb-3 text-xs text-app-muted">Método: {preview.has_lms ? 'L, M y S (Z exacto)' : 'percentiles'}. Huella SHA-256: <span class="break-all font-mono">{preview.sha256}</span></p>
          <div class="overflow-x-auto">
            <table class="w-full min-w-[28rem] text-left text-sm">
              <thead><tr><th class="th !px-2">Indicador</th><th class="th !px-2">Sexo</th><th class="th !px-2">Filas</th><th class="th !px-2">Edades (meses)</th></tr></thead>
              <tbody>
                {#each preview.groups as g (g.indicator + g.sex)}
                  <tr class="border-t border-app-ink/10"><td class="td !px-2 !py-2">{INDICATOR[g.indicator] ?? g.indicator}</td><td class="td !px-2 !py-2">{sexLabel(g.sex)}</td><td class="td !px-2 !py-2">{g.rows}</td><td class="td !px-2 !py-2">{g.min_age_months} – {g.max_age_months}</td></tr>
                {/each}
              </tbody>
            </table>
          </div>
          <details class="mt-3 text-sm">
            <summary class="cursor-pointer text-app-muted">Ver las primeras filas</summary>
            <pre class="mt-2 overflow-x-auto rounded-xl bg-app-ink/5 p-3 text-xs">{preview.sample.map((r) => JSON.stringify(r)).join('\n')}</pre>
          </details>
          <OpError op={saveOp} class="mt-3" />
          <div class="mt-4 flex flex-wrap gap-2">
            <button type="button" class="btn-primary" disabled={saveOp.phase === 'loading'} onclick={confirm}>{#if saveOp.phase === 'loading'}<span class="spin"></span>{/if}Confirmar y cargar</button>
            <button type="button" class="btn-ghost" onclick={() => (preview = null)}>Descartar</button>
          </div>
        {:else}
          <Alert>El archivo tiene {preview.error_count} {preview.error_count === 1 ? 'error' : 'errores'}: corrígelos y vuelve a revisarlo. No se guardó nada.</Alert>
          <ul class="mt-3 grid gap-1 text-sm">
            {#each preview.errors as er}<li><span class="font-mono text-xs text-app-muted">{er.line ? `Línea ${er.line}` : 'Archivo'}</span> · {er.message}</li>{/each}
          </ul>
          {#if preview.error_count > preview.errors.length}<p class="mt-2 text-xs text-app-muted">Se muestran los primeros {preview.errors.length}.</p>{/if}
        {/if}
      </div>
    {/if}

    <div>
      <h4 class="section-title mb-2">Tablas disponibles</h4>
      {#if ld.error}
        <Alert>{ld.error}</Alert>
      {:else if !imports}
        <LoadingRows />
      {:else if imports.length === 0}
        <p class="text-sm text-app-muted">Aún no hay tablas disponibles.</p>
      {:else}
        <div class="overflow-x-auto">
          <table class="w-full text-left text-sm">
            <thead><tr><th class="th !px-2">Estándar</th><th class="th !px-2">Fuente</th><th class="th !px-2 whitespace-nowrap">Origen</th></tr></thead>
            <tbody>
              {#each imports as i (i.id)}
                <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_noninteractive_element_interactions -->
                <tr class="cursor-pointer border-t border-app-ink/10 transition hover:bg-app-ink/4" onclick={() => (viewing = i)}>
                  <td class="td !px-2 !py-2 font-medium"><button type="button" class="text-left underline decoration-app-ink/25 underline-offset-4 hover:decoration-current" aria-label="Ver qué cubre la tabla {i.standard}, versión {i.version}" onclick={(e) => { e.stopPropagation(); viewing = i; }}>{i.standard}</button><span class="block text-xs font-normal text-app-muted">versión {i.version} · {i.row_count} filas</span></td>
                  <td class="td !px-2 !py-2 break-words">{clean(i.source_name)}{#if i.file_name}<span class="block text-xs text-app-muted">{i.file_name}</span>{/if}</td>
                  <td class="td !px-2 !py-2">{#if i.platform}<span class="inline-flex items-center gap-1 whitespace-nowrap rounded-full bg-app-primary/10 px-2.5 py-1 text-xs font-semibold text-app-primary"><Icon name="check" size={13} stroke={2.4} />De Caresia</span>{:else}{fmtDate(i.created_at)}<span class="block text-xs text-app-muted">{i.created_by_name}</span>{/if}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </div>
  </div>
</section>

<GrowthTableDetail table={viewing} onclose={() => (viewing = null)} />
