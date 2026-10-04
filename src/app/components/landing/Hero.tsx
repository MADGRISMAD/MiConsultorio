"use client"
import React, { useRef } from 'react';
import { MotionValue, motion, useMotionValue, useReducedMotion, useScroll, useSpring, useTransform } from 'framer-motion';
import { ButtonLabel, Ecg, Icon, Magnetic, Mark, Split, arrow, btn, check, ease } from './motion';
import { demoHref } from './data';

// Day view shown in the product window: two doctors, two specialties, one clinic
const HOUR = 8; // first hour on the grid
const doctors = [
  {
    name: "Dra. Ana Ruiz",
    area: "Odontología",
    tone: "dental",
    slots: [
      { start: 8, len: 0.75, who: "Mariana López", what: "Limpieza dental" },
      { start: 9.25, len: 1, who: "Diego Hernández", what: "Ajuste de ortodoncia" },
      { start: 11, len: 1.25, who: "Andrés Torres", what: "Endodoncia · 2.ª sesión" },
    ],
  },
  {
    name: "Dr. Luis Paredes",
    area: "Medicina general",
    tone: "medica",
    slots: [
      { start: 8.5, len: 0.5, who: "Jorge Medina", what: "Consulta general" },
      { start: 10, len: 0.75, who: "Sofía Ramírez", what: "Control de presión" },
      { start: 11.5, len: 0.5, who: "Lucía Gómez", what: "Certificado médico" },
    ],
  },
];
const hours = [8, 9, 10, 11, 12];
const NOW = 10.6;

