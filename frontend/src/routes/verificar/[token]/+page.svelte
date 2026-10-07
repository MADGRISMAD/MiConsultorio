<script lang="ts">
  import { page } from '$app/state';
  import { rxApi } from '$lib/api/rx';
  import Brand from '$lib/components/ui/Brand.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import { theme } from '$lib/theme.svelte';
  import type { RxVerification, VerifyStatus } from '$lib/types/rx';

  let data = $state<RxVerification | null>(null);
  let phase = $state<'loading' | 'ok' | 'missing' | 'limited' | 'error'>('loading');

  $effect(() => theme.init());

  $effect(() => {
    const token = page.params.token ?? '';
    phase = 'loading';
    rxApi
      .verify(token)
      .then((r) => {
        data = r;
        phase = 'ok';
      })
      .catch((e) => {
        phase = e?.status === 404 ? 'missing' : e?.status === 429 ? 'limited' : 'error';
      });
  });

  const LOOK: Record<VerifyStatus, { title: string; text: string; tone: string; icon: 'check' | 'clock' | 'ban' }> = {
    vigente: { title: 'Receta auténtica y vigente', text: 'Esta receta fue emitida desde Caresia y sigue vigente.', tone: 'text-app-success bg-app-success/12', icon: 'check' },
    vencida: { title: 'Receta auténtica, pero vencida', text: 'Fue emitida desde Caresia, pero ya pasó su fecha de vigencia.', tone: 'text-app-warning bg-app-warning/15', icon: 'clock' },
    anulada: { title: 'Receta anulada', text: 'El profesional la canceló: no debe surtirse.', tone: 'text-app-danger bg-app-danger/10', icon: 'ban' }
  };

  const day = (iso: string) => {
    const d = iso.length === 10 ? new Date(`${iso}T12:00:00`) : new Date(iso);
    return d.toLocaleDateString('es-MX', { day: 'numeric', month: 'long', year: 'numeric' });
  };
  const look = $derived(data ? LOOK[data.status] : null);
</script>

<svelte:head>
  <title>Verificar receta · Caresia</title>
  <meta name="robots" content="noindex, nofollow" />
  <meta name="referrer" content="no-referrer" />
</svelte:head>

<div
  class="app fixed inset-0 overflow-y-auto overflow-x-hidden px-4 py-6 sm:py-10"
  data-theme={theme.mode}
  style="background-image: radial-gradient(ellipse 60% 420px at 50% 0%, rgb(var(--app-primary) / 0.1), transparent 75%); background-repeat: no-repeat"
>
  <div class="mx-auto w-full max-w-lg">
    <p class="mb-5 flex justify-center"><a href="/" aria-label="Caresia"><Brand size={26} /></a></p>

    <main class="card p-6 sm:p-8" aria-live="polite">
      {#if phase === 'loading'}
        <div class="flex items-center justify-center gap-3 py-10 text-app-muted"><span class="spin"></span>Verificando…</div>
      {:else if phase === 'ok' && data && look}
        <div class="flex flex-col items-center text-center">
          <span class="grid h-14 w-14 place-items-center rounded-2xl {look.tone}"><Icon name={look.icon} size={26} /></span>
          <h1 class="display mt-4 text-2xl sm:text-3xl">{look.title}</h1>
          <p class="mt-2 max-w-sm text-sm text-app-muted">{look.text}</p>
        </div>
        <dl class="mt-6 divide-y divide-app-ink/10 text-sm">
          <div class="flex justify-between gap-4 py-2.5"><dt class="text-app-muted">Folio</dt><dd class="font-medium">{String(data.folio).padStart(6, '0')}</dd></div>
          <div class="flex justify-between gap-4 py-2.5"><dt class="text-app-muted">Emitida</dt><dd class="text-right font-medium">{day(data.issued_at)}</dd></div>
          {#if data.valid_until}
            <div class="flex justify-between gap-4 py-2.5"><dt class="text-app-muted">Vigente hasta</dt><dd class="text-right font-medium">{day(data.valid_until)}</dd></div>
          {/if}
          <div class="flex justify-between gap-4 py-2.5"><dt class="text-app-muted">Establecimiento</dt><dd class="text-right font-medium">{data.clinic_name}</dd></div>
          <div class="flex justify-between gap-4 py-2.5">
            <dt class="text-app-muted">Profesional</dt>
            <dd class="text-right font-medium">{data.professional_name}{#if data.professional_title}<span class="block text-xs font-normal text-app-muted">{data.professional_title}</span>{/if}</dd>
          </div>
          <div class="flex justify-between gap-4 py-2.5"><dt class="text-app-muted">Cédula profesional</dt><dd class="font-medium">{data.professional_license}</dd></div>
          <div class="flex justify-between gap-4 py-2.5"><dt class="text-app-muted">Paciente (iniciales)</dt><dd class="font-medium">{data.patient_initials}</dd></div>
          <div class="flex justify-between gap-4 py-2.5"><dt class="text-app-muted">Receta retenida</dt><dd class="font-medium">{data.retained ? 'Sí, la farmacia la conserva' : 'No'}</dd></div>
        </dl>
        <p class="mt-5 rounded-xl bg-app-elevated px-4 py-3 text-xs text-app-muted">
          Esta página solo confirma que el documento existe y quién lo emitió. No muestra medicamentos ni diagnóstico. Compara los datos con la receta que tienes en la mano.
        </p>
      {:else if phase === 'missing'}
        <div class="flex flex-col items-center py-4 text-center">
          <span class="grid h-14 w-14 place-items-center rounded-2xl bg-app-danger/10 text-app-danger"><Icon name="alert" size={26} /></span>
          <h1 class="display mt-4 text-2xl">No encontramos esta receta</h1>
          <p class="mt-2 max-w-sm text-sm text-app-muted">El código no corresponde a ninguna receta emitida desde Caresia. Si tienes la receta impresa, desconfía de ella y consulta al profesional o al establecimiento.</p>
        </div>
      {:else if phase === 'limited'}
        <div class="py-4 text-center">
          <h1 class="display text-2xl">Demasiadas consultas</h1>
          <p class="mt-2 text-sm text-app-muted">Espera un minuto e intenta de nuevo.</p>
        </div>
      {:else}
        <div class="py-4 text-center">
          <h1 class="display text-2xl">No pudimos verificar</h1>
          <p class="mt-2 text-sm text-app-muted">Revisa tu conexión e intenta de nuevo.</p>
        </div>
      {/if}
    </main>
    <p class="mt-6 flex items-center justify-center gap-2 text-xs text-app-muted">Verificación de recetas con <a href="/" class="inline-flex" aria-label="Caresia"><Brand size={16} class="text-[0.7rem]" /></a></p>
  </div>
</div>
