<script lang="ts">
  /** Directorio público: buscar especialistas por especialidad, estado, ciudad o nombre y agendar. */
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { onMount } from 'svelte';
  import { directoryApi, type DirectoryHit, type DirectoryOptions } from '$lib/api/profile';
  import Icon from '$lib/components/ui/Icon.svelte';
  import Stars from '$lib/components/ui/Stars.svelte';
  import { moneyCents } from '$lib/format';
  import { theme } from '$lib/theme.svelte';

  let { areaSlug = '', citySlug = '' }: { areaSlug?: string; citySlug?: string } = $props();

  let opts = $state<DirectoryOptions | null>(null);
  let results = $state<DirectoryHit[]>([]);
  let total = $state(0);
  let pageSize = $state(12);
  let loading = $state(true);
  let failed = $state(false);

  // filtros (la especialidad y la ciudad viven en la dirección: /directorio/dentistas/tijuana)
  let area = $state('');
  let city = $state('');
  let st = $state('');
  let q = $state('');
  let pg = $state(1);

  const areaOf = (slug: string) => opts?.areas.find((a) => a.slug === slug);
  const cityOf = (slug: string) => opts?.cities.find((c) => c.slug === slug);
  const citiesForState = $derived((opts?.cities ?? []).filter((c) => !st || c.state === st));
  const areaLabel = $derived(opts?.areas.find((a) => a.code === area)?.label ?? '');
  const cityLabel = $derived(opts?.cities.find((c) => c.slug === city)?.city ?? '');

  const heading = $derived(
    areaLabel && cityLabel ? `${areaLabel} en ${cityLabel}` : areaLabel ? `${areaLabel} cerca de ti` : cityLabel ? `Especialistas en ${cityLabel}` : 'Encuentra a tu especialista'
  );

  $effect(() => theme.init());

  async function search() {
    loading = true;
    failed = false;
    try {
      const r = await directoryApi.search({ q, area, state: st, city, page: pg });
      results = r.results;
      total = r.total;
      pageSize = r.page_size;
    } catch {
      failed = true;
    }
    loading = false;
  }

  /** Lleva los filtros a la dirección (así se puede compartir y Google la indexa) y busca. */
  function apply(resetPage = true) {
    if (resetPage) pg = 1;
    const a = opts?.areas.find((x) => x.code === area)?.slug ?? '';
    let path = '/directorio';
    if (a) path += '/' + a;
    if (a && city) path += '/' + city;
    const p = new URLSearchParams();
    if (!a && city) p.set('ciudad', city);
    if (st) p.set('estado', st);
    if (q) p.set('q', q);
    if (pg > 1) p.set('pagina', String(pg));
    const qs = p.toString();
    void goto(path + (qs ? '?' + qs : ''), { replaceState: true, noScroll: true, keepFocus: true });
    void search();
  }

  onMount(async () => {
    try {
      opts = await directoryApi.options();
    } catch {
      failed = true;
    }
    const u = page.url.searchParams;
    area = areaOf(areaSlug)?.code ?? '';
    city = citySlug || u.get('ciudad') || '';
    st = u.get('estado') ?? cityOf(city)?.state ?? '';
    q = u.get('q') ?? '';
    pg = Math.max(1, Number(u.get('pagina')) || 1);
    void search();
  });

  const fmtDay = (d: string) => new Date(d + 'T12:00:00').toLocaleDateString('es-MX', { weekday: 'short', day: 'numeric', month: 'short' });
  const pages = $derived(Math.max(1, Math.ceil(total / pageSize)));
  const field = 'h-12 w-full rounded-2xl bg-panel px-4 text-[15px] ring-1 ring-ink/15 focus:outline-none focus:ring-2 focus:ring-signal';
  const btn = 'inline-flex h-11 items-center justify-center gap-2 rounded-full px-5 text-sm font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-signal focus-visible:ring-offset-2';
</script>

<svelte:head>
  <title>{heading} · Directorio Caresia</title>
  <meta name="description" content="{heading}: compara especialistas con opiniones verificadas, precios y horarios disponibles, y agenda tu cita en línea." />
  <link rel="canonical" href={page.url.origin + page.url.pathname} />
</svelte:head>

