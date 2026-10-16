<script lang="ts">
  import SectionHead from './SectionHead.svelte';
  import { parallax, reveal } from './motion';

  const cards = [
    { label: 'Seguimiento', title: 'Recuerda la próxima visita.', text: 'Vacunas, limpieza a los 6 meses o lo que tú recomiendes: el paciente recibe un correo con una liga para agendar.', bg: 'bg-signal-soft', speed: 0.05 },
    { label: 'Cumpleaños', title: 'Una felicitación que sí recuerdan.', text: 'Cada paciente recibe un correo en su cumpleaños con el nombre de tu consultorio, y puede darse de baja en un clic.', bg: 'bg-mint-soft', speed: -0.07 },
    { label: 'Lista de espera', title: 'Aprovecha cada cancelación.', text: 'Si alguien cancela, Caresia avisa al primero de la lista y le aparta el lugar unos minutos.', bg: 'bg-panel ring-1 ring-ink/10', speed: 0.05 }
  ];
  const confetti = [
    ['left-[12%] top-3', 'bg-signal'],
    ['left-[30%] top-6', 'bg-amber-400'],
    ['right-[18%] top-2', 'bg-mint'],
    ['right-[8%] top-8', 'bg-signal'],
    ['left-[55%] top-1', 'bg-amber-400']
  ];
</script>

<section id="retencion" class="relative isolate scroll-mt-24">
  <div class="mx-auto max-w-6xl px-5 py-16 sm:px-8 lg:py-24">
    <SectionHead title={'Dales seguimiento,\n*sin tantos pendientes.*'} blurb="Un consultorio crece con pacientes que regresan. Caresia se encarga de recordárselos por correo." />
    <ul class="mt-12 grid gap-5 md:grid-cols-3 md:items-start">
      {#each cards as c, i}
        <li use:parallax={c.speed} class={i === 1 ? 'md:mt-10' : ''}>
          <div use:reveal={{ y: 30, delay: i * 0.08 }} class="overflow-hidden rounded-[24px] p-5 shadow-[0_30px_60px_-30px_rgb(var(--l-shade)/0.3)] sm:p-6 {c.bg}">
            <!-- mini pantalla de ejemplo -->
            <div aria-hidden="true" class="relative h-40 overflow-hidden rounded-2xl bg-panel p-4 shadow-sm ring-1 ring-ink/10">
              {#if i === 0}
                <p class="font-mono text-[10px] uppercase tracking-[0.12em] text-ink-faint">Correo · Recordatorio</p>
                <p class="mt-2 font-display text-xl leading-tight">Toca tu limpieza dental</p>
                <p class="mt-1 text-[12px] text-ink-soft">Han pasado 6 meses desde tu última visita.</p>
                <span class="mt-3 inline-flex rounded-full bg-ink px-3.5 py-1.5 text-[12px] font-medium text-paper">Agendar mi cita</span>
              {:else if i === 1}
                {#each confetti as [pos, col]}<span class="absolute h-2 w-2 rounded-sm {pos} {col}"></span>{/each}
                <p class="mt-6 text-center font-display text-2xl leading-tight">Feliz cumpleaños,<br /><em class="italic text-signal">Mariana</em></p>
                <p class="mt-2 text-center text-[12px] text-ink-soft">Te desea tu consultorio</p>
              {:else}
                <p class="font-mono text-[10px] uppercase tracking-[0.12em] text-ink-faint">Lugar disponible · Jue 10:30</p>
                <ul class="mt-3 space-y-2 text-[13px]">
                  {#each [['Diego H.', 'Avisado ahora', true], ['Lucía G.', 'En la lista', false], ['Andrés T.', 'En la lista', false]] as [n, s, on]}
                    <li class="flex items-center justify-between rounded-xl px-3 py-1.5 {on ? 'bg-mint-soft' : 'bg-ink/5'}"><span class="font-medium">{n}</span><span class="text-[11px] text-ink-soft">{s}</span></li>
                  {/each}
                </ul>
              {/if}
            </div>
            <p class="mt-5 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-soft">{c.label}</p>
            <h3 class="mt-2 font-display text-[1.9rem] leading-[1.05] tracking-[-0.01em]">{c.title}</h3>
            <p class="mt-3 text-[15px] leading-relaxed text-ink-soft">{c.text}</p>
          </div>
        </li>
      {/each}
    </ul>
  </div>
</section>
