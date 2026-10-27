<script lang="ts">
  import { fly } from 'svelte/transition';
  import { cubicOut } from 'svelte/easing';
  import Icon from './Icon.svelte';
  import Split from './Split.svelte';
  import SpecialtyArt from './SpecialtyArt.svelte';
  import { specialties } from './data';
  import { especialidades } from './especialidades';
  import { parallax, reducedMotion } from './motion';

  // Cada especialidad: una maqueta de producto propia (SpecialtyArt) y sus dos funciones distintivas.
  const items = [
    { kind: 'dental', tab: 'Dental', area: 'Odontología', text: 'Citas cortas y seguidas, y tratamientos de varias sesiones.', chips: ['Odontograma con historial de versiones', 'Planes por fases, con presupuesto y firma'], glow: 'mint' },
    { kind: 'medico', tab: 'Médico', area: 'Medicina general e interna', text: 'Conoce a tu paciente antes de que entre y dale seguimiento consulta tras consulta.', chips: ['Diagnósticos CIE-10 y signos vitales', 'Recetas con alertas de alergias y dosis'], glow: 'signal' },
    { kind: 'nutricion', tab: 'Nutrición', area: 'Nutrición', text: 'De las medidas al menú de la semana, con el cálculo hecho por ti o con ayuda de IA.', chips: ['Plan de 7 días con macronutrientes', 'Seguimiento de peso y citas'], glow: 'mint' },
    { kind: 'veterinaria', tab: 'Veterinaria', area: 'Veterinaria', text: 'El propietario primero y todas sus mascotas juntas, aunque se llamen igual.', chips: ['Varias mascotas por propietario', 'Carnet de vacunación y desparasitación'], glow: 'signal' },
    { kind: 'pediatria', tab: 'Pediatría y más', area: 'Pediatría, ginecología y dermatología', text: 'Cada especialidad pide sus propios datos; Caresia pregunta solo lo que te sirve.', chips: ['Curvas de crecimiento y vacunas', 'Antecedentes ginecológicos y fototipo'], glow: 'mint' },
    { kind: 'psicologia', tab: 'Psicología y Terapia', area: 'Psicología y Terapia', text: 'Sesiones y notas con la reserva que tu práctica necesita.', chips: ['Notas privadas, solo para ti', 'Consentimiento firmado en pantalla'], glow: 'signal' },
    { kind: 'rehab', tab: 'Rehabilitación', area: 'Fisioterapia, quiropráctica y ortopedia', text: 'Dónde duele, cuánto y cómo evoluciona, a la vista.', chips: ['Esquema corporal con zonas de dolor', 'Comparación con la visita anterior'], glow: 'mint' }
  ];

  let active = $state(0);
  const cur = $derived(items[active]);
  // páginas propias de cuatro giros (enlace real, para personas y buscadores)
  const page = $derived(especialidades.find((x) => x.tab === cur.tab));
  const reduce = reducedMotion();
  const btns: HTMLButtonElement[] = [];

  function onKey(e: KeyboardEvent) {
    const d = ['ArrowDown', 'ArrowRight'].includes(e.key) ? 1 : ['ArrowUp', 'ArrowLeft'].includes(e.key) ? -1 : 0;
    if (!d) return;
    e.preventDefault();
    active = (active + d + items.length) % items.length;
    btns[active]?.focus();
  }

  // Pointer parallax for the floating chips (mouse only).
  function aim(e: PointerEvent) {
    if (e.pointerType !== 'mouse' || reduce) return;
    const el = e.currentTarget as HTMLElement;
    const r = el.getBoundingClientRect();
    el.style.setProperty('--mx', ((e.clientX - r.left) / r.width - 0.5).toFixed(3));
    el.style.setProperty('--my', ((e.clientY - r.top) / r.height - 0.5).toFixed(3));
  }
</script>

