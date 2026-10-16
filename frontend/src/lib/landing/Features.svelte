<script lang="ts">
  import type { Component } from 'svelte';
  import { fade, fly } from 'svelte/transition';
  import { cubicOut } from 'svelte/easing';
  import Em from './Em.svelte';
  import Icon from './Icon.svelte';
  import SectionHead from './SectionHead.svelte';
  import { parallax, reducedMotion, reveal } from './motion';
  import Agenda from './screens/Agenda.svelte';
  import Expediente from './screens/Expediente.svelte';
  import Historial from './screens/Historial.svelte';

  interface Tab {
    tab: string;
    title: string;
    text: string;
    points: string[];
    Screen: Component;
    chips: { icon: 'check' | 'warn'; text: string }[];
  }
  // Tres herramientas, una a la vez y a elección de quien lee: sin scroll fijado ni texto atenuado.
  const tabs: Tab[] = [
    {
      tab: 'Agenda',
      title: 'Organiza las citas *de tu equipo.*',
      text: 'Crea, reprograma o cancela en segundos. Odontología y medicina general en la misma vista, sin empalmes ni recados perdidos.',
      points: ['Vista por día y por profesional', 'Reprograma en segundos'],
      Screen: Agenda,
      chips: [
        { icon: 'check', text: 'Cita confirmada · Jorge Medina' },
        { icon: 'check', text: 'Sin empalmes entre profesionales' }
      ]
    },
    {
      tab: 'Expediente',
      title: 'El paciente completo, *en una pantalla.*',
      text: 'Datos generales, antecedentes, alergias y hábitos de salud. Se capturan una vez y quedan siempre a la mano. Búscalo por CURP al instante.',
      points: ['Antecedentes patológicos', 'Búsqueda por CURP'],
      Screen: Expediente,
      chips: [
        { icon: 'warn', text: 'Alergia a penicilina' },
        { icon: 'check', text: 'Aviso de privacidad firmado' }
      ]
    },
    {
      tab: 'Historial',
      title: 'Cada consulta, *en su lugar.*',
      text: 'El historial se ordena solo. Llega a la consulta sabiendo qué pasó la vez anterior, sea una limpieza o un control de presión.',
      points: ['Orden cronológico', 'Tratamientos de varias sesiones'],
      Screen: Historial,
      chips: [
        { icon: 'check', text: 'Tratamiento en curso · 2 de 4' },
        { icon: 'check', text: 'Próxima cita agendada' }
      ]
    }
  ];

  let active = $state(0);
  const reduce = reducedMotion();
  const Screen = $derived(tabs[active].Screen);
  const btns: HTMLButtonElement[] = [];

  function onKey(e: KeyboardEvent) {
    const d = e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : 0;
    if (!d) return;
    e.preventDefault();
    active = (active + d + tabs.length) % tabs.length;
    btns[active]?.focus();
  }
</script>

<section id="funciones" class="relative isolate scroll-mt-24">
  <div class="mx-auto max-w-6xl px-5 py-16 sm:px-8 lg:py-24">
    <SectionHead title={'Tu consulta,\n*en orden.*'} blurb="Tres herramientas que reemplazan la libreta, el archivero y las hojas de cálculo." />

    <div class="mt-10 grid gap-10 lg:grid-cols-[0.9fr_1.1fr] lg:items-center lg:gap-16">
      <div>
        <div role="tablist" aria-label="Herramientas" tabindex="-1" onkeydown={onKey} class="flex flex-wrap gap-2">
          {#each tabs as t, i}
            <button
              bind:this={btns[i]}
              role="tab"
              id="f-tab-{i}"
              aria-selected={active === i}
              aria-controls="f-panel"
              tabindex={active === i ? 0 : -1}
              onclick={() => (active = i)}
              class="min-h-11 rounded-full px-5 text-[15px] font-medium transition-[background-color,color,box-shadow,transform] duration-200 ease-out active:scale-[0.97] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-signal {active === i ? 'bg-ink text-paper' : 'text-ink ring-1 ring-ink/20 hover:bg-ink/5'}"
            >
              {t.tab}
            </button>
          {/each}
        </div>

        <div id="f-panel" role="tabpanel" tabindex="0" aria-labelledby="f-tab-{active}" class="mt-8 min-h-[17rem] focus-visible:outline-none">
          {#key active}
            <div in:fly={{ y: reduce ? 0 : 14, duration: reduce ? 0 : 320, easing: cubicOut }}>
              <h3 class="font-display text-[clamp(2.1rem,3.8vw,3.25rem)] leading-[1] tracking-[-0.02em]"><Em text={tabs[active].title} /></h3>
              <p class="mt-5 max-w-md text-base leading-relaxed text-ink-soft [text-wrap:pretty] sm:text-lg">{tabs[active].text}</p>
              <ul class="mt-6 flex flex-wrap gap-2">
                {#each tabs[active].points as p}
                  <li class="flex items-center gap-1.5 rounded-full border border-ink/15 px-3 py-1.5 text-[13px]">
                    <Icon name="check" class="h-3.5 w-3.5 text-mint" strokeWidth={2.4} />{p}
                  </li>
                {/each}
              </ul>
            </div>
          {/key}
        </div>
      </div>

      <div aria-hidden="true" use:reveal={{ y: 30 }} class="relative px-3 py-6 sm:px-6">
        <div use:parallax={0.05} class="relative">
          <div class="relative grid min-h-[24rem] overflow-hidden rounded-[26px] bg-panel p-6 shadow-[0_50px_90px_-30px_rgb(var(--l-shade)/0.45),0_2px_4px_rgb(var(--l-shade)/0.06)] ring-1 ring-ink/10 sm:min-h-[27rem] sm:p-7">
            {#key active}
              <div class="[grid-area:1/1]" in:fade={{ duration: reduce ? 0 : 280 }}>
                <Screen />
              </div>
            {/key}
          </div>
        </div>
        {#key active}
          {#each tabs[active].chips as c, ci}
            <div use:parallax={ci ? -0.07 : 0.1} class="absolute z-10 hidden sm:block {ci ? '-bottom-2 -right-2 lg:-right-6' : '-left-3 top-2 lg:-left-8'}" in:fly={{ y: reduce ? 0 : 16, duration: reduce ? 0 : 420, delay: 160 + ci * 90, easing: cubicOut }}>
              <div class="flex items-center gap-2.5 rounded-2xl bg-panel/90 px-3.5 py-2.5 text-[13px] font-medium shadow-[0_24px_48px_-16px_rgb(var(--l-shade)/0.35)] ring-1 ring-ink/10 backdrop-blur">
                <span class="grid h-6 w-6 flex-none place-items-center rounded-full {c.icon === 'warn' ? 'bg-amber-100 text-amber-700' : 'bg-mint text-white'}"><Icon name={c.icon} class="h-3.5 w-3.5" strokeWidth={2.4} /></span>{c.text}
              </div>
            </div>
          {/each}
        {/key}
      </div>
    </div>
  </div>
</section>