<div class="landing grain min-h-screen overflow-x-clip bg-paper font-body text-ink antialiased" data-theme={theme.mode}>
  <header class="sticky top-0 z-30 border-b border-ink/10 bg-paper/85 backdrop-blur-xl">
    <div class="mx-auto flex h-16 max-w-6xl items-center gap-4 px-5 sm:px-8">
      <a href="/" class="flex items-center gap-2 font-display text-xl"><span class="grid h-8 w-8 place-items-center rounded-xl bg-ink text-paper"><Icon name="plus" size={16} stroke={2.6} /></span>Caresia</a>
      <span class="hidden text-sm text-ink-soft sm:inline">Directorio de especialistas</span>
      <a href="/" class="ml-auto text-sm font-medium text-ink-soft hover:text-ink">¿Eres especialista? Completa tu perfil y sube</a>
    </div>
  </header>

  <section class="mx-auto max-w-6xl px-5 pt-12 sm:px-8">
    <h1 class="font-display text-[clamp(2.4rem,6vw,4.2rem)] leading-[0.98] tracking-[-0.02em]">{heading}</h1>
    <p class="mt-3 max-w-2xl text-lg text-ink-soft">Opiniones verificadas de pacientes reales, precios y horarios disponibles. Agenda en línea en menos de un minuto.</p>

    <form class="mt-8 grid gap-3 rounded-[28px] bg-panel p-4 ring-1 ring-ink/10 md:grid-cols-[1.4fr_1fr_1fr_1fr_auto]" onsubmit={(e) => { e.preventDefault(); apply(); }} role="search" aria-label="Buscar especialistas">
      <label class="sr-only" for="d-q">Nombre, especialidad o padecimiento</label>
      <input id="d-q" class={field} placeholder="Nombre, clínica o especialidad" bind:value={q} maxlength="60" />
      <label class="sr-only" for="d-area">Especialidad</label>
      <select id="d-area" class={field} bind:value={area} onchange={() => apply()}>
        <option value="">Especialidad</option>
        {#each opts?.areas ?? [] as a (a.code)}<option value={a.code}>{a.label}</option>{/each}
      </select>
      <label class="sr-only" for="d-state">Estado</label>
      <select id="d-state" class={field} bind:value={st} onchange={() => { city = ''; apply(); }}>
        <option value="">Todo México</option>
        {#each opts?.states ?? [] as s (s)}<option value={s}>{s}</option>{/each}
      </select>
      <label class="sr-only" for="d-city">Ciudad</label>
      <select id="d-city" class={field} bind:value={city} onchange={() => apply()} disabled={!citiesForState.length}>
        <option value="">{citiesForState.length ? 'Todas las ciudades' : 'Sin ciudades aún'}</option>
        {#each citiesForState as c (c.state + c.slug)}<option value={c.slug}>{c.city} ({c.count})</option>{/each}
      </select>
      <button class="{btn} h-12 bg-signal text-white hover:bg-ink" type="submit"><Icon name="search" size={18} />Buscar</button>
    </form>
  </section>

  <section class="mx-auto mt-8 max-w-6xl px-5 pb-20 sm:px-8" aria-live="polite">
    {#if loading}
      <div class="grid gap-4">{#each [1, 2, 3] as i (i)}<div class="h-40 animate-pulse rounded-[28px] bg-ink/5"></div>{/each}</div>
    {:else if failed}
      <p class="rounded-3xl bg-panel p-8 text-center text-ink-soft ring-1 ring-ink/10">No pudimos cargar el directorio. <button class="text-signal underline" onclick={() => search()}>Reintentar</button></p>
    {:else if !results.length}
      <div class="rounded-[28px] bg-panel p-10 text-center ring-1 ring-ink/10">
        <p class="font-display text-3xl">Aún no hay especialistas con estos filtros</p>
        <p class="mt-2 text-ink-soft">Prueba otra ciudad o especialidad, o quita algún filtro.</p>
        <button class="{btn} mt-6 bg-ink text-paper hover:bg-signal" onclick={() => { q = ''; area = ''; st = ''; city = ''; apply(); }}>Ver todos</button>
      </div>
    {:else}
      <p class="mb-4 text-sm text-ink-soft">{total} {total === 1 ? 'resultado' : 'resultados'}</p>
      <ul class="grid gap-4">
        {#each results as r, i (r.slug || `sin-pagina-${i}`)}
          <li class="grid gap-5 rounded-[28px] p-5 ring-1 transition sm:grid-cols-[auto_1fr_auto] sm:p-6 {r.has_page ? 'bg-panel ring-ink/10 hover:shadow-[0_24px_48px_-20px_rgba(11,37,64,0.3)]' : 'bg-ink/[0.03] ring-ink/8'}">
            {#if r.has_page}
              <a href="/{r.slug}" class="block h-24 w-24 flex-none overflow-hidden rounded-3xl bg-ink/5 ring-1 ring-ink/10" aria-hidden="true" tabindex="-1">
                {#if r.photo_url || r.cover_url}<img src={r.photo_url || r.cover_url} alt="" class="h-full w-full object-cover" loading="lazy" />{:else}<span class="grid h-full w-full place-items-center bg-ink text-paper"><Icon name="plus" size={32} stroke={2.4} /></span>{/if}
              </a>
            {:else}
              <span class="grid h-24 w-24 flex-none place-items-center rounded-3xl bg-ink/8 text-ink-faint" aria-hidden="true"><Icon name="building" size={32} /></span>
            {/if}
            <div class="min-w-0">
              <h2 class="font-display text-[1.7rem] leading-tight">{#if r.has_page}<a href="/{r.slug}" class="hover:text-signal">{r.name}</a>{:else}{r.name}{/if}</h2>
              {#if r.tagline}<p class="text-[15px] text-ink-soft">{r.tagline}</p>{/if}
              <p class="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm">
                {#if r.rating.count > 0}<span class="inline-flex items-center gap-1.5"><Stars value={r.rating.average} size={15} /><strong>{r.rating.average.toFixed(1)}</strong><span class="text-ink-faint">({r.rating.count})</span></span>{:else}<span class="text-ink-faint">Sin opiniones aún</span>{/if}
                {#if r.city}<span class="inline-flex items-center gap-1 text-ink-soft"><Icon name="building" size={14} />{r.city}, {r.state}</span>{/if}
                {#if r.price_from_cents > 0}<span class="text-ink-soft">Desde <strong class="text-ink">{moneyCents(r.price_from_cents)}</strong></span>{/if}
              </p>
              {#if r.areas.length}<p class="mt-2 flex flex-wrap gap-1.5">{#each r.areas as a (a)}<span class="rounded-full bg-signal-soft px-2.5 py-0.5 text-xs font-medium text-signal">{a}</span>{/each}{#if r.completeness >= 90}<span class="rounded-full bg-mint-soft px-2.5 py-0.5 text-xs font-medium text-mint">Perfil completo</span>{/if}</p>{/if}
              {#if r.insurances.length}<p class="mt-2 text-xs text-ink-faint">Acepta: {r.insurances.join(', ')}</p>{/if}
            </div>
            <div class="flex flex-col gap-2 sm:w-56 sm:items-stretch">
              {#if r.has_page}
                {#if r.next_slot}
                  <p class="rounded-2xl bg-ink/[0.04] px-3 py-2 text-sm"><span class="block text-xs text-ink-faint">Próxima cita disponible</span><strong>{fmtDay(r.next_slot.date)} · {r.next_slot.start}</strong></p>
                {/if}
                {#if r.booking}<a href="/{r.slug}/reservar" class="{btn} bg-signal text-white hover:bg-ink"><Icon name="calendar" size={16} />Agendar cita</a>{/if}
                <a href="/{r.slug}" class="{btn} bg-panel text-ink ring-1 ring-ink/15 hover:bg-ink hover:text-paper">Ver perfil</a>
              {:else}
                <p class="rounded-2xl bg-ink/[0.04] px-3 py-2 text-sm text-ink-soft">Aún no completa su perfil: sin horarios ni reservas en línea.</p>
              {/if}
            </div>
          </li>
        {/each}
      </ul>
      {#if pages > 1}
        <nav class="mt-8 flex items-center justify-center gap-2" aria-label="Páginas">
          <button class="{btn} bg-panel ring-1 ring-ink/15 disabled:opacity-40" disabled={pg <= 1} onclick={() => { pg--; apply(false); }}>Anterior</button>
          <span class="px-3 text-sm text-ink-soft">Página {pg} de {pages}</span>
          <button class="{btn} bg-panel ring-1 ring-ink/15 disabled:opacity-40" disabled={pg >= pages} onclick={() => { pg++; apply(false); }}>Siguiente</button>
        </nav>
      {/if}
    {/if}
  </section>

  <footer class="mx-auto max-w-6xl px-5 pb-10 text-center text-xs text-ink-faint sm:px-8">
    Directorio de <a href="/" class="font-medium text-ink-soft hover:text-signal">Caresia</a> · Las opiniones solo las dejan pacientes que tuvieron su cita.
  </footer>
</div>
