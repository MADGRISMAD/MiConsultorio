<script lang="ts">
  import { page } from '$app/state';
  import { onMount } from 'svelte';
  import { profileApi } from '$lib/api/profile';
  import PublicShell from '$lib/components/booking/PublicShell.svelte';
  import Stars from '$lib/components/ui/Stars.svelte';
  import { Op } from '$lib/op.svelte';

  const token = $derived(page.params.token ?? '');
  let clinic = $state('');
  let pro = $state('');
  let view = $state<'loading' | 'ready' | 'done' | 'invalid'>('loading');
  let rating = $state(0);
  let comment = $state('');
  let publicOk = $state(false);
  let reviewUrl = $state('');
  const op = new Op();

  onMount(async () => {
    try {
      const r = await profileApi.survey(token);
      clinic = r.clinic_name;
      pro = r.professional;
      reviewUrl = r.review_url;
      view = r.answered ? 'done' : 'ready';
    } catch {
      view = 'invalid';
    }
  });

  async function send() {
    if (!rating) return op.fail('Elige de 1 a 5 estrellas.');
    if (await op.run(async () => (reviewUrl = (await profileApi.answer(token, { rating, comment: comment.trim(), public_ok: publicOk })).review_url))) view = 'done';
  }
</script>

<svelte:head>
  <title>¿Cómo te atendimos?</title>
  <meta name="robots" content="noindex, nofollow" />
  <meta name="referrer" content="no-referrer" />
</svelte:head>

<PublicShell>
  <div class="card p-6 text-center">
    {#if view === 'loading'}
      <div class="mx-auto h-24 animate-pulse rounded-xl bg-app-ink/5"></div>
    {:else if view === 'invalid'}
      <h1 class="display text-2xl">Enlace no válido</h1>
      <p class="mt-2 text-sm text-app-muted">Este enlace no existe o ya no es válido.</p>
    {:else if view === 'done'}
      <h1 class="display text-2xl">¡Gracias por tu opinión!</h1>
      <p class="mt-2 text-sm text-app-muted">{clinic} la recibió y la usará para mejorar.</p>
      {#if reviewUrl}
        <p class="mt-5 text-sm">¿Nos ayudas a que más personas nos encuentren? Una reseña en Google toma un minuto.</p>
        <a class="btn-primary mt-3 inline-flex" href={reviewUrl} target="_blank" rel="noopener noreferrer">Calificarnos en Google Maps</a>
      {/if}
    {:else}
      <h1 class="display text-2xl">¿Cómo fue tu visita?</h1>
      <p class="mt-2 text-sm text-app-muted">Tu opinión sobre {clinic}{pro ? ` y la atención de ${pro}` : ''} nos ayuda a mejorar. Toma menos de un minuto.</p>
      <div class="mt-5 flex justify-center"><Stars value={rating} size={38} onpick={(n) => (rating = n)} label="Calificación de tu visita" /></div>
      <div class="mt-5 text-left">
        <label class="label" for="sv-c">Cuéntanos más (opcional)</label>
        <textarea id="sv-c" class="field" rows="4" maxlength="1000" bind:value={comment} placeholder="¿Qué te gustó? ¿Qué podemos mejorar? No escribas datos de salud."></textarea>
        {#if comment.trim()}
          <label class="mt-2 flex cursor-pointer items-start gap-2 text-sm"><input type="checkbox" class="mt-1 h-4 w-4 accent-[rgb(var(--app-primary))]" bind:checked={publicOk} />Pueden mostrar mi comentario (sin mi nombre) en la página pública de {clinic}.</label>
        {/if}
      </div>
      {#if op.phase === 'error'}<p class="alert mt-3" role="alert">{op.message}</p>{/if}
      <button type="button" class="btn-primary mt-5" disabled={op.phase === 'loading'} onclick={send}>{#if op.phase === 'loading'}<span class="spin"></span>{/if}Enviar</button>
    {/if}
  </div>
</PublicShell>
