
<script lang="ts">
  import { api } from '$lib/api';
  import { arcoApi } from '$lib/api/arco';
  import { Op } from '$lib/op.svelte';
  import { toast } from '$lib/toast.svelte';
  import type { ArcoEvent, ArcoPackage, ArcoRequest } from '$lib/types/arco';
  import type { PatientRow } from '$lib/types';
  import ConfirmModal from '$lib/components/ConfirmModal.svelte';
  import Modal from '$lib/components/Modal.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import { KIND_LABEL, LEGAL_REMINDER, STATUS_LABEL, deadlinePill, deadlineText, fmtDateTime, fmtDay, statusPill } from './labels';

  let { id, onclose, onchanged }: { id: string | null; onclose: () => void; onchanged: () => void } = $props();

  let req = $state<ArcoRequest | null>(null);
  let events = $state<ArcoEvent[]>([]);
  let pkg = $state<ArcoPackage | null>(null);
  let loadError = $state('');
  let tab = $state<'resumen' | 'responder' | 'paquete' | 'historial'>('resumen');

  const op = new Op();
  let identityOn = $state(false);
  let identityMethod = $state('');
  let statusNote = $state('');
  let statusMail = $state(false);
  let note = $state('');
  let outcome = $state<'procedente' | 'improcedente'>('procedente');
  let responseText = $state('');
  let denialReason = $state('');
  let executed = $state(false);
  let sendMail = $state(false);
  let patientQ = $state('');
  let patientHits = $state<PatientRow[]>([]);
  let confirmArchive = $state(false);
  const archiveOp = new Op();

  async function load() {
    if (!id) return;
    loadError = '';
    try {
      const r = await arcoApi.get(id);
      apply(r.request);
      events = r.events;
    } catch (e) {
      loadError = e instanceof Error ? e.message : 'No se pudo cargar la solicitud.';
    }
  }

  function apply(r: ArcoRequest) {
    req = r;
    identityOn = r.identity_verified;
    identityMethod = r.identity_method;
  }

  $effect(() => {
    if (id) {
      req = null;
      pkg = null;
      events = [];
      tab = 'resumen';
      statusNote = note = responseText = denialReason = patientQ = '';
      executed = statusMail = false;
      sendMail = false;
      op.reset();
      void load();
    }
  });

  /** Runs one action on the request, refreshes the view and tells the list. */
  async function act(fn: () => Promise<ArcoRequest>, done: string) {
    if (await op.run(async () => apply(await fn()))) {
      toast.show(done);
      await load();
      pkg = null;
      onchanged();
    }
  }

  const saveIdentity = () => act(() => arcoApi.patch(id!, { identity_verified: identityOn, identity_method: identityMethod }), 'Identidad registrada');
  const setStatus = (s: string) => act(() => arcoApi.setStatus(id!, s, statusNote, statusMail), 'Estado actualizado').then(() => (statusNote = ''));
  const addNote = () => act(() => arcoApi.note(id!, note), 'Nota agregada').then(() => (note = ''));
  const respond = () =>
    act(() => arcoApi.respond(id!, { outcome, response_text: responseText, denial_reason: denialReason, executed, send_email: sendMail }), 'Respuesta registrada').then(() => (tab = 'resumen'));
  const execute = () => act(() => arcoApi.execute(id!), 'Solicitud marcada como ejecutada');
  const linkPatient = (pid: string) => act(() => arcoApi.patch(id!, { patient_id: pid }), pid ? 'Expediente vinculado' : 'Expediente desvinculado').then(() => ((patientQ = ''), (patientHits = [])));

  async function searchPatients() {
    if (patientQ.trim().length < 2) {
      patientHits = [];
      return;
    }
    patientHits = await api.patients.lookup(patientQ.trim()).catch(() => []);
  }

  async function loadPackage() {
    tab = 'paquete';
    if (pkg || !id) return;
    await op.run(async () => (pkg = await arcoApi.package(id!)));
  }

  function useDraft(kind: 'procedente' | 'improcedente') {
    if (!pkg) return;
    responseText = kind === 'procedente' ? pkg.draft_response : pkg.draft_denial;
  }

  async function openRespond() {
    tab = 'responder';
    if (!pkg && id) {
      let loaded: ArcoPackage | undefined;
      await op.run(async () => (loaded = await arcoApi.package(id!)));
      if (loaded) {
        pkg = loaded;
        if (!responseText) responseText = loaded.draft_response;
      }
    }
    sendMail = !!req?.requester_email;
  }

  async function archivePatient() {
    if (await archiveOp.run(async () => apply(await arcoApi.archivePatient(id!)))) {
      confirmArchive = false;
      toast.show('Expediente archivado: uso bloqueado y registros conservados');
      await load();
      pkg = null;
      onchanged();
    }
  }

  const canBlock = $derived(req && ['cancelacion', 'oposicion', 'revocacion'].includes(req.kind));
  const tabs = [
    ['resumen', 'Resumen'],
    ['responder', 'Responder'],
    ['paquete', 'Paquete de respuesta'],
    ['historial', 'Historial']
  ] as const;
