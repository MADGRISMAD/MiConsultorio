<script lang="ts">
  import { onMount } from 'svelte';
  import Split from './Split.svelte';
  import { reveal, track } from './motion';

  const steps = [
    { title: 'Agenda una demo', text: 'Te mostramos Caresia en 20 minutos con ejemplos de tu especialidad.' },
    { title: 'Damos de alta tu clínica', text: 'Configuramos tu cuenta, tus usuarios y sus permisos.' },
    { title: 'Empieza a atender', text: 'Registra pacientes y citas desde el primer día. Tu equipo aprende en una tarde.' }
  ];

  let wrap: HTMLDivElement;
  let line: HTMLDivElement;
  onMount(() => track(wrap, ['start 80%', 'end 60%'], (p) => (line.style.transform = `scaleX(${p})`)));
</script>

<section class="l-deep relative overflow-hidden rounded-t-[36px] bg-ink px-5 py-28 text-paper sm:px-8 lg:py-40">
  <div class="mx-auto max-w-6xl">
    <div class="grid gap-6 md:grid-cols-[1.7fr_1fr] md:items-end">
      <Split text={'Listo en días,\n*no en meses.*'} class="font-display text-[clamp(2.75rem,6.5vw,5.25rem)] leading-[0.95] tracking-[-0.03em]" accent="italic text-signal" />
      <p class="max-w-sm text-lg leading-relaxed text-paper/60 md:justify-self-end">Sin instalaciones, sin servidores y sin capacitaciones eternas.</p>
    </div>
    <div bind:this={wrap} class="relative mt-20">
      <div class="absolute left-0 right-0 top-[27px] hidden h-px bg-paper/15 md:block">
        <div bind:this={line} class="h-px origin-left scale-x-0 bg-signal"></div>
      </div>
      <ol class="grid gap-12 md:grid-cols-3 md:gap-8">
        {#each steps as s, i}
          <li class="relative">
            <div use:reveal={{ delay: i * 0.12 }}>
              <span class="relative grid h-14 w-14 place-items-center rounded-full bg-ink font-display text-2xl italic text-signal ring-1 ring-paper/20">{i + 1}</span>
              <h3 class="mt-8 font-display text-4xl leading-none">{s.title}</h3>
              <p class="mt-4 max-w-xs text-[16px] leading-relaxed text-paper/60">{s.text}</p>
            </div>
          </li>
        {/each}
      </ol>
    </div>
  </div>
</section>
