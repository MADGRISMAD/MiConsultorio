<script lang="ts">
  import OpError from '$lib/components/ui/OpError.svelte';
  import { labApi } from '$lib/api/lab';
  import { Op } from '$lib/op.svelte';
  import type { Patient } from '$lib/types';
  import type { LabScan } from '$lib/types/lab';
  import Modal from '../Modal.svelte';
  import Icon from '../ui/Icon.svelte';

  interface Props {
    open: boolean;
    patient: Patient;
    onclose: () => void;
    /** the reading, to review in the capture window */
    onread: (scan: LabScan) => void;
  }
  let { open, patient, onclose, onread }: Props = $props();

  const op = new Op();
  let file = $state<File | null>(null);
  let input = $state<HTMLInputElement>();

  $effect(() => {
    if (!open) return;
    op.reset();
    file = null;
    if (input) input.value = '';
  });

  /** Photos are shrunk to what is needed to read text (long side 2200 px, JPEG); PDFs go as they are. */
  async function encode(f: File): Promise<{ data: string; mime: string }> {
    if (f.type === 'application/pdf') {
      if (f.size > 8 * 1024 * 1024) throw new Error('El PDF pesa demasiado (máximo 8 MB).');
      return { data: await toBase64(f), mime: f.type };
    }
    if (!f.type.startsWith('image/')) throw new Error('Elige una foto (JPG, PNG o WebP) o un PDF.');
    const bmp = await createImageBitmap(f).catch(() => {
      throw new Error('No se pudo leer la imagen. Prueba con otra.');
    });
    const scale = Math.min(1, 2200 / Math.max(bmp.width, bmp.height));
    const canvas = document.createElement('canvas');
    canvas.width = Math.max(1, Math.round(bmp.width * scale));
    canvas.height = Math.max(1, Math.round(bmp.height * scale));
    const ctx = canvas.getContext('2d')!;
    ctx.fillStyle = '#fff';
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    ctx.drawImage(bmp, 0, 0, canvas.width, canvas.height);
    bmp.close?.();
    return { data: canvas.toDataURL('image/jpeg', 0.85).split(',')[1], mime: 'image/jpeg' };
  }
  const toBase64 = (f: File) =>
    new Promise<string>((res, rej) => {
      const r = new FileReader();
      r.onload = () => res(String(r.result).split(',')[1] ?? '');
      r.onerror = () => rej(new Error('No se pudo leer el archivo.'));
      r.readAsDataURL(f);
    });

  async function read() {
    const f = file;
    if (!f) return op.fail('Elige la foto o el PDF del reporte.');
    let scan: LabScan | undefined;
    const ok = await op.run(async () => {
      const { data, mime } = await encode(f);
      scan = await labApi.scan(patient.id, data, mime);
      if (!scan.results.length) throw new Error('No encontramos resultados en el documento. Prueba con una foto más nítida y completa.');
    });
    if (ok && scan) onread(scan);
  }
</script>

<Modal {open} title="Escanear resultados con IA" {onclose}>
  <div class="grid gap-4">
    <p class="text-sm text-app-muted">Sube la foto o el PDF del reporte que trajo el paciente. La IA lee los resultados, sin importar el formato ni el orden del laboratorio, y los pone en la captura para que los revises y los guardes.</p>
    <label class="flex cursor-pointer flex-col items-center gap-2 rounded-2xl border-2 border-dashed border-app-ink/20 px-4 py-8 text-center transition hover:border-app-primary">
      <Icon name="upload" size={28} class="text-app-primary" />
      <span class="font-medium">{file ? file.name : 'Elegir foto o PDF'}</span>
      <span class="text-xs text-app-muted">JPG, PNG, WebP o PDF · en el celular puedes tomar la foto al momento</span>
      <input bind:this={input} type="file" accept="image/*,application/pdf" class="sr-only" onchange={(e) => { file = e.currentTarget.files?.[0] ?? null; op.reset(); }} />
    </label>
    <p class="rounded-xl bg-app-warning/10 px-3.5 py-2.5 text-xs text-app-ink">
      Revisa siempre cada valor contra el documento antes de guardar: la IA puede equivocarse. El archivo se envía al servicio de IA para leerlo y no se guarda; no se manda el nombre del paciente. Cuenta como un uso de magia de tu plan.
    </p>
    <OpError {op} />
  </div>
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={onclose}>Cancelar</button>
    <button type="button" class="btn-primary" disabled={!file || op.phase === 'loading'} onclick={read}>
      {#if op.phase === 'loading'}<span class="spin"></span>Leyendo…{:else}<Icon name="sparkles" size={16} />Leer resultados{/if}
    </button>
  {/snippet}
</Modal>
