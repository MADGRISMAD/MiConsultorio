<script lang="ts">
  import dicomParser from 'dicom-parser';
  import { filesApi } from '$lib/api/files';
  import Icon from '../ui/Icon.svelte';

  let { fileId }: { fileId: string } = $props();

  const UNCOMPRESSED: Record<string, boolean> = {
    '1.2.840.10008.1.2': true, // implicit VR little endian
    '1.2.840.10008.1.2.1': true, // explicit VR little endian
    '1.2.840.10008.1.2.2': true // explicit VR big endian
  };

  let canvas = $state<HTMLCanvasElement>();
  let status = $state<'loading' | 'ready' | 'unsupported' | 'error'>('loading');
  let message = $state('');
  let info = $state<{ rows: number; cols: number; patient: string; modality: string } | null>(null);

  let values = $state.raw<Float32Array | null>(null); // monochrome values after rescale
  let rgb: Uint8ClampedArray | null = null; // RGB frames are shown as they are
  let dims = { w: 0, h: 0 };
  let invert = false;
  let min = $state(0);
  let max = $state(255);
  let center = $state(128);
  let width = $state(256);

  function str(ds: dicomParser.DataSet, tag: string) {
    return (ds.string(tag) ?? '').trim();
  }

  async function load() {
    status = 'loading';
    try {
      const buf = await filesApi.bytes(fileId);
      const ds = dicomParser.parseDicom(new Uint8Array(buf));
      const ts = str(ds, 'x00020010') || '1.2.840.10008.1.2';
      if (!UNCOMPRESSED[ts]) {
        status = 'unsupported';
        message = 'Este estudio DICOM está comprimido y no se puede mostrar aquí. Descárgalo para verlo en un visor DICOM.';
        return;
      }
      const rows = ds.uint16('x00280010') ?? 0;
      const cols = ds.uint16('x00280011') ?? 0;
      const bits = ds.uint16('x00280100') ?? 8;
      const signed = (ds.uint16('x00280103') ?? 0) === 1;
      const samples = ds.uint16('x00280002') ?? 1;
      const photometric = str(ds, 'x00280004');
      const px = ds.elements.x7fe00010;
      if (!rows || !cols || !px || (bits !== 8 && bits !== 16) || (samples !== 1 && samples !== 3)) {
        status = 'unsupported';
        message = 'No se puede mostrar este formato DICOM. Descárgalo para verlo en un visor DICOM.';
        return;
      }
      const littleEndian = ts !== '1.2.840.10008.1.2.2';
      const total = rows * cols;
      const dv = new DataView(ds.byteArray.buffer, ds.byteArray.byteOffset + px.dataOffset, Math.min(px.length, ds.byteArray.length - px.dataOffset));
      const need = total * samples * (bits / 8);
      if (dv.byteLength < need) throw new Error('El estudio está incompleto.');
      dims = { w: cols, h: rows };
      info = { rows, cols, patient: '', modality: str(ds, 'x00080060') }; // patient name is deliberately not shown
      if (samples === 3) {
        if (bits !== 8 || (ds.uint16('x00280006') ?? 0) !== 0) {
          status = 'unsupported';
          message = 'No se puede mostrar este formato DICOM a color. Descárgalo para verlo en un visor DICOM.';
          return;
        }
        rgb = new Uint8ClampedArray(total * 4);
        for (let i = 0; i < total; i++) {
          rgb[i * 4] = dv.getUint8(i * 3);
          rgb[i * 4 + 1] = dv.getUint8(i * 3 + 1);
          rgb[i * 4 + 2] = dv.getUint8(i * 3 + 2);
          rgb[i * 4 + 3] = 255;
        }
        status = 'ready';
        await Promise.resolve();
        draw();
        return;
      }
      const slope = parseFloat(str(ds, 'x00281053')) || 1;
      const intercept = parseFloat(str(ds, 'x00281052')) || 0;
      const v = new Float32Array(total);
      let lo = Infinity;
      let hi = -Infinity;
      for (let i = 0; i < total; i++) {
        const raw = bits === 8 ? (signed ? dv.getInt8(i) : dv.getUint8(i)) : signed ? dv.getInt16(i * 2, littleEndian) : dv.getUint16(i * 2, littleEndian);
        const x = raw * slope + intercept;
        v[i] = x;
        if (x < lo) lo = x;
        if (x > hi) hi = x;
      }
      values = v;
      invert = photometric === 'MONOCHROME1';
      min = lo;
      max = hi;
      const wc = parseFloat(str(ds, 'x00281050').split('\\')[0]);
      const ww = parseFloat(str(ds, 'x00281051').split('\\')[0]);
      center = Number.isFinite(wc) ? wc : (lo + hi) / 2;
      width = Number.isFinite(ww) && ww > 0 ? ww : Math.max(1, hi - lo);
      status = 'ready';
      await Promise.resolve();
      draw();
    } catch (e) {
      status = 'error';
      message = e instanceof Error ? e.message : 'No se pudo abrir el estudio DICOM.';
    }
  }

  function draw() {
    if (!canvas) return;
    canvas.width = dims.w;
    canvas.height = dims.h;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    const img = ctx.createImageData(dims.w, dims.h);
    if (rgb) {
      img.data.set(rgb);
    } else if (values) {
      const lo = center - 0.5 - (width - 1) / 2;
      const span = Math.max(1, width - 1);
      for (let i = 0; i < values.length; i++) {
        let g = ((values[i] - lo) / span) * 255;
        g = g < 0 ? 0 : g > 255 ? 255 : g;
        if (invert) g = 255 - g;
        const o = i * 4;
        img.data[o] = img.data[o + 1] = img.data[o + 2] = g;
        img.data[o + 3] = 255;
      }
    }
    ctx.putImageData(img, 0, 0);
  }

  $effect(() => {
    void center;
    void width;
    if (status === 'ready') draw();
  });

  $effect(() => {
    void fileId;
    load();
  });

  const span = $derived(Math.max(1, max - min));
  function reset() {
    center = (min + max) / 2;
    width = span;
  }
