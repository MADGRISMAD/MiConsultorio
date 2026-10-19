<script lang="ts">
  import { onMount } from 'svelte';
  import ButtonLabel from './ButtonLabel.svelte';
  import Ecg from './Ecg.svelte';
  import Icon from './Icon.svelte';
  import Split from './Split.svelte';
  import { registerHref } from './data';
  import { btn, interp, reducedMotion, track } from './motion';

  let section: HTMLElement;
  let card: HTMLDivElement;

  onMount(() => {
    const reduce = reducedMotion();
    return track(section, ['start end', 'end end'], (p) => {
      card.style.transform = `scale(${reduce ? 1 : interp(p, [0, 1], [0.94, 1])})`;
      card.style.borderRadius = `${reduce ? 36 : interp(p, [0, 1], [64, 36])}px`;
    });
  });
</script>

<section id="contacto" bind:this={section} class="scroll-mt-24 px-3 sm:px-5">
  <div bind:this={card} class="l-deep relative isolate overflow-hidden bg-signal px-6 py-16 text-center text-white sm:py-24">
    <Ecg class="absolute inset-x-0 bottom-3 -z-10 h-32 w-full" base="stroke-white/15" sweep="stroke-white/70" beats={5} />
    <Split text={'Conoce Caresia\n*en una demo.*'} class="mx-auto max-w-4xl font-display text-[clamp(2.5rem,6vw,5rem)] leading-[0.95] tracking-[-0.035em]" accent="italic" />
    <p class="mx-auto mt-5 max-w-md text-lg leading-relaxed text-white/85">Te mostramos cómo funciona para tu especialidad.</p>
    <div class="mt-9 flex justify-center">
      <a href={registerHref} class="{btn} h-14 bg-ink pl-7 pr-2 text-[16px] text-paper hover:bg-paper hover:text-ink focus-visible:ring-offset-signal">
        <ButtonLabel>Solicitar demo</ButtonLabel>
        <span class="ml-2 grid h-10 w-10 place-items-center rounded-full bg-signal text-white transition-transform duration-500 ease-out-strong group-hover:rotate-[-45deg]">
          <Icon name="arrow" class="h-4 w-4" />
        </span>
      </a>
    </div>
    <p class="mt-5 font-mono text-[11px] uppercase tracking-[0.16em] text-white/90">Demo gratuita · 20 minutos · Sin compromiso</p>
  </div>
</section>
