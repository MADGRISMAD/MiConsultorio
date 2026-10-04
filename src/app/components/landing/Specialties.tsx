"use client"
import React, { useRef } from 'react';
import {
  motion,
  useAnimationFrame,
  useMotionValue,
  useReducedMotion,
  useScroll,
  useSpring,
  useTransform,
  useVelocity,
} from 'framer-motion';
import { Icon, Split, check, ease } from './motion';
import { specialties } from './data';

const toothPath = "M100 40c-14-14-34-19-50-13-26 9-34 38-26 66 6 21 14 33 17 56 3 21 6 45 20 45 17 0 15-52 39-52s22 52 39 52c14 0 17-24 20-45 3-23 11-35 17-56 8-28 0-57-26-66-16-6-36-1-50 13z";
const stethPaths = [
  "M50 20v60a45 45 0 0 0 90 0V20",
  "M95 125v20a50 50 0 0 0 100 0v-15",
  "M195 110a20 20 0 1 0 0.1 0",
  "M38 20h24M128 20h24",
];

function Drawing({ paths, viewBox, className }: { paths: string[]; viewBox: string; className: string }) {
  const ref = useRef<HTMLDivElement>(null);
  const { scrollYProgress } = useScroll({ target: ref, offset: ["start end", "end start"] });
  const reduce = useReducedMotion();
  const y = useTransform(scrollYProgress, [0, 1], reduce ? [0, 0] : [60, -60]);
  const r = useTransform(scrollYProgress, [0, 1], reduce ? [0, 0] : [-6, 6]);
  return (
    <motion.div ref={ref} aria-hidden="true" className={className} style={{ y, rotate: r }}>
    <svg viewBox={viewBox} fill="none" className="h-full w-full">
      {paths.map((d, i) => (
        <motion.path
          key={d}
          d={d}
          stroke="currentColor"
          strokeWidth={2}
          strokeLinecap="round"
          strokeLinejoin="round"
          initial={{ pathLength: 0 }}
          whileInView={{ pathLength: 1 }}
          viewport={{ once: true, margin: "-20% 0px" }}
          transition={{ duration: 2.2, delay: 0.2 + i * 0.3, ease }}
        />
      ))}
    </svg>
    </motion.div>
  );
}

const panels = [
  {
    key: "A",
    area: "Odontología",
    title: "Clínica *dental*",
    text: "Para el ritmo de un consultorio dental: citas cortas y seguidas, tratamientos de varias sesiones y un equipo entre el sillón y la recepción.",
    points: ["Ortodoncia, endodoncia o implantes con todas sus sesiones en un expediente", "Agenda para limpiezas, revisiones y urgencias del día", "Asistentes con acceso a la agenda, sin ver lo clínico"],
    bg: "bg-mint-soft",
    accent: "text-mint",
    drawing: <Drawing paths={[toothPath]} viewBox="0 0 200 220" className="absolute -right-8 -top-6 h-64 w-64 text-mint/50 sm:h-80 sm:w-80" />,
  },
  {
    key: "B",
    area: "Medicina general",
    title: "Consultorio *médico*",
    text: "Un expediente completo para conocer a tu paciente antes de que entre al consultorio, y su seguimiento consulta tras consulta.",
    points: ["Antecedentes: diabetes, cardiopatías, alergias, cirugías y más", "Peso, estatura y hábitos de salud en cada expediente", "Seguimiento de pacientes crónicos con su historial a la mano"],
    bg: "bg-signal-soft",
    accent: "text-signal",
    drawing: <Drawing paths={stethPaths} viewBox="0 0 230 230" className="absolute -right-6 -top-4 h-64 w-64 text-signal/50 sm:h-80 sm:w-80" />,
  },
];

