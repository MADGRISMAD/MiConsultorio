<script lang="ts">
  import { onMount } from 'svelte';
  import Wordmark from './Wordmark.svelte';
  import { contactEmail, demoHref, loginPath } from './data';
  import { interp, reducedMotion, track } from './motion';

  const cols: [string, [string, string][]][] = [
    ['Producto', [['#funciones', 'Funciones'], ['#incluye', 'Todo incluido'], ['#especialidades', 'Especialidades'], ['#precios', 'Precios']]],
    ['Soporte', [['#preguntas', 'Preguntas frecuentes'], [`mailto:${contactEmail}`, 'Contacto']]],
    ['Cuenta', [[loginPath, 'Iniciar sesión'], [demoHref, 'Solicitar demo']]]
  ];

  let footer: HTMLElement;
  let big: HTMLParagraphElement;
  onMount(() => {
    const reduce = reducedMotion();
    return track(footer, ['start end', 'end end'], (p) => {
      big.style.transform = `translate3d(0, ${reduce ? 0 : interp(p, [0, 1], [55, 0])}%, 0)`;
    });
  });
</script>

<footer bind:this={footer} class="l-deep relative mt-3 overflow-hidden bg-ink text-paper">
  <div class="mx-auto grid max-w-6xl gap-12 px-5 pb-10 pt-20 sm:px-8 md:grid-cols-[1.6fr_1fr_1fr_1fr]">
    <div>
      <Wordmark light />
      <p class="mt-5 max-w-xs text-[15px] leading-relaxed text-paper/55">Software de gestión para clínicas, consultorios, nutriólogos, veterinarias y más.</p>
    </div>
    {#each cols as [title, links]}
      <div>
        <p class="font-mono text-[11px] uppercase tracking-[0.16em] text-paper/45">{title}</p>
        <ul class="mt-5 space-y-3 text-[15px]">
          {#each links as [href, label]}
            <li><a {href} class="text-paper/80 transition-colors hover:text-signal">{label}</a></li>
          {/each}
        </ul>
      </div>
    {/each}
  </div>
  <div class="mx-auto max-w-6xl px-5 sm:px-8">
    <div class="flex justify-between border-t border-paper/10 py-6 font-mono text-[11px] uppercase tracking-[0.12em] text-paper/45">
      <span>© {new Date().getFullYear()} Caresia</span>
      <span>Todos los derechos reservados</span>
    </div>
  </div>
  <div aria-hidden="true" class="overflow-hidden">
    <p bind:this={big} class="select-none text-center font-display text-[27vw] leading-[0.78] tracking-[-0.04em] text-paper/[0.07]">Caresia</p>
  </div>
</footer>