function AppWindow() {
  const reduce = useReducedMotion();
  return (
    <div className="overflow-hidden rounded-[22px] bg-white shadow-[0_2px_4px_rgba(20,33,29,0.04),0_40px_80px_-24px_rgba(20,33,29,0.35)] ring-1 ring-ink/10">
      <div className="flex items-center gap-1.5 border-b border-ink/5 px-4 py-3">
        <span className="h-2.5 w-2.5 rounded-full bg-ink/10" />
        <span className="h-2.5 w-2.5 rounded-full bg-ink/10" />
        <span className="h-2.5 w-2.5 rounded-full bg-ink/10" />
        <span className="mx-auto rounded-md bg-paper px-3 py-1 font-mono text-[10px] text-ink-faint sm:text-[11px]">caresia.app/agenda</span>
      </div>
      <div className="flex">
        <aside className="hidden w-48 flex-none border-r border-ink/5 p-4 md:block">
          <div className="flex items-center gap-2"><Mark className="h-6 w-6" /><span className="font-display text-xl">Caresia</span></div>
          <ul className="mt-6 space-y-0.5 text-[13px]">
            {["Agenda", "Pacientes", "Historiales", "Usuarios"].map((s, i) => (
              <li key={s} className={`rounded-lg px-3 py-2 ${i === 0 ? "bg-paper font-medium text-ink" : "text-ink-soft"}`}>{s}</li>
            ))}
          </ul>
          <div className="mt-10 rounded-xl bg-paper p-3">
            <p className="font-mono text-[10px] uppercase tracking-[0.12em] text-ink-faint">Hoy</p>
            <p className="mt-1 font-display text-3xl leading-none">18</p>
            <p className="mt-1 text-[12px] text-ink-soft">citas agendadas</p>
          </div>
        </aside>
        <div className="min-w-0 flex-1 p-3 sm:p-5">
          <div className="flex items-end justify-between gap-3">
            <div>
              <p className="font-mono text-[10px] uppercase tracking-[0.14em] text-ink-faint">Martes</p>
              <p className="font-display text-2xl leading-none sm:text-3xl">14 de octubre</p>
            </div>
            <span className="rounded-full bg-ink px-3 py-1 text-[11px] font-medium text-paper">+ Nueva cita</span>
          </div>
          <div className="mt-4 grid grid-cols-[2.25rem_1fr_1fr] gap-x-2 sm:grid-cols-[3rem_1fr_1fr] sm:gap-x-3">
            <span />
            {doctors.map((d) => (
              <div key={d.name} className="flex min-w-0 items-center gap-2 pb-2">
                <span className={`h-2 w-2 flex-none rounded-full ${d.tone === "dental" ? "bg-mint" : "bg-signal"}`} />
                <span className="min-w-0">
                  <span className="block truncate text-[12px] font-medium sm:text-[13px]">{d.name}</span>
                  <span className="block truncate text-[10px] text-ink-faint sm:text-[11px]">{d.area}</span>
                </span>
              </div>
            ))}
            <div className="relative col-span-3 grid grid-cols-[2.25rem_1fr_1fr] gap-x-2 [--h:44px] sm:grid-cols-[3rem_1fr_1fr] sm:gap-x-3 sm:[--h:58px]" style={{ height: "calc(var(--h) * 5)" }}>
              {/* hour lines */}
              {hours.map((h, i) => (
                <div key={h} className="pointer-events-none absolute inset-x-0 border-t border-dashed border-ink/[0.08]" style={{ top: `calc(var(--h) * ${i})` }}>
                  <span className="absolute -top-2 left-0 bg-white pr-1 font-mono text-[9px] tabular-nums text-ink-faint sm:text-[10px]">{String(h).padStart(2, "0")}:00</span>
                </div>
              ))}
              <span />
              {doctors.map((d, di) => (
                <div key={d.name} className="relative">
                  {d.slots.map((s, si) => (
                    <motion.div
                      key={s.who}
                      className={`absolute inset-x-0 overflow-hidden rounded-lg border-l-[3px] px-2 py-1 sm:px-2.5 sm:py-1.5 ${d.tone === "dental" ? "border-mint bg-mint-soft" : "border-signal bg-signal-soft"}`}
                      style={{ top: `calc(var(--h) * ${s.start - HOUR} + 2px)`, height: `calc(var(--h) * ${s.len} - 4px)`, transformOrigin: "top" }}
                      initial={reduce ? { opacity: 0 } : { opacity: 0, scaleY: 0.4 }}
                      animate={{ opacity: 1, scaleY: 1 }}
                      transition={{ duration: 0.7, delay: 1.1 + (si * 2 + di) * 0.09, ease }}
                    >
                      <p className="truncate text-[10px] font-semibold sm:text-[12px]">{s.who}</p>
                      <p className="truncate text-[9px] text-ink-soft sm:text-[11px]">{s.what}</p>
                    </motion.div>
                  ))}
                </div>
              ))}
              {/* current time */}
              <div className="pointer-events-none absolute inset-x-0 flex items-center" style={{ top: `calc(var(--h) * ${NOW - HOUR})` }}>
                <span className="now-pulse h-2 w-2 -translate-x-1/2 rounded-full bg-signal" />
                <span className="h-px flex-1 bg-signal" />
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function Float({
  depth,
  progress,
  mx,
  my,
  delay,
  className,
  children,
}: {
  depth: number;
  progress: MotionValue<number>;
  mx: MotionValue<number>;
  my: MotionValue<number>;
  delay: number;
  className: string;
  children: React.ReactNode;
}) {
  const reduce = useReducedMotion();
  const k = reduce ? 0 : depth;
  const y = useTransform([progress, my], ([p, m]: number[]) => -p * k * 220 + m * k * 36);
  const x = useTransform(mx, (m) => m * k * 36);
  return (
    <motion.div aria-hidden="true" className={`absolute z-20 ${className}`} style={{ x, y }}>
      <motion.div
        initial={{ opacity: 0, y: 40, scale: 0.92 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        transition={{ duration: 1.1, delay, ease }}
      >
        {children}
      </motion.div>
    </motion.div>
  );
}

const chip = "rounded-2xl bg-white/90 shadow-[0_24px_48px_-16px_rgba(20,33,29,0.3)] ring-1 ring-ink/10 backdrop-blur";

export default function Hero() {
  const reduce = useReducedMotion();
  const section = useRef<HTMLElement>(null);
  const stage = useRef<HTMLDivElement>(null);

  const { scrollYProgress: heroP } = useScroll({ target: section, offset: ["start start", "end start"] });
  const { scrollYProgress: stageP } = useScroll({ target: stage, offset: ["start end", "center 55%"] });

  const smooth = useSpring(stageP, { stiffness: 90, damping: 24, mass: 0.4 });
  const rotateX = useTransform(smooth, [0, 1], reduce ? [0, 0] : [28, 0]);
  const scale = useTransform(smooth, [0, 1], reduce ? [1, 1] : [0.86, 1]);
  const textY = useTransform(heroP, [0, 1], reduce ? [0, 0] : [0, -160]);
  const textO = useTransform(heroP, [0, 0.6], [1, 0]);
  const ecgY = useTransform(heroP, [0, 1], reduce ? [0, 0] : [0, 240]);

  // pointer position, normalised to [-0.5, 0.5]
  const mxRaw = useMotionValue(0);
  const myRaw = useMotionValue(0);
  const mx = useSpring(mxRaw, { stiffness: 60, damping: 18 });
  const my = useSpring(myRaw, { stiffness: 60, damping: 18 });

  return (
    <section
      id="inicio"
      ref={section}
      className="relative overflow-hidden pb-20 pt-32 sm:pt-40"
      onPointerMove={(e) => {
        if (e.pointerType !== "mouse") return;
        mxRaw.set(e.clientX / window.innerWidth - 0.5);
        myRaw.set(e.clientY / window.innerHeight - 0.5);
      }}
    >
      {/* soft light behind the headline */}
      <div aria-hidden="true" className="pointer-events-none absolute left-1/2 top-0 -z-10 h-[60rem] w-[90rem] -translate-x-1/2 bg-[radial-gradient(closest-side,rgba(232,85,61,0.10),transparent)]" />

      <motion.div style={{ y: textY, opacity: textO }} className="mx-auto max-w-6xl px-5 sm:px-8">
        <motion.div
          className="flex items-center justify-between border-b border-ink/15 pb-4 font-mono text-[11px] uppercase tracking-[0.16em] text-ink-soft"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ duration: 1, delay: 0.1 }}
        >
          <span className="flex items-center gap-2"><span className="now-pulse h-1.5 w-1.5 rounded-full bg-signal" />Software clínico</span>
          <span className="hidden sm:block">Odontología · Medicina general</span>
        </motion.div>

        <Split
          as="h1"
          text={"Menos papeleo,\n*más consulta.*"}
          className="mt-8 font-display text-[clamp(3.4rem,15vw,11rem)] font-normal leading-[0.88] tracking-[-0.035em] text-ink"
          accent="italic text-signal"
          delay={0.15}
          stagger={0.09}
        />

        <div className="mt-10 grid gap-8 md:grid-cols-[1.1fr_1fr] md:items-end">
          <motion.p
            className="max-w-md text-lg leading-relaxed text-ink-soft [text-wrap:pretty] sm:text-xl"
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 1, delay: 0.7, ease }}
          >
            Agenda, expedientes e historial clínico para <span className="text-ink">clínicas dentales</span> y <span className="text-ink">consultorios médicos</span>. Todo en un solo lugar.
          </motion.p>
          <motion.div
            className="flex flex-wrap items-center gap-3 md:justify-end"
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 1, delay: 0.85, ease }}
          >
            <Magnetic>
              <a href={demoHref} className={`${btn} h-14 bg-ink pl-7 pr-2 text-[16px] text-paper hover:bg-signal`}>
                <ButtonLabel>Solicitar demo gratis</ButtonLabel>
                <span className="ml-2 grid h-10 w-10 place-items-center rounded-full bg-paper text-ink transition-transform duration-500 ease-out-strong group-hover:rotate-[-45deg]">
                  <Icon className="h-4 w-4">{arrow}</Icon>
                </span>
              </a>
            </Magnetic>
            <a href="#precios" className="group px-3 py-2 text-[16px] font-medium text-ink">
              <span className="bg-[linear-gradient(currentColor,currentColor)] bg-[length:100%_1px] bg-[position:0_100%] bg-no-repeat pb-0.5 transition-[background-size] duration-500 ease-out-strong group-hover:bg-[length:0%_1px] group-hover:bg-[position:100%_100%]">Ver precios</span>
            </a>
          </motion.div>
        </div>
      </motion.div>

      {/* Product stage */}
      <div ref={stage} className="relative mx-auto mt-20 max-w-6xl px-3 sm:mt-24 sm:px-8 [perspective:1600px]">
        <motion.div aria-hidden="true" style={{ y: ecgY }} className="pointer-events-none absolute inset-x-[-20vw] top-[30%] -z-10 h-32">
          <Ecg className="h-full w-full" beats={6} />
        </motion.div>

        <motion.div style={{ rotateX, scale, transformOrigin: "50% 0%" }} className="relative z-10 will-change-transform" aria-hidden="true">
          <AppWindow />
        </motion.div>

        <Float depth={1.3} progress={heroP} mx={mx} my={my} delay={1.5} className="-left-2 top-[-3rem] hidden md:block lg:-left-14">
          <div className={`${chip} w-64 p-4`}>
            <div className="flex items-center gap-3">
              <span className="grid h-10 w-10 place-items-center rounded-full bg-mint-soft font-display text-lg text-mint">ML</span>
              <span>
                <span className="block text-sm font-semibold">Mariana López</span>
                <span className="block font-mono text-[10px] uppercase tracking-[0.12em] text-ink-faint">Expediente · 34 años</span>
              </span>
            </div>
            <div className="mt-3 flex items-center gap-2 rounded-xl bg-signal-soft px-3 py-2 text-[12px] font-medium text-signal">
              <Icon className="h-3.5 w-3.5" strokeWidth={2.2}><path d="M12 9v4M12 17h.01M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z" /></Icon>
              Alergia a penicilina
            </div>
          </div>
        </Float>

        <Float depth={2} progress={heroP} mx={mx} my={my} delay={1.7} className="-right-1 top-[18%] sm:right-2 lg:-right-12">
          <div className={`${chip} flex items-center gap-3 py-3 pl-3 pr-5`}>
            <span className="grid h-9 w-9 place-items-center rounded-full bg-mint text-white"><Icon className="h-4 w-4" strokeWidth={2.4}>{check}</Icon></span>
            <span>
              <span className="block text-[13px] font-semibold">Cita confirmada</span>
              <span className="block text-[12px] text-ink-soft">Jorge Medina · 08:30</span>
            </span>
          </div>
        </Float>

        <Float depth={0.9} progress={heroP} mx={mx} my={my} delay={1.9} className="-left-4 bottom-[8%] hidden sm:block lg:-left-20">
          <div className={`${chip} grid grid-cols-2 gap-4 px-5 py-4`}>
            <span>
              <span className="block font-mono text-[10px] uppercase tracking-[0.12em] text-ink-faint">Peso</span>
              <span className="mt-1 block font-display text-3xl leading-none">78<span className="text-base text-ink-soft"> kg</span></span>
            </span>
            <span>
              <span className="block font-mono text-[10px] uppercase tracking-[0.12em] text-ink-faint">Estatura</span>
              <span className="mt-1 block font-display text-3xl leading-none">172<span className="text-base text-ink-soft"> cm</span></span>
            </span>
          </div>
        </Float>

        <Float depth={2.6} progress={heroP} mx={mx} my={my} delay={2.1} className="-bottom-10 right-6 hidden md:block lg:-right-6">
          <div className="w-52 rotate-[5deg] bg-[#F5E7A0] p-4 pb-6 font-hand text-[24px] leading-[1.05] text-ink shadow-[0_20px_40px_-14px_rgba(20,33,29,0.4)]">
            ¡Adiós a la agenda de papel!
          </div>
        </Float>
      </div>
    </section>
  );
}
