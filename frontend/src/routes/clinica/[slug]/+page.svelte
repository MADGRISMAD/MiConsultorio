<script lang="ts">
  import { page } from '$app/state';
  import { onMount } from 'svelte';
  import { profileApi, type PublicClinic } from '$lib/api/profile';
  import PublicShell from '$lib/components/booking/PublicShell.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import Stars from '$lib/components/ui/Stars.svelte';
  import { dateShort } from '$lib/format';

  const slug = $derived(page.params.slug ?? '');
  let c = $state<PublicClinic | null>(null);
  let view = $state<'loading' | 'ready' | 'invalid'>('loading');

  onMount(async () => {
    try {
      c = await profileApi.publicClinic(slug);
      view = 'ready';
    } catch {
      view = 'invalid';
    }
  });

  const wa = $derived(c?.whatsapp ? `https://wa.me/${c.whatsapp.replace(/\D/g, '')}` : '');
  const mapsHref = $derived(c?.maps_url || (c?.address ? `https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(c.address)}` : ''));
</script>

<svelte:head>
  <title>{c ? `${c.name} · Caresia` : 'Consultorio'}</title>
  {#if c}<meta name="description" content={c.tagline || `Agenda tu cita en ${c.name}`} />{/if}
</svelte:head>

<PublicShell wide>
  {#if view === 'loading'}
    <div class="card h-64 animate-pulse"></div>
  {:else if view === 'invalid' || !c}
    <div class="card p-6 text-center">
      <h1 class="display text-2xl">Página no disponible</h1>
      <p class="mt-2 text-sm text-app-muted">Este consultorio no tiene una página pública activa.</p>
    </div>
  {:else}
    <div class="space-y-4">
      <section class="card p-6 text-center sm:p-8">
        <h1 class="display text-3xl sm:text-4xl">{c.name}</h1>
        {#if c.tagline}<p class="mt-2 text-lg text-app-muted">{c.tagline}</p>{/if}
        {#if c.rating && c.rating.count > 0}
          <p class="mt-3 flex items-center justify-center gap-2 text-sm"><Stars value={c.rating.average} size={20} /><strong>{c.rating.average.toFixed(1)}</strong><span class="text-app-muted">({c.rating.count} {c.rating.count === 1 ? 'opinión' : 'opiniones'})</span></p>
        {/if}
        {#if c.areas.length}
          <p class="mt-3 flex flex-wrap justify-center gap-1.5">{#each c.areas as a}<span class="badge">{a}</span>{/each}</p>
        {/if}
        <div class="mt-5 flex flex-wrap justify-center gap-2">
          {#if c.booking_url}<a class="btn-primary" href={c.booking_url}><Icon name="calendar" size={18} />Agendar cita</a>{/if}
          {#if wa}<a class="btn-secondary" href={wa} target="_blank" rel="noopener noreferrer"><Icon name="chat" size={18} />WhatsApp</a>{/if}
          {#if c.phone}<a class="btn-secondary" href="tel:{c.phone.replace(/[^\d+]/g, '')}"><Icon name="phone" size={18} />Llamar</a>{/if}
        </div>
      </section>

      {#if c.about}
        <section class="card p-6"><h2 class="display mb-2 text-xl">Sobre nosotros</h2><p class="whitespace-pre-line text-[15px] leading-relaxed">{c.about}</p></section>
      {/if}

      {#if c.professionals.length}
        <section class="card p-6">
          <h2 class="display mb-3 text-xl">Nuestro equipo</h2>
          <ul class="grid gap-2 sm:grid-cols-2">{#each c.professionals as p}<li class="rounded-xl bg-app-primary/6 px-3 py-2"><p class="font-medium">{p.name}</p>{#if p.title}<p class="text-sm text-app-muted">{p.title}</p>{/if}</li>{/each}</ul>
        </section>
      {/if}

      {#if c.address || c.hours_text || c.website}
        <section class="card p-6">
          <h2 class="display mb-3 text-xl">Visítanos</h2>
          <dl class="space-y-2 text-[15px]">
            {#if c.address}<div><dt class="text-xs text-app-muted">Dirección</dt><dd>{c.address}{#if mapsHref} · <a class="text-app-primary underline" href={mapsHref} target="_blank" rel="noopener noreferrer">Ver en Google Maps</a>{/if}</dd></div>{/if}
            {#if c.hours_text}<div><dt class="text-xs text-app-muted">Horario</dt><dd class="whitespace-pre-line">{c.hours_text}</dd></div>{/if}
            {#if c.website}<div><dt class="text-xs text-app-muted">Sitio web</dt><dd><a class="break-all text-app-primary underline" href={c.website} target="_blank" rel="noopener noreferrer">{c.website}</a></dd></div>{/if}
          </dl>
        </section>
      {/if}

      {#if c.rating && c.rating.count > 0}
        <section class="card p-6">
          <h2 class="display mb-3 text-xl">Lo que dicen nuestros pacientes</h2>
          {#if c.reviews?.length}
            <ul class="space-y-3">{#each c.reviews as r}<li class="border-t border-app-ink/8 pt-3 first:border-0 first:pt-0"><Stars value={r.rating} size={16} /><p class="mt-1 text-[15px]">{r.comment}</p><p class="text-xs text-app-muted">{dateShort(r.date)}</p></li>{/each}</ul>
          {/if}
          {#if c.review_url}<a class="btn-secondary mt-4 inline-flex" href={c.review_url} target="_blank" rel="noopener noreferrer">Déjanos tu reseña en Google</a>{/if}
        </section>
      {/if}
    </div>
  {/if}
</PublicShell>
