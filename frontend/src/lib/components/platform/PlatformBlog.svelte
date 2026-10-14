<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { Loader } from '$lib/loader.svelte';
  import { Op } from '$lib/op.svelte';
  import { session } from '$lib/session.svelte';
  import { ago, dateTimeWeekday } from '$lib/format';
  import { blogApi, blogDate, type BlogCard, type BlogPanel } from '$lib/api/blog';
  import Alert from '../ui/Alert.svelte';
  import EmptyState from '../ui/EmptyState.svelte';
  import Icon from '../ui/Icon.svelte';
  import LoadingRows from '../ui/LoadingRows.svelte';
  import OpError from '../ui/OpError.svelte';
  import PageHeader from '../ui/PageHeader.svelte';
  import Pill from '../ui/Pill.svelte';
  import ConfirmModal from '../ConfirmModal.svelte';

  // Blog de Caresia: la IA escribe un artículo cada lunes y jueves. Aquí se revisan, publican u ocultan,
  // y se puede pedir uno al momento.
  let data = $state<BlogPanel | null>(null);
  const ld = new Loader('No se pudo cargar el blog.');
  const isAdmin = $derived(session.isPlatformAdmin);

  let topic = $state('');
  let publish = $state(false);
  const genOp = new Op();
  const rowOp = new Op();
  let removing = $state<BlogCard | null>(null);
  const removeOp = new Op();

  let timer: ReturnType<typeof setTimeout> | undefined;
  async function load() {
    clearTimeout(timer);
    await ld.run(async () => {
      data = await blogApi.panel();
    });
    // mientras la IA escribe, se revisa cada 5 segundos
    if (data?.generating) timer = setTimeout(load, 5000);
  }
  onMount(load);
  onDestroy(() => clearTimeout(timer));

  async function generate(e: SubmitEvent) {
    e.preventDefault();
    if (await genOp.run(() => blogApi.generate(topic.trim(), publish))) {
      topic = '';
      await load();
    }
  }

  async function toggle(p: BlogCard) {
    if (await rowOp.run(() => blogApi.setStatus(p.id, p.status === 'published' ? 'draft' : 'published'))) await load();
  }

  async function confirmRemove() {
    const p = removing;
    if (p && (await removeOp.run(() => blogApi.remove(p.id)))) {
      removing = null;
      await load();
    }
  }
</script>

