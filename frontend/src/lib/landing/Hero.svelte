<script lang="ts">
  import { onMount, tick } from 'svelte';
  import ButtonLabel from './ButtonLabel.svelte';
  import Ecg from './Ecg.svelte';
  import Float, { type FloatCtx } from './Float.svelte';
  import Icon from './Icon.svelte';
  import Mark from './Mark.svelte';
  import Split from './Split.svelte';
  import { registerHref } from './data';
  import { btn, clamp, interp, onFrame, reducedMotion, Spring, track } from './motion';

  // Day view shown in the product window: two doctors, two specialties, one clinic
  const HOUR = 8; // first hour on the grid
  const doctors = [
    { name: 'Dra. Ana Ruiz', area: 'Odontología', tone: 'dental' },
    { name: 'Dr. Luis Paredes', area: 'Medicina general', tone: 'medica' }
  ];

  interface Appt {
    id: number;
    col: number;
    start: number;
    len: number;
    who: string;
    what: string;
  }
  // The day is live: appointments can be dragged to another time or doctor, and a clash springs back.
  let appts = $state<Appt[]>([
    { id: 0, col: 0, start: 8, len: 0.75, who: 'Mariana López', what: 'Limpieza dental' },
    { id: 1, col: 0, start: 9.25, len: 1, who: 'Diego Hernández', what: 'Ajuste de ortodoncia' },
    { id: 2, col: 0, start: 11, len: 1.25, who: 'Andrés Torres', what: 'Endodoncia · 2.ª sesión' },
    { id: 3, col: 1, start: 8.5, len: 0.5, who: 'Jorge Medina', what: 'Consulta general' },
    { id: 4, col: 1, start: 10, len: 0.75, who: 'Sofía Ramírez', what: 'Control de presión' },
    { id: 5, col: 1, start: 11.5, len: 0.5, who: 'Lucía Gómez', what: 'Certificado médico' }
  ]);
  let status = $state('');
  const stories = [
    { tone: 'bg-mint', title: 'Cita confirmada', sub: 'Jorge Medina · 08:30' },
    { tone: 'bg-signal', title: 'Recordatorio enviado', sub: 'Sofía Ramírez · mañana 11:15' },
    { tone: 'bg-mint', title: 'Receta firmada', sub: 'Andrés Torres · Dra. Ruiz' }
  ];
  let story = $state(0);
  let revenue = $state(0);
  let tiltEl: HTMLDivElement;
  const hours = [8, 9, 10, 11, 12];
  const NOW = 10.6;
  const chip = 'rounded-2xl bg-panel/90 shadow-[0_24px_48px_-16px_rgba(11,37,64,0.3)] ring-1 ring-ink/10 backdrop-blur';

  let section: HTMLElement;
  let stage: HTMLDivElement;
  let text: HTMLDivElement;
  let ecg: HTMLDivElement;
  let window3d: HTMLDivElement;

  const ctx: FloatCtx = { p: 0, mx: 0, my: 0 };

  let lane: HTMLDivElement;
  const STEP = 0.25; // snap to 15 minutes
  const DAY = 5; // hours on the grid
  const hhmm = (h: number) => `${String(Math.floor(h)).padStart(2, '0')}:${String(Math.round((h % 1) * 60)).padStart(2, '0')}`;

  // Visual offset of each card from its slot: dragged 1:1, then released into a spring that inherits the pointer velocity.
  interface Fly {
    el: HTMLElement;
    x: Spring;
    y: Spring;
    dragging: boolean;
  }
  const flying = new Map<number, Fly>();
  const SPRING = { stiffness: 260, damping: 26, mass: 1 }; // slightly under-damped: a thrown card overshoots a touch
  const paint = (f: Fly) => {
    f.el.style.transform = `translate3d(${f.x.value}px, ${f.y.value}px, 0) scale(${f.dragging ? 1.04 : 1})`;
  };

  let drag: { id: number; sx: number; sy: number; bx: number; by: number; vx: number; vy: number; t: number } | null = null;

  function press(e: PointerEvent, a: Appt) {
    if (e.pointerType === 'touch' || e.button !== 0 || reducedMotion()) return;
    const el = e.currentTarget as HTMLElement;
    touched = true;
    if (ghostEl) ghostEl.dataset.on = '0';
    el.setPointerCapture(e.pointerId);
    el.classList.remove('slot-in');
    // grabbing a card that is still flying picks it up where it is (never from its target)
    const f = flying.get(a.id) ?? { el, x: new Spring(0, SPRING), y: new Spring(0, SPRING), dragging: true };
    f.dragging = true;
    f.x.velocity = 0;
    f.y.velocity = 0;
    flying.set(a.id, f);
    el.dataset.drag = '1';
    drag = { id: a.id, sx: e.clientX, sy: e.clientY, bx: f.x.value, by: f.y.value, vx: 0, vy: 0, t: e.timeStamp };
    status = '';
  }

  function move(e: PointerEvent, a: Appt) {
    if (!drag || drag.id !== a.id) return;
    const f = flying.get(a.id)!;
    const x = drag.bx + e.clientX - drag.sx;
    const y = drag.by + e.clientY - drag.sy;
    const dt = Math.max((e.timeStamp - drag.t) / 1000, 0.001);
    drag.vx = drag.vx * 0.7 + ((x - f.x.value) / dt) * 0.3; // smoothed release velocity (px/s)
    drag.vy = drag.vy * 0.7 + ((y - f.y.value) / dt) * 0.3;
    drag.t = e.timeStamp;
    f.x.value = x;
    f.y.value = y;
    paint(f);
  }

  function release(a: Appt) {
    if (!drag || drag.id !== a.id) return;
    const { vx, vy } = drag;
    drag = null;
    return drop(a, vx, vy);
  }

  /** Lands a card on the nearest free slot (or sends it home on a clash) and lets the spring carry it there. */
  async function drop(a: Appt, vx: number, vy: number) {
    const f = flying.get(a.id)!;
    f.dragging = false;
    delete f.el.dataset.drag;

    const lr = lane.getBoundingClientRect();
    const gap = window.innerWidth >= 640 ? 12 : 8;
    const colW = (lr.width - gap) / 2;
    const rowH = lr.height / DAY;
    const left0 = a.col * (colW + gap);
    const top0 = (a.start - HOUR) * rowH;
    const cx = left0 + colW / 2 + f.x.value;
    const col = cx > lr.width / 2 ? 1 : 0;
    const start = clamp(Math.round((top0 + f.y.value) / rowH / STEP) * STEP, 0, DAY - a.len) + HOUR;
    const clash = appts.some((o) => o.id !== a.id && o.col === col && start < o.start + o.len && o.start < start + a.len);

    // new slot (or back to the old one on a clash); keep the card visually where it was and let the spring carry it home
    const nx = clash ? left0 : col * (colW + gap);
    const ny = clash ? top0 : (start - HOUR) * rowH;
    const ox = left0 + f.x.value - nx;
    const oy = top0 + f.y.value - ny;
    if (!clash) {
      a.col = col;
      a.start = start;
      status = `${a.who} → ${hhmm(start)} · ${doctors[col].name}`;
    } else {
      status = 'Ese horario ya está ocupado · sin empalmes';
    }
    await tick();
    f.x.value = ox;
    f.y.value = oy;
    f.x.velocity = vx;
    f.y.velocity = vy;
    paint(f);
  }

  // Autoplay: a "teammate" cursor moves a card on its own, so the demo shows itself without anyone having to discover it.
  let ghostEl: HTMLDivElement;
  let touched = false;
  let onScreen = false;
  const ease = (u: number) => (u < 0.5 ? 4 * u * u * u : 1 - (-2 * u + 2) ** 3 / 2);
  const tween = (ms: number, fn: (e: number) => void) =>
    new Promise<void>((done) => {
      const t0 = performance.now();
      const stop = onFrame(() => {
        const u = clamp((performance.now() - t0) / ms, 0, 1);
        fn(ease(u));
        if (u >= 1) {
          stop();
          done();
        }
      });
    });
  const wait = (ms: number) => new Promise<void>((r) => setTimeout(r, ms));
  const place = (x: number, y: number) => (ghostEl.style.transform = `translate3d(${x}px, ${y}px, 0)`);

  async function autoMove(id: number, toCol: number, toStart: number, from: { x: number; y: number }) {
    const a = appts.find((o) => o.id === id)!;
    const el = lane.querySelectorAll<HTMLElement>('.appt')[appts.indexOf(a)];
    const lr = lane.getBoundingClientRect();
    const gap = window.innerWidth >= 640 ? 12 : 8;
    const colW = (lr.width - gap) / 2;
    const rowH = lr.height / DAY;
    const left0 = a.col * (colW + gap);
    const top0 = (a.start - HOUR) * rowH;
    const g0 = { x: left0 + colW * 0.35, y: top0 + rowH * 0.3 };
    const dx = toCol * (colW + gap) - left0;
    const dy = (toStart - HOUR) * rowH - top0;

    ghostEl.dataset.on = '1';
    await tween(900, (e) => place(from.x + (g0.x - from.x) * e, from.y + (g0.y - from.y) * e));
    if (touched) return g0;
    ghostEl.dataset.down = '1'; // press
    const f: Fly = flying.get(id) ?? { el, x: new Spring(0, SPRING), y: new Spring(0, SPRING), dragging: true };
    f.dragging = true;
    flying.set(id, f);
    el.classList.remove('slot-in');
    el.dataset.drag = '1';
    await tween(1200, (e) => {
      f.x.value = dx * e;
      f.y.value = dy * e;
      paint(f);
      place(g0.x + dx * e, g0.y + dy * e);
    });
    delete ghostEl.dataset.down; // release
    await drop(a, 0, 0);
    return { x: g0.x + dx, y: g0.y + dy };
  }

  async function story_loop() {
    let pos = { x: 40, y: -30 };
    const script: [number, number, number][] = [[1, 1, 9], [1, 0, 9.25]]; // Diego → Dr. Luis 09:00, then back home
    await wait(2600);
    for (let i = 0; !touched; i = (i + 1) % script.length) {
      while (!onScreen && !touched) await wait(400);
      if (touched) break;
      pos = await autoMove(...script[i], pos);
      await wait(2200);
    }
    ghostEl.dataset.on = '0';
  }



  onMount(() => {
    const reduce = reducedMotion();
    const stops = [
      track(section, ['start start', 'end start'], (p) => {
        ctx.p = p;
        text.style.transform = `translate3d(0, ${reduce ? 0 : interp(p, [0, 1], [0, -160])}px, 0)`;
        text.style.opacity = String(interp(p, [0, 0.6], [1, 0]));
        ecg.style.transform = `translate3d(0, ${reduce ? 0 : interp(p, [0, 1], [0, 240])}px, 0)`;
      }),
      track(
        stage,
        ['start end', 'start 65%'],
        (p) => {
          const flat = reduce || window.innerWidth < 640;
          const rotate = flat ? 0 : interp(p, [0, 1], [12, 0]);
          const scale = flat ? 1 : interp(p, [0, 1], [0.97, 1]);
          window3d.style.transform = `rotateX(${rotate}deg) scale(${scale})`;
        },
        { stiffness: 90, damping: 24, mass: 0.4 }
      )
    ];

    // pointer position, normalised to [-0.5, 0.5], eased
    const sx = new Spring(0, { stiffness: 60, damping: 18 });
    const sy = new Spring(0, { stiffness: 60, damping: 18 });
    let tx = 0;
    let ty = 0;
    stops.push(
      onFrame((dt) => {
        ctx.mx = sx.step(tx, dt);
        ctx.my = sy.step(ty, dt);
        if (tiltEl && !reduce && window.innerWidth >= 1024) tiltEl.style.transform = `rotateY(${ctx.mx * 9}deg) rotateX(${-ctx.my * 6}deg)`;
      })
    );
    const mouse = (e: PointerEvent) => {
      if (e.pointerType !== 'mouse') return;
      tx = e.clientX / window.innerWidth - 0.5;
      ty = e.clientY / window.innerHeight - 0.5;
    };
    section.addEventListener('pointermove', mouse);
    stops.push(
      onFrame((dt) => {
        for (const [id, f] of flying) {
          if (f.dragging) continue;
          f.x.step(0, dt);
          f.y.step(0, dt);
          if (f.x.settled && f.y.settled && Math.abs(f.x.value) < 0.3 && Math.abs(f.y.value) < 0.3) {
            f.el.style.transform = '';
            flying.delete(id);
          } else paint(f);
        }
      })
    );
    // ghost-cursor demo, only while the product window is on screen
    if (!reduce) {
      const io = new IntersectionObserver(([en]) => (onScreen = en.isIntersecting), { threshold: 0.35 });
      io.observe(stage);
      stops.push(() => io.disconnect());
      story_loop();
      // revenue card: count up once the hero has settled
      const t = setTimeout(() => {
        const t0 = performance.now();
        const stop = onFrame(() => {
          const u = clamp((performance.now() - t0) / 1600, 0, 1);
          revenue = Math.round(18450 * (1 - (1 - u) ** 4));
          if (u >= 1) stop();
        });
      }, 2400);
      stops.push(() => clearTimeout(t));
    } else revenue = 18450;
    // a short story in the notification chip: confirmation, reminder, prescription
    if (!reduce) {
      const t = setInterval(() => {
        if (!document.hidden) story = (story + 1) % stories.length;
      }, 3600);
      stops.push(() => clearInterval(t));
    }
    return () => {
      stops.forEach((s) => s());
      section.removeEventListener('pointermove', mouse);
    };
  });

