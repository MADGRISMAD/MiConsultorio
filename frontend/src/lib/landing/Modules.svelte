<script lang="ts">
  import Icon from './Icon.svelte';
  import SectionHead from './SectionHead.svelte';
  import { parallax, reveal } from './motion';

  const card = 'relative overflow-hidden rounded-[24px] bg-panel p-6 shadow-[0_30px_60px_-34px_rgb(var(--l-shade)/0.3)] ring-1 ring-ink/10 sm:p-7';
  const bars = [38, 52, 44, 66, 58, 80, 72];
  const qr = Array.from({ length: 49 }, (_, i) => ((i * 7 + (i % 5) * 3 + Math.floor(i / 7)) % 3 !== 0));
</script>

<section id="incluye" class="relative isolate scroll-mt-24">
  <div class="mx-auto max-w-6xl px-5 py-16 sm:px-8 lg:py-24">
    <SectionHead title={'También resuelve\n*lo administrativo.*'} blurb="Del primer mensaje de la recepción al reporte del mes, en un solo sistema." />
    <div class="mt-12 grid gap-4 md:grid-cols-6">
      <!-- Cobros y caja (grande) -->
      <div use:reveal={{ y: 30 }} class="{card} md:col-span-4">
        <div class="grid gap-6 sm:grid-cols-[1fr_0.9fr] sm:items-center">
          <div>
            <div class="flex items-center gap-3"><h3 class="font-display text-3xl leading-none">Cobros y caja</h3><span class="rounded-full bg-signal-soft px-3 py-1 font-mono text-[10px] uppercase tracking-[0.12em] text-signal">Desde Crecimiento</span></div>
            <p class="mt-3 text-base leading-relaxed text-ink-soft">Al terminar la consulta, la cuenta pasa sola a caja: sin capturar dos veces.</p>
            <ul class="mt-4 space-y-2 text-[15px]">
              {#each ['Punto de venta, caja con corte e inventario', 'Terminal y ligas de Mercado Pago, y facturación CFDI'] as p}
                <li class="flex gap-3 leading-snug"><Icon name="check" class="mt-0.5 h-4 w-4 flex-none text-mint" strokeWidth={2.4} />{p}</li>
              {/each}
            </ul>
          </div>
          <div use:parallax={0.04} aria-hidden="true" class="rotate-[2deg] rounded-2xl bg-paper p-4 text-[13px] shadow-[0_24px_48px_-20px_rgb(var(--l-shade)/0.4)] ring-1 ring-ink/10">
            <p class="font-mono text-[10px] uppercase tracking-[0.12em] text-ink-faint">Cuenta · Ejemplo</p>
            <ul class="mt-3 divide-y divide-ink/10">
              <li class="flex justify-between py-2"><span>Consulta general</span><span class="tabular-nums">$650</span></li>
              <li class="flex justify-between py-2"><span>Receta y estudios</span><span class="tabular-nums">$300</span></li>
              <li class="flex justify-between py-2 font-semibold"><span>Total</span><span class="tabular-nums">$950</span></li>
            </ul>
            <span class="mt-3 flex h-9 items-center justify-center rounded-full bg-ink text-[12px] font-medium text-paper">Cobrar con terminal</span>
          </div>
        </div>
      </div>

      <!-- Recetas -->
      <div use:reveal={{ y: 30, delay: 0.08 }} class="{card} md:col-span-2">
        <h3 class="font-display text-3xl leading-none">Recetas</h3>
        <p class="mt-3 text-[15px] leading-relaxed text-ink-soft">Con alertas de alergias antes de recetar y dosis por peso.</p>
        <div aria-hidden="true" class="mt-5 flex items-center gap-4 rounded-2xl bg-paper p-4 ring-1 ring-ink/10">
          <div class="grid grid-cols-7 gap-[2px]">{#each qr as on}<span class="h-2 w-2 rounded-[1px] {on ? 'bg-ink' : 'bg-transparent'}"></span>{/each}</div>
          <p class="text-[12px] leading-snug text-ink-soft">Código QR para verificar la receta</p>
        </div>
      </div>

      <!-- Portal -->
      <div use:reveal={{ y: 30 }} class="{card} md:col-span-2">
        <h3 class="font-display text-3xl leading-none">Portal del paciente</h3>
        <p class="mt-3 text-[15px] leading-relaxed text-ink-soft">Tus pacientes reservan en línea y consultan sus citas, recetas y vacunas.</p>
        <div aria-hidden="true" class="mt-5 rounded-2xl bg-paper p-4 ring-1 ring-ink/10">
          <p class="font-mono text-[10px] uppercase tracking-[0.12em] text-ink-faint">Tu próxima cita</p>
          <p class="mt-1 font-display text-2xl leading-none">Jue 16 · 09:00</p>
          <span class="mt-3 inline-flex rounded-full bg-signal px-3.5 py-1.5 text-[12px] font-medium text-white">Reprogramar</span>
        </div>
      </div>

      <!-- Reportes -->
      <div use:reveal={{ y: 30, delay: 0.08 }} class="{card} md:col-span-4">
        <div class="grid gap-6 sm:grid-cols-[1fr_1fr] sm:items-end">
          <div>
            <h3 class="font-display text-3xl leading-none">Reportes</h3>
            <p class="mt-3 text-base leading-relaxed text-ink-soft">Cómo va tu consultorio, con números reales y exportables.</p>
            <ul class="mt-4 space-y-2 text-[15px]">
              {#each ['Consultas, ausentismo, ocupación y tiempos de espera', 'Pacientes nuevos, recurrentes y los que no vuelven'] as p}
                <li class="flex gap-3 leading-snug"><Icon name="check" class="mt-0.5 h-4 w-4 flex-none text-mint" strokeWidth={2.4} />{p}</li>
              {/each}
            </ul>
          </div>
          <div aria-hidden="true" class="rounded-2xl bg-paper p-4 ring-1 ring-ink/10">
            <p class="font-mono text-[10px] uppercase tracking-[0.12em] text-ink-faint">Ocupación semanal · Ejemplo</p>
            <div class="mt-3 flex h-24 items-end gap-2">
              {#each bars as h, i}<span class="flex-1 rounded-t-md {i === 5 ? 'bg-signal' : 'bg-ink/15'}" style="height: {h}%"></span>{/each}
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</section>
