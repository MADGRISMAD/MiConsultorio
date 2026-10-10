<script lang="ts">
  import { toast } from '$lib/toast.svelte';
  import Icon from '$lib/components/ui/Icon.svelte';

  let { codes, onclose }: { codes: string[]; onclose: () => void } = $props();
  let saved = $state(false);

  const text = $derived(
    `Caresia: códigos de recuperación\nCada código sirve una sola vez. Guárdalos en un lugar seguro, fuera de este dispositivo.\n\n${codes.join('\n')}\n`
  );

  function download() {
    const url = URL.createObjectURL(new Blob([text], { type: 'text/plain;charset=utf-8' }));
    const a = document.createElement('a');
    a.href = url;
    a.download = 'caresia-codigos-de-recuperacion.txt';
    a.click();
    URL.revokeObjectURL(url);
  }

  async function copy() {
    try {
      await navigator.clipboard.writeText(codes.join('\n'));
      toast.show('Códigos copiados');
    } catch {
      toast.show('No se pudo copiar; descárgalos o imprímelos.');
    }
  }

  /** Prints from a hidden frame so no popup blocker gets in the way. */
  function print() {
    const frame = document.createElement('iframe');
    frame.setAttribute('aria-hidden', 'true');
    frame.style.cssText = 'position:fixed;width:0;height:0;border:0;right:0;bottom:0';
    const items = codes.map((c) => `<li>${c}</li>`).join('');
    frame.srcdoc = `<!doctype html><meta charset="utf-8"><title>Códigos de recuperación</title>
      <style>@page{size:letter;margin:0}body{font-family:system-ui,sans-serif;margin:0;padding:10mm}ul{columns:2;font:20px/2 ui-monospace,monospace;list-style:none;padding:0}</style>
      <h1>Caresia: códigos de recuperación</h1><p>Cada código sirve una sola vez. Guárdalos en un lugar seguro.</p><ul>${items}</ul>`;
    frame.onload = () => {
      frame.contentWindow?.print();
      setTimeout(() => frame.remove(), 2000);
    };
    document.body.appendChild(frame);
  }
</script>

<div class="mt-5" role="region" aria-labelledby="rec-title">
  <h3 id="rec-title" class="text-base font-semibold">Guarda tus códigos de recuperación</h3>
  <p class="mt-1 text-sm text-app-muted">
    Si pierdes tu teléfono, uno de estos códigos te deja entrar. <strong>Solo se muestran ahora</strong> y cada uno sirve una vez.
  </p>
  <ul class="mt-3 grid grid-cols-2 gap-x-6 gap-y-1 rounded-lg bg-app-ink/5 p-4 font-mono text-[15px] sm:max-w-md" aria-label="Códigos de recuperación">
    {#each codes as c (c)}<li class="select-all">{c}</li>{/each}
  </ul>
  <div class="mt-3 flex flex-wrap gap-2">
    <button type="button" class="btn-secondary" onclick={download}><Icon name="download" size={16} />Descargar</button>
    <button type="button" class="btn-secondary" onclick={print}>Imprimir</button>
    <button type="button" class="btn-secondary" onclick={copy}>Copiar</button>
  </div>
  <label class="mt-4 flex items-start gap-2 text-sm">
    <input type="checkbox" class="mt-0.5" bind:checked={saved} />
    <span>Ya guardé mis códigos en un lugar seguro.</span>
  </label>
  <div class="mt-3"><button type="button" class="btn-primary" disabled={!saved} onclick={onclose}>Listo</button></div>
</div>
