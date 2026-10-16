<script lang="ts">
  import { onMount } from 'svelte';
  import { clamp, interp, reducedMotion, track } from './motion';

  type Kind = 'sticky' | 'card' | 'folder';
  interface Scrap {
    messy: string;
    kind: Kind;
    label: string;
    title: string;
    meta: string;
    tone: 'dental' | 'medica';
    from: [number, number, number];
  }

  // Each scrap starts scattered (offset from its grid slot, in vw/vh) and lands in the grid as a tidy record.
  const scraps: Scrap[] = [
    { messy: 'Llamar a Sra. López p/ cambiar cita ¿jueves?', kind: 'sticky', label: 'Cita', title: 'Mariana López', meta: 'Jue 16 · 09:00', tone: 'dental', from: [-4, 8, -14] },
    { messy: 'Expediente 0042 ¿¿dónde está??', kind: 'folder', label: 'Expediente', title: 'Diego Hernández', meta: 'Ortodoncia', tone: 'dental', from: [4, 18, 9] },
    { messy: 'Alergia: PENICILINA !!', kind: 'card', label: 'Antecedente', title: 'Alergia a penicilina', meta: 'Mariana López', tone: 'medica', from: [-8, 10, 12] },
    { messy: 'Lunes 10:30 — ¿doble cita?', kind: 'sticky', label: 'Agenda', title: 'Lun 10:30 · Dra. Ruiz', meta: 'Sin empalmes', tone: 'dental', from: [8, 4, -7] },
    { messy: 'Radiografía Diego H. → archivero 3', kind: 'card', label: 'Historial', title: 'Radiografía panorámica', meta: '12 feb · Odontología', tone: 'dental', from: [-8, -2, 6] },
    { messy: 'Peso 78 / talla 172 — J. Medina', kind: 'card', label: 'Expediente', title: 'Jorge Medina', meta: '78 kg · 172 cm', tone: 'medica', from: [12, 14, -11] },
    { messy: 'Control de presión S. Ramírez en 3 meses', kind: 'folder', label: 'Próxima consulta', title: 'Sofía Ramírez', meta: '15 ene · 11:15', tone: 'medica', from: [-4, -22, 15] },
    { messy: '¿Quién movió la cita de las 12?', kind: 'sticky', label: 'Permisos', title: 'Recepción', meta: 'Solo agenda', tone: 'medica', from: [6, 8, -16] }
  ];

  const paper: Record<Kind, string> = {
    sticky: 'bg-[#F5E7A0] shadow-[0_1px_2px_rgb(var(--l-shade)/0.10),0_12px_20px_-12px_rgb(var(--l-shade)/0.28)]',
    card: 'bg-[#FBFAF6] shadow-[0_1px_2px_rgb(var(--l-shade)/0.10),0_12px_20px_-12px_rgb(var(--l-shade)/0.28)] bg-[repeating-linear-gradient(transparent,transparent_23px,rgba(70,110,170,0.18)_24px)]',
    folder: 'bg-[#E4CF9E] shadow-[0_1px_2px_rgb(var(--l-shade)/0.10),0_12px_20px_-12px_rgb(var(--l-shade)/0.28),0_18px_28px_-16px_rgb(var(--l-shade)/0.32)] rounded-tr-xl'
  };

  let section: HTMLElement;
  let before: HTMLElement, after: HTMLElement, count: HTMLElement, bar: HTMLElement, desk: HTMLElement;
  const items: HTMLElement[] = [];
  const messyEls: HTMLElement[] = [];
  const cleanEls: HTMLElement[] = [];

  onMount(() => {
    const k = reducedMotion() ? 0 : 1;
    const small = window.innerWidth < 640;
    const vh = window.innerHeight / 100;
    const capY = small ? 24 : 72;
    return track(
      section,
      ['start start', 'end end'],
      (p) => {
        const t0 = clamp((p - 0.1) / 0.55, 0, 1); // 0 = messy desk, 1 = tidy paper
        desk.style.opacity = String(1 - t0);
        const b = interp(p, [0.3, 0.42], [1, 0]);
        before.style.opacity = String(b);
        before.style.transform = `translateY(${interp(p, [0.3, 0.42], [0, -12])}%)`;
        after.style.opacity = String(interp(p, [0.42, 0.55], [0, 1]));
        after.style.transform = `translateY(${interp(p, [0.42, 0.55], [12, 0])}%)`;
        count.textContent = String(Math.round(interp(p, [0.12, 0.75], [scraps.length, 0]))).padStart(2, '0');
        bar.style.transform = `scaleX(${interp(p, [0.12, 0.75], [0, 1])})`;

        scraps.forEach((s, i) => {
          const a = 0.12 + i * 0.025;
          const e = 0.55 + i * 0.025;
          const t = 1 - clamp((p - a) / (e - a), 0, 1); // 1 = scattered, 0 = tidy
          const dy = clamp(s.from[1] * vh, -capY, capY);
          items[i].style.transform = `translate3d(${s.from[0] * (small ? 0.5 : 1) * k * t}vw, ${dy * k * t}px, 0) rotate(${s.from[2] * (small ? 0.65 : 1) * k * t}deg)`;
          messyEls[i].style.opacity = String(interp(p, [e - 0.12, e], [1, 0]));
          cleanEls[i].style.opacity = String(interp(p, [e - 0.1, e + 0.02], [0, 1]));
        });
      },
      { stiffness: 120, damping: 30, mass: 0.3 }
    );
  });