</script>

{#if status === 'loading'}
  <p class="py-10 text-center text-sm text-app-muted" role="status"><span class="spin"></span> Cargando estudio…</p>
{:else if status === 'ready'}
  <div class="grid place-items-center overflow-hidden rounded-2xl bg-black">
    <canvas bind:this={canvas} class="max-h-[60dvh] max-w-full" aria-label="Imagen DICOM{info?.modality ? ` (${info.modality})` : ''}"></canvas>
  </div>
  {#if values}
    <div class="mt-4 grid gap-3 sm:grid-cols-[1fr_1fr_auto] sm:items-end">
      <label class="block text-sm">
        <span class="label">Nivel (brillo)</span>
        <input type="range" class="w-full accent-[rgb(var(--app-primary))]" min={min} max={max} step={span > 100 ? 1 : span / 100} bind:value={center} />
      </label>
      <label class="block text-sm">
        <span class="label">Ventana (contraste)</span>
        <input type="range" class="w-full accent-[rgb(var(--app-primary))]" min="1" max={span * 2} step={span > 100 ? 1 : span / 100} bind:value={width} />
      </label>
      <button type="button" class="btn-secondary" onclick={reset}><Icon name="refresh" size={16} />Restablecer</button>
    </div>
  {/if}
  {#if info}<p class="mt-2 text-xs text-app-muted">{info.cols} × {info.rows} px{info.modality ? ` · ${info.modality}` : ''}. Visor básico: para diagnóstico usa un visor DICOM.</p>{/if}
{:else}
  <p class="alert" role={status === 'error' ? 'alert' : 'status'}><Icon name="alert" size={18} />{message}</p>
{/if}
