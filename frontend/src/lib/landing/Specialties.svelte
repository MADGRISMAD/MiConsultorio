<script lang="ts">
  import Icon from './Icon.svelte';
  import Split from './Split.svelte';
  import { reveal } from './motion';

  const panels = [
    {
      key: 'A',
      area: 'Odontología',
      title: 'Clínica *dental*',
      text: 'Para el ritmo de un consultorio dental: citas cortas y seguidas y tratamientos de varias sesiones.',
      points: ['Odontograma interactivo con historial de versiones', 'Planes de tratamiento por fases, con presupuesto y firma', 'Dentición automática según la edad del paciente'],
      bg: 'bg-mint-soft',
      accent: 'text-mint'
    },
    {
      key: 'B',
      area: 'Medicina general e interna',
      title: 'Consultorio *médico*',
      text: 'Un expediente completo para conocer a tu paciente antes de que entre, y su seguimiento consulta tras consulta.',
      points: ['Antecedentes, signos vitales y diagnósticos CIE-10', 'Laboratorio con tendencias por estudio', 'Recetas con alertas de alergias y calculadora de dosis'],
      bg: 'bg-signal-soft',
      accent: 'text-signal'
    },
    {
      key: 'C',
      area: 'Nutrición',
      title: '*Nutrición*',
      text: 'De las medidas al menú de la semana, con el cálculo hecho por ti o con ayuda de IA.',
      points: ['Plan nutricional de 7 días con calorías y macronutrientes', 'Genera el menú con IA y evita lo que el paciente no come', 'Seguimiento de peso y cita de seguimiento al guardar'],
      bg: 'bg-panel ring-1 ring-ink/10',
      accent: 'text-mint'
    },
    {
      key: 'D',
      area: 'Veterinaria',
      title: '*Veterinaria*',
      text: 'El propietario primero y todas sus mascotas juntas, aunque se llamen igual.',
      points: ['Propietarios con varias mascotas, agrupadas', 'Carnet de vacunación y desparasitación', 'Registro de estética, paseos, adiestramiento y hospedaje'],
      bg: 'bg-signal-soft',
      accent: 'text-signal'
    },
    {
      key: 'E',
      area: 'Pediatría, ginecología y dermatología',
      title: '*Pediatría* y más',
      text: 'Cada especialidad pide sus propios datos; Caresia pregunta solo lo que te sirve.',
      points: ['Carnet de vacunación y curvas de crecimiento', 'Antecedentes ginecológicos y fototipo de piel', 'Formularios y signos vitales a la medida'],
      bg: 'bg-mint-soft',
      accent: 'text-mint'
    },
    {
      key: 'F',
      area: 'Psicología',
      title: '*Psicología*',
      text: 'Sesiones y notas con la reserva que tu práctica necesita.',
      points: ['Notas privadas que solo tú puedes leer', 'Seguimiento por sesión: ánimo y valoración de riesgo', 'Consentimiento para atención psicológica firmado en pantalla'],
      bg: 'bg-panel ring-1 ring-ink/10',
      accent: 'text-signal'
    },
    {
      key: 'G',
      area: 'Fisioterapia, quiropráctica y ortopedia',
      title: '*Rehabilitación*',
      text: 'Dónde duele, cuánto y cómo evoluciona, a la vista.',
      points: ['Esquema corporal frente y espalda con zonas de dolor', 'Planes por sesiones con costo', 'Comparación entre una visita y la anterior'],
      bg: 'bg-signal-soft',
      accent: 'text-signal'
    }
  ];
</script>

<section id="especialidades" class="mx-auto max-w-6xl px-5 py-28 sm:px-8 lg:py-40">
  <div class="grid gap-6 border-b border-ink/15 pb-10 md:grid-cols-[1.7fr_1fr] md:items-end">
    <Split text={'Una plataforma.\n*Cada especialidad*\ncon sus herramientas.'} class="font-display text-[clamp(2.75rem,6.5vw,5.25rem)] leading-[0.95] tracking-[-0.03em]" accent="italic text-signal" />
    <p class="max-w-sm text-lg leading-relaxed text-ink-soft md:justify-self-end">Elige tu giro y Caresia ajusta formularios, secciones y reportes. Si atiendes más de uno, conviven en el mismo consultorio.</p>
  </div>
  <ul class="mt-10 grid gap-4 md:grid-cols-2">
    {#each panels as p, i}
      <li class="flex {i === panels.length - 1 && panels.length % 2 ? 'md:col-span-2' : ''}" use:reveal={{ y: 50, delay: (i % 2) * 0.1, margin: '0px 0px -8% 0px' }}>
        <article class="flex w-full flex-col justify-between rounded-[28px] p-7 sm:p-9 {p.bg}">
          <div class="flex items-center justify-between font-mono text-[11px] uppercase tracking-[0.16em] text-ink-soft"><span>{p.key} — {p.area}</span></div>
          <div class="mt-8">
            <Split tag="h3" text={p.title} class="font-display text-[clamp(2.5rem,5vw,4.25rem)] leading-[0.92] tracking-[-0.03em]" accent="italic {p.accent}" />
            <p class="mt-4 max-w-md text-[16px] leading-relaxed text-ink-soft">{p.text}</p>
            <ul class="mt-5 max-w-lg space-y-2.5">
              {#each p.points as t}
                <li class="flex gap-3 text-[15px] leading-snug">
                  <span class="mt-0.5 grid h-5 w-5 flex-none place-items-center rounded-full bg-ink text-paper"><Icon name="check" class="h-3 w-3" strokeWidth={2.6} /></span>
                  {t}
                </li>
              {/each}
            </ul>
          </div>
        </article>
      </li>
    {/each}
  </ul>
</section>
