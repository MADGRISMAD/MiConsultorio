<script lang="ts">
  import type { Component } from 'svelte';
  import Icon from './Icon.svelte';
  import Split from './Split.svelte';
  import { EASE, reducedMotion, whileInView } from './motion';
  import Agenda from './screens/Agenda.svelte';
  import Expediente from './screens/Expediente.svelte';
  import Historial from './screens/Historial.svelte';
  import Permisos from './screens/Permisos.svelte';

  interface Step {
    n: string;
    title: string;
    text: string;
    points: string[];
    Screen: Component;
  }
  const steps: Step[] = [
    {
      n: '01',
      title: 'Una agenda que *todo tu equipo* comparte.',
      text: 'Crea, reprograma o cancela citas en segundos. Odontología y medicina general en la misma vista, sin empalmes ni recados perdidos.',
      points: ['Vista por día y por profesional', 'Reprograma en segundos'],
      Screen: Agenda
    },
    {
      n: '02',
      title: 'El paciente completo, *en una pantalla.*',
      text: 'Datos generales, antecedentes, alergias y hábitos de salud. Capturados una vez y siempre a la mano. Búscalo por CURP al instante.',
      points: ['Antecedentes patológicos', 'Búsqueda por CURP'],
      Screen: Expediente
    },
    {
      n: '03',
      title: 'Cada consulta, *en su lugar.*',
      text: 'El historial se ordena solo. Llega a la consulta sabiendo exactamente qué pasó la vez anterior, sea una limpieza o un control de presión.',
      points: ['Orden cronológico', 'Tratamientos de varias sesiones'],
      Screen: Historial
    },
    {
      n: '04',
      title: 'Cada quien ve *lo que le toca.*',
      text: 'Recepción agenda, el doctor consulta y tú administras. Los permisos por rol protegen la información clínica de tus pacientes.',
      points: ['Roles por usuario', 'Acceso con contraseña'],
      Screen: Permisos
    }
  ];

  let active = $state(0);
  const reduce = reducedMotion();

  // The screen swaps with a short blur-out, then the next one rises in.
  function swap(_: Element, { enter }: { enter: boolean }) {
    return {
      duration: enter ? 500 : 250,
      delay: enter ? 250 : 0,
      css: (t: number, u: number) => {
        if (reduce) return `opacity: ${t}`;
        const dir = enter ? 40 : -40;
        return `opacity: ${t}; transform: translateY(${u * dir}px); filter: blur(${u * 8}px)`;
      }
    };
  }

  const Screen = $derived(steps[active].Screen);
</script>

<section id="funciones" class="relative">
  <div class="mx-auto max-w-6xl px-5 pt-28 sm:px-8 lg:pt-40">
    <div class="grid gap-6 border-b border-ink/15 pb-10 md:grid-cols-[1.7fr_1fr] md:items-end">
      <Split text={'Todo tu consultorio.\n*Nada que estorbe.*'} class="font-display text-[clamp(2.75rem,6.5vw,5.25rem)] leading-[0.95] tracking-[-0.03em]" accent="italic text-signal" />
      <p class="max-w-sm text-lg leading-relaxed text-ink-soft md:justify-self-end">Cuatro herramientas que reemplazan la libreta, el archivero y las hojas de cálculo.</p>
    </div>

    <div class="grid lg:grid-cols-2 lg:gap-16">
      <div>
        {#each steps as s, i}
          {@const Inline = s.Screen}
          <div class="flex flex-col justify-center py-14 lg:min-h-[85vh] lg:py-0" use:whileInView={{ margin: '-45% 0px -45% 0px', cb: (v) => v && (active = i) }}>
            <div class="transition-opacity duration-500 lg:pr-8" style="opacity: {active === i ? 1 : 0.25}">
              <p class="font-mono text-sm text-signal">{s.n} <span class="text-ink-faint">/ 04</span></p>
              <Split tag="h3" text={s.title} class="mt-4 font-display text-[clamp(2.25rem,4.5vw,3.75rem)] leading-[1] tracking-[-0.02em]" accent="italic" />
              <p class="mt-5 max-w-md text-lg leading-relaxed text-ink-soft [text-wrap:pretty]">{s.text}</p>
              <ul class="mt-6 flex flex-wrap gap-2">
                {#each s.points as p}
                  <li class="flex items-center gap-1.5 rounded-full border border-ink/15 px-3 py-1.5 text-[13px]">
                    <Icon name="check" class="h-3.5 w-3.5 text-mint" strokeWidth={2.4} />{p}
                  </li>
                {/each}
              </ul>
            </div>
            <!-- small screens: the screen follows its text -->
            <div class="mt-8 rounded-[22px] bg-panel p-5 shadow-[0_30px_60px_-30px_rgba(11,37,64,0.35)] ring-1 ring-ink/10 lg:hidden" aria-hidden="true">
              <Inline />
            </div>
          </div>
        {/each}
      </div>
      <div class="hidden lg:block" aria-hidden="true">
        <div class="sticky top-0 flex h-screen items-center">
          <div class="relative w-full">
            <div class="absolute -inset-6 -z-10 rounded-[40px] bg-gradient-to-br from-mint-soft via-transparent to-signal-soft opacity-70 blur-2xl"></div>
            <div class="relative grid h-[29rem] overflow-hidden rounded-[26px] bg-panel p-7 shadow-[0_40px_80px_-30px_rgba(11,37,64,0.4)] ring-1 ring-ink/10">
              {#key active}
                <div class="[grid-area:1/1]" in:swap={{ enter: true }} out:swap={{ enter: false }}>
                  <Screen />
                </div>
              {/key}
            </div>
            <div class="mt-6 flex gap-2">
              {#each steps as s, i}
                <span class="h-[3px] flex-1 overflow-hidden rounded-full bg-ink/10">
                  <span class="block h-full origin-left bg-ink" style="transform: scaleX({i <= active ? 1 : 0}); transition: transform 0.6s {EASE}"></span>
                </span>
              {/each}
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</section>
