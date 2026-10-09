
<script lang="ts">
  import Alert from '$lib/components/ui/Alert.svelte';
  import { arcoApi } from '$lib/api/arco';
  import { toast } from '$lib/toast.svelte';
  import type { ArcoRequest, ArcoSettings, ArcoSummary } from '$lib/types/arco';
  import EmptyState from '$lib/components/ui/EmptyState.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import ArcoDetail from './ArcoDetail.svelte';
  import ArcoNewModal from './ArcoNewModal.svelte';
  import { KINDS, KIND_LABEL, LEGAL_REMINDER, STATUS_LABEL, deadlinePill, deadlineText, fmtDay, statusPill } from './labels';

  let { newOpen = $bindable(false) }: { newOpen?: boolean } = $props();

  let rows = $state<ArcoRequest[] | null>(null);
  let summary = $state<ArcoSummary>({ open: 0, overdue: 0, soon: 0, pending_execute: 0 });
  let settings = $state<ArcoSettings | null>(null);
  let error = $state('');
  let status = $state('abiertas');
  let kind = $state('');
  let q = $state('');
  let selected = $state<string | null>(null);
  let timer: ReturnType<typeof setTimeout> | undefined;

  async function load() {
    error = '';
    try {
      const r = await arcoApi.list({ status, kind, q: q.trim() });
      rows = r.requests;
      summary = r.summary;
    } catch (e) {
      error = e instanceof Error ? e.message : 'No se pudieron cargar las solicitudes.';
    }
  }

  $effect(() => {
    status;
    kind;
    q;
    clearTimeout(timer);
    timer = setTimeout(load, 200);
    return () => clearTimeout(timer);
  });

  void arcoApi.settings().then((s) => (settings = s)).catch(() => {});

  async function copyLink() {
    if (!settings) return;
    try {
      await navigator.clipboard.writeText(settings.url);
      toast.show('Enlace copiado');
    } catch {
      toast.show('No se pudo copiar: selecciónalo y cópialo a mano', 'error');
    }
  }

  function created(r: ArcoRequest) {
    newOpen = false;
    toast.show(`Solicitud ${r.folio} registrada`);
    void load();
    selected = r.id;
  }

  const chips = $derived([
    { label: 'Abiertas', n: summary.open, tone: 'pill-info', filter: 'abiertas' },
    { label: 'Por vencer', n: summary.soon, tone: 'pill-warn', filter: 'abiertas' },
    { label: 'Vencidas', n: summary.overdue, tone: 'pill-bad', filter: 'vencida' }
  ]);
</script>