<section id="especialidades" class="scroll-mt-24 px-3 py-10 sm:px-5 lg:py-14">
  <div class="l-deep relative isolate overflow-hidden rounded-[36px] bg-ink text-paper">

    <div class="mx-auto grid max-w-6xl gap-10 px-5 py-12 sm:px-8 lg:grid-cols-[0.95fr_1.05fr] lg:items-center lg:gap-14 lg:py-14">
      <div>
        <p class="flex items-center gap-2 font-mono text-[11px] uppercase tracking-[0.16em] text-paper/60"><span class="h-1.5 w-1.5 rounded-full bg-mint"></span>Especialidades</p>
        <Split text={'Herramientas para\n*tu especialidad.*'} class="mt-5 font-display text-[clamp(2.25rem,4.5vw,4rem)] leading-[0.98] tracking-[-0.03em]" accent="italic text-signal-soft" />
        <p class="mt-5 max-w-md text-base leading-relaxed text-paper/65 sm:text-lg">Elige tu giro y Caresia ajusta formularios, secciones y reportes. Si atiendes más de uno, conviven en el mismo consultorio.</p>

        <div role="tablist" aria-label="Especialidades" aria-orientation="vertical" tabindex="-1" onkeydown={onKey} class="relative mt-8 flex gap-2 overflow-x-auto pb-1 lg:flex-col lg:gap-0 lg:overflow-visible lg:pb-0">
          <span aria-hidden="true" class="absolute left-0 top-0 hidden h-12 w-[3px] rounded-full bg-mint transition-transform duration-500 ease-[cubic-bezier(0.23,1,0.32,1)] lg:block" style="transform: translateY({active * 48}px)"></span>
          {#each items as it, i}
            <button
              bind:this={btns[i]}
              role="tab"
              id="sp-tab-{i}"
              aria-selected={active === i}
              aria-controls="sp-panel"
              tabindex={active === i ? 0 : -1}
              onclick={() => (active = i)}
              class="group flex min-h-11 flex-none items-center gap-4 rounded-full px-4 py-2 text-left transition-[color,background-color,transform] duration-200 ease-out active:scale-[0.98] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-signal-soft lg:h-12 lg:rounded-none lg:px-6 lg:py-0 {active === i ? 'bg-paper/12 text-paper lg:bg-transparent' : 'text-paper/50 ring-1 ring-paper/15 hover:text-paper/85 lg:ring-0'}"
            >
              <span class="hidden font-mono text-[11px] tabular-nums text-paper/40 lg:block">0{i + 1}</span>
              <span class="whitespace-nowrap font-display text-[1.15rem] leading-none lg:text-[1.75rem]">{it.tab}</span>
            </button>
          {/each}
        </div>
        <p class="mt-7 text-[14px] leading-relaxed text-paper/70"><span class="font-mono text-[10px] uppercase tracking-[0.14em] text-paper/45">Páginas por especialidad</span><br />{#each especialidades as sp, i}<a href={sp.path} class="underline decoration-paper/25 underline-offset-4 transition-colors hover:text-paper hover:decoration-paper">{sp.tab}</a>{i < especialidades.length - 1 ? ' · ' : ''}{/each}</p>
        <p class="mt-5 hidden text-[13px] leading-relaxed text-paper/50 lg:block"><span class="font-mono text-[10px] uppercase tracking-[0.14em] text-paper/40">También atiende</span><br />{specialties.join(' · ')}</p>
      </div>

      <div id="sp-panel" role="tabpanel" tabindex="0" aria-labelledby="sp-tab-{active}" class="relative focus-visible:outline-none" onpointermove={aim} style="--mx: 0; --my: 0">
        <div use:parallax={-0.04} class="relative mx-auto max-w-md pb-12 pt-10 lg:max-w-none lg:px-8">
          {#key active}
            <div in:fly={{ y: reduce ? 0 : 22, duration: reduce ? 0 : 420, easing: cubicOut }} class="relative">
              <div class="rounded-[28px] bg-paper/[0.07] p-5 shadow-[0_40px_80px_-30px_rgb(0_0_0/0.6)] ring-1 ring-paper/15 backdrop-blur-xl sm:p-7">
                <div class="mb-5 flex items-center justify-between gap-3">
                  <div class="min-w-0"><p class="font-display text-3xl leading-none">{cur.tab}</p>{#if cur.area !== cur.tab}<p class="mt-1.5 truncate text-[12px] text-paper/55">{cur.area}</p>{/if}</div>
                  <span class="rounded-full bg-paper/10 px-2.5 py-1 font-mono text-[10px] uppercase tracking-[0.12em] text-paper/60">Ejemplo</span>
                </div>
                <SpecialtyArt kind={cur.kind} />
              </div>
              {#each cur.chips as c, ci}
                <div aria-hidden="true" class="pointer-events-none absolute z-10 hidden max-w-[15rem] sm:block {ci === 0 ? '-left-4 -top-9 lg:-left-8' : '-bottom-9 -right-4 lg:-right-8'}" style="transform: translate3d(calc(var(--mx) * {ci ? -34 : 26}px), calc(var(--my) * {ci ? -26 : 22}px), 0); transition: transform 0.6s cubic-bezier(0.2, 0.8, 0.2, 1)">
                  <div class="flex items-center gap-2.5 rounded-2xl bg-paper px-3.5 py-2.5 text-[13px] font-medium text-ink shadow-[0_20px_40px_-16px_rgb(0_0_0/0.5)]">
                    <span class="grid h-6 w-6 flex-none place-items-center rounded-full bg-mint text-white"><Icon name="check" class="h-3.5 w-3.5" strokeWidth={2.6} /></span>{c}
                  </div>
                </div>
              {/each}
            </div>
          {/key}
          <p class="mt-14 text-center text-[15px] leading-relaxed text-paper/60 lg:text-left">{cur.text}</p>
          {#if page}<p class="mt-3 text-center lg:text-left"><a href={page.path} class="text-[15px] font-medium text-paper underline decoration-paper/30 underline-offset-4 transition-colors hover:decoration-paper">{page.h1} →</a></p>{/if}
        </div>
      </div>
    </div>
  </div>
</section>
