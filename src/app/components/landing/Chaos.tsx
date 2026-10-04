"use client"
import React, { useRef } from 'react';
import { MotionValue, motion, useReducedMotion, useScroll, useSpring, useTransform } from 'framer-motion';

type Kind = "sticky" | "card" | "folder";

// Each scrap starts scattered (offset from its grid slot, in vw/vh) and lands in the grid as a tidy record.
const scraps: { messy: string; kind: Kind; label: string; title: string; meta: string; tone: "dental" | "medica"; from: [number, number, number] }[] = [
  { messy: "Llamar a Sra. López p/ cambiar cita ¿jueves?", kind: "sticky", label: "Cita", title: "Mariana López", meta: "Jue 16 · 09:00", tone: "dental", from: [-4, 8, -14] },
  { messy: "Expediente 0042 ¿¿dónde está??", kind: "folder", label: "Expediente", title: "Diego Hernández", meta: "Ortodoncia", tone: "dental", from: [4, 18, 9] },
  { messy: "Alergia: PENICILINA !!", kind: "card", label: "Antecedente", title: "Alergia a penicilina", meta: "Mariana López", tone: "medica", from: [-8, 10, 12] },
  { messy: "Lunes 10:30 — ¿doble cita?", kind: "sticky", label: "Agenda", title: "Lun 10:30 · Dra. Ruiz", meta: "Sin empalmes", tone: "dental", from: [8, 4, -7] },
  { messy: "Radiografía Diego H. → archivero 3", kind: "card", label: "Historial", title: "Radiografía panorámica", meta: "12 feb · Odontología", tone: "dental", from: [-8, -2, 6] },
  { messy: "Peso 78 / talla 172 — J. Medina", kind: "card", label: "Expediente", title: "Jorge Medina", meta: "78 kg · 172 cm", tone: "medica", from: [12, 14, -11] },
  { messy: "Control de presión S. Ramírez en 3 meses", kind: "folder", label: "Próxima consulta", title: "Sofía Ramírez", meta: "15 ene · 11:15", tone: "medica", from: [-4, -22, 15] },
  { messy: "¿Quién movió la cita de las 12?", kind: "sticky", label: "Permisos", title: "Recepción", meta: "Solo agenda", tone: "medica", from: [6, 8, -16] },
];

const paper: Record<Kind, string> = {
  sticky: "bg-[#F5E7A0] shadow-[0_14px_28px_-12px_rgba(20,33,29,0.45)]",
  card: "bg-[#FBFAF6] shadow-[0_14px_28px_-12px_rgba(20,33,29,0.4)] bg-[repeating-linear-gradient(transparent,transparent_23px,rgba(70,110,170,0.18)_24px)]",
  folder: "bg-[#E4CF9E] shadow-[0_14px_28px_-12px_rgba(20,33,29,0.45)] rounded-tr-xl",
};

function Scrap({ s, p, i }: { s: (typeof scraps)[number]; p: MotionValue<number>; i: number }) {
  const reduce = useReducedMotion();
  // stagger the landing a little so they don't all move as one block
  const a = 0.12 + i * 0.025;
  const b = 0.55 + i * 0.025;
  const k = reduce ? 0 : 1;
  const x = useTransform(p, [a, b], [`${s.from[0] * k}vw`, "0vw"]);
  const y = useTransform(p, [a, b], [`${s.from[1] * k}vh`, "0vh"]);
  const rotate = useTransform(p, [a, b], [s.from[2] * k, 0]);
  const messy = useTransform(p, [b - 0.12, b], [1, 0]);
  const clean = useTransform(p, [b - 0.1, b + 0.02], [0, 1]);

  return (
    <motion.li style={{ x, y, rotate }} className="relative h-[92px] will-change-transform sm:h-[112px]">
      <motion.div style={{ opacity: messy }} className={`absolute inset-0 p-3 sm:p-4 ${paper[s.kind]}`} aria-hidden="true">
        {s.kind === "folder" && <span className="absolute -top-3 left-0 h-3 w-16 rounded-t-md bg-[#E4CF9E]" />}
        <p className="font-hand text-[17px] leading-[1.05] text-ink/85 sm:text-[22px]">{s.messy}</p>
      </motion.div>
      <motion.div style={{ opacity: clean }} className="absolute inset-0 flex flex-col justify-between rounded-2xl bg-white p-3 ring-1 ring-ink/10 sm:p-4">
        <span className="flex items-center gap-2 font-mono text-[9px] uppercase tracking-[0.14em] text-ink-faint sm:text-[10px]">
          <span className={`h-1.5 w-1.5 rounded-full ${s.tone === "dental" ? "bg-mint" : "bg-signal"}`} />
          {s.label}
        </span>
        <span>
          <span className="block truncate text-[13px] font-semibold sm:text-[15px]">{s.title}</span>
          <span className="block truncate text-[11px] text-ink-soft sm:text-[13px]">{s.meta}</span>
        </span>
      </motion.div>
    </motion.li>
  );
}

