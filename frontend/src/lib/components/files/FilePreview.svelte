<script lang="ts">
  import { filesApi } from '$lib/api/files';
  import type { Attachment } from '$lib/types/files';
  import Modal from '../Modal.svelte';
  import Icon from '../ui/Icon.svelte';
  import DicomViewer from './DicomViewer.svelte';
  import { isDicom, isImage, isPdf, kindLabel } from './util';

  let { file, onclose }: { file: Attachment | null; onclose: () => void } = $props();

  let zoom = $state(1);
  let turn = $state(0);
  $effect(() => {
    void file?.id;
    zoom = 1;
    turn = 0;
  });
  const clamp = (z: number) => Math.min(5, Math.max(0.25, z));
</script>

<Modal open={!!file} title={file?.title || 'Archivo'} {onclose} wide>
  {#if file}
    <p class="mb-3 text-sm text-app-muted">{kindLabel(file.kind)} · {file.original_name}</p>
    {#if isImage(file.mime)}
      <div class="mb-3 flex flex-wrap items-center gap-1" role="toolbar" aria-label="Controles de imagen">
        <button type="button" class="icon-btn" aria-label="Acercar" onclick={() => (zoom = clamp(zoom * 1.25))}><Icon name="zoom-in" /></button>
        <button type="button" class="icon-btn" aria-label="Alejar" onclick={() => (zoom = clamp(zoom / 1.25))}><Icon name="zoom-out" /></button>
        <button type="button" class="icon-btn" aria-label="Girar a la derecha" onclick={() => (turn = (turn + 90) % 360)}><Icon name="rotate" /></button>
        <button type="button" class="btn-ghost" onclick={() => ((zoom = 1), (turn = 0))}>Restablecer</button>
        <span class="ml-auto text-xs text-app-muted" aria-live="polite">{Math.round(zoom * 100)}%</span>
      </div>
      <div class="max-h-[62dvh] overflow-auto rounded-2xl bg-black/90 p-2">
        <div class="grid min-h-[40dvh] place-items-center">
          <img
            src={filesApi.url(file.id)}
            alt={file.title || file.original_name}
            class="max-w-full origin-center transition-transform"
            style="transform: rotate({turn}deg) scale({zoom});"
          />
        </div>
      </div>
    {:else if isPdf(file.mime)}
      <iframe title={file.title || file.original_name} src={filesApi.url(file.id)} class="h-[68dvh] w-full rounded-2xl border border-app-ink/10 bg-white"></iframe>
    {:else if isDicom(file.mime)}
      <DicomViewer fileId={file.id} />
    {:else}
      <p class="alert"><Icon name="info" size={18} />Este tipo de archivo no tiene vista previa. Descárgalo para abrirlo.</p>
    {/if}
    {#if file.note}<p class="mt-3 whitespace-pre-line text-sm">{file.note}</p>{/if}
  {/if}
  {#snippet footer()}
    {#if file}
      <a class="btn-secondary" href={filesApi.url(file.id, true)} download={file.original_name}><Icon name="download" size={18} />Descargar</a>
    {/if}
    <button type="button" class="btn-primary" onclick={onclose}>Cerrar</button>
  {/snippet}
</Modal>
