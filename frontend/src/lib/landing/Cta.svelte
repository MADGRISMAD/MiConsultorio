<script lang="ts">
  import { onMount } from 'svelte';
  import ButtonLabel from './ButtonLabel.svelte';
  import Ecg from './Ecg.svelte';
  import Icon from './Icon.svelte';
  import Split from './Split.svelte';
  import { demoHref, loginPath } from './data';
  import { btn, interp, magnetic, reducedMotion, track } from './motion';

  let section: HTMLElement;
  let card: HTMLDivElement;

  onMount(() => {
    const reduce = reducedMotion();
    return track(section, ['start end', 'end end'], (p) => {
      card.style.transform = `scale(${reduce ? 1 : interp(p, [0, 1], [0.92, 1])})`;
      card.style.borderRadius = `${reduce ? 36 : interp(p, [0, 1], [80, 36])}px`;
    });
  });
</script>

<section id="contacto" bind:this={section} class="px-3 sm:px-5">
  <div bind:this={card} class="l-deep relative isolate overflow-hidden bg-signal px-6 py-24 text-center text-white sm:py-32">
    <Ecg class="absolute inset-x-0 top-1/2 -z-10 h-40 w-full -translate-y-1/2" base="stroke-white/15" sweep="stroke-white/70" beats={5} />
    <p class="font-mono text-[11px] uppercase tracking-[0.16em] text-white">Demo gratuita · 20 minutos</p>
    <Split text={'¿Listo para *ordenar*\ntu consulta?'} class="mx-auto mt-6 max-w-5xl font-display text-[clamp(3rem,9vw,8.5rem)] leading-[0.9] tracking-[-0.035em]" accent="italic" />
    <div class="mt-12 flex flex-wrap items-center justify-center gap-4">
      <div class="inline-flex" use:magnetic>
        <a href={demoHref} class="{btn} h-16 bg-ink pl-8 pr-2 text-[17px] text-paper hover:bg-paper hover:text-ink focus-visible:ring-offset-signal">
          <ButtonLabel>Solicitar demo gratis</ButtonLabel>
          <span class="ml-2 grid h-12 w-12 place-items-center rounded-full bg-signal text-white transition-transform duration-500 ease-out-strong group-hover:rotate-[-45deg]">
            <Icon name="arrow" class="h-5 w-5" />
          </span>
        </a>
      </div>
      <a href={loginPath} class="px-4 py-2 text-[16px] font-medium text-white underline decoration-white/40 underline-offset-4 hover:decoration-white">Ya soy cliente</a>
    </div>
  </div>
</section>
