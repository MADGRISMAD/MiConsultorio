<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { filesApi } from '$lib/api/files';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { Encounter, Patient, PatientSchema } from '$lib/types';
  import type { Attachment, FileKind, FilesUsage } from '$lib/types/files';
  import ConfirmModal from '../../ConfirmModal.svelte';
  import FilePreview from '../../files/FilePreview.svelte';
  import UploadModal from '../../files/UploadModal.svelte';
  import { ACCEPT, canPreview, formatBytes, isImage, kindLabel, KINDS } from '../../files/util';
  import EmptyState from '../../ui/EmptyState.svelte';
  import Icon from '../../ui/Icon.svelte';
  import Pill from '../../ui/Pill.svelte';

  let { patient, canWrite, isAdmin }: { patient: Patient; schema: PatientSchema | null; canWrite: boolean; isAdmin: boolean } = $props();

  let files = $state<Attachment[]>([]);
  let usage = $state<FilesUsage | null>(null);
  let maxBytes = $state(15 << 20);
  let encounters = $state<Encounter[]>([]);
  let loading = $state(true);
  let error = $state('');
  let showArchived = $state(false);
  let filter = $state<'all' | FileKind>('all');
  let dragging = $state(false);
  let queue = $state<File[]>([]);
  let preview = $state<Attachment | null>(null);
  let archiving = $state<Attachment | null>(null);
  let reason = $state('');
  const archiveOp = new Op();
  let picker = $state<HTMLInputElement>();
  let camera = $state<HTMLInputElement>();

  async function load() {
    try {
      const r = await filesApi.list(patient.id, showArchived);
      files = r.files;
      usage = r.usage;
      maxBytes = r.max_upload_bytes;
      error = '';
    } catch (e) {
      error = e instanceof Error ? e.message : 'No se pudieron cargar los archivos.';
    } finally {
      loading = false;
    }
  }
  onMount(() => {
    load();
    if (canWrite) api.patients.encounters(patient.id).then((l) => (encounters = l)).catch(() => {});
  });

  function toggleArchived() {
    showArchived = !showArchived;
    loading = true;
    load();
  }

  const visible = $derived(filter === 'all' ? files : files.filter((f) => f.kind === filter));
  const counts = $derived(KINDS.map((k) => ({ ...k, n: files.filter((f) => f.kind === k.value).length })).filter((k) => k.n > 0));
  const pct = $derived(usage ? Math.min(100, (usage.used_bytes / usage.quota_bytes) * 100) : 0);
  const dt = (iso: string) => new Date(iso).toLocaleString('es-MX', { day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false });

  function take(list: FileList | null | undefined) {
    if (list && list.length) queue = Array.from(list);
    if (picker) picker.value = '';
    if (camera) camera.value = '';
  }
  function ondrop(e: DragEvent) {
    e.preventDefault();
    dragging = false;
    if (canWrite && !showArchived) take(e.dataTransfer?.files);
  }

  function startArchive(f: Attachment) {
    archiving = f;
    reason = '';
    archiveOp.reset();
  }
  async function confirmArchive() {
    const f = archiving;
    if (!f) return;
    if (reason.trim().length < 3) return archiveOp.fail('Escribe el motivo para archivar.');
    if (await archiveOp.run(() => filesApi.archive(f.id, reason.trim()))) {
      archiving = null;
      toast.show('Archivo archivado');
      load();
    }
  }
  const linked = (f: Attachment) => {
    const e = encounters.find((x) => x.id === f.encounter_id);
    return e ? new Date(e.occurred_at).toLocaleDateString('es-MX', { day: 'numeric', month: 'short' }) : '';
  };
</script>

