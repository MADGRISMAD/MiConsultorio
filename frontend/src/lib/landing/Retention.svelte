<script lang="ts">
  import Icon from './Icon.svelte';
  import Split from './Split.svelte';
  import { reveal } from './motion';

  // Todo esto ya funciona en el sistema: avisos de seguimiento por correo, felicitaciones con baja en un clic y lista de espera.
  const cards = [
    {
      kicker: 'Seguimiento automático',
      title: 'Les avisamos cuándo toca volver.',
      text: 'Cuando registras algo que se repite, Caresia le recuerda al paciente por correo, con un enlace para agendar. Sin que tu recepción tenga que acordarse.',
      points: ['Vacunas y refuerzos', 'Limpieza dental cada 6 meses', 'Papanicolaou', 'Renovación de anticonceptivo', 'Siguiente consulta que tú recomiendes'],
      tone: 'bg-signal-soft'
    },
    {
      kicker: 'Cumpleaños',
      title: 'Una felicitación que sí recuerdan.',
      text: 'Cada paciente recibe un correo en su cumpleaños con el nombre de tu consultorio. Es un detalle pequeño que los hace sentir cuidados y regresar.',
      points: ['Se envía sola, sin que lo captures', 'Cada paciente puede darse de baja en un clic', 'Respeta quién no quiere recibirlos'],
      tone: 'bg-mint-soft'
    },
    {
      kicker: 'Lista de espera',
      title: 'Un lugar libre no se queda vacío.',
      text: 'Si un paciente quiere una fecha que ya está llena, queda en la lista. Cuando alguien cancela, Caresia le avisa y su lugar se aparta unos minutos.',
      points: ['El paciente se apunta desde tu página de citas', 'Avisa al primero al que le queda el horario', 'Menos huecos por cancelaciones'],
      tone: 'bg-panel ring-1 ring-ink/10'
    }
  ];
</script>

<section id="retencion" class="mx-auto max-w-6xl px-5 py-28 sm:px-8 lg:py-36">
  <div class="grid gap-6 md:grid-cols-[1.6fr_1fr] md:items-end">
    <Split text={'Que tus pacientes\n*vuelvan solos.*'} class="font-display text-[clamp(2.4rem,5.6vw,4.6rem)] leading-[0.98] tracking-[-0.03em]" accent="italic text-signal" />
    <p class="max-w-sm text-lg leading-relaxed text-ink-soft md:justify-self-end">Un consultorio crece con pacientes que regresan. Caresia se encarga de recordárselos, felicitarlos y llenar los huecos.</p>
  </div>

  <ul class="mt-14 grid gap-5 lg:grid-cols-3">
    {#each cards as c, i}
      <li use:reveal={{ delay: i * 0.08 }} class="flex flex-col rounded-[28px] p-7 sm:p-8 {c.tone}">
        <p class="text-xs font-semibold uppercase tracking-[0.18em] text-ink/55">{c.kicker}</p>
        <h3 class="mt-3 font-display text-[1.9rem] leading-[1.05] tracking-[-0.02em]">{c.title}</h3>
        <p class="mt-3 text-[16px] leading-relaxed text-ink/65">{c.text}</p>
        <ul class="mt-6 grid gap-2.5 border-t border-ink/10 pt-5">
          {#each c.points as p}
            <li class="flex gap-3 text-[15px] leading-snug">
              <span class="mt-0.5 grid h-5 w-5 shrink-0 place-items-center rounded-full bg-white/70 text-signal"><Icon name="check" class="h-3.5 w-3.5" strokeWidth={2.2} /></span>
              {p}
            </li>
          {/each}
        </ul>
      </li>
    {/each}
  </ul>
</section>
