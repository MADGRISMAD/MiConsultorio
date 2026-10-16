<script lang="ts">
  import SectionHead from './SectionHead.svelte';
  import { reveal } from './motion';

  const cards = [
    { label: 'Seguimiento', title: 'Recuerda la próxima visita.', text: 'Vacunas, limpieza a los 6 meses o lo que tú recomiendes: el paciente recibe un correo con una liga para agendar.', bg: 'bg-signal-soft' },
    { label: 'Cumpleaños', title: 'Una felicitación que sí recuerdan.', text: 'Cada paciente recibe un correo en su cumpleaños con el nombre de tu consultorio, y puede darse de baja en un clic.', bg: 'bg-mint-soft' },
    { label: 'Lista de espera', title: 'Aprovecha cada cancelación.', text: 'Si alguien cancela, Caresia avisa al primero de la lista y le aparta el lugar unos minutos.', bg: 'bg-panel ring-1 ring-ink/10' }
  ];
</script>

<section id="retencion" class="mx-auto max-w-6xl scroll-mt-24 px-5 py-16 sm:px-8 lg:py-24">
  <SectionHead title={'Dales seguimiento,\n*sin tantos pendientes.*'} blurb="Un consultorio crece con pacientes que regresan. Caresia se encarga de recordárselos por correo." />
  <ul class="mt-10 grid gap-4 md:grid-cols-3">
    {#each cards as c, i}
      <li use:reveal={{ y: 30, delay: i * 0.08 }} class="rounded-[22px] p-6 sm:p-7 {c.bg}">
        <p class="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-soft">{c.label}</p>
        <h3 class="mt-4 font-display text-[1.9rem] leading-[1.05] tracking-[-0.01em]">{c.title}</h3>
        <p class="mt-3 text-[15px] leading-relaxed text-ink-soft">{c.text}</p>
      </li>
    {/each}
  </ul>
</section>
