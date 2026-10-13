<script lang="ts">
  import { onMount, type Component } from 'svelte';

  /**
   * Carga una sección de la landing solo cuando hace falta: al acercarse a la pantalla o, a más tardar, cuando el
   * navegador queda libre. Así lo que se ve al abrir (menú y primer bloque) no compite con ~70 % del DOM y del JS.
   * El marcador conserva el id de ancla (#funciones, #precios…) y una altura mínima, y cede el id a la sección real.
   */
  let { load, id = undefined, height = '70vh' }: { load: () => Promise<{ default: Component }>; id?: string; height?: string } = $props();

  let Loaded = $state<Component | null>(null);
  let host = $state<HTMLElement>();

  function go() {
    if (!Loaded) load().then((m) => (Loaded = m.default));
  }

  onMount(() => {
    const io = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) {
          go();
          io.disconnect();
        }
      },
      { rootMargin: '1500px 0px' }
    );
    if (host) io.observe(host);
    // aunque el usuario no baje, se cargan todas en cuanto hay un respiro: así los enlaces del menú siempre encuentran su sección
    const idle = window.requestIdleCallback ?? ((f: () => void) => window.setTimeout(f, 2500));
    idle(go, { timeout: 5000 });
    return () => io.disconnect();
  });
</script>

{#if Loaded}
  <Loaded />
{:else}
  <div bind:this={host} {id} style="min-height: {height}" aria-hidden="true"></div>
{/if}
