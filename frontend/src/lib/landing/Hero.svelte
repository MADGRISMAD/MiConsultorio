<script lang="ts">
  import { onMount } from 'svelte';
  import ButtonLabel from './ButtonLabel.svelte';
  import Ecg from './Ecg.svelte';
  import Float, { type FloatCtx } from './Float.svelte';
  import Icon from './Icon.svelte';
  import Mark from './Mark.svelte';
  import Split from './Split.svelte';
  import { demoHref } from './data';
  import { btn, interp, magnetic, onFrame, reducedMotion, Spring, track } from './motion';

  // Day view shown in the product window: two doctors, two specialties, one clinic
  const HOUR = 8; // first hour on the grid
  const doctors = [
    {
      name: 'Dra. Ana Ruiz',
      area: 'Odontología',
      tone: 'dental',
      slots: [
        { start: 8, len: 0.75, who: 'Mariana López', what: 'Limpieza dental' },
        { start: 9.25, len: 1, who: 'Diego Hernández', what: 'Ajuste de ortodoncia' },
        { start: 11, len: 1.25, who: 'Andrés Torres', what: 'Endodoncia · 2.ª sesión' }
      ]
    },
    {
      name: 'Dr. Luis Paredes',
      area: 'Medicina general',
      tone: 'medica',
      slots: [
        { start: 8.5, len: 0.5, who: 'Jorge Medina', what: 'Consulta general' },
        { start: 10, len: 0.75, who: 'Sofía Ramírez', what: 'Control de presión' },
        { start: 11.5, len: 0.5, who: 'Lucía Gómez', what: 'Certificado médico' }
      ]
    }
  ];
  const hours = [8, 9, 10, 11, 12];
  const NOW = 10.6;
  const chip = 'rounded-2xl bg-panel/90 shadow-[0_24px_48px_-16px_rgba(11,37,64,0.3)] ring-1 ring-ink/10 backdrop-blur';

  let section: HTMLElement;
  let stage: HTMLDivElement;
  let text: HTMLDivElement;
  let ecg: HTMLDivElement;
  let window3d: HTMLDivElement;

  const ctx: FloatCtx = { p: 0, mx: 0, my: 0 };

  onMount(() => {
    const reduce = reducedMotion();
    const stops = [
      track(section, ['start start', 'end start'], (p) => {
        ctx.p = p;
        text.style.transform = `translate3d(0, ${reduce ? 0 : interp(p, [0, 1], [0, -160])}px, 0)`;
        text.style.opacity = String(interp(p, [0, 0.6], [1, 0]));
        ecg.style.transform = `translate3d(0, ${reduce ? 0 : interp(p, [0, 1], [0, 240])}px, 0)`;
      }),
      track(
        stage,
        ['start end', 'center 55%'],
        (p) => {
          const rotate = reduce ? 0 : interp(p, [0, 1], [28, 0]);
          const scale = reduce ? 1 : interp(p, [0, 1], [0.86, 1]);
          window3d.style.transform = `rotateX(${rotate}deg) scale(${scale})`;
        },
        { stiffness: 90, damping: 24, mass: 0.4 }
      )
    ];

    // pointer position, normalised to [-0.5, 0.5], eased
    const sx = new Spring(0, { stiffness: 60, damping: 18 });
    const sy = new Spring(0, { stiffness: 60, damping: 18 });
    let tx = 0;
    let ty = 0;
    stops.push(
      onFrame((dt) => {
        ctx.mx = sx.step(tx, dt);
        ctx.my = sy.step(ty, dt);
      })
    );
    const move = (e: PointerEvent) => {
      if (e.pointerType !== 'mouse') return;
      tx = e.clientX / window.innerWidth - 0.5;
      ty = e.clientY / window.innerHeight - 0.5;
    };
    section.addEventListener('pointermove', move);
    return () => {
      stops.forEach((s) => s());
      section.removeEventListener('pointermove', move);
    };
  });

</script>