export default function Chaos() {
  const ref = useRef<HTMLElement>(null);
  const { scrollYProgress } = useScroll({ target: ref, offset: ["start start", "end end"] });
  const p = useSpring(scrollYProgress, { stiffness: 120, damping: 30, mass: 0.3 });

  const bg = useTransform(p, [0.1, 0.65], ["#E6DFCE", "#F4F1EA"]);
  const before = useTransform(p, [0.3, 0.42], [1, 0]);
  const beforeY = useTransform(p, [0.3, 0.42], ["0%", "-30%"]);
  const after = useTransform(p, [0.42, 0.55], [0, 1]);
  const afterY = useTransform(p, [0.42, 0.55], ["30%", "0%"]);
  const loose = useTransform(p, [0.12, 0.75], [scraps.length, 0]);
  const looseText = useTransform(loose, (v) => String(Math.round(v)).padStart(2, "0"));
  const bar = useTransform(p, [0.12, 0.75], [0, 1]);

  return (
    <motion.section ref={ref} style={{ backgroundColor: bg }} className="relative h-[300vh]" aria-labelledby="chaos-title">
      <div className="sticky top-0 flex h-screen flex-col justify-center overflow-hidden px-4 sm:px-8">
        <div className="mx-auto w-full max-w-6xl">
          <div className="flex items-end justify-between gap-6">
            <div className="relative h-[2.2em] flex-1 font-display text-[clamp(2.2rem,6vw,5rem)] leading-[1] tracking-[-0.02em]">
              <h2 id="chaos-title" className="sr-only">Del papel al orden: así cambia tu consultorio con Caresia</h2>
              <motion.p aria-hidden="true" style={{ opacity: before, y: beforeY }} className="absolute inset-0">
                Así se ve un consultorio<br /><span className="italic text-signal">con papel.</span>
              </motion.p>
              <motion.p aria-hidden="true" style={{ opacity: after, y: afterY }} className="absolute inset-0">
                Y así se ve<br /><span className="italic text-mint">con Caresia.</span>
              </motion.p>
            </div>
            <div className="hidden w-48 flex-none text-right sm:block" aria-hidden="true">
              <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-soft">Papeles sueltos</p>
              <motion.p className="font-display text-6xl tabular-nums leading-none">{looseText}</motion.p>
              <div className="mt-3 h-px w-full bg-ink/15">
                <motion.div style={{ scaleX: bar }} className="h-px origin-left bg-ink" />
              </div>
            </div>
          </div>

          <ul className="mt-10 grid grid-cols-2 gap-2.5 sm:mt-14 sm:gap-4 lg:grid-cols-4">
            {scraps.map((s, i) => <Scrap key={s.title + i} s={s} p={p} i={i} />)}
          </ul>

          <p className="mt-8 max-w-md text-[15px] leading-relaxed text-ink-soft sm:mt-12 sm:text-base">
            Notas, carpetas y agendas de papel se convierten en expedientes, citas e historiales que cualquiera de tu equipo encuentra en segundos.
          </p>
        </div>
      </div>
    </motion.section>
  );
}
