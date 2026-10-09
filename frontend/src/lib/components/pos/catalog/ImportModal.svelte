<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import { api } from '$lib/api';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import { moneyCents } from '$lib/format';
  import Modal from '$lib/components/Modal.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { parseCatalogText } from './helpers';

  interface Props {
    open: boolean;
    ondone: () => void;
    onclose: () => void;
  }
  let { open, ondone, onclose }: Props = $props();

  const uid = $props.id();
  const op = new Op();
  let text = $state('');
  let skipped = $state<string[]>([]);
  let done = $state(false);

  $effect(() => {
    if (open) {
      text = '';
      skipped = [];
      done = false;
      op.reset();
    }
  });

  const rows = $derived(parseCatalogText(text));
  const valid = $derived(rows.filter((r) => !r.errors.length));
  const invalid = $derived(rows.length - valid.length);

  async function onfile(e: Event) {
    const f = (e.currentTarget as HTMLInputElement).files?.[0];
    if (f) text = await f.text();
    (e.currentTarget as HTMLInputElement).value = '';
  }

  async function confirm() {
    let res: { created: number; skipped: string[] } | undefined;
    const ok = await op.run(async () => {
      res = await api.pos.importItems(valid.map((r) => r.input));
    });
    if (!ok || !res) return;
    skipped = res.skipped ?? [];
    toast.show(res.created === 1 ? '1 artículo importado' : `${res.created} artículos importados`);
    ondone();
    if (skipped.length) done = true;
    else onclose();
  }
</script>

<Modal {open} title="Importar artículos" {onclose} wide>
  {#if done}
    <div class="space-y-3" aria-live="polite">
      <p class="text-sm">Se importaron los artículos, pero estos <strong>{skipped.length}</strong> se omitieron (probablemente ya existían):</p>
      <ul class="max-h-60 list-disc space-y-1 overflow-y-auto rounded-xl bg-app-elevated py-3 pl-8 pr-4 text-sm">
        {#each skipped as s}<li>{s}</li>{/each}
      </ul>
    </div>
  {:else}
    <div class="space-y-4">
      <p class="text-sm text-app-muted">
        Copia las filas desde Excel o Google Sheets (o pega un CSV). Columnas, en este orden:
        <span class="font-mono text-xs text-app-ink">tipo, nombre, categoría, precio, costo, existencia, unidad, código de barras</span>.
        La fila de encabezado es opcional; “tipo” es <em>servicio</em> o <em>producto</em>.
      </p>
      <div>
        <div class="mb-1.5 flex items-center justify-between gap-2">
          <label class="label !mb-0" for="{uid}-t">Datos</label>
          <label class="btn-ghost min-h-8 cursor-pointer px-3 text-xs">
            <Icon name="plus" size={14} />Cargar archivo .csv
            <input type="file" accept=".csv,.tsv,.txt,text/csv,text/plain" class="sr-only" onchange={onfile} />
          </label>
        </div>
        <textarea id="{uid}-t" class="field font-mono text-xs" rows="6" bind:value={text} spellcheck="false" placeholder={'servicio\tConsulta general\tConsultas\t500\t\t\tsesión\nproducto\tGasas 10x10\tMaterial\t45,50\t30\t120\tcaja'}></textarea>
      </div>

      {#if rows.length}
        <div aria-live="polite" class="flex flex-wrap gap-2 text-sm">
          <span class="pill pill-ok">{valid.length} listos</span>
          {#if invalid}<span class="pill pill-bad">{invalid} con errores (no se importarán)</span>{/if}
        </div>
        <div class="overflow-x-auto rounded-2xl border border-app-ink/10">
          <table class="w-full min-w-[40rem]">
            <thead class="border-b border-app-ink/10 bg-app-elevated">
              <tr>
                <th class="th">#</th><th class="th">Tipo</th><th class="th">Nombre</th><th class="th">Categoría</th>
                <th class="th text-right">Precio</th><th class="th text-right">Costo</th><th class="th text-right">Exist.</th><th class="th">Revisión</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-app-ink/8">
              {#each rows.slice(0, 200) as r}
                <tr class={r.errors.length ? 'bg-app-danger/6' : ''}>
                  <td class="td text-app-muted">{r.line}</td>
                  <td class="td">{r.input.kind === 'product' ? 'Producto' : 'Servicio'}</td>
                  <td class="td font-medium">{r.input.name || '—'}</td>
                  <td class="td text-app-muted">{r.input.category || '—'}</td>
                  <td class="td text-right tabular-nums">{moneyCents(r.input.price_cents)}</td>
                  <td class="td text-right tabular-nums">{r.input.cost_cents ? moneyCents(r.input.cost_cents) : '—'}</td>
                  <td class="td text-right tabular-nums">{r.input.track_stock ? r.input.stock : '—'}</td>
                  <td class="td">
                    {#if r.errors.length}<span class="text-app-danger">{r.errors.join('. ')}</span>{:else}<span class="pill pill-ok">Listo</span>{/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
        {#if rows.length > 200}<p class="hint">Mostrando las primeras 200 filas de {rows.length}.</p>{/if}
      {/if}

      <OpError op={op} />
    </div>
  {/if}
  {#snippet footer()}
    {#if done}
      <button type="button" class="btn-primary" onclick={onclose}>Listo</button>
    {:else}
      <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
      <button type="button" class="btn-primary" disabled={!valid.length || op.phase === 'loading'} onclick={confirm}>
        {#if op.phase === 'loading'}<span class="spin"></span>{/if}Importar {valid.length || ''} {valid.length === 1 ? 'artículo' : 'artículos'}
      </button>
    {/if}
  {/snippet}
</Modal>