<section id="inicio" bind:this={section} class="relative overflow-hidden pb-20 pt-32 sm:pt-40">
  <!-- soft light behind the headline -->
  <div aria-hidden="true" class="pointer-events-none absolute left-1/2 top-0 -z-10 h-[60rem] w-[90rem] -translate-x-1/2 bg-[radial-gradient(closest-side,rgba(22,115,209,0.10),transparent)]"></div>

  <div bind:this={text} class="mx-auto max-w-6xl px-5 sm:px-8">
    <div class="enter-fade flex items-center justify-between border-b border-ink/15 pb-4 font-mono text-[11px] uppercase tracking-[0.16em] text-ink-soft" style="--d: 0.1s">
      <span class="flex items-center gap-2"><span class="now-pulse h-1.5 w-1.5 rounded-full bg-signal"></span>Software clínico</span>
      <span class="hidden sm:block">Odontología · Medicina general</span>
    </div>

    <Split
      tag="h1"
      text={'Menos papeleo,\n*más consulta.*'}
      class="mt-8 font-display text-[clamp(3.4rem,15vw,11rem)] font-normal leading-[0.88] tracking-[-0.035em] text-ink"
      accent="italic text-signal"
      delay={0.15}
      stagger={0.09}
    />

    <div class="mt-10 grid gap-8 md:grid-cols-[1.1fr_1fr] md:items-end">
      <p class="enter-up max-w-md text-lg leading-relaxed text-ink-soft [text-wrap:pretty] sm:text-xl" style="--d: 0.7s">
        Agenda, expedientes e historial clínico para <span class="text-ink">clínicas dentales</span> y <span class="text-ink">consultorios médicos</span>. Todo en un solo lugar.
      </p>
      <div class="enter-up flex flex-wrap items-center gap-3 md:justify-end" style="--d: 0.85s">
        <div class="inline-flex" use:magnetic>
          <a href={demoHref} class="{btn} h-14 bg-ink pl-7 pr-2 text-[16px] text-paper hover:bg-signal">
            <ButtonLabel>Solicitar demo gratis</ButtonLabel>
            <span class="ml-2 grid h-10 w-10 place-items-center rounded-full bg-paper text-ink transition-transform duration-500 ease-out-strong group-hover:rotate-[-45deg]">
              <Icon name="arrow" class="h-4 w-4" />
            </span>
          </a>
        </div>
        <a href="#precios" class="group px-3 py-2 text-[16px] font-medium text-ink">
          <span class="bg-[linear-gradient(currentColor,currentColor)] bg-[length:100%_1px] bg-[position:0_100%] bg-no-repeat pb-0.5 transition-[background-size] duration-500 ease-out-strong group-hover:bg-[length:0%_1px] group-hover:bg-[position:100%_100%]">Ver precios</span>
        </a>
      </div>
    </div>
  </div>

  <!-- Product stage -->
  <div bind:this={stage} class="relative mx-auto mt-20 max-w-6xl px-3 sm:mt-24 sm:px-8 [perspective:1600px]">
    <div bind:this={ecg} aria-hidden="true" class="pointer-events-none absolute inset-x-[-20vw] top-[30%] -z-10 h-32">
      <Ecg class="h-full w-full" beats={6} />
    </div>

    <div bind:this={window3d} class="relative z-10 will-change-transform [transform-origin:50%_0%]" aria-hidden="true">
      <div class="overflow-hidden rounded-[22px] bg-panel shadow-[0_2px_4px_rgba(11,37,64,0.04),0_40px_80px_-24px_rgba(11,37,64,0.35)] ring-1 ring-ink/10">
        <div class="flex items-center gap-1.5 border-b border-ink/5 px-4 py-3">
          <span class="h-2.5 w-2.5 rounded-full bg-ink/10"></span>
          <span class="h-2.5 w-2.5 rounded-full bg-ink/10"></span>
          <span class="h-2.5 w-2.5 rounded-full bg-ink/10"></span>
          <span class="mx-auto rounded-md bg-paper px-3 py-1 font-mono text-[10px] text-ink-faint sm:text-[11px]">caresia.app/agenda</span>
        </div>
        <div class="flex">
          <aside class="hidden w-48 flex-none border-r border-ink/5 p-4 md:block">
            <div class="flex items-center gap-2"><Mark class="h-6 w-6" /><span class="font-display text-xl">Caresia</span></div>
            <ul class="mt-6 space-y-0.5 text-[13px]">
              {#each ['Agenda', 'Pacientes', 'Historiales', 'Usuarios'] as s, i}
                <li class="rounded-lg px-3 py-2 {i === 0 ? 'bg-paper font-medium text-ink' : 'text-ink-soft'}">{s}</li>
              {/each}
            </ul>
            <div class="mt-10 rounded-xl bg-paper p-3">
              <p class="font-mono text-[10px] uppercase tracking-[0.12em] text-ink-faint">Hoy</p>
              <p class="mt-1 font-display text-3xl leading-none">18</p>
              <p class="mt-1 text-[12px] text-ink-soft">citas agendadas</p>
            </div>
          </aside>
          <div class="min-w-0 flex-1 p-3 sm:p-5">
            <div class="flex items-end justify-between gap-3">
              <div>
                <p class="font-mono text-[10px] uppercase tracking-[0.14em] text-ink-faint">Martes</p>
                <p class="font-display text-2xl leading-none sm:text-3xl">14 de octubre</p>
              </div>
              <span class="rounded-full bg-ink px-3 py-1 text-[11px] font-medium text-paper">+ Nueva cita</span>
            </div>
            <div class="mt-4 grid grid-cols-[2.25rem_1fr_1fr] gap-x-2 sm:grid-cols-[3rem_1fr_1fr] sm:gap-x-3">
              <span></span>
              {#each doctors as d}
                <div class="flex min-w-0 items-center gap-2 pb-2">
                  <span class="h-2 w-2 flex-none rounded-full {d.tone === 'dental' ? 'bg-mint' : 'bg-signal'}"></span>
                  <span class="min-w-0">
                    <span class="block truncate text-[12px] font-medium sm:text-[13px]">{d.name}</span>
                    <span class="block truncate text-[10px] text-ink-faint sm:text-[11px]">{d.area}</span>
                  </span>
                </div>
              {/each}
              <div class="relative col-span-3 grid grid-cols-[2.25rem_1fr_1fr] gap-x-2 [--h:44px] sm:grid-cols-[3rem_1fr_1fr] sm:gap-x-3 sm:[--h:58px]" style="height: calc(var(--h) * 5)">
                <!-- hour lines -->
                {#each hours as h, i}
                  <div class="pointer-events-none absolute inset-x-0 border-t border-dashed border-ink/[0.08]" style="top: calc(var(--h) * {i})">
                    <span class="absolute -top-2 left-0 bg-panel pr-1 font-mono text-[9px] tabular-nums text-ink-faint sm:text-[10px]">{String(h).padStart(2, '0')}:00</span>
                  </div>
                {/each}
                <span></span>
                {#each doctors as d, di}
                  <div class="relative">
                    {#each d.slots as s, si}
                      <div
                        class="slot-in absolute inset-x-0 overflow-hidden rounded-lg border-l-[3px] px-2 py-1 sm:px-2.5 sm:py-1.5 {d.tone === 'dental' ? 'border-mint bg-mint-soft' : 'border-signal bg-signal-soft'}"
                        style="top: calc(var(--h) * {s.start - HOUR} + 2px); height: calc(var(--h) * {s.len} - 4px); animation-delay: {1.1 + (si * 2 + di) * 0.09}s"
                      >
                        <p class="truncate text-[10px] font-semibold sm:text-[12px]">{s.who}</p>
                        <p class="truncate text-[9px] text-ink-soft sm:text-[11px]">{s.what}</p>
                      </div>
                    {/each}
                  </div>
                {/each}
                <!-- current time -->
                <div class="pointer-events-none absolute inset-x-0 flex items-center" style="top: calc(var(--h) * {NOW - HOUR})">
                  <span class="now-pulse h-2 w-2 -translate-x-1/2 rounded-full bg-signal"></span>
                  <span class="h-px flex-1 bg-signal"></span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <Float depth={1.3} {ctx} delay={1.5} class="-left-2 top-[-3rem] hidden md:block lg:-left-14">
      <div class="{chip} w-64 p-4">
        <div class="flex items-center gap-3">
          <span class="grid h-10 w-10 place-items-center rounded-full bg-mint-soft font-display text-lg text-mint">ML</span>
          <span>
            <span class="block text-sm font-semibold">Mariana López</span>
            <span class="block font-mono text-[10px] uppercase tracking-[0.12em] text-ink-faint">Expediente · 34 años</span>
          </span>
        </div>
        <div class="mt-3 flex items-center gap-2 rounded-xl bg-amber-50 px-3 py-2 text-[12px] font-medium text-amber-700 ring-1 ring-inset dark:bg-amber-400/10 dark:text-amber-300 dark:ring-amber-400/25 ring-amber-600/15">
          <Icon name="warn" class="h-3.5 w-3.5" strokeWidth={2.2} />
          Alergia a penicilina
        </div>
      </div>
    </Float>

    <Float depth={2} {ctx} delay={1.7} class="-right-1 top-[18%] sm:right-2 lg:-right-12">
      <div class="{chip} flex items-center gap-3 py-3 pl-3 pr-5">
        <span class="grid h-9 w-9 place-items-center rounded-full bg-mint text-white"><Icon name="check" class="h-4 w-4" strokeWidth={2.4} /></span>
        <span>
          <span class="block text-[13px] font-semibold">Cita confirmada</span>
          <span class="block text-[12px] text-ink-soft">Jorge Medina · 08:30</span>
        </span>
      </div>
    </Float>

    <Float depth={0.9} {ctx} delay={1.9} class="-left-4 bottom-[8%] hidden sm:block lg:-left-20">
      <div class="{chip} grid grid-cols-2 gap-4 px-5 py-4">
        <span>
          <span class="block font-mono text-[10px] uppercase tracking-[0.12em] text-ink-faint">Peso</span>
          <span class="mt-1 block font-display text-3xl leading-none">78<span class="text-base text-ink-soft"> kg</span></span>
        </span>
        <span>
          <span class="block font-mono text-[10px] uppercase tracking-[0.12em] text-ink-faint">Estatura</span>
          <span class="mt-1 block font-display text-3xl leading-none">172<span class="text-base text-ink-soft"> cm</span></span>
        </span>
      </div>
    </Float>

    <Float depth={2.6} {ctx} delay={2.1} class="-bottom-10 right-6 hidden md:block lg:-right-6">
      <div class="w-52 rotate-[5deg] bg-[#F5E7A0] p-4 pb-6 font-hand text-[24px] leading-[1.05] text-ink shadow-[0_20px_40px_-14px_rgba(11,37,64,0.4)]">¡Adiós a la agenda de papel!</div>
    </Float>
  </div>
</section>