</script>

<section id="inicio" bind:this={section} class="relative isolate overflow-hidden pb-16 pt-28 sm:pb-20 sm:pt-32">

  <div bind:this={text} class="mx-auto max-w-6xl px-5 sm:px-8">
    <div class="enter-fade flex items-center justify-between border-b border-ink/15 pb-4 font-mono text-[11px] uppercase tracking-[0.16em] text-ink-soft" style="--d: 0.1s">
      <span class="flex items-center gap-2"><span class="now-pulse h-1.5 w-1.5 rounded-full bg-signal"></span>Software clínico</span>
      <span class="hidden sm:block">Dental · Médico · Nutrición · Veterinaria · y más</span>
    </div>

    <Split
      tag="h1"
      text={'Atiende pacientes,\n*no papeles.*'}
      class="mt-6 font-display text-[clamp(3.25rem,14vw,5rem)] font-normal leading-[0.96] tracking-[-0.025em] text-ink sm:text-[clamp(5rem,12vw,10rem)] sm:leading-[0.92] sm:tracking-[-0.035em]"
      accent="italic text-signal"
      delay={0}
      stagger={0.035}
      duration={0.6}
    />

    <div class="mt-7 grid gap-5 md:grid-cols-[1.1fr_1fr] md:items-start md:gap-10">
      <!-- Este párrafo es el elemento más grande al abrir la página (LCP): sin animación de entrada para que pinte de inmediato -->
      <p class="max-w-[34rem] text-base leading-[1.55] text-ink-soft [text-wrap:pretty] sm:text-lg">
        Agenda, expedientes, recetas, cobros y reportes para <span class="text-ink">clínicas dentales</span>, <span class="text-ink">consultorios médicos</span>, <span class="text-ink">nutriólogos</span>, <span class="text-ink">veterinarias</span> y más. Todo en un solo lugar, con IA donde ayuda.
      </p>
      <div class="enter-up flex flex-wrap items-center gap-3 md:justify-start md:pt-1" style="--d: 0.35s; --t: 0.6s">
        <div class="inline-flex">
          <a href={registerHref} class="{btn} h-14 bg-ink pl-7 pr-2 text-[16px] text-paper hover:bg-signal">
            <ButtonLabel>Solicitar demo gratis</ButtonLabel>
            <span class="ml-2 grid h-10 w-10 place-items-center rounded-full bg-paper text-ink transition-transform duration-500 ease-out-strong group-hover:rotate-[-45deg]">
              <Icon name="arrow" class="h-4 w-4" />
            </span>
          </a>
        </div>
        <a href="#precios" class="group inline-flex min-h-11 items-center px-3 py-2 text-[16px] font-medium text-ink">
          <span class="bg-[linear-gradient(currentColor,currentColor)] bg-[length:100%_1px] bg-[position:0_100%] bg-no-repeat pb-0.5 transition-[background-size] duration-500 ease-out-strong group-hover:bg-[length:0%_1px] group-hover:bg-[position:100%_100%]">Ver precios</span>
        </a>
      </div>
    </div>
  </div>

  <!-- Product stage -->
  <div bind:this={stage} class="relative mx-auto mt-10 max-w-6xl px-3 sm:mt-12 sm:px-8 [perspective:1600px]">
    <div bind:this={ecg} aria-hidden="true" class="pointer-events-none absolute inset-x-[-20vw] top-[30%] -z-10 h-32">
      <Ecg class="h-full w-full" beats={6} />
    </div>

    <div bind:this={window3d} class="relative z-10 will-change-transform [transform-origin:50%_0%]" aria-hidden="true">
      <div bind:this={tiltEl} class="[transform-style:preserve-3d] will-change-transform" style="transition: transform 120ms linear">
      <div class="relative overflow-hidden rounded-[22px] bg-panel shadow-[0_2px_4px_rgb(var(--l-shade)/0.06),0_16px_32px_-16px_rgb(var(--l-shade)/0.18),0_40px_72px_-28px_rgb(var(--l-shade)/0.24),inset_0_1px_0_rgb(var(--l-panel)/0.8)] ring-1 ring-ink/10">
        <div class="flex items-center gap-1.5 border-b border-ink/5 px-4 py-3">
          <span class="h-2.5 w-2.5 rounded-full bg-ink/10"></span>
          <span class="h-2.5 w-2.5 rounded-full bg-ink/10"></span>
          <span class="h-2.5 w-2.5 rounded-full bg-ink/10"></span>
          <span class="mx-auto rounded-md bg-paper px-3 py-1 font-mono text-[10px] text-ink-faint sm:text-[11px]">caresia.app/agenda</span>
        </div>
        <div class="flex">
          <aside class="hidden w-48 flex-none border-r border-ink/5 p-4 md:block">
            <div class="flex items-center gap-2"><Mark class="h-6 w-6" /><span class="font-display text-xl">Caresia</span></div>
            <ul class="mt-6 space-y-0.5 text-[13px]">
              {#each ['Agenda', 'Pacientes', 'Historiales', 'Usuarios'] as s, i}
                <li class="rounded-lg px-3 py-2 {i === 0 ? 'bg-paper font-medium text-ink' : 'text-ink-soft'}">{s}</li>
              {/each}
            </ul>
            <div class="mt-10 rounded-xl bg-paper p-3">
              <p class="font-mono text-[10px] uppercase tracking-[0.12em] text-ink-faint">Hoy</p>
              <p class="mt-1 font-display text-3xl leading-none">18</p>
              <p class="mt-1 text-[12px] text-ink-soft">citas agendadas</p>
            </div>
          </aside>
          <div class="min-w-0 flex-1 p-3 sm:p-5">
            <div class="flex items-end justify-between gap-3">
              <div>
                <p class="font-mono text-[10px] uppercase tracking-[0.14em] text-ink-faint">Martes</p>
                <p class="font-display text-2xl leading-none sm:text-3xl">14 de octubre</p>
              </div>
              <span class="hidden min-w-0 flex-1 truncate text-center font-mono text-[10px] uppercase tracking-[0.12em] text-ink-faint md:block">
                {#if status}{status}{:else}<span class="inline-flex items-center gap-1.5"><span class="now-pulse h-1.5 w-1.5 rounded-full bg-signal"></span>Prueba: arrastra una cita</span>{/if}
              </span>
              <span class="rounded-full bg-ink px-3 py-1 text-[11px] font-medium text-paper">+ Nueva cita</span>
            </div>
            <div class="mt-4 grid grid-cols-[2.25rem_1fr_1fr] gap-x-2 sm:grid-cols-[3rem_1fr_1fr] sm:gap-x-3">
              <span></span>
              {#each doctors as d}
                <div class="flex min-w-0 items-center gap-2 pb-2">
                  <span class="h-2 w-2 flex-none rounded-full {d.tone === 'dental' ? 'bg-mint' : 'bg-signal'}"></span>
                  <span class="min-w-0">
                    <span class="block truncate text-[12px] font-medium sm:text-[13px]">{d.name}</span>
                    <span class="block truncate text-[10px] text-ink-faint sm:text-[11px]">{d.area}</span>
                  </span>
                </div>
              {/each}
              <div class="relative col-span-3 grid grid-cols-[2.25rem_1fr_1fr] gap-x-2 [--h:44px] sm:grid-cols-[3rem_1fr_1fr] sm:gap-x-3 sm:[--h:58px]" style="height: calc(var(--h) * 5)">
                <!-- hour lines -->
                {#each hours as h, i}
                  <div class="pointer-events-none absolute inset-x-0 border-t border-dashed border-ink/[0.08]" style="top: calc(var(--h) * {i})">
                    <span class="absolute -top-2 left-0 bg-panel pr-1 font-mono text-[9px] tabular-nums text-ink-faint sm:text-[10px]">{String(h).padStart(2, '0')}:00</span>
                  </div>
                {/each}
                <span></span>
                <div bind:this={lane} class="relative col-span-2 [--g:0.5rem] sm:[--g:0.75rem]">
                  <div bind:this={ghostEl} aria-hidden="true" data-on="0" class="ghost pointer-events-none absolute left-0 top-0 z-40">
                    <svg viewBox="0 0 24 24" class="h-7 w-7 drop-shadow-md" fill="rgb(var(--l-ink))" stroke="rgb(var(--l-panel))" stroke-width="1.5" stroke-linejoin="round"><path d="M5 3l14 7-6 2-2 6z" /></svg>
                    <span class="ml-4 -mt-1 block whitespace-nowrap rounded-full bg-signal px-3 py-1 text-[11px] font-semibold text-white shadow-lg">Recepción</span>
                  </div>
                  {#each appts as a (a.id)}
                    <div
                      role="presentation"
                      class="appt slot-in absolute overflow-hidden rounded-lg border-l-[3px] px-2 py-1 sm:px-2.5 sm:py-1.5 {doctors[a.col].tone === 'dental' ? 'border-mint bg-mint-soft' : 'border-signal bg-signal-soft'}"
                      style="animation-delay: {0.45 + (a.id % 3 * 2 + Math.floor(a.id / 3)) * 0.05}s"
                      style:top="calc(var(--h) * {a.start - HOUR} + 2px)"
                      style:height="calc(var(--h) * {a.len} - 4px)"
                      style:left="calc({a.col} * (50% + var(--g) / 2))"
                      style:width="calc(50% - var(--g) / 2)"
                      onpointerdown={(e) => press(e, a)}
                      onpointermove={(e) => move(e, a)}
                      onpointerup={() => release(a)}
                      onpointercancel={() => release(a)}
                      onanimationend={(e) => e.currentTarget.classList.remove('slot-in')}
                    >
                      <p class="truncate text-[10px] font-semibold sm:text-[12px]">{a.who}</p>
                      <p class="truncate text-[9px] text-ink-soft sm:text-[11px]">{a.what}</p>
                    </div>
                  {/each}
                </div>
                <!-- current time -->
                <div class="pointer-events-none absolute inset-x-0 flex items-center" style="top: calc(var(--h) * {NOW - HOUR})">
                  <span class="now-pulse h-2 w-2 -translate-x-1/2 rounded-full bg-signal"></span>
                  <span class="h-px flex-1 bg-signal"></span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
      </div>
    </div>

    <Float depth={1.3} {ctx} delay={1.5} class="-left-2 top-[-1.5rem] hidden md:block lg:-left-14">
      <div class="{chip} w-64 p-4">
        <div class="flex items-center gap-3">
          <span class="grid h-10 w-10 place-items-center rounded-full bg-mint-soft font-display text-lg text-mint">ML</span>
          <span>
            <span class="block text-sm font-semibold">Mariana López</span>
            <span class="block font-mono text-[10px] uppercase tracking-[0.12em] text-ink-faint">Expediente · 34 años</span>
          </span>
        </div>
        <div class="mt-3 flex items-center gap-2 rounded-xl bg-amber-50 px-3 py-2 text-[12px] font-medium text-amber-700 ring-1 ring-inset dark:bg-amber-400/10 dark:text-amber-300 dark:ring-amber-400/25 ring-amber-600/15">
          <Icon name="warn" class="h-3.5 w-3.5" strokeWidth={2.2} />
          Alergia a penicilina
        </div>
      </div>
    </Float>

    <Float depth={2} {ctx} delay={1.7} class="-bottom-4 -right-1 top-auto sm:bottom-auto sm:right-2 sm:top-[3%] lg:-right-12">
      <div class="{chip} flex items-center gap-3 py-3 pl-3 pr-5">
        {#key story}
          <span class="story-in flex items-center gap-3">
            <span class="grid h-9 w-9 place-items-center rounded-full {stories[story].tone} text-white"><Icon name="check" class="h-4 w-4" strokeWidth={2.4} /></span>
            <span>
              <span class="block text-[13px] font-semibold">{stories[story].title}</span>
              <span class="block text-[12px] text-ink-soft">{stories[story].sub}</span>
            </span>
          </span>
        {/key}
      </div>
    </Float>

    <Float depth={0.9} {ctx} delay={1.9} class="-left-4 bottom-[8%] hidden sm:block lg:-left-20">
      <div class="{chip} grid grid-cols-2 gap-4 px-5 py-4">
        <span>
          <span class="block font-mono text-[10px] uppercase tracking-[0.12em] text-ink-faint">Peso</span>
          <span class="mt-1 block font-display text-3xl leading-none">78<span class="text-base text-ink-soft"> kg</span></span>
        </span>
        <span>
          <span class="block font-mono text-[10px] uppercase tracking-[0.12em] text-ink-faint">Estatura</span>
          <span class="mt-1 block font-display text-3xl leading-none">172<span class="text-base text-ink-soft"> cm</span></span>
        </span>
      </div>
    </Float>

    <Float depth={1.6} {ctx} delay={2.3} class="right-0 top-[40%] hidden xl:-right-14 xl:block">
      <div class="{chip} w-56 p-4">
        <p class="font-mono text-[10px] uppercase tracking-[0.12em] text-ink-faint">Ingresos de hoy</p>
        <p class="mt-1 font-display text-3xl leading-none tabular-nums">${revenue.toLocaleString('es-MX')}</p>
        <svg viewBox="0 0 120 36" class="mt-3 h-9 w-full" fill="none" aria-hidden="true">
          <path class="spark" pathLength="1" d="M0 30 C12 28 16 22 28 23 S46 30 58 20 S76 8 88 12 S108 6 120 3" stroke="rgb(var(--l-signal))" stroke-width="2" stroke-linecap="round" />
        </svg>
      </div>
    </Float>

    <Float depth={2.6} {ctx} delay={2.1} class="-bottom-6 right-6 hidden md:block lg:-right-6">
      <div class="w-52 rotate-[5deg] bg-[#F5E7A0] p-4 pb-6 font-hand text-[24px] leading-[1.05] text-ink shadow-[0_20px_40px_-14px_rgba(11,37,64,0.4)]">¡Adiós a la agenda de papel!</div>
    </Float>
  </div>
</section>

<style>
  .ghost {
    opacity: 0;
    transition: opacity 400ms ease-out;
  }
  .ghost:global([data-on='1']) {
    opacity: 1;
  }
  .ghost:global([data-down='1']) svg {
    transform: scale(0.85);
  }
  .ghost svg {
    transition: transform 120ms ease-out;
  }
  .spark {
    stroke-dasharray: 1;
  }
  @media (prefers-reduced-motion: no-preference) {
    .spark {
      animation: spark-draw 1.6s cubic-bezier(0.23, 1, 0.32, 1) 2.4s both;
    }
  }
  @keyframes spark-draw {
    from {
      stroke-dashoffset: 1;
    }
    to {
      stroke-dashoffset: 0;
    }
  }
  .appt {
    cursor: grab;
    user-select: none;
    touch-action: pan-y;
    transition: box-shadow 160ms ease-out;
  }
  .appt:global([data-drag='1']) {
    cursor: grabbing;
    z-index: 30;
    box-shadow:
      0 1px 2px rgb(var(--l-shade) / 0.12),
      0 18px 28px -10px rgb(var(--l-shade) / 0.4);
  }
  @media (prefers-reduced-motion: no-preference) {
    .story-in {
      animation: story-in 450ms cubic-bezier(0.23, 1, 0.32, 1) both;
    }
  }
  @keyframes story-in {
    from {
      opacity: 0;
      transform: translateY(8px);
      filter: blur(4px);
    }
  }
</style>
