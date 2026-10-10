<script lang="ts">
  import { page } from '$app/state';
  import { onMount } from 'svelte';
  import { profileApi, type PublicClinic } from '$lib/api/profile';
  import TeamPolaroids from '$lib/components/clinic/TeamPolaroids.svelte';
  import PhotoCarousel from '$lib/components/clinic/PhotoCarousel.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';
  import Stars from '$lib/components/ui/Stars.svelte';
  import { dateShort, moneyCents } from '$lib/format';
  import { theme } from '$lib/theme.svelte';
  import ButtonLabel from '$lib/landing/ButtonLabel.svelte';
  import Ecg from '$lib/landing/Ecg.svelte';
  import LandingIcon from '$lib/landing/Icon.svelte';
  import { btn as ctaBtn } from '$lib/landing/motion';
  import { CLINIC_KINDS, type ClinicKind } from '$lib/types';

  const slug = $derived(page.params.slug ?? '');
  let c = $state<PublicClinic | null>(null);
  let view = $state<'loading' | 'ready' | 'invalid'>('loading');

  $effect(() => theme.init());
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
  const giros = $derived((c?.kinds ?? []).map((k) => CLINIC_KINDS[k as ClinicKind]).filter(Boolean));
  const place = $derived([c?.neighborhood, c?.city, c?.state].filter(Boolean).join(', '));
  const credentialed = $derived((c?.professionals ?? []).filter((p) => p.cedula || p.bio));
  const SEP = 'https://www.cedulaprofesional.sep.gob.mx/';
  const initials = (n: string) => n.split(/\s+/).filter(Boolean).slice(0, 2).map((w) => w[0]?.toUpperCase()).join('');
  const btn = 'inline-flex h-12 items-center justify-center gap-2 rounded-full px-6 text-[15px] font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-signal focus-visible:ring-offset-2';
</script>