<PageHeader title="Blog" subtitle="Artículos para atraer consultorios desde Google. La IA escribe uno cada lunes y jueves; aquí los revisas y decides qué se publica.">
  {#snippet actions()}<button type="button" class="btn-secondary" onclick={load}><Icon name="refresh" size={17} />Actualizar</button>{/snippet}
</PageHeader>

{#if ld.error}
  <Alert>{ld.error}</Alert>
{:else if !data}
  <section class="card"><LoadingRows /></section>
{:else}
  <div class="grid gap-4 lg:grid-cols-[minmax(0,1fr)_22rem]">
    <section class="card p-5">
      <h2 class="font-semibold">Escritura automática</h2>
      {#if !data.ai_ready}
        <Alert class="mt-3">La IA no está configurada en el servidor (falta GEMINI_API_KEY). Sin ella no se escriben artículos.</Alert>
      {:else}
        <dl class="mt-3 grid gap-3 text-sm sm:grid-cols-2">
          <div>
            <dt class="text-app-muted">Próximo artículo</dt>
            <dd class="mt-0.5 font-medium">{data.next_run ? dateTimeWeekday(data.next_run) : 'Piloto automático apagado'}</dd>
          </div>
          <div>
            <dt class="text-app-muted">Al terminar</dt>
            <dd class="mt-0.5 font-medium">{data.auto_publish ? 'Se publica solo' : 'Queda en borrador para revisión'}</dd>
          </div>
          <div>
            <dt class="text-app-muted">Siguiente tema</dt>
            <dd class="mt-0.5 font-medium">{data.next_topic || 'La IA propone uno nuevo'}</dd>
          </div>
          <div>
            <dt class="text-app-muted">Temas pendientes en el calendario</dt>
            <dd class="mt-0.5 font-medium">{data.topics_left}</dd>
          </div>
        </dl>
        {#if data.generating}
          <p class="mt-4 flex items-center gap-2 rounded-xl bg-app-primary/8 px-4 py-3 text-sm text-app-primary"><span class="spin"></span>La IA está escribiendo un artículo. Tarda uno o dos minutos.</p>
        {:else if data.last_error}
          <Alert class="mt-4">El último intento falló: {data.last_error}</Alert>
        {/if}
      {/if}
    </section>

    {#if isAdmin && data.ai_ready}
      <form class="card p-5" onsubmit={generate}>
        <h2 class="font-semibold">Escribir uno ahora</h2>
        <label class="label mt-3" for="blog-topic">Tema (opcional)</label>
        <textarea id="blog-topic" class="field" rows="3" maxlength="300" bind:value={topic} placeholder="Vacío: toma el siguiente tema del calendario. Ej.: Cómo cobrar consultas de seguimiento"></textarea>
        <label class="mt-3 flex items-center gap-2 text-sm"><input type="checkbox" class="accent-[rgb(var(--app-primary))]" bind:checked={publish} />Publicar en cuanto esté listo</label>
        <OpError op={genOp} class="mt-3" />
        <button type="submit" class="btn-primary mt-4 w-full" disabled={data.generating || genOp.phase === 'loading'}>
          {#if genOp.phase === 'loading'}<span class="spin"></span>{:else}<Icon name="sparkles" size={17} />{/if}Escribir artículo
        </button>
      </form>
    {/if}
  </div>

  <section class="card mt-4 overflow-hidden">
    <OpError op={rowOp} class="m-4" />
    {#if data.posts.length === 0}
      <EmptyState icon="file" title="Todavía no hay artículos" text="El primero se escribe el próximo lunes o jueves, o pídelo ahora." />
    {:else}
      <ul class="divide-y divide-app-ink/8">
        {#each data.posts as p (p.id)}
          <li class="flex flex-wrap items-center gap-x-4 gap-y-2 px-5 py-4">
            <div class="min-w-0 flex-1">
              <p class="flex flex-wrap items-center gap-2">
                <a href="/plataforma/blog/{p.id}" class="font-medium hover:text-app-primary hover:underline">{p.title}</a>
                {#if p.status === 'published'}<Pill tone="ok">Publicado</Pill>{:else}<Pill tone="warn">Borrador</Pill>{/if}
              </p>
              <p class="mt-0.5 text-xs text-app-muted">
                {p.category} · {p.reading_minutes} min ·
                {#if p.published_at}publicado el {blogDate(p.published_at)}{:else}escrito {ago(p.created_at)}{/if}
              </p>
            </div>
            <div class="flex flex-wrap gap-2">
              <a class="btn-secondary" href="/plataforma/blog/{p.id}"><Icon name="eye" size={16} />Revisar</a>
              {#if p.status === 'published'}<a class="btn-secondary" href="/blog/{p.slug}" target="_blank" rel="noopener">Ver en el sitio</a>{/if}
              {#if isAdmin}
                <button type="button" class="btn-secondary" disabled={rowOp.phase === 'loading'} onclick={() => toggle(p)}>
                  {p.status === 'published' ? 'Ocultar' : 'Publicar'}
                </button>
                <button type="button" class="btn-secondary text-app-danger" aria-label="Eliminar {p.title}" onclick={() => (removing = p)}><Icon name="trash" size={16} /></button>
              {/if}
            </div>
          </li>
        {/each}
      </ul>
    {/if}
  </section>
{/if}

<ConfirmModal open={removing !== null} title="¿Eliminar el artículo?" confirmLabel="Eliminar" op={removeOp} onconfirm={confirmRemove} onclose={() => (removing = null)}>
  {#if removing}<p><strong class="text-app-ink">{removing.title}</strong> se borra para siempre{removing.status === 'published' ? ' y deja de verse en el blog' : ''}. Si solo quieres quitarlo del sitio, mejor ocúltalo.</p>{/if}
</ConfirmModal>
