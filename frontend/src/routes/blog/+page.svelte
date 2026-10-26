<script lang="ts">
  // Blog alojado en Soro (app.trysoro.com): su script pinta los artículos dentro de #soro-blog.
  // La llave es el identificador del sitio en Soro; para cambiar de cuenta solo se cambia SORO_KEY.
  import { onMount } from 'svelte';
  import { theme } from '$lib/theme.svelte';
  import Footer from '$lib/landing/Footer.svelte';
  import Nav from '$lib/landing/Nav.svelte';
  import PostCard from '$lib/blog/PostCard.svelte';
  import { blogApi, type BlogCard } from '$lib/api/blog';
  import { faroList } from '$lib/api/faro';

  const SORO_KEY = 'f89f47a2-4b3e-4fd4-8dbc-e856c6ebb5a6';
  const SORO_SRC = `https://app.trysoro.com/api/embed/${SORO_KEY}`;

  const title = 'Blog · Caresia';
  const description = 'Guías y consejos para administrar tu clínica dental, consultorio médico, nutrición, veterinaria y más: agenda, expedientes, pacientes y crecimiento.';

  $effect(() => theme.init());

  // Soro pinta sus textos con colores oscuros fijos, pensados para fondo claro. En modo oscuro se les ponen los
  // colores de la portada: en el contenedor y, si Soro usa Shadow DOM, también dentro de cada raíz.
  const darkCss = (p: string) => `
    ${p}:is(p, li, td, dd, blockquote, figcaption, time, small, span, div, label, em, i, cite, button) { color: rgb(var(--l-ink-soft)) !important; }
    ${p}:is(h1, h2, h3, h4, h5, h6, strong, b, th, dt), ${p}:is(h1, h2, h3, h4, h5, h6) * { color: rgb(var(--l-ink)) !important; }
    ${p}* { border-color: rgb(var(--l-ink) / 0.14) !important; }`;

  let box: HTMLDivElement;
  let roots = $state<ShadowRoot[]>([]);

  // Artículos propios (escritos con IA y revisados desde el panel de plataforma) y los de FaroContenido, del más nuevo al más viejo.
  // Soro sigue debajo mientras dure la transición.
  let posts = $state<BlogCard[] | null>(null);
  onMount(() => {
    Promise.all([blogApi.list().catch(() => [] as BlogCard[]), faroList()]).then(([own, faro]) => {
      posts = [...own, ...faro].sort((a, b) => (b.published_at ?? b.created_at).localeCompare(a.published_at ?? a.created_at));
    });
  });

  onMount(() => {
    // La app es de una sola página: el script se inyecta al entrar a /blog (también al llegar navegando desde
    // otra página) y se retira al salir, para que la siguiente visita lo vuelva a ejecutar.
    const s = document.createElement('script');
    s.src = SORO_SRC;
    s.defer = true;
    document.body.appendChild(s);

    const findRoots = () => {
      const found = [box, ...box.querySelectorAll('*')].map((el) => el.shadowRoot).filter((r): r is ShadowRoot => !!r);
      if (found.length !== roots.length || found.some((r, i) => r !== roots[i])) roots = found;
    };
    const mo = new MutationObserver(findRoots);
    mo.observe(box, { childList: true, subtree: true });

    return () => {
      mo.disconnect();
      s.remove();
      box?.replaceChildren();
    };
  });

  $effect(() => {
    if (theme.mode !== 'dark') return;
    const styles = [
      Object.assign(document.createElement('style'), { textContent: darkCss('#soro-blog ') }),
      ...roots.map((r) => Object.assign(document.createElement('style'), { textContent: darkCss('') }))
    ];
    document.head.appendChild(styles[0]);
    roots.forEach((r, i) => r.appendChild(styles[i + 1]));
    return () => styles.forEach((st) => st.remove());
  });
</script>

<svelte:head>
  <title>{title}</title>
  <meta name="description" content={description} />
  <meta property="og:type" content="website" />
  <meta property="og:title" content={title} />
  <meta property="og:description" content={description} />
</svelte:head>

<div class="landing grain min-h-screen overflow-x-clip bg-paper font-body text-ink antialiased selection:bg-signal selection:text-white" data-theme={theme.mode}>
  <Nav home="/" />

  <main class="mx-auto max-w-6xl px-5 pb-28 pt-32 sm:px-8 sm:pt-40">
    <p class="flex items-center gap-2 font-mono text-[11px] uppercase tracking-[0.16em] text-ink-soft">
      <span class="h-1.5 w-1.5 rounded-full bg-signal"></span>Blog
    </p>
    <h1 class="mt-6 font-display text-[clamp(3rem,10vw,6.5rem)] font-normal leading-[0.92] tracking-[-0.035em]">
      Ideas para tu <span class="italic text-signal">consultorio.</span>
    </h1>
    <p class="mt-6 max-w-2xl text-lg leading-relaxed text-ink-soft [text-wrap:pretty]">{description}</p>

    <div class="mx-auto mt-14 max-w-[44rem] border-t border-ink/10 pt-12">
      {#if posts === null}
        <div class="grid gap-4 md:grid-cols-2" aria-hidden="true">
          {#each [0, 1, 2] as i}<div class="h-64 animate-pulse rounded-[26px] bg-ink/5 {i === 0 ? 'md:col-span-2' : ''}"></div>{/each}
        </div>
      {:else if posts.length}
        <ul class="grid gap-4">
          {#each posts as post, i (post.id)}
            <li><PostCard {post} featured={i === 0} /></li>
          {/each}
        </ul>
      {/if}
    </div>

    <div class="mx-auto mt-4 max-w-[44rem]">
      <div id="soro-blog" bind:this={box} class="scroll-mt-28"></div>
      <noscript><p class="text-ink-soft">Activa JavaScript para leer el blog.</p></noscript>
    </div>
  </main>

  <Footer home="/" />
</div>
