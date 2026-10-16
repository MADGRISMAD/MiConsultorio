<script lang="ts">
  import Icon from './Icon.svelte';
  import SectionHead from './SectionHead.svelte';
  import { reveal } from './motion';

  const cards = [
    { title: 'Recetas', text: 'Con alertas de alergias antes de recetar y calculadora de dosis por peso.', points: ['Catálogo de medicamentos y el tuyo propio', 'Código QR para verificar la receta'] },
    { title: 'Cobros y caja', badge: 'Desde Crecimiento', text: 'Al terminar la consulta, la cuenta pasa sola a caja: sin capturar dos veces.', points: ['Punto de venta, caja con corte e inventario', 'Terminal y ligas de Mercado Pago, y facturación CFDI'] },
    { title: 'Portal del paciente', text: 'Tus pacientes reservan en línea y consultan sus citas, recetas y vacunas.', points: ['Liga de reserva de tu consultorio', 'Recordatorios automáticos por correo'] },
    { title: 'Reportes', text: 'Cómo va tu consultorio, con números reales y exportables.', points: ['Consultas, ausentismo, ocupación y tiempos de espera', 'Pacientes nuevos, recurrentes y los que no vuelven'] }
  ];
</script>

<section id="incluye" class="mx-auto max-w-6xl scroll-mt-24 px-5 py-16 sm:px-8 lg:py-24">
  <SectionHead title={'También resuelve\n*lo administrativo.*'} blurb="Del primer mensaje de la recepción al reporte del mes, en un solo sistema." />
  <ul class="mt-10 grid gap-4 md:grid-cols-2">
    {#each cards as c, i}
      <li use:reveal={{ y: 30, delay: (i % 2) * 0.08 }} class="rounded-[22px] bg-panel p-6 ring-1 ring-ink/10 sm:p-7">
        <div class="flex items-center justify-between gap-3">
          <h3 class="font-display text-3xl leading-none">{c.title}</h3>
          {#if c.badge}<span class="rounded-full bg-signal-soft px-3 py-1 font-mono text-[10px] uppercase tracking-[0.12em] text-signal">{c.badge}</span>{/if}
        </div>
        <p class="mt-3 text-base leading-relaxed text-ink-soft">{c.text}</p>
        <ul class="mt-5 space-y-2 border-t border-ink/10 pt-5">
          {#each c.points as p}
            <li class="flex gap-3 text-[15px] leading-snug"><Icon name="check" class="mt-0.5 h-4 w-4 flex-none text-mint" strokeWidth={2.4} />{p}</li>
          {/each}
        </ul>
      </li>
    {/each}
  </ul>
</section>