</script>

<Modal open={!!id} title={req ? `Solicitud ${req.folio}` : 'Solicitud ARCO'} {onclose} wide>
  {#if loadError}
    <p class="alert" role="alert"><Icon name="alert" size={18} />{loadError}</p>
  {:else if !req}
    <LoadingRows />
  {:else}
    <div class="flex flex-wrap items-center gap-2">
      <span class="pill pill-info">{KIND_LABEL[req.kind]}</span>
      <span class="pill {statusPill(req.status)}">{STATUS_LABEL[req.status]}{req.status === 'atendida' && req.resolution === 'procedente' ? ' · procedente' : ''}</span>
      {#if deadlineText(req)}<span class="pill {deadlinePill(req.deadline_state)}"><Icon name="clock" size={13} />{deadlineText(req)}</span>{/if}
      <span class="pill">{req.created_via === 'public' ? 'Formulario público' : 'Registrada por el consultorio'}</span>
    </div>

    <div class="mt-4 flex gap-1 overflow-x-auto rounded-full bg-app-ink/5 p-1" role="tablist" aria-label="Secciones de la solicitud">
      {#each tabs as [key, label] (key)}
        <button
          type="button"
          role="tab"
          aria-selected={tab === key}
          class="whitespace-nowrap rounded-full px-4 py-1.5 text-sm font-medium transition {tab === key ? 'bg-app-panel text-app-ink shadow-sm' : 'text-app-muted hover:text-app-ink'}"
          onclick={() => (key === 'responder' ? openRespond() : key === 'paquete' ? loadPackage() : (tab = key))}>{label}</button>
      {/each}
    </div>

    {#if op.phase === 'error'}<p class="alert mt-4" role="alert"><Icon name="alert" size={18} />{op.message}</p>{/if}

    {#if tab === 'resumen'}
      <div class="mt-5 grid gap-6 md:grid-cols-2">
        <section aria-labelledby="ad-who">
          <h3 id="ad-who" class="section-title">Quién solicita</h3>
          <p class="mt-2 text-[15px] font-semibold">{req.requester_name}</p>
          <p class="text-sm text-app-muted">{req.requester_email || 'Sin correo'}{req.requester_phone ? ` · ${req.requester_phone}` : ''}</p>
          <h3 class="section-title mt-5">Solicitud</h3>
          <p class="mt-2 whitespace-pre-wrap break-words text-[15px]">{req.description}</p>
        </section>
        <section aria-labelledby="ad-dates">
          <h3 id="ad-dates" class="section-title">Plazos</h3>
          <dl class="mt-2 grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
            <dt class="text-app-muted">Recibida</dt><dd>{fmtDateTime(req.received_at)}</dd>
            <dt class="text-app-muted">Acuse (meta interna)</dt><dd>{fmtDay(req.due_ack_at)}</dd>
            <dt class="text-app-muted">Comunicar determinación</dt><dd>{fmtDay(req.due_answer_at)}</dd>
            {#if req.due_execute_at}<dt class="text-app-muted">Hacerla efectiva</dt><dd>{fmtDay(req.due_execute_at)}</dd>{/if}
            {#if req.answered_at}<dt class="text-app-muted">Respondida</dt><dd>{fmtDateTime(req.answered_at)}</dd>{/if}
            {#if req.executed_at}<dt class="text-app-muted">Ejecutada</dt><dd>{fmtDateTime(req.executed_at)}</dd>{/if}
          </dl>
          <p class="hint mt-2">{LEGAL_REMINDER}</p>
          {#if req.pending_execute}
            <button type="button" class="btn-primary mt-3" disabled={op.phase === 'loading'} onclick={execute}><Icon name="check" size={16} />Marcar como ejecutada</button>
          {/if}
        </section>
      </div>

      {#if req.response_text}
        <section class="mt-6 rounded-xl bg-app-ink/5 p-4" aria-label="Respuesta registrada">
          <h3 class="section-title">Respuesta registrada</h3>
          <p class="mt-2 whitespace-pre-wrap break-words text-sm">{req.response_text}</p>
          {#if req.denial_reason}<p class="mt-3 text-sm"><strong>Motivo de la negativa:</strong> {req.denial_reason}</p>{/if}
        </section>
      {/if}

      <section class="mt-6 rounded-xl border border-app-ink/10 p-4" aria-labelledby="ad-ident">
        <h3 id="ad-ident" class="section-title">Verificación de identidad</h3>
        <p class="mt-1 text-sm text-app-muted">Antes de entregar, corregir o bloquear datos, comprueba que quien solicita es el titular (o su representante) y anota cómo.</p>
        <label class="mt-3 flex items-center gap-2.5 text-sm font-medium">
          <input type="checkbox" class="h-5 w-5 accent-[rgb(var(--app-primary))]" bind:checked={identityOn} />Identidad verificada
        </label>
        <div class="mt-3 flex flex-col gap-2 sm:flex-row">
          <label class="sr-only" for="ad-method">Cómo se verificó</label>
          <input id="ad-method" class="field" placeholder="Cómo se verificó (ej. INE mostrada en el consultorio)" maxlength="200" bind:value={identityMethod} />
          <button type="button" class="btn-secondary shrink-0" disabled={op.phase === 'loading'} onclick={saveIdentity}>Guardar</button>
        </div>
      </section>

      <section class="mt-4 rounded-xl border border-app-ink/10 p-4" aria-labelledby="ad-pat">
        <h3 id="ad-pat" class="section-title">Expediente vinculado</h3>
        {#if req.patient_id}
          <div class="mt-2 flex flex-wrap items-center justify-between gap-2">
            <a href={`/pacientes/${req.patient_id}`} class="text-[15px] font-semibold underline">{req.patient_name || 'Ver expediente'}</a>
            <button type="button" class="btn-ghost" disabled={op.phase === 'loading'} onclick={() => linkPatient('')}>Desvincular</button>
          </div>
        {:else}
          <div class="mt-2 flex gap-2">
            <label class="sr-only" for="ad-psearch">Buscar paciente</label>
            <input id="ad-psearch" class="field" placeholder="Buscar paciente por nombre, CURP o teléfono" bind:value={patientQ} oninput={searchPatients} autocomplete="off" />
          </div>
          {#if patientHits.length}
            <ul class="mt-2 divide-y divide-app-ink/10 rounded-xl border border-app-ink/10">
              {#each patientHits.slice(0, 6) as p (p.id)}
                <li><button type="button" class="flex min-h-11 w-full items-center justify-between gap-3 px-3 py-2 text-left text-sm hover:bg-app-ink/5" onclick={() => linkPatient(p.id)}>
                  <span class="font-medium">{p.names} {p.last_names}</span><span class="text-app-muted">Exp. {p.file_number}</span>
                </button></li>
              {/each}
            </ul>
          {/if}
        {/if}
      </section>

      {#if req.open}
        <section class="mt-4 rounded-xl border border-app-ink/10 p-4" aria-labelledby="ad-status">
          <h3 id="ad-status" class="section-title">Cambiar estado</h3>
          <label class="label mt-3" for="ad-snote">Nota o información que se pide (obligatoria para «Requiere información»)</label>
          <textarea id="ad-snote" class="field" rows="2" maxlength="1000" bind:value={statusNote}></textarea>
          {#if req.requester_email}
            <label class="mt-2 flex items-center gap-2 text-sm"><input type="checkbox" class="h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={statusMail} />Enviar esta nota por correo a quien solicita (solo en «Requiere información»)</label>
          {/if}
          <div class="mt-3 flex flex-wrap gap-2">
            <button type="button" class="btn-secondary" disabled={op.phase === 'loading' || req.status === 'en_revision'} onclick={() => setStatus('en_revision')}>En revisión</button>
            <button type="button" class="btn-secondary" disabled={op.phase === 'loading' || !statusNote.trim()} onclick={() => setStatus('requiere_info')}>Requiere información</button>
            <button type="button" class="btn-primary" onclick={openRespond}>Responder…</button>
          </div>
        </section>
      {/if}

      <section class="mt-4 rounded-xl border border-app-ink/10 p-4" aria-labelledby="ad-note">
        <h3 id="ad-note" class="section-title">Anexar nota interna</h3>
        <div class="mt-2 flex flex-col gap-2 sm:flex-row">
          <label class="sr-only" for="ad-noteinput">Nota</label>
          <input id="ad-noteinput" class="field" maxlength="2000" placeholder="Ej. Se llamó al solicitante" bind:value={note} />
          <button type="button" class="btn-secondary shrink-0" disabled={op.phase === 'loading' || note.trim().length < 2} onclick={addNote}>Agregar</button>
        </div>
      </section>
    {:else if tab === 'responder'}
      <div class="mt-5">
        {#if !req.open}
          <p class="rounded-xl bg-app-ink/5 px-4 py-3 text-sm text-app-muted">Esta solicitud ya tiene una determinación. Puedes consultarla en «Resumen».</p>
        {:else}
          <p class="rounded-xl bg-app-warning/10 px-4 py-3 text-sm text-app-warning">{LEGAL_REMINDER} Revisa el texto con tu asesor antes de enviarlo.</p>
          {#if !req.identity_verified}
            <p class="alert mt-3" role="alert"><Icon name="alert" size={18} />Primero verifica y registra la identidad de quien solicita (pestaña «Resumen»).</p>
          {/if}
          <fieldset class="mt-4">
            <legend class="label">Determinación</legend>
            <div class="flex flex-wrap gap-4 text-sm">
              <label class="flex items-center gap-2"><input type="radio" bind:group={outcome} value="procedente" class="h-4 w-4 accent-[rgb(var(--app-primary))]" onchange={() => useDraft('procedente')} />Procedente (se atenderá)</label>
              <label class="flex items-center gap-2"><input type="radio" bind:group={outcome} value="improcedente" class="h-4 w-4 accent-[rgb(var(--app-primary))]" onchange={() => useDraft('improcedente')} />No procedente (se niega)</label>
            </div>
          </fieldset>
          <label class="label mt-4" for="ad-resp">Respuesta para quien solicita</label>
          <textarea id="ad-resp" class="field" rows="9" maxlength="5000" bind:value={responseText}></textarea>
          <p class="hint">No incluyas datos clínicos en este texto: entrégalos por un medio seguro.</p>
          {#if outcome === 'improcedente'}
            <label class="label mt-4" for="ad-deny">Motivo de la negativa</label>
            <textarea id="ad-deny" class="field" rows="3" maxlength="2000" bind:value={denialReason}></textarea>
          {:else}
            <label class="mt-4 flex items-center gap-2 text-sm"><input type="checkbox" class="h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={executed} />Ya se hizo efectiva con esta respuesta (por ejemplo, se entregó la copia)</label>
          {/if}
          {#if req.requester_email}
            <label class="mt-2 flex items-center gap-2 text-sm"><input type="checkbox" class="h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={sendMail} />Enviar esta respuesta por correo a {req.requester_email}</label>
          {/if}
          <div class="mt-5 flex justify-end">
            <button type="button" class="btn-primary" disabled={op.phase === 'loading' || !req.identity_verified || responseText.trim().length < 10 || (outcome === 'improcedente' && denialReason.trim().length < 10)} onclick={respond}>
              {#if op.phase === 'loading'}<span class="spin"></span>{/if}Registrar respuesta
            </button>
          </div>
        {/if}
      </div>
    {:else if tab === 'paquete'}
      <div class="mt-5">
        {#if !pkg}
          <LoadingRows />
        {:else}
          <p class="rounded-xl bg-app-warning/10 px-4 py-3 text-sm text-app-warning">{pkg.legal_note}</p>
          <h3 class="section-title mt-5">Lista de pasos · {pkg.kind_label}</h3>
          <ul class="mt-2 space-y-2">
            {#each pkg.steps as s}
              <li class="flex items-start gap-2.5 text-sm">
                <span class="mt-0.5 grid h-5 w-5 flex-none place-items-center rounded-full {s.done ? 'bg-app-accent/15 text-app-accent' : 'bg-app-ink/8 text-app-muted'}"><Icon name={s.done ? 'check' : 'clock'} size={12} stroke={2.4} /></span>
                <span>{s.text}</span>
              </li>
            {/each}
          </ul>
          {#if pkg.export_path}
            <a href={arcoApi.exportUrl(pkg.export_path)} class="btn-secondary mt-4" target="_blank" rel="noopener"><Icon name="download" size={16} />Descargar exportación del expediente</a>
            <p class="hint">Abrir la exportación queda registrado en la bitácora de accesos.</p>
          {:else if pkg.kind === 'acceso' && !req.patient_id}
            <p class="hint mt-3">Vincula el expediente del paciente para habilitar la exportación.</p>
          {/if}
          {#if pkg.retention_note}
            <p class="mt-4 rounded-xl bg-app-primary/8 px-4 py-3 text-sm">{pkg.retention_note}</p>
          {/if}
          {#if canBlock && req.patient_id}
            <button type="button" class="btn-secondary mt-3" disabled={!req.identity_verified} onclick={() => (confirmArchive = true)}><Icon name="archive" size={16} />Archivar expediente (bloquear su uso)</button>
            {#if !req.identity_verified}<p class="hint">Verifica la identidad primero.</p>{/if}
          {/if}
          <h3 class="section-title mt-6">Borrador de respuesta</h3>
          <pre class="mt-2 whitespace-pre-wrap break-words rounded-xl bg-app-ink/5 p-4 font-body text-sm">{pkg.draft_response}</pre>
          <p class="hint mt-2">Para usar este texto, abre «Responder»: se carga como punto de partida.</p>
        {/if}
      </div>
    {:else}
      <ol class="mt-5 space-y-3" aria-label="Historial de la solicitud">
        {#each events as ev (ev.id)}
          <li class="rounded-xl border border-app-ink/10 px-4 py-3">
            <p class="text-sm">{ev.message}</p>
            <p class="mt-1 text-xs text-app-muted">{fmtDateTime(ev.created_at)}{ev.actor ? ` · ${ev.actor}` : ''}</p>
          </li>
        {/each}
      </ol>
    {/if}
  {/if}
</Modal>

<ConfirmModal
  open={confirmArchive}
  title="Archivar expediente"
  op={archiveOp}
  confirmLabel="Archivar"
  onclose={() => (confirmArchive = false)}
  onconfirm={archivePatient}>
  <p>Se bloqueará el uso del expediente (queda archivado) pero <strong>no se borra nada</strong>: la NOM-004-SSA3-2012 obliga a conservarlo al menos 5 años desde el último acto médico. Puedes reactivarlo desde el expediente si fuera necesario.</p>
</ConfirmModal>
