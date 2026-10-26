<script lang="ts">
  import { blogDate, type BlogCard } from '$lib/api/blog';

  let { post, featured = false }: { post: BlogCard; featured?: boolean } = $props();
</script>

{#if post.image}
  <!-- Con portada (artículos de Faro): fila con miniatura -->
  <a
    href="/blog/{post.slug}"
    class="group flex h-full flex-col gap-5 overflow-hidden rounded-[26px] border border-ink/10 bg-panel/70 p-4 transition-[border-color,transform,box-shadow] duration-300 ease-out-strong hover:-translate-y-0.5 hover:border-ink/25 hover:shadow-[0_24px_48px_-28px_rgba(11,37,64,0.35)] sm:flex-row sm:items-center sm:gap-6 sm:p-5"
  >
    <img src={post.image.url} alt={post.image.alt} width="1376" height="768" loading="lazy" class="aspect-video w-full flex-none rounded-[18px] bg-ink/5 object-cover sm:w-60" />
    <div class="min-w-0">
      <p class="flex flex-wrap items-center gap-x-3 gap-y-1 font-mono text-[11px] uppercase tracking-[0.16em] text-ink-soft">
        <span class="text-signal">{post.category || 'Blog'}</span>
        <span aria-hidden="true" class="h-1 w-1 rounded-full bg-ink/25"></span>
        <span>{post.reading_minutes} min de lectura</span>
      </p>
      <h2 class="mt-3 font-display text-[clamp(1.6rem,3vw,2.1rem)] font-normal leading-[1.05] tracking-[-0.02em] text-ink">{post.title}</h2>
      <p class="mt-3 text-[15px] leading-relaxed text-ink-soft [text-wrap:pretty]">{post.excerpt}</p>
      <time class="mt-4 block text-[13px] text-ink-faint" datetime={post.published_at ?? post.created_at}>{blogDate(post.published_at ?? post.created_at)}</time>
    </div>
  </a>
{:else}
<a
  href="/blog/{post.slug}"
  class="group relative flex h-full flex-col justify-between overflow-hidden rounded-[26px] border border-ink/10 bg-panel/70 p-6 transition-[border-color,transform,box-shadow] duration-300 ease-out-strong hover:-translate-y-0.5 hover:border-ink/25 hover:shadow-[0_24px_48px_-28px_rgba(11,37,64,0.35)] sm:p-8"
>
  {#if featured}
    <span aria-hidden="true" class="pointer-events-none absolute -right-24 -top-24 h-72 w-72 rounded-full bg-signal/10 blur-3xl"></span>
  {/if}
  <div class="relative">
    <p class="flex flex-wrap items-center gap-x-3 gap-y-1 font-mono text-[11px] uppercase tracking-[0.16em] text-ink-soft">
      <span class="text-signal">{post.category || 'Blog'}</span>
      <span aria-hidden="true" class="h-1 w-1 rounded-full bg-ink/25"></span>
      <span>{post.reading_minutes} min de lectura</span>
    </p>
    <h2 class="mt-5 font-display font-normal leading-[1.02] tracking-[-0.02em] text-ink {featured ? 'text-[clamp(2.2rem,5vw,3.6rem)]' : 'text-[clamp(1.7rem,3vw,2.1rem)]'}">
      {post.title}
    </h2>
    <p class="mt-4 max-w-2xl leading-relaxed text-ink-soft [text-wrap:pretty] {featured ? 'text-lg' : 'text-[15px]'}">{post.excerpt}</p>
  </div>
  <div class="relative mt-8 flex items-center justify-between gap-4">
    <time class="text-[13px] text-ink-faint" datetime={post.published_at ?? post.created_at}>{blogDate(post.published_at ?? post.created_at)}</time>
    <span class="grid h-10 w-10 flex-none place-items-center rounded-full bg-ink text-paper transition-transform duration-500 ease-out-strong group-hover:-rotate-45" aria-hidden="true">
      <svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12h14M13 6l6 6-6 6" /></svg>
    </span>
  </div>
</a>
{/if}
