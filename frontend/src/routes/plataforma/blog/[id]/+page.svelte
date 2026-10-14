<script lang="ts">
  // Vista previa de un artículo del blog (borradores incluidos), tal como se verá en el sitio.
  import { page } from '$app/state';
  import Guard from '$lib/components/Guard.svelte';
  import Alert from '$lib/components/ui/Alert.svelte';
  import LoadingRows from '$lib/components/ui/LoadingRows.svelte';
  import Pill from '$lib/components/ui/Pill.svelte';
  import ArticleBody from '$lib/blog/ArticleBody.svelte';
  import { blogApi, blogDate, type BlogPost } from '$lib/api/blog';
  import { Loader } from '$lib/loader.svelte';

  let post = $state<BlogPost | null>(null);
  const ld = new Loader('No se pudo cargar el artículo.');
  $effect(() => {
    const id = page.params.id ?? '';
    ld.run(async () => {
      post = await blogApi.draft(id);
    });
  });
</script>

<svelte:head><title>{post ? post.title : 'Artículo'} · Blog · Caresia</title></svelte:head>

<Guard title="Blog" roles={['platform_admin', 'platform_support']}>
  <a href="/plataforma/blog" class="text-sm text-app-primary hover:underline">← Volver al blog</a>
  {#if ld.error}
    <Alert class="mt-4">{ld.error}</Alert>
  {:else if !post}
    <section class="card mt-4"><LoadingRows /></section>
  {:else}
    <div class="card mt-4 p-5 text-sm">
      <p class="flex flex-wrap items-center gap-2">
        {#if post.status === 'published'}<Pill tone="ok">Publicado {blogDate(post.published_at)}</Pill>{:else}<Pill tone="warn">Borrador</Pill>{/if}
        <span class="text-app-muted">{post.category} · {post.reading_minutes} min · palabra clave: <strong class="text-app-ink">{post.keyword}</strong></span>
      </p>
      <p class="mt-2 text-app-muted">Descripción para Google ({post.description.length} caracteres): <span class="text-app-ink">{post.description}</span></p>
      <p class="mt-1 text-app-muted">Dirección: <span class="text-app-ink">/blog/{post.slug}</span></p>
    </div>
    <!-- con los colores del sitio público, en claro -->
    <div class="landing mt-4 rounded-[24px] bg-paper px-5 py-10 font-body text-ink sm:px-10" data-theme="light">
      <p class="font-mono text-[11px] uppercase tracking-[0.16em] text-signal">{post.category}</p>
      <h1 class="mt-4 font-display text-[clamp(2.2rem,5vw,4rem)] font-normal leading-[0.95] tracking-[-0.03em]">{post.title}</h1>
      <p class="mt-5 max-w-2xl text-lg leading-relaxed text-ink-soft">{post.excerpt}</p>
      <div class="mt-10 max-w-[44rem] border-t border-ink/10 pt-10">
        <ArticleBody body={post.body} faqs={post.faqs} />
      </div>
    </div>
  {/if}
</Guard>
