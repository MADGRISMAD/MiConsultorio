<script lang="ts">
  import Drawing from './Drawing.svelte';
  import Icon from './Icon.svelte';
  import Split from './Split.svelte';
  import { reveal } from './motion';

  const toothPath = 'M100 40c-14-14-34-19-50-13-26 9-34 38-26 66 6 21 14 33 17 56 3 21 6 45 20 45 17 0 15-52 39-52s22 52 39 52c14 0 17-24 20-45 3-23 11-35 17-56 8-28 0-57-26-66-16-6-36-1-50 13z';
  const stethPaths = ['M50 20v60a45 45 0 0 0 90 0V20', 'M95 125v20a50 50 0 0 0 100 0v-15', 'M195 110a20 20 0 1 0 0.1 0', 'M38 20h24M128 20h24'];

  const panels = [
    {
      key: 'A',
      area: 'Odontología',
      title: 'Clínica *dental*',
      text: 'Para el ritmo de un consultorio dental: citas cortas y seguidas, tratamientos de varias sesiones y un equipo entre el sillón y la recepción.',
      points: ['Ortodoncia, endodoncia o implantes con todas sus sesiones en un expediente', 'Agenda para limpiezas, revisiones y urgencias del día', 'Asistentes con acceso a la agenda, sin ver lo clínico'],
      bg: 'bg-mint-soft',
      accent: 'text-mint',
      paths: [toothPath],
      viewBox: '0 0 200 220',
      drawingClass: 'absolute -right-8 -top-6 h-64 w-64 text-mint/50 sm:h-80 sm:w-80'
    },
    {
      key: 'B',
      area: 'Medicina general',
      title: 'Consultorio *médico*',
      text: 'Un expediente completo para conocer a tu paciente antes de que entre al consultorio, y su seguimiento consulta tras consulta.',
      points: ['Antecedentes: diabetes, cardiopatías, alergias, cirugías y más', 'Peso, estatura y hábitos de salud en cada expediente', 'Seguimiento de pacientes crónicos con su historial a la mano'],
      bg: 'bg-signal-soft',
      accent: 'text-signal',
      paths: stethPaths,
      viewBox: '0 0 230 230',
      drawingClass: 'absolute -right-6 -top-4 h-64 w-64 text-signal/50 sm:h-80 sm:w-80'
    }
  ];
</script>

<section id="especialidades" class="mx-auto max-w-6xl px-5 py-28 sm:px-8 lg:py-40">
  <div class="grid gap-6 border-b border-ink/15 pb-10 md:grid-cols-[1.7fr_1fr] md:items-end">
    <Split text={'Dos especialidades.\n*Una sola plataforma.*'} class="font-display text-[clamp(2.75rem,6.5vw,5.25rem)] leading-[0.95] tracking-[-0.03em]" accent="italic text-signal" />
    <p class="max-w-sm text-lg leading-relaxed text-ink-soft md:justify-self-end">Caresia se adapta a la forma en que trabaja tu especialidad, sin configuraciones complicadas.</p>
  </div>
  <div class="mt-10 flex flex-col gap-4 lg:h-[36rem] lg:flex-row">
    {#each panels as p, i}
      <article
        use:reveal={{ y: 60, delay: i * 0.12, duration: 1, margin: '0px 0px -10% 0px' }}
        class="group relative flex min-h-[32rem] flex-col justify-between overflow-hidden rounded-[28px] p-7 transition-[flex-grow] duration-700 ease-out-strong sm:p-10 lg:min-h-0 lg:flex-1 lg:hover:flex-[1.45] {p.bg}"
      >
        <Drawing paths={p.paths} viewBox={p.viewBox} class={p.drawingClass} />
        <div class="relative flex items-center justify-between font-mono text-[11px] uppercase tracking-[0.16em] text-ink-soft">
          <span>{p.key} — {p.area}</span>
        </div>
        <div class="relative">
          <Split tag="h3" text={p.title} class="font-display text-[clamp(3rem,6vw,5.5rem)] leading-[0.9] tracking-[-0.03em]" accent="italic {p.accent}" />
          <p class="mt-5 max-w-md text-[17px] leading-relaxed text-ink-soft">{p.text}</p>
          <ul class="mt-6 max-w-md space-y-2.5">
            {#each p.points as t}
              <li class="flex gap-3 text-[15px] leading-snug">
                <span class="mt-0.5 grid h-5 w-5 flex-none place-items-center rounded-full bg-ink text-paper"><Icon name="check" class="h-3 w-3" strokeWidth={2.6} /></span>
                {t}
              </li>
            {/each}
          </ul>
        </div>
      </article>
    {/each}
  </div>
</section>
