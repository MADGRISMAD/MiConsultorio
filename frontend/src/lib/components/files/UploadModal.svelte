<script lang="ts">
  import { filesApi } from '$lib/api/files';
  import type { Encounter } from '$lib/types';
  import type { FileKind } from '$lib/types/files';
  import Modal from '../Modal.svelte';
  import Icon from '../ui/Icon.svelte';
  import { formatBytes, KINDS } from './util';

  interface Props {
    patientId: string;
    files: File[];
    encounters: Encounter[];
    maxBytes: number;
    onclose: () => void;
    ondone: () => void;
  }
  let { patientId, files, encounters, maxBytes, onclose, ondone }: Props = $props();

  type Row = { file: File; progress: number; state: 'wait' | 'up' | 'done' | 'error'; error: string };
  let rows = $state<Row[]>([]);
  let kind = $state<FileKind>('document');
  let title = $state('');
  let note = $state('');
  let encounterId = $state('');
  let running = $state(false);

  $effect(() => {
    rows = files.map((file) => ({
      file,
      progress: 0,
      state: 'wait' as const,
      error: file.size > maxBytes ? `Supera el límite de ${formatBytes(maxBytes)}.` : ''
    }));
    kind = files.length && files.every((f) => f.type.startsWith('image/')) ? 'photo' : 'document';
    title = '';
    note = '';
    encounterId = '';
  });

  const fmt = (e: Encounter) => `${new Date(e.occurred_at).toLocaleDateString('es-MX', { day: 'numeric', month: 'short', year: 'numeric' })} · ${e.reason || e.kind}`;
  const pending = $derived(rows.filter((r) => r.state !== 'done' && !(r.error && r.state === 'wait' && r.file.size > maxBytes)));

  async function send() {
    running = true;
    for (const r of rows) {
      if (r.state === 'done' || r.file.size > maxBytes) continue;
      r.state = 'up';
      r.error = '';
      try {
        await filesApi.upload(patientId, r.file, { kind, title: rows.length === 1 ? title.trim() : '', note: note.trim(), encounter_id: encounterId }, (f) => (r.progress = f));
        r.progress = 1;
        r.state = 'done';
      } catch (e) {
        r.state = 'error';
        r.error = e instanceof Error ? e.message : 'No se pudo subir.';
        // a full quota or rate limit will fail every remaining file the same way
        if (e && typeof e === 'object' && 'code' in e && ['QUOTA_EXCEEDED', 'RATE_LIMITED'].includes(String(e.code))) break;
      }
    }
    running = false;
    ondone();
    if (rows.every((r) => r.state === 'done')) onclose();
  }
</script>

<Modal open={files.length > 0} title={rows.length > 1 ? `Subir ${rows.length} archivos` : 'Subir archivo'} onclose={() => !running && onclose()}>
  <form
    class="space-y-4"
    onsubmit={(e) => {
      e.preventDefault();
      if (!running) send();
    }}
  >
    <ul class="space-y-2" aria-label="Archivos por subir">
      {#each rows as r (r.file)}
        <li class="rounded-xl border border-app-ink/10 p-3 text-sm">
          <div class="flex items-center gap-2">
            <Icon name="file" size={18} class="shrink-0 text-app-muted" />
            <span class="min-w-0 flex-1 truncate">{r.file.name}</span>
            <span class="shrink-0 text-xs text-app-muted">{formatBytes(r.file.size)}</span>
            {#if r.state === 'done'}<Icon name="check" size={18} class="text-app-ok" />{/if}
          </div>
          {#if r.state === 'up' || r.state === 'done'}
            <div class="mt-2 h-1.5 overflow-hidden rounded-full bg-app-ink/10" role="progressbar" aria-label="Progreso de {r.file.name}" aria-valuemin="0" aria-valuemax="100" aria-valuenow={Math.round(r.progress * 100)}>
              <div class="h-full bg-app-primary transition-[width]" style="width: {r.progress * 100}%"></div>
            </div>
          {/if}
          {#if r.error}<p class="mt-1 text-xs text-app-danger" role="alert">{r.error}</p>{/if}
        </li>
      {/each}
    </ul>

    <div class="grid gap-4 sm:grid-cols-2">
      <div>
        <label class="label" for="up-kind">Tipo</label>
        <select id="up-kind" class="field" bind:value={kind} disabled={running}>
          {#each KINDS as k (k.value)}<option value={k.value}>{k.label}</option>{/each}
        </select>
      </div>
      <div>
        <label class="label" for="up-enc">Consulta (opcional)</label>
        <select id="up-enc" class="field" bind:value={encounterId} disabled={running}>
          <option value="">Sin vincular</option>
          {#each encounters.filter((x) => !x.hidden && x.kind !== 'adenda') as e (e.id)}<option value={e.id}>{fmt(e)}</option>{/each}
        </select>
      </div>
    </div>
    {#if rows.length === 1}
      <div>
        <label class="label" for="up-title">Título</label>
        <input id="up-title" class="field" maxlength="120" placeholder="Ej. Radiografía panorámica" bind:value={title} disabled={running} />
      </div>
    {/if}
    <div>
      <label class="label" for="up-note">Nota (opcional)</label>
      <textarea id="up-note" class="field" rows="2" maxlength="1000" bind:value={note} disabled={running}></textarea>
    </div>
    <p class="hint">Los archivos se guardan cifrados y no se pueden borrar, solo archivar (NOM-004). Máximo {formatBytes(maxBytes)} por archivo.</p>
    <div class="flex justify-end gap-2">
      <button type="button" class="btn-secondary" disabled={running} onclick={onclose}>Cancelar</button>
      <button type="submit" class="btn-primary" disabled={running || pending.length === 0}>
        {#if running}<span class="spin"></span>Subiendo…{:else}<Icon name="upload" size={18} />Subir{/if}
      </button>
    </div>
  </form>
</Modal>