<div class="grid gap-4">
  <section class="grid gap-3 sm:grid-cols-3" aria-label="Resumen de solicitudes">
    {#each chips as c (c.label)}
      <button type="button" class="card flex items-center justify-between gap-3 px-5 py-4 text-left transition hover:border-app-ink/25" onclick={() => (status = c.filter)}>
        <span class="text-sm font-medium text-app-muted">{c.label}</span>
        <span class="pill {c.n ? c.tone : ''} text-base">{c.n}</span>
      </button>
    {/each}
  </section>

  <section class="card p-5" aria-labelledby="arco-link-title">
    <h2 id="arco-link-title" class="section-title">Formulario público del consultorio</h2>
    <p class="mt-2 text-sm text-app-muted">Cualquier persona puede enviar su solicitud desde esta página. Compártela en tu aviso de privacidad, sitio web o recepción.</p>
    {#if settings}
      <div class="mt-3 flex flex-col gap-2 sm:flex-row sm:items-center">
        <a href={settings.path} target="_blank" rel="noopener" class="min-w-0 flex-1 break-all rounded-xl bg-app-ink/5 px-3.5 py-2.5 font-mono text-sm underline">{settings.url}</a>
        <button type="button" class="btn-secondary shrink-0" onclick={copyLink}>Copiar enlace</button>
      </div>
      {#if !settings.privacy_contact_set}
        <p class="mt-3 text-sm text-app-warning">Completa el contacto de privacidad (nombre y correo) en <a class="underline" href="/ajustes?s=cumplimiento">Ajustes · Cumplimiento</a> para que aparezca en el formulario y recibas aviso por correo.</p>
      {/if}
      {#if !settings.mail_enabled}
        <p class="hint mt-2">Este servidor no tiene correo configurado: las solicitudes se guardan, pero no se envían acuses por correo.</p>
      {/if}
    {/if}
    <p class="mt-3 note">{LEGAL_REMINDER}</p>
  </section>

  <section class="card" aria-label="Solicitudes">
    <div class="flex flex-col gap-3 border-b border-app-ink/10 p-4 sm:flex-row">
      <div class="relative flex-1">
        <label class="sr-only" for="arco-q">Buscar</label>
        <Icon name="search" size={16} class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-app-muted" />
        <input id="arco-q" class="field pl-10" type="search" placeholder="Buscar por folio, nombre o correo" bind:value={q} />
      </div>
      <div class="grid grid-cols-2 gap-3 sm:flex">
        <div>
          <label class="sr-only" for="arco-status">Estado</label>
          <select id="arco-status" class="field" bind:value={status}>
            <option value="abiertas">Abiertas</option>
            <option value="">Todas</option>
            <option value="vencida">Vencidas</option>
            {#each ['recibida', 'en_revision', 'requiere_info', 'atendida', 'negada'] as s (s)}<option value={s}>{STATUS_LABEL[s as keyof typeof STATUS_LABEL]}</option>{/each}
          </select>
        </div>
        <div>
          <label class="sr-only" for="arco-kind">Tipo</label>
          <select id="arco-kind" class="field" bind:value={kind}>
            <option value="">Todos los tipos</option>
            {#each KINDS as k (k)}<option value={k}>{KIND_LABEL[k]}</option>{/each}
          </select>
        </div>
      </div>
    </div>

    {#if error}
      <Alert class="m-4">{error} <button type="button" class="ml-2 underline" onclick={load}>Reintentar</button></Alert>
    {:else if !rows}
      <LoadingRows />
    {:else if rows.length === 0}
      <EmptyState icon="shield" title="Sin solicitudes" text={status === 'abiertas' ? 'No hay solicitudes abiertas. Cuando alguien envíe una desde el formulario público, aparecerá aquí.' : 'Ninguna solicitud coincide con los filtros.'}>
        <button type="button" class="btn-secondary" onclick={() => (newOpen = true)}>Registrar una solicitud</button>
      </EmptyState>
    {:else}
      <ul class="divide-y divide-app-ink/10">
        {#each rows as r (r.id)}
          <li>
            <button type="button" class="flex w-full flex-col gap-2 px-4 py-4 text-left transition hover:bg-app-ink/5 sm:flex-row sm:items-center sm:justify-between sm:gap-4" onclick={() => (selected = r.id)} data-testid="arco-row">
              <span class="min-w-0">
                <span class="flex flex-wrap items-center gap-2">
                  <span class="font-mono text-sm font-semibold">{r.folio}</span>
                  <span class="pill pill-info">{KIND_LABEL[r.kind]}</span>
                  <span class="pill {statusPill(r.status)}">{STATUS_LABEL[r.status]}</span>
                  {#if !r.identity_verified && r.open}<span class="pill">Identidad sin verificar</span>{/if}
                </span>
                <span class="mt-1 block truncate text-[15px] font-medium">{r.requester_name}</span>
                <span class="block truncate text-sm text-app-muted">Recibida {fmtDay(r.received_at)} · {r.created_via === 'public' ? 'formulario público' : 'registrada por el consultorio'}</span>
              </span>
              {#if deadlineText(r)}
                <span class="pill {deadlinePill(r.deadline_state)} self-start sm:self-center"><Icon name="clock" size={13} />{deadlineText(r)}</span>
              {/if}
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </section>
</div>

<ArcoNewModal open={newOpen} onclose={() => (newOpen = false)} oncreated={created} />
<ArcoDetail id={selected} onclose={() => (selected = null)} onchanged={load} />