<div class="mb-4 flex flex-wrap items-center justify-between gap-3">
  <p class="max-w-xl text-sm text-app-muted">Radiografías, estudios de laboratorio, consentimientos y fotografías. Se guardan cifrados; no se borran, solo se archivan con un motivo.</p>
  <div class="flex flex-wrap gap-2">
    {#if isAdmin}
      <button type="button" class="btn-secondary" aria-pressed={showArchived} onclick={toggleArchived}><Icon name="archive" size={18} />{showArchived ? 'Ver activos' : 'Ver archivados'}</button>
    {/if}
    {#if canWrite && !showArchived}
      <button type="button" class="btn-secondary sm:hidden" onclick={() => camera?.click()}><Icon name="eye" size={18} />Tomar foto</button>
      <button type="button" class="btn-primary" onclick={() => picker?.click()}><Icon name="upload" size={18} />Subir archivo</button>
    {/if}
  </div>
</div>
<input bind:this={picker} type="file" class="sr-only" tabindex="-1" aria-hidden="true" accept={ACCEPT} multiple onchange={(e) => take(e.currentTarget.files)} />
<input bind:this={camera} type="file" class="sr-only" tabindex="-1" aria-hidden="true" accept="image/*" capture="environment" onchange={(e) => take(e.currentTarget.files)} />

{#if usage}
  <div class="mb-4" aria-label="Espacio usado">
    <div class="flex justify-between text-xs text-app-muted"><span>Espacio del consultorio</span><span>{formatBytes(usage.used_bytes)} de {formatBytes(usage.quota_bytes)}</span></div>
    <div class="mt-1 h-1.5 overflow-hidden rounded-full bg-app-ink/10" role="progressbar" aria-label="Espacio usado" aria-valuemin="0" aria-valuemax="100" aria-valuenow={Math.round(pct)}>
      <div class="h-full {pct > 90 ? 'bg-app-danger' : 'bg-app-primary'}" style="width: {pct}%"></div>
    </div>
  </div>
{/if}

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="rounded-[22px] border-2 border-dashed p-1 transition {dragging ? 'border-app-primary bg-app-primary/5' : 'border-transparent'}"
  ondragover={(e) => {
    if (canWrite && !showArchived) {
      e.preventDefault();
      dragging = true;
    }
  }}
  ondragleave={() => (dragging = false)}
  ondrop={ondrop}
>
  {#if counts.length > 1}
    <div class="mb-3 flex flex-wrap gap-2" role="group" aria-label="Filtrar por tipo">
      <button type="button" class="btn-ghost !min-h-8 !px-3 {filter === 'all' ? 'bg-app-ink/8 !text-app-ink' : ''}" aria-pressed={filter === 'all'} onclick={() => (filter = 'all')}>Todos ({files.length})</button>
      {#each counts as k (k.value)}
        <button type="button" class="btn-ghost !min-h-8 !px-3 {filter === k.value ? 'bg-app-ink/8 !text-app-ink' : ''}" aria-pressed={filter === k.value} onclick={() => (filter = k.value)}>{k.label} ({k.n})</button>
      {/each}
    </div>
  {/if}

  {#if loading}
    <div class="card p-6 text-sm text-app-muted" role="status"><span class="spin"></span> Cargando…</div>
  {:else if error}
    <p class="alert" role="alert"><Icon name="alert" size={18} />{error}</p>
  {:else if visible.length === 0}
    <div class="card"><EmptyState icon="folder" title={showArchived ? 'Sin archivos archivados' : 'Sin archivos'} text={showArchived ? '' : canWrite ? 'Arrastra aquí un archivo o usa «Subir archivo».' : 'Aún no hay archivos en este expediente.'} /></div>
  {:else}
    <ul class="grid gap-3 sm:grid-cols-2">
      {#each visible as f (f.id)}
        <li class="card flex gap-3 p-3 sm:p-4 {f.archived_at ? 'opacity-85' : ''}">
          <button type="button" class="grid h-20 w-20 shrink-0 place-items-center overflow-hidden rounded-xl bg-app-ink/5 text-app-muted" aria-label="Abrir {f.title || f.original_name}" onclick={() => (preview = f)}>
            {#if isImage(f.mime)}
              <img src={filesApi.url(f.id)} alt="" loading="lazy" class="h-full w-full object-cover" />
            {:else}
              <Icon name="file" size={30} />
            {/if}
          </button>
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-2">
              <Pill tone="info">{kindLabel(f.kind)}</Pill>
              {#if f.archived_at}<Pill tone="muted">Archivado</Pill>{/if}
            </div>
            <p class="mt-1 truncate font-medium">{f.title || f.original_name}</p>
            <p class="text-xs text-app-muted">{dt(f.created_at)} · {f.uploaded_by_name} · {formatBytes(f.size_bytes)}{#if linked(f)} · consulta {linked(f)}{/if}</p>
            {#if f.note}<p class="mt-1 line-clamp-2 text-sm">{f.note}</p>{/if}
            {#if f.archived_at}<p class="mt-1 text-xs text-app-muted">Archivado por {f.archived_by_name}: {f.archive_reason}</p>{/if}
            <div class="mt-2 flex flex-wrap gap-1">
              {#if canPreview(f.mime)}<button type="button" class="btn-ghost !min-h-8 !px-3" onclick={() => (preview = f)}><Icon name="eye" size={16} />Ver</button>{/if}
              <a class="btn-ghost !min-h-8 !px-3" href={filesApi.url(f.id, true)} download={f.original_name}><Icon name="download" size={16} />Descargar</a>
              {#if f.can_archive}<button type="button" class="btn-ghost !min-h-8 !px-3" onclick={() => startArchive(f)}><Icon name="archive" size={16} />Archivar</button>{/if}
            </div>
          </div>
        </li>
      {/each}
    </ul>
  {/if}
</div>

{#if queue.length}
  <UploadModal
    patientId={patient.id}
    files={queue}
    {encounters}
    {maxBytes}
    onclose={() => (queue = [])}
    ondone={() => {
      toast.show('Archivos subidos');
      load();
    }}
  />
{/if}
<FilePreview file={preview} onclose={() => (preview = null)} />

<ConfirmModal open={!!archiving} title="Archivar archivo" op={archiveOp} onconfirm={confirmArchive} onclose={() => (archiving = null)} confirmLabel="Archivar">
  <p>«{archiving?.title || archiving?.original_name}» dejará de mostrarse en la lista; se conserva cifrado y solo el administrador podrá abrirlo.</p>
  <label class="label mt-4" for="arch-reason">Motivo</label>
  <input id="arch-reason" class="field" maxlength="300" bind:value={reason} placeholder="Ej. Archivo subido al expediente equivocado" />
</ConfirmModal>