export function Specialties() {
  return (
    <section id="especialidades" className="mx-auto max-w-6xl px-5 py-28 sm:px-8 lg:py-40">
      <div className="grid gap-6 border-b border-ink/15 pb-10 md:grid-cols-[1.7fr_1fr] md:items-end">
        <Split text={"Dos especialidades.\n*Una sola plataforma.*"} className="font-display text-[clamp(2.75rem,6.5vw,5.25rem)] leading-[0.95] tracking-[-0.03em]" accent="italic text-signal" />
        <p className="max-w-sm text-lg leading-relaxed text-ink-soft md:justify-self-end">Caresia se adapta a la forma en que trabaja tu especialidad, sin configuraciones complicadas.</p>
      </div>
      <div className="mt-10 flex flex-col gap-4 lg:h-[36rem] lg:flex-row">
        {panels.map((p, i) => (
          <motion.article
            key={p.key}
            initial={{ opacity: 0, y: 60 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true, margin: "-10% 0px" }}
            transition={{ duration: 1, delay: i * 0.12, ease }}
            className={`group relative flex min-h-[32rem] flex-col justify-between overflow-hidden rounded-[28px] p-7 transition-[flex-grow] duration-700 ease-out-strong sm:p-10 lg:min-h-0 lg:flex-1 lg:hover:flex-[1.45] ${p.bg}`}
          >
            {p.drawing}
            <div className="relative flex items-center justify-between font-mono text-[11px] uppercase tracking-[0.16em] text-ink-soft">
              <span>{p.key} — {p.area}</span>
            </div>
            <div className="relative">
              <Split as="h3" text={p.title} className="font-display text-[clamp(3rem,6vw,5.5rem)] leading-[0.9] tracking-[-0.03em]" accent={`italic ${p.accent}`} />
              <p className="mt-5 max-w-md text-[17px] leading-relaxed text-ink-soft">{p.text}</p>
              <ul className="mt-6 max-w-md space-y-2.5">
                {p.points.map((t) => (
                  <li key={t} className="flex gap-3 text-[15px] leading-snug">
                    <span className="mt-0.5 grid h-5 w-5 flex-none place-items-center rounded-full bg-ink text-paper"><Icon className="h-3 w-3" strokeWidth={2.6}>{check}</Icon></span>
                    {t}
                  </li>
                ))}
              </ul>
            </div>
          </motion.article>
        ))}
      </div>
    </section>
  );
}

const wrap = (min: number, max: number, v: number) => {
  const r = max - min;
  return ((((v - min) % r) + r) % r) + min;
};

function Row({ base, items, outline }: { base: number; items: string[]; outline?: boolean }) {
  const reduce = useReducedMotion();
  const x = useMotionValue(0);
  const { scrollY } = useScroll();
  const v = useSpring(useVelocity(scrollY), { damping: 50, stiffness: 400 });
  const boost = useTransform(v, [-1500, 0, 1500], [-4, 0, 4], { clamp: false });
  const dir = useRef(1);
  const pos = useTransform(x, (n) => `${wrap(-50, 0, n)}%`);

  useAnimationFrame((_, delta) => {
    if (reduce) return;
    const b = boost.get();
    if (b < 0) dir.current = -1;
    else if (b > 0) dir.current = 1;
    x.set(x.get() + dir.current * base * (delta / 1000) * (1 + Math.abs(b)));
  });

  const content = items.map((s, i) => (
    <span key={i} className="flex items-center">
      <span className={`px-6 sm:px-10 ${i % 2 ? "italic" : ""} ${outline ? "text-transparent [-webkit-text-stroke:1.2px_#14211D]" : ""}`}>{s}</span>
      <svg viewBox="0 0 24 24" className="h-6 w-6 flex-none text-signal sm:h-9 sm:w-9" aria-hidden="true"><path d="M12 3v18M3 12h18" stroke="currentColor" strokeWidth="3" strokeLinecap="round" /></svg>
    </span>
  ));
  return (
    <div className="overflow-hidden whitespace-nowrap">
      <motion.div className="flex w-max font-display text-[clamp(3rem,9vw,8rem)] leading-[1.05] tracking-[-0.02em]" style={{ x: pos }}>
        <span className="flex">{content}</span>
        <span className="flex" aria-hidden="true">{content}</span>
      </motion.div>
    </div>
  );
}

export function Marquee() {
  return (
    <section aria-label="Especialidades compatibles" className="border-y border-ink/15 py-10 sm:py-14">
      <p className="mb-6 text-center font-mono text-[11px] uppercase tracking-[0.16em] text-ink-soft">Funciona para cualquier consultorio que atiende con cita</p>
      <Row base={-3} items={specialties.slice(0, 5)} />
      <Row base={3} items={specialties.slice(5)} outline />
    </section>
  );
}
