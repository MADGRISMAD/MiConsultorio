<script lang="ts">
  import Alert from '$lib/components/ui/Alert.svelte';
  import { onMount } from 'svelte';
  import { mediaApi, type MediaOverview } from '$lib/api/profile';
  import { fileToPhoto } from '$lib/image';
  import { toast } from '$lib/toast.svelte';
  import Icon from '../ui/Icon.svelte';
  import LoadingRows from '../ui/LoadingRows.svelte';
  import Pill from '../ui/Pill.svelte';

  let ov = $state<MediaOverview | null>(null);
  let error = $state('');
  let busy = $state('');

  const load = async () => {
    try {
      ov = await mediaApi.overview();
    } catch (e) {
      error = e instanceof Error ? e.message : 'No se pudieron cargar las fotos.';
    }
  };
  onMount(load);

  /** one action at a time: pick a file, shrink it, send it, reload */
  async function put(key: string, slot: 'profile' | 'cover' | 'gallery' | 'pro', files: FileList | null, max: number, userId = '') {
    const file = files?.[0];
    if (!file) return;
    busy = key;
    error = '';
    try {
      await mediaApi.upload(slot, await fileToPhoto(file, max), userId);
      await load();
      toast.show('Foto guardada');
    } catch (e) {
      error = e instanceof Error ? e.message : 'No se pudo subir la foto.';
    } finally {
      busy = '';
    }
  }
  async function drop(id: string) {
    busy = id;
    try {
      await mediaApi.remove(id);
      await load();
    } catch (e) {
      error = e instanceof Error ? e.message : 'No se pudo quitar la foto.';
    } finally {
      busy = '';
    }
  }
  async function hide(id: string, hidden: boolean) {
    busy = id;
    try {
      await mediaApi.setHidden(id, hidden);
      await load();
    } catch (e) {
      error = e instanceof Error ? e.message : 'No se pudo guardar.';
    } finally {
      busy = '';
    }
  }
  const initials = (n: string) => n.split(/\s+/).filter(Boolean).slice(0, 2).map((w) => w[0]?.toUpperCase()).join('');
</script>

