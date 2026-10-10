<script lang="ts">
  import { untrack } from 'svelte';
  import Modal from '$lib/components/Modal.svelte';
  import { canvasToPhoto } from '$lib/image';

  interface Props {
    /** the picture the person chose; null closes the window */
    file: File | null;
    /** width / height of the frame the photo will have on the public page */
    aspect: number;
    /** pixels of the width of the saved photo */
    outWidth: number;
    title?: string;
    hint?: string;
    ondone: (dataUrl: string) => void;
    oncancel: () => void;
  }
  let { file, aspect, outWidth, title = 'Elige qué parte mostrar', hint = '', ondone, oncancel }: Props = $props();

  let bitmap = $state<ImageBitmap | null>(null);
  let src = $state('');
  $effect(() => {
    if (!file) return;
    const u = URL.createObjectURL(file);
    src = u;
    return () => URL.revokeObjectURL(u);
  });
  let error = $state('');
  let fw = $state(300); // frame size on screen
  const fh = $derived(fw / aspect);
  let zoom = $state(1);
  let cx = $state(0); // the point of the picture (natural pixels) under the center of the frame
  let cy = $state(0);

  $effect(() => {
    const f = file;
    untrack(() => bitmap?.close?.());
    bitmap = null;
    error = '';
    if (!f) return;
    if (!f.type.startsWith('image/')) {
      error = 'Elige una imagen (JPG, PNG o WebP).';
      return;
    }
    if (f.size > 25 * 1024 * 1024) {
      error = 'La imagen pesa demasiado. Elige una de menos de 25 MB.';
      return;
    }
    createImageBitmap(f).then(
      (b) => {
        bitmap = b;
        zoom = 1;
        cx = b.width / 2;
        cy = b.height / 2;
      },
      () => (error = 'No se pudo leer la imagen. Prueba con otra.')
    );
  });

  const base = $derived(bitmap ? Math.max(fw / bitmap.width, fh / bitmap.height) : 1);
  const s = $derived(base * zoom); // screen pixels per picture pixel
  function clamp() {
    if (!bitmap) return;
    cx = Math.min(Math.max(cx, fw / (2 * s)), bitmap.width - fw / (2 * s));
    cy = Math.min(Math.max(cy, fh / (2 * s)), bitmap.height - fh / (2 * s));
  }
  $effect(() => {
    void zoom;
    void fw;
    clamp();
  });

  let drag = $state<{ x: number; y: number } | null>(null);
  function down(e: PointerEvent) {
    drag = { x: e.clientX, y: e.clientY };
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
  }
  function move(e: PointerEvent) {
    if (!drag) return;
    cx -= (e.clientX - drag.x) / s;
    cy -= (e.clientY - drag.y) / s;
    drag = { x: e.clientX, y: e.clientY };
    clamp();
  }
  const up = () => (drag = null);

  let busy = $state(false);
  function confirm() {
    if (!bitmap) return;
    busy = true;
    try {
      const canvas = document.createElement('canvas');
      canvas.width = Math.round(Math.min(outWidth, (fw / s) * 1)); // never upscale past the picture's own pixels
      canvas.height = Math.round(canvas.width / aspect);
      const ctx = canvas.getContext('2d')!;
      ctx.fillStyle = '#fff';
      ctx.fillRect(0, 0, canvas.width, canvas.height);
      ctx.drawImage(bitmap, cx - fw / (2 * s), cy - fh / (2 * s), fw / s, fh / s, 0, 0, canvas.width, canvas.height);
      ondone(canvasToPhoto(canvas));
    } catch (e) {
      error = e instanceof Error ? e.message : 'No se pudo recortar la foto.';
    } finally {
      busy = false;
    }
  }
</script>

<Modal open={!!file} {title} onclose={oncancel}>
  {#if error}
    <p class="alert" role="alert">{error}</p>
  {:else if !bitmap}
    <p class="text-sm text-app-muted" role="status">Cargando la imagen…</p>
  {:else}
    <p class="mb-3 text-sm text-app-muted">Arrastra la foto para encuadrarla y usa el zoom. Lo que ves dentro del marco es lo que se mostrará.{hint ? ` ${hint}` : ''}</p>
    <div class="mx-auto w-full max-w-sm" bind:clientWidth={fw}>
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div
        class="relative touch-none select-none overflow-hidden rounded-xl bg-app-ink/10 ring-2 ring-app-primary"
        style="height: {fh}px; cursor: {drag ? 'grabbing' : 'grab'};"
        onpointerdown={down}
        onpointermove={move}
        onpointerup={up}
        onpointercancel={up}
      >
        <img
          {src}
          alt="Vista previa del recorte"
          draggable="false"
          class="pointer-events-none absolute max-w-none"
          style="width: {bitmap.width * s}px; height: {bitmap.height * s}px; left: {fw / 2 - cx * s}px; top: {fh / 2 - cy * s}px;"
        />
      </div>
    </div>
    <label class="mx-auto mt-4 flex max-w-sm items-center gap-3 text-sm font-medium">
      Zoom
      <input type="range" class="w-full accent-[rgb(var(--app-primary))]" min="1" max="4" step="0.01" bind:value={zoom} aria-label="Zoom" />
    </label>
  {/if}
  {#snippet footer()}
    <button type="button" class="btn-secondary" onclick={oncancel}>Cancelar</button>
    <button type="button" class="btn-primary" disabled={!bitmap || busy} onclick={confirm}>Usar esta parte</button>
  {/snippet}
</Modal>
