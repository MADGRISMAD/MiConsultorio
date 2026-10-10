<script lang="ts">
  import Icon from './Icon.svelte';
  import Split from './Split.svelte';
  import { reveal } from './motion';

  const panels = [
    {
      key: 'A',
      tab: 'Dental',
      area: 'Odontología',
      title: 'Clínica *dental*',
      text: 'Para el ritmo de un consultorio dental: citas cortas y seguidas y tratamientos de varias sesiones.',
      points: ['Odontograma interactivo con historial de versiones', 'Planes de tratamiento por fases, con presupuesto y firma', 'Dentición automática según la edad del paciente'],
      bg: 'bg-mint-soft',
      accent: 'text-mint'
    },
    {
      key: 'B',
      tab: 'Médico',
      area: 'Medicina general e interna',
      title: 'Consultorio *médico*',
      text: 'Un expediente completo para conocer a tu paciente antes de que entre, y su seguimiento consulta tras consulta.',
      points: ['Antecedentes, signos vitales y diagnósticos CIE-10', 'Laboratorio con tendencias por estudio', 'Recetas con alertas de alergias y calculadora de dosis'],
      bg: 'bg-signal-soft',
      accent: 'text-signal'
    },
    {
      key: 'C',
      tab: 'Nutrición',
      area: 'Nutrición',
      title: '*Nutrición*',
      text: 'De las medidas al menú de la semana, con el cálculo hecho por ti o con ayuda de IA.',
      points: ['Plan nutricional de 7 días con calorías y macronutrientes', 'Con IA (desde Crecimiento) genera el menú y evita lo que el paciente no come', 'Seguimiento de peso y cita de seguimiento al guardar'],
      bg: 'bg-panel ring-1 ring-ink/10',
      accent: 'text-mint'
    },
    {
      key: 'D',
      tab: 'Veterinaria',
      area: 'Veterinaria',
      title: '*Veterinaria*',
      text: 'El propietario primero y todas sus mascotas juntas, aunque se llamen igual.',
      points: ['Propietarios con varias mascotas, agrupadas', 'Carnet de vacunación y desparasitación', 'Registro de estética, paseos, adiestramiento y hospedaje'],
      bg: 'bg-signal-soft',
      accent: 'text-signal'
    },
    {
      key: 'E',
      tab: 'Pediatría y más',
      area: 'Pediatría, ginecología y dermatología',
      title: '*Pediatría* y más',
      text: 'Cada especialidad pide sus propios datos; Caresia pregunta solo lo que te sirve.',
      points: ['Carnet de vacunación y curvas de crecimiento', 'Antecedentes ginecológicos y fototipo de piel', 'Formularios y signos vitales a la medida'],
      bg: 'bg-mint-soft',
      accent: 'text-mint'
    },
    {
      key: 'F',
      tab: 'Psicología',
      area: 'Psicología',
      title: '*Psicología*',
      text: 'Sesiones y notas con la reserva que tu práctica necesita.',
      points: ['Notas privadas que solo tú puedes leer', 'Seguimiento por sesión: ánimo y valoración de riesgo', 'Consentimiento para atención psicológica firmado en pantalla'],
      bg: 'bg-panel ring-1 ring-ink/10',
      accent: 'text-signal'
    },
    {
      key: 'G',
      tab: 'Rehabilitación',
      area: 'Fisioterapia, quiropráctica y ortopedia',
      title: '*Rehabilitación*',
      text: 'Dónde duele, cuánto y cómo evoluciona, a la vista.',
      points: ['Esquema corporal frente y espalda con zonas de dolor', 'Planes por sesiones con costo', 'Comparación entre una visita y la anterior'],
      bg: 'bg-signal-soft',
      accent: 'text-signal'
    }
  ];

  let active = $state(0);
  const tabs: HTMLButtonElement[] = [];
  // Flechas izquierda/derecha entre pestañas (patrón de pestañas accesibles).
  function onKey(e: KeyboardEvent) {
    const d = e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : 0;
    if (!d) return;
    e.preventDefault();
    active = (active + d + panels.length) % panels.length;
    tabs[active]?.focus();
  }
</script>

<section id="especialidades" class="mx-auto max-w-6xl px-5 py-28 sm:px-8 lg:py-40">
  <div class="grid gap-6 border-b border-ink/15 pb-10 md:grid-cols-[1.7fr_1fr] md:items-end">
    <Split text={'Una plataforma.\n*Cada especialidad*\ncon sus herramientas.'} class="font-display text-[clamp(2.75rem,6.5vw,5.25rem)] leading-[0.95] tracking-[-0.03em]" accent="italic text-signal" />
    <p class="max-w-sm text-lg leading-relaxed text-ink-soft md:justify-self-end">Elige tu giro y Caresia ajusta formularios, secciones y reportes. Si atiendes más de uno, conviven en el mismo consultorio.</p>
  </div>
  <div role="tablist" aria-label="Especialidades" tabindex="-1" class="-mx-5 mt-10 flex gap-2 overflow-x-auto px-5 pb-1 [scrollbar-width:none] sm:mx-0 sm:flex-wrap sm:px-0" onkeydown={onKey}>
    {#each panels as p, i}
      <button
        bind:this={tabs[i]}
        role="tab"
        id="esp-tab-{p.key}"
        aria-controls="esp-panel-{p.key}"
        aria-selected={active === i}
        tabindex={active === i ? 0 : -1}
        class="flex-none rounded-full px-4 py-2 text-[15px] font-medium transition-colors {active === i ? 'bg-ink text-paper' : 'bg-panel text-ink ring-1 ring-ink/15 hover:ring-ink/40'}"
        onclick={() => (active = i)}>{p.tab}</button
      >
    {/each}
  </div>
  <div class="mt-5" use:reveal={{ y: 50 }}>
    {#each panels as p, i}
      <div id="esp-panel-{p.key}" role="tabpanel" aria-labelledby="esp-tab-{p.key}" hidden={active !== i} class="rounded-[28px] p-7 sm:p-9 {p.bg}">
        <div class="grid gap-8 md:grid-cols-[1fr_1fr] md:items-end">
          <div>
            <p class="font-mono text-[11px] uppercase tracking-[0.16em] text-ink-soft">{p.key} — {p.area}</p>
            <Split tag="h3" text={p.title} class="mt-8 font-display text-[clamp(2.5rem,5vw,4.25rem)] leading-[0.92] tracking-[-0.03em]" accent="italic {p.accent}" />
            <p class="mt-4 max-w-md text-[16px] leading-relaxed text-ink-soft">{p.text}</p>
          </div>
          <ul class="max-w-lg space-y-2.5">
            {#each p.points as t}
              <li class="flex gap-3 text-[15px] leading-snug">
                <span class="mt-0.5 grid h-5 w-5 flex-none place-items-center rounded-full bg-ink text-paper"><Icon name="check" class="h-3 w-3" strokeWidth={2.6} /></span>
                {t}
              </li>
            {/each}
          </ul>
        </div>
      </div>
    {/each}
  </div>
</section>