</script>

<section bind:this={section} class="relative h-[260svh] bg-paper" aria-labelledby="chaos-title">
  <!-- the desk fades out as the papers get tidy; a static paper-coloured fade at the top blends the hero into it -->
  <div bind:this={desk} aria-hidden="true" class="pointer-events-none absolute inset-0 bg-desk"></div>
  <div aria-hidden="true" class="pointer-events-none absolute inset-x-0 top-0 h-24 bg-gradient-to-b from-paper to-transparent sm:h-36"></div>
  <div class="sticky top-0 z-10 flex h-[100svh] flex-col justify-center overflow-hidden px-5 sm:px-8">
    <div class="mx-auto w-full max-w-6xl">
      <div class="flex items-start justify-between gap-4 sm:items-end sm:gap-6">
        <div class="relative h-[2.35em] flex-1 font-display text-[clamp(2rem,8.2vw,3.25rem)] leading-[1.02] tracking-[-0.025em] sm:text-[clamp(3.25rem,6vw,5rem)]">
          <h2 id="chaos-title" class="sr-only">Del papel al orden: así cambia tu consultorio con Caresia</h2>
          <p bind:this={before} aria-hidden="true" class="absolute inset-0">
            Así se ve un consultorio<br /><span class="italic text-signal">con papel.</span>
          </p>
          <p bind:this={after} aria-hidden="true" class="absolute inset-0" style="opacity: 0">
            Y así se ve<br /><span class="italic text-mint">con Caresia.</span>
          </p>
        </div>
        <div class="block w-16 flex-none text-right sm:w-40" aria-hidden="true">
          <p class="font-mono text-[9px] uppercase leading-[1.4] tracking-[0.1em] text-ink-soft sm:text-[11px] sm:tracking-[0.14em]">Papeles sueltos</p>
          <p bind:this={count} class="font-display text-5xl tabular-nums leading-none sm:text-7xl">{String(scraps.length).padStart(2, '0')}</p>
          <div class="mt-2 h-px w-full bg-ink/15 sm:mt-3">
            <div bind:this={bar} class="h-px origin-left scale-x-0 bg-ink"></div>
          </div>
        </div>
      </div>

      <ul class="mt-8 grid grid-cols-2 gap-2.5 sm:mt-12 sm:gap-4 lg:grid-cols-4">
        {#each scraps as s, i}
          <li bind:this={items[i]} class="relative h-[100px] will-change-transform sm:h-[120px]">
            <div bind:this={messyEls[i]} class="absolute inset-0 p-3 sm:p-4 {paper[s.kind]}" aria-hidden="true">
              {#if s.kind === 'folder'}<span class="absolute -top-3 left-0 h-3 w-16 rounded-t-md bg-[#E4CF9E]"></span>{/if}
              <p class="font-hand text-[17px] leading-[1.05] text-[#0B2540]/85 sm:text-[22px]">{s.messy}</p>
            </div>
            <div bind:this={cleanEls[i]} class="absolute inset-0 flex flex-col justify-between rounded-2xl bg-panel p-3 ring-1 ring-ink/10 sm:p-4" style="opacity: 0">
              <span class="flex items-center gap-2 font-mono text-[9px] uppercase tracking-[0.14em] text-ink-faint sm:text-[10px]">
                <span class="h-1.5 w-1.5 rounded-full {s.tone === 'dental' ? 'bg-mint' : 'bg-signal'}"></span>
                {s.label}
              </span>
              <span>
                <span class="block truncate text-[13px] font-semibold sm:text-[15px]">{s.title}</span>
                <span class="block truncate text-[11px] text-ink-soft sm:text-[13px]">{s.meta}</span>
              </span>
            </div>
          </li>
        {/each}
      </ul>

      <p class="mt-8 max-w-md text-[15px] leading-relaxed text-ink-soft sm:mt-12 sm:text-base">
        Notas, carpetas y agendas de papel se convierten en expedientes, citas e historiales que cualquiera de tu equipo encuentra en segundos.
      </p>
    </div>
  </div>
</section>