<section aria-labelledby="pm-h" class="mt-10">
  <h3 id="pm-h" class="section-title mb-1">Imágenes de tu página</h3>
  <p class="mb-4 text-sm text-app-muted">Las fotos se ajustan solas de tamaño. Se guardan en cuanto las eliges (no hace falta el botón Guardar de arriba).</p>
  {#if error}<Alert class="mb-3">{error}</Alert>{/if}
  {#if !ov}
    <LoadingRows />
  {:else}
    <div class="grid gap-5 sm:grid-cols-[auto_1fr]">
      <div>
        <p class="label">Imagen de perfil</p>
        <div class="relative h-28 w-28 overflow-hidden rounded-2xl bg-app-primary/10 ring-1 ring-app-ink/10">
          {#if ov.profile}<img src={mediaApi.url(ov.profile)} alt="Imagen de perfil" class="h-full w-full object-cover" />{:else}<span class="grid h-full place-items-center text-app-muted"><Icon name="building" size={30} /></span>{/if}
        </div>
        <div class="mt-2 flex flex-wrap gap-1.5">
          <label class="btn-secondary cursor-pointer !min-h-9 !px-3 text-sm"><input type="file" accept="image/*" class="sr-only" disabled={busy === 'profile'} onchange={(e) => { put('profile', 'profile', e.currentTarget.files, 600); e.currentTarget.value = ''; }} />{busy === 'profile' ? 'Subiendo…' : ov.profile ? 'Cambiar' : 'Subir'}</label>
          {#if ov.profile}<button type="button" class="btn-ghost !min-h-9 text-app-danger" onclick={() => drop(ov!.profile!)}>Quitar</button>{/if}
        </div>
      </div>
      <div class="min-w-0">
        <p class="label">Portada (banner)</p>
        <div class="relative aspect-[3/1] w-full overflow-hidden rounded-2xl bg-app-primary/10 ring-1 ring-app-ink/10">
          {#if ov.cover}<img src={mediaApi.url(ov.cover)} alt="Portada" class="h-full w-full object-cover" />{:else}<span class="grid h-full place-items-center text-sm text-app-muted">Sin portada: se usa un fondo con los colores de Caresia</span>{/if}
        </div>
        <div class="mt-2 flex flex-wrap gap-1.5">
          <label class="btn-secondary cursor-pointer !min-h-9 !px-3 text-sm"><input type="file" accept="image/*" class="sr-only" disabled={busy === 'cover'} onchange={(e) => { put('cover', 'cover', e.currentTarget.files, 1800); e.currentTarget.value = ''; }} />{busy === 'cover' ? 'Subiendo…' : ov.cover ? 'Cambiar' : 'Subir'}</label>
          {#if ov.cover}<button type="button" class="btn-ghost !min-h-9 text-app-danger" onclick={() => drop(ov!.cover!)}>Quitar</button>{/if}
          <span class="self-center text-xs text-app-muted">Mejor una foto horizontal (3:1).</span>
        </div>
      </div>
    </div>

    <div class="mt-6">
      <p class="label">Galería ({ov.gallery.length} de {ov.max_gallery}) <span class="font-normal text-app-muted">· se muestra como carrusel</span></p>
      <ul class="grid grid-cols-2 gap-3 sm:grid-cols-5">
        {#each ov.gallery as id (id)}
          <li class="relative aspect-[4/3] overflow-hidden rounded-xl ring-1 ring-app-ink/10">
            <img src={mediaApi.url(id)} alt="Foto de la galería" class="h-full w-full object-cover" />
            <button type="button" class="absolute right-1.5 top-1.5 grid h-7 w-7 place-items-center rounded-full bg-black/60 text-white" aria-label="Quitar foto" disabled={busy === id} onclick={() => drop(id)}><Icon name="x" size={14} /></button>
          </li>
        {/each}
        {#if ov.gallery.length < ov.max_gallery}
          <li class="aspect-[4/3]">
            <label class="grid h-full cursor-pointer place-items-center rounded-xl border-2 border-dashed border-app-ink/20 text-center text-sm text-app-muted transition hover:border-app-primary hover:text-app-primary">
              <input type="file" accept="image/*" class="sr-only" disabled={busy === 'gallery'} onchange={(e) => { put('gallery', 'gallery', e.currentTarget.files, 1600); e.currentTarget.value = ''; }} />
              <span><Icon name="plus" size={20} class="mx-auto" />{busy === 'gallery' ? 'Subiendo…' : 'Agregar foto'}</span>
            </label>
          </li>
        {/if}
      </ul>
    </div>

    <div class="mt-6">
      <p class="label">Especialistas en tu página</p>
      <p class="mb-3 text-xs text-app-muted">Aparece quien trabaja hoy en el consultorio y atiende pacientes en un giro que tienes activo. Quien se da de baja o cambia de giro deja de mostrarse solo; con «Mostrar» puedes ocultar a alguien sin dar de baja su cuenta.</p>
      <ul class="space-y-2">
        {#each ov.professionals as p (p.id)}
          <li class="flex flex-wrap items-center gap-3 rounded-xl border border-app-ink/10 p-3 {p.hidden || !p.active ? 'opacity-70' : ''}">
            <span class="relative h-14 w-14 flex-none overflow-hidden rounded-full bg-app-primary/10 ring-1 ring-app-ink/10">
              {#if p.photo}<img src={mediaApi.url(p.photo)} alt="Foto de {p.name}" class="h-full w-full object-cover" />{:else}<span class="grid h-full place-items-center text-sm font-semibold text-app-primary">{initials(p.name)}</span>{/if}
            </span>
            <span class="min-w-0 flex-1">
              <span class="block truncate font-medium">{p.name}</span>
              <span class="block truncate text-xs text-app-muted">{p.title || 'Sin título registrado (se captura en Mi cuenta)'}</span>
              {#if !p.active}<Pill tone="warn">No aparece: no atiende en un giro activo</Pill>{:else if p.hidden}<Pill tone="muted">Oculto</Pill>{/if}
            </span>
            <span class="flex flex-wrap items-center gap-1.5">
              <label class="btn-secondary cursor-pointer !min-h-9 !px-3 text-sm"><input type="file" accept="image/*" class="sr-only" disabled={busy === p.id} onchange={(e) => { put(p.id, 'pro', e.currentTarget.files, 600, p.id); e.currentTarget.value = ''; }} />{busy === p.id ? 'Subiendo…' : p.photo ? 'Cambiar foto' : 'Subir foto'}</label>
              {#if p.photo}<button type="button" class="btn-ghost !min-h-9 text-app-danger" onclick={() => drop(p.photo!)}>Quitar foto</button>{/if}
              <label class="flex cursor-pointer items-center gap-2 text-sm"><input type="checkbox" class="h-4 w-4 accent-[rgb(var(--app-primary))]" checked={!p.hidden} disabled={busy === p.id} onchange={(e) => hide(p.id, !e.currentTarget.checked)} />Mostrar</label>
            </span>
          </li>
        {/each}
      </ul>
    </div>
  {/if}
</section>
