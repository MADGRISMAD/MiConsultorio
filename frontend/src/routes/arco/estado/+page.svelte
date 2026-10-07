<script lang="ts">
  import { page } from '$app/state';
  import { arcoApi } from '$lib/api/arco';
  import { ApiError } from '$lib/api';
  import type { ArcoPublicStatus } from '$lib/types/arco';
  import ArcoShell from '$lib/components/arco/ArcoShell.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { STATUS_LABEL, fmtDay, statusPill } from '$lib/components/arco/labels';

  const token = $derived(page.url.searchParams.get('t') ?? '');
  let req = $state<ArcoPublicStatus | null>(null);
  let view = $state<'loading' | 'ready' | 'missing' | 'error'>('loading');

  $effect(() => {
    const t = token;
    if (!t) {
      view = 'missing';
      return;
    }
    view = 'loading';
    arcoApi
      .publicStatus(t)
      .then((r) => {
        req = r;
        view = 'ready';
      })
      .catch((e) => {
        view = e instanceof ApiError && e.status === 404 ? 'missing' : 'error';
      });
  });
</script>

<svelte:head>
  <title>Estado de tu solicitud ARCO</title>
  <meta name="robots" content="noindex" />
</svelte:head>

<ArcoShell>
  {#if view === 'loading'}
    <div class="card px-6 py-10 text-center text-sm text-app-muted" role="status">Cargando…</div>
  {:else if view === 'ready' && req}
    <section class="card px-6 py-8 sm:px-9">
      <p class="section-title">{req.clinic}</p>
      <h1 class="display mt-3 text-4xl leading-tight">Estado de tu solicitud</h1>
      <dl class="mt-6 grid gap-4 sm:grid-cols-2">
        <div><dt class="section-title">Folio</dt><dd class="mt-1 font-mono text-lg font-semibold" data-testid="arco-status-folio">{req.folio}</dd></div>
        <div><dt class="section-title">Tipo</dt><dd class="mt-1 text-[15px]">{req.kind_label}</dd></div>
        <div><dt class="section-title">Recibida el</dt><dd class="mt-1 text-[15px]">{fmtDay(req.received_on)}</dd></div>
        <div><dt class="section-title">Estado</dt><dd class="mt-1"><span class="pill {statusPill(req.status)}">{req.status === 'negada' ? 'Con respuesta' : STATUS_LABEL[req.status]}</span></dd></div>
      </dl>
      <p class="mt-5 rounded-xl bg-app-ink/5 px-4 py-3 text-sm text-app-muted">{req.status_label}</p>
      <p class="hint mt-4">Por tu seguridad, aquí no mostramos el contenido de tu solicitud ni de la respuesta. Si necesitas algo más, comunícate con el consultorio e indica tu folio.</p>
    </section>
  {:else if view === 'missing'}
    <section class="card px-6 py-9">
      <div class="grid h-12 w-12 place-items-center rounded-full bg-app-ink/8 text-app-muted"><Icon name="search" size={24} /></div>
      <h1 class="display mt-4 text-3xl leading-tight">No encontramos esa solicitud</h1>
      <p class="mt-3 text-app-muted">Abre el enlace completo que te enviamos por correo al registrar tu solicitud.</p>
    </section>
  {:else}
    <section class="card px-6 py-9" role="alert">
      <p class="alert"><Icon name="alert" size={18} />No se pudo cargar la página. Inténtalo de nuevo.</p>
      <button class="btn-secondary mt-5" onclick={() => location.reload()}>Reintentar</button>
    </section>
  {/if}
</ArcoShell>