<svelte:head>
  <title>{c ? `${c.name} · ${c.tagline || 'Consultorio'}` : 'Consultorio'}</title>
  {#if c}
    <meta name="description" content={c.tagline || `Agenda tu cita en ${c.name}`} />
    <meta property="og:title" content={c.name} />
    {#if c.cover_url}<meta property="og:image" content={c.cover_url} />{/if}
  {/if}
</svelte:head>

<div class="landing grain min-h-screen overflow-x-clip bg-paper font-body text-ink antialiased" data-theme={theme.mode}>
  {#if view === 'loading'}
    <div class="mx-auto max-w-5xl px-5 py-16"><div class="h-64 animate-pulse rounded-[28px] bg-ink/5"></div></div>
  {:else if view === 'invalid' || !c}
    <div class="mx-auto grid min-h-screen max-w-md place-items-center px-5 text-center">
      <div>
        <h1 class="font-display text-4xl">Página no disponible</h1>
        <p class="mt-3 text-ink-soft">Este consultorio no tiene una página pública activa.</p>
        <a href="/" class="{btn} mt-6 bg-ink text-paper hover:bg-signal">Ir a Caresia</a>
      </div>
    </div>
  {:else}
    <!-- top bar -->
    <header class="sticky top-0 z-30 border-b border-ink/10 bg-paper/85 backdrop-blur-xl">
      <div class="mx-auto flex h-16 max-w-6xl items-center gap-4 px-5 sm:px-8">
        <a href="#inicio" class="flex min-w-0 items-center gap-3">
          {#if c.profile_url}<img src={c.profile_url} alt="" class="h-9 w-9 flex-none rounded-xl object-cover" />{:else}<span class="grid h-9 w-9 flex-none place-items-center rounded-xl bg-ink text-paper"><Icon name="plus" size={18} stroke={2.6} /></span>{/if}
          <span class="truncate font-display text-xl">{c.name}</span>
        </a>
        <nav class="ml-auto hidden items-center gap-6 text-sm text-ink-soft md:flex" aria-label="Secciones">
          {#if giros.length}<a href="#servicios" class="hover:text-ink">Especialidades</a>{/if}
          {#if c.services.length}<a href="#precios" class="hover:text-ink">Precios</a>{/if}
          {#if c.reviews?.length}<a href="#opiniones" class="hover:text-ink">Opiniones</a>{/if}
          {#if c.professionals.length}<a href="#equipo" class="hover:text-ink">Equipo</a>{/if}
          {#if c.address || c.phone || c.email || wa || c.hours_text || c.website}<a href="#contacto" class="hover:text-ink">Contacto</a>{/if}
        </nav>
        {#if c.booking_url}<a href={c.booking_url} class="{btn} ml-auto h-10 bg-ink px-5 text-sm text-paper hover:bg-signal md:ml-0">Agendar cita</a>{/if}
      </div>
    </header>

    <!-- cover, profile and name -->
    <section id="inicio" class="mx-auto max-w-6xl px-5 pt-6 sm:px-8">
      <div class="relative overflow-hidden rounded-[28px] bg-ink">
        {#if c.cover_url}
          <img src={c.cover_url} alt="" class="aspect-[2/1] w-full object-cover sm:aspect-[3/1]" />
          <div class="absolute inset-0 bg-gradient-to-t from-ink/70 via-ink/10 to-transparent" aria-hidden="true"></div>
        {:else}
          <div class="aspect-[2/1] w-full bg-[linear-gradient(120deg,#0B2540_0%,#12467F_55%,#1673D1_100%)] sm:aspect-[3/1]" aria-hidden="true">
            <svg viewBox="0 0 160 14" class="absolute inset-x-0 bottom-6 w-full opacity-30" preserveAspectRatio="none" aria-hidden="true"><path d="M0 7h52l5-6 7 12 6-9 4 3h86" fill="none" stroke="white" stroke-width="0.5" stroke-linejoin="round" /></svg>
          </div>
        {/if}
      </div>
      <div class="relative -mt-14 flex flex-col gap-5 px-4 sm:-mt-16 sm:flex-row sm:items-start sm:gap-6 sm:px-8">
        <div class="h-28 w-28 flex-none overflow-hidden rounded-3xl bg-panel p-1 shadow-[0_18px_40px_-14px_rgba(11,37,64,0.45)] ring-1 ring-ink/10 sm:h-32 sm:w-32">
          {#if c.profile_url}<img src={c.profile_url} alt="Logo de {c.name}" class="h-full w-full rounded-[20px] object-cover" />{:else}<span class="grid h-full w-full place-items-center rounded-[20px] bg-ink text-paper"><Icon name="plus" size={44} stroke={2.4} /></span>{/if}
        </div>
        <div class="min-w-0 pb-1 sm:mt-[4.5rem]">
          <h1 class="font-display text-[clamp(2.4rem,6vw,4rem)] leading-[0.98] tracking-[-0.02em]">{c.name}</h1>
          {#if c.tagline}<p class="mt-2 max-w-2xl text-lg text-ink-soft">{c.tagline}</p>{/if}
          {#if place}<p class="mt-2 flex items-center gap-1.5 text-[15px] text-ink-soft"><Icon name="building" size={16} />{place}</p>{/if}
          {#if c.rating && c.rating.count > 0}
            <p class="mt-3 flex items-center gap-2 text-sm"><Stars value={c.rating.average} size={18} /><strong>{c.rating.average.toFixed(1)}</strong><span class="text-ink-soft">({c.rating.count} {c.rating.count === 1 ? 'opinión' : 'opiniones'})</span></p>
          {/if}
        </div>
      </div>
      <div class="mt-6 flex flex-wrap gap-3">
        {#if c.booking_url}<a href={c.booking_url} class="{btn} bg-signal text-white hover:bg-ink"><Icon name="calendar" size={18} />Agendar cita</a>{/if}
        {#if c.phone}<a href="tel:{c.phone.replace(/[^\d+]/g, '')}" class="{btn} bg-panel text-ink ring-1 ring-ink/15 hover:bg-ink hover:text-paper"><Icon name="phone" size={18} />Llamar</a>{/if}
        {#if wa}<a href={wa} target="_blank" rel="noopener noreferrer" class="{btn} bg-panel text-ink ring-1 ring-ink/15 hover:bg-ink hover:text-paper"><Icon name="chat" size={18} />WhatsApp</a>{/if}
        {#if mapsHref}<a href={mapsHref} target="_blank" rel="noopener noreferrer" class="{btn} bg-panel text-ink ring-1 ring-ink/15 hover:bg-ink hover:text-paper"><Icon name="building" size={18} />Cómo llegar</a>{/if}
      </div>
    </section>

    {#if c.gallery.length}
      <section class="mx-auto mt-12 max-w-6xl px-5 sm:px-8" aria-label="Galería"><PhotoCarousel photos={c.gallery} label="Fotos de {c.name}" /></section>
    {/if}

    {#if c.about}
      <section class="mx-auto mt-20 grid max-w-6xl gap-8 px-5 sm:px-8 md:grid-cols-[1fr_1.6fr]">
        <h2 class="font-display text-[clamp(2rem,4.5vw,3.2rem)] leading-[1]">Conócenos</h2>
        <p class="whitespace-pre-line text-lg leading-relaxed text-ink-soft">{c.about}</p>
      </section>
    {/if}

    {#if giros.length}
      <section id="servicios" class="mx-auto mt-20 max-w-6xl scroll-mt-24 px-5 sm:px-8">
        <h2 class="font-display text-[clamp(2rem,4.5vw,3.2rem)] leading-[1]">Lo que atendemos</h2>
        <ul class="mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {#each giros as g (g.label)}
            <li class="flex items-start gap-4 rounded-3xl bg-panel p-6 ring-1 ring-ink/10 transition hover:-translate-y-0.5 hover:shadow-[0_24px_48px_-20px_rgba(11,37,64,0.3)]">
              <span class="grid h-12 w-12 flex-none place-items-center rounded-2xl bg-signal-soft text-signal"><Icon name={g.icon} size={24} /></span>
              <span>
                <span class="block font-display text-2xl leading-tight">{g.label}</span>
                <span class="mt-1 block text-[15px] text-ink-soft">{g.hint}</span>
              </span>
            </li>
          {/each}
        </ul>
      </section>
    {/if}

    {#if c.services.length}
      <section id="precios" class="mx-auto mt-20 max-w-6xl scroll-mt-24 px-5 sm:px-8">
        <h2 class="font-display text-[clamp(2rem,4.5vw,3.2rem)] leading-[1]">Servicios y precios</h2>
        <ul class="mt-8 divide-y divide-ink/10 rounded-[28px] bg-panel ring-1 ring-ink/10">
          {#each c.services as sv (sv.name)}
            <li class="flex items-center justify-between gap-4 px-6 py-4">
              <span><span class="block text-[17px] font-medium">{sv.name}</span>{#if sv.duration_minutes}<span class="text-sm text-ink-faint">{sv.duration_minutes} min</span>{/if}</span>
              <span class="whitespace-nowrap font-display text-2xl">{sv.price_cents > 0 ? moneyCents(sv.price_cents) : 'Consultar'}</span>
            </li>
          {/each}
        </ul>
        <p class="mt-3 text-xs text-ink-faint">Precios de referencia; el consultorio confirma el costo final según tu caso.</p>
      </section>
    {/if}

    {#if c.professionals.length}
      <section id="equipo" class="mx-auto mt-20 max-w-6xl scroll-mt-24 px-5 sm:px-8">
        <h2 class="font-display text-[clamp(2rem,4.5vw,3.2rem)] leading-[1]">Nuestro equipo</h2>
        <div class="mt-10"><TeamPolaroids people={c.professionals} /></div>
        {#if credentialed.length}
          <ul class="mt-10 grid gap-4 md:grid-cols-2">
            {#each credentialed as p (p.name)}
              <li class="rounded-3xl bg-panel p-6 ring-1 ring-ink/10">
                <p class="font-display text-2xl leading-tight">{p.name}</p>
                {#if p.title}<p class="text-[15px] text-ink-soft">{p.title}</p>{/if}
                {#if p.cedula}
                  <p class="mt-3 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm">
                    <span class="inline-flex items-center gap-1.5 rounded-full bg-signal-soft px-2.5 py-1 font-medium text-signal"><Icon name="check" size={14} />Cédula profesional {p.cedula}</span>
                    {#if p.cedula_specialty}<span class="text-ink-faint">· {p.cedula_specialty}</span>{/if}
                    <a class="text-signal underline" href={SEP} target="_blank" rel="noopener noreferrer">Verificar en la SEP</a>
                  </p>
                {/if}
                {#if p.bio}<p class="mt-3 whitespace-pre-line text-[15px] leading-relaxed text-ink-soft">{p.bio}</p>{/if}
              </li>
            {/each}
          </ul>
        {/if}
      </section>
    {/if}

    {#if c.rating && c.rating.count > 0}
      <section id="opiniones" class="mx-auto mt-20 max-w-6xl scroll-mt-24 px-5 sm:px-8">
        <h2 class="font-display text-[clamp(2rem,4.5vw,3.2rem)] leading-[1]">Lo que dicen nuestros pacientes</h2>
        <p class="mt-3 flex items-center gap-2 text-sm text-ink-soft"><Icon name="check" size={16} />Opiniones verificadas: solo las pueden dejar pacientes que tuvieron su cita.</p>
        {#if c.reviews?.length}
          <ul class="mt-8 grid gap-4 md:grid-cols-2">
            {#each c.reviews as r}
              <li class="rounded-3xl bg-panel p-6 ring-1 ring-ink/10">
                <Stars value={r.rating} size={16} /><p class="mt-3 text-[17px] leading-relaxed">«{r.comment}»</p>
                <p class="mt-3 text-xs text-ink-faint">{dateShort(r.date + 'T12:00:00')}{r.verified ? ' · Paciente verificado' : ''}</p>
                {#if r.reply}<div class="mt-4 rounded-2xl bg-ink/[0.04] p-4"><p class="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-faint">Respuesta de {c.name}</p><p class="mt-1 text-[15px] text-ink-soft">{r.reply}</p></div>{/if}
              </li>
            {/each}
          </ul>
        {/if}
        {#if c.review_url}<a class="{btn} mt-6 bg-panel text-ink ring-1 ring-ink/15 hover:bg-ink hover:text-paper" href={c.review_url} target="_blank" rel="noopener noreferrer">Déjanos tu reseña en Google</a>{/if}
      </section>
    {/if}

    <!-- contact -->
    {#if c.address || c.phone || c.email || wa || c.hours_text || c.website || c.insurances.length || c.payment_methods.length}
    <section id="contacto" class="mx-auto mt-20 max-w-6xl scroll-mt-24 px-5 sm:px-8">
      <h2 class="font-display text-[clamp(2rem,4.5vw,3.2rem)] leading-[1]">Visítanos</h2>
      <dl class="mt-8 grid gap-4 rounded-[28px] bg-panel p-6 ring-1 ring-ink/10 sm:grid-cols-2 sm:p-8 lg:grid-cols-3">
        {#if c.address}<div><dt class="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-faint">Dirección</dt><dd class="mt-1">{c.address}{#if mapsHref}<br /><a class="text-signal underline" href={mapsHref} target="_blank" rel="noopener noreferrer">Ver en Google Maps</a>{/if}</dd></div>{/if}
        {#if c.phone}<div><dt class="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-faint">Teléfono</dt><dd class="mt-1"><a class="hover:text-signal" href="tel:{c.phone.replace(/[^\d+]/g, '')}">{c.phone}</a></dd></div>{/if}
        {#if c.email}<div><dt class="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-faint">Correo</dt><dd class="mt-1 break-all"><a class="hover:text-signal" href="mailto:{c.email}">{c.email}</a></dd></div>{/if}
        {#if wa}<div><dt class="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-faint">WhatsApp</dt><dd class="mt-1"><a class="hover:text-signal" href={wa} target="_blank" rel="noopener noreferrer">{c.whatsapp}</a></dd></div>{/if}
        {#if c.hours_text}<div><dt class="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-faint">Horario</dt><dd class="mt-1 whitespace-pre-line">{c.hours_text}</dd></div>{/if}
        {#if c.insurances.length}<div><dt class="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-faint">Aseguradoras</dt><dd class="mt-1">{c.insurances.join(', ')}</dd></div>{/if}
        {#if c.payment_methods.length}<div><dt class="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-faint">Formas de pago</dt><dd class="mt-1">{c.payment_methods.join(', ')}</dd></div>{/if}
        {#if c.languages.length}<div><dt class="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-faint">Idiomas</dt><dd class="mt-1">{c.languages.join(', ')}</dd></div>{/if}
        {#if c.website}<div><dt class="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-faint">Sitio web</dt><dd class="mt-1 break-all"><a class="text-signal underline" href={c.website} target="_blank" rel="noopener noreferrer">{c.website.replace(/^https:\/\//, '')}</a></dd></div>{/if}
      </dl>
    </section>
    {/if}

    {#if c.booking_url}
      <section class="mt-20 px-3 sm:px-5" aria-labelledby="cta-h">
        <div class="l-deep relative isolate overflow-hidden rounded-[36px] bg-signal px-6 py-20 text-center text-white sm:py-28">
          <Ecg class="absolute inset-x-0 top-1/2 -z-10 h-40 w-full -translate-y-1/2" base="stroke-white/15" sweep="stroke-white/70" beats={5} />
          <p class="font-mono text-[11px] uppercase tracking-[0.16em] text-white/75">Agenda en línea · menos de 1 minuto</p>
          <h2 id="cta-h" class="mx-auto mt-6 max-w-5xl font-display text-[clamp(2.8rem,8vw,7rem)] leading-[0.92] tracking-[-0.035em]">¿Listo para <em>tu cita</em>?</h2>
          <p class="mx-auto mt-5 max-w-md text-[17px] text-white/80">Elige a tu especialista y el horario que mejor te acomode en {c.name}.</p>
          <div class="mt-10 flex flex-wrap items-center justify-center gap-4">
            <a href={c.booking_url} class="{ctaBtn} h-16 bg-ink pl-8 pr-2 text-[17px] text-paper hover:bg-paper hover:text-ink focus-visible:ring-offset-signal">
              <ButtonLabel>Agendar mi cita</ButtonLabel>
              <span class="ml-2 grid h-12 w-12 place-items-center rounded-full bg-signal text-white transition-transform duration-500 ease-out group-hover:rotate-[-45deg]"><LandingIcon name="arrow" class="h-5 w-5" /></span>
            </a>
            {#if c.phone}<a href="tel:{c.phone.replace(/[^\d+]/g, '')}" class="px-4 py-2 text-[16px] font-medium text-white/90 underline decoration-white/40 underline-offset-4 hover:decoration-white">Prefiero llamar</a>{:else if wa}<a href={wa} target="_blank" rel="noopener noreferrer" class="px-4 py-2 text-[16px] font-medium text-white/90 underline decoration-white/40 underline-offset-4 hover:decoration-white">Prefiero escribir por WhatsApp</a>{/if}
          </div>
        </div>
      </section>
    {/if}

    <footer class="mx-auto mt-16 max-w-6xl px-5 pb-10 text-center text-xs text-ink-faint sm:px-8">
      {#if c.listed}<a href="/directorio" class="mb-3 inline-block font-medium text-ink-soft hover:text-signal">← Buscar más especialistas en el directorio de Caresia</a><br />{/if}
      © {new Date().getFullYear()} {c.name} · Página creada con <a href="/" class="font-medium text-ink-soft hover:text-signal">Caresia</a>
    </footer>
  {/if}
</div>
