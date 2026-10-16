<script lang="ts">
  import Icon from './Icon.svelte';
  import SectionHead from './SectionHead.svelte';
  import Split from './Split.svelte';
  import Carousel from './Carousel.svelte';
  import { reveal } from './motion';
  import { specialties } from './data';

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
      tab: 'Psicología y Terapia',
      area: 'Psicología y Terapia',
      title: '*Psicología y Terapia*',
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

</script>

<section id="especialidades" class="mx-auto max-w-6xl scroll-mt-24 px-5 py-16 sm:px-8 lg:py-24">
  <SectionHead title={'Herramientas para\n*tu especialidad.*'} blurb="Elige tu giro y Caresia ajusta formularios, secciones y reportes. Si atiendes más de uno, conviven en el mismo consultorio." />
  <div class="mt-10" use:reveal={{ y: 50 }}>
    <Carousel items={panels} label={(p) => p.tab} name="Especialidades" id="esp" autoplay={false}>
      {#snippet panel(p)}
        <div class="rounded-[28px] p-7 sm:p-9 {p.bg}">
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
      {/snippet}
    </Carousel>
  </div>
  <p class="mt-8 text-[15px] leading-relaxed text-ink-soft"><span class="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-faint">También atiende</span><br />{specialties.join(' · ')}</p>
</section>
