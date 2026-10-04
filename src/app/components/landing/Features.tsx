"use client"
import React, { useEffect, useRef, useState } from 'react';
import { AnimatePresence, motion, useInView, useReducedMotion } from 'framer-motion';
import { Icon, Split, check, ease } from './motion';

const label = "font-mono text-[10px] uppercase tracking-[0.14em] text-ink-faint";

function AgendaScreen() {
  const days = ["Lun", "Mar", "Mié", "Jue", "Vie"];
  const blocks = [
    [0, 0, 2, "dental"], [0, 3, 1, "medica"], [1, 1, 2, "medica"], [1, 4, 2, "dental"], [2, 0, 1, "dental"],
    [2, 2, 2, "dental"], [3, 1, 1, "medica"], [3, 3, 2, "medica"], [4, 0, 2, "dental"], [4, 4, 1, "medica"],
  ] as const;
  return (
    <div>
      <div className="flex items-end justify-between">
        <div><p className={label}>Semana 42</p><p className="font-display text-3xl leading-none">Agenda del equipo</p></div>
        <span className="flex gap-3 text-[11px] text-ink-soft">
          <span className="flex items-center gap-1.5"><span className="h-2 w-2 rounded-full bg-mint" />Dental</span>
          <span className="flex items-center gap-1.5"><span className="h-2 w-2 rounded-full bg-signal" />Médica</span>
        </span>
      </div>
      <div className="mt-6 grid grid-cols-5 gap-2">
        {days.map((d, di) => (
          <div key={d}>
            <p className="pb-2 text-center text-[12px] font-medium text-ink-soft">{d}</p>
            <div className="relative h-56 rounded-xl bg-paper sm:h-64">
              {blocks.filter((b) => b[0] === di).map((b, i) => (
                <motion.span
                  key={i}
                  className={`absolute inset-x-1 rounded-md border-l-2 ${b[3] === "dental" ? "border-mint bg-mint-soft" : "border-signal bg-signal-soft"}`}
                  style={{ top: `${b[1] * 16 + 3}%`, height: `${b[2] * 16 - 2}%`, transformOrigin: "top" }}
                  initial={{ scaleY: 0, opacity: 0 }}
                  animate={{ scaleY: 1, opacity: 1 }}
                  transition={{ duration: 0.6, delay: 0.15 + (di + i) * 0.05, ease }}
                />
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function ExpedienteScreen() {
  const rows = [["CURP", "MEJJ700312HDFDRR04"], ["Edad", "54 años"], ["Peso", "78 kg"], ["Estatura", "172 cm"], ["Ocupación", "Contador"], ["Actividad física", "3 veces / semana"]];
  const ant = [["Diabetes", true], ["Alergias", false], ["Cardiopatías", false], ["Cirugías", true], ["Fracturas", false], ["Transfusiones", false]] as const;
  return (
    <div>
      <div className="flex items-center gap-4">
        <span className="grid h-14 w-14 place-items-center rounded-full bg-signal-soft font-display text-2xl text-signal">JM</span>
        <div><p className={label}>Expediente</p><p className="font-display text-3xl leading-none">Jorge Medina</p></div>
      </div>
      <dl className="mt-6 grid grid-cols-2 gap-x-6 gap-y-3 border-t border-ink/10 pt-5">
        {rows.map(([k, v], i) => (
          <motion.div key={k} initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: 0.1 + i * 0.05, duration: 0.5, ease }}>
            <dt className={label}>{k}</dt>
            <dd className="mt-0.5 truncate text-[14px] font-medium tabular-nums">{v}</dd>
          </motion.div>
        ))}
      </dl>
      <p className={`${label} mt-6`}>Antecedentes patológicos</p>
      <ul className="mt-2 flex flex-wrap gap-1.5">
        {ant.map(([a, on], i) => (
          <motion.li
            key={a}
            initial={{ opacity: 0, scale: 0.8 }}
            animate={{ opacity: 1, scale: 1 }}
            transition={{ delay: 0.4 + i * 0.05, duration: 0.4, ease }}
            className={`rounded-full px-3 py-1 text-[12px] font-medium ${on ? "bg-ink text-paper" : "bg-paper text-ink-faint"}`}
          >
            {a}
          </motion.li>
        ))}
      </ul>
    </div>
  );
}

function HistorialScreen() {
  const items = [
    ["12 feb", "Valoración inicial", "Radiografía panorámica", "dental"],
    ["04 mar", "Colocación de brackets", "Arcada superior e inferior", "dental"],
    ["21 mar", "Consulta general", "Dolor de garganta · tratamiento indicado", "medica"],
    ["08 abr", "Primer ajuste", "Sin molestias reportadas", "dental"],
  ] as const;
  return (
    <div>
      <p className={label}>Historial clínico</p>
      <p className="font-display text-3xl leading-none">Diego Hernández</p>
      <ol className="relative mt-6 space-y-3">
        <motion.span
          className="absolute bottom-3 left-[5px] top-3 w-px origin-top bg-ink/15"
          initial={{ scaleY: 0 }}
          animate={{ scaleY: 1 }}
          transition={{ duration: 0.9, ease }}
        />
        {items.map(([d, t, n, tone], i) => (
          <motion.li
            key={t}
            className="relative pl-7"
            initial={{ opacity: 0, x: 16 }}
            animate={{ opacity: 1, x: 0 }}
            transition={{ delay: 0.15 + i * 0.1, duration: 0.6, ease }}
          >
            <span className={`absolute left-0 top-4 h-[11px] w-[11px] rounded-full ring-4 ring-white ${tone === "dental" ? "bg-mint" : "bg-signal"}`} />
            <div className="rounded-xl bg-paper px-4 py-3">
              <div className="flex items-baseline justify-between gap-3">
                <span className="text-[14px] font-medium">{t}</span>
                <span className="font-mono text-[11px] tabular-nums text-ink-faint">{d}</span>
              </div>
              <p className="mt-0.5 text-[13px] text-ink-soft">{n}</p>
            </div>
          </motion.li>
        ))}
      </ol>
    </div>
  );
}

function PermisosScreen() {
  const users = [
    ["Dra. Ana Ruiz", "Odontóloga", [true, true, true]],
    ["Dr. Luis Paredes", "Médico general", [true, true, true]],
    ["Karla Núñez", "Recepción", [true, false, false]],
    ["Tú", "Administrador", [true, true, true]],
  ] as const;
  return (
    <div>
      <p className={label}>Usuarios</p>
      <p className="font-display text-3xl leading-none">Permisos por rol</p>
      <div className="mt-6 grid grid-cols-[1fr_repeat(3,3.5rem)] items-center gap-y-1 text-[12px] sm:grid-cols-[1fr_repeat(3,4.5rem)]">
        <span />
        {["Agenda", "Expedientes", "Historial"].map((h) => <span key={h} className={`${label} truncate text-center`}>{h}</span>)}
        {users.map(([name, role, perms], ui) => (
          <React.Fragment key={name}>
            <span className="min-w-0 border-t border-ink/5 py-3">
              <span className="block truncate text-[14px] font-medium">{name}</span>
              <span className="block text-ink-soft">{role}</span>
            </span>
            {perms.map((on, pi) => (
              <span key={pi} className="flex justify-center border-t border-ink/5 py-3">
                <span className={`relative h-5 w-9 rounded-full transition-colors ${on ? "bg-mint" : "bg-ink/10"}`}>
                  <motion.span
                    className="absolute top-0.5 h-4 w-4 rounded-full bg-white shadow"
                    initial={{ left: 2 }}
                    animate={{ left: on ? 18 : 2 }}
                    transition={{ delay: 0.25 + (ui * 3 + pi) * 0.04, type: "spring", stiffness: 500, damping: 30 }}
                  />
                </span>
              </span>
            ))}
          </React.Fragment>
        ))}
      </div>
    </div>
  );
}

const steps = [
  {
    n: "01",
    title: "Una agenda que *todo tu equipo* comparte.",
    text: "Crea, reprograma o cancela citas en segundos. Odontología y medicina general en la misma vista, sin empalmes ni recados perdidos.",
    points: ["Vista por día y por profesional", "Reprograma en segundos"],
    Screen: AgendaScreen,
  },
  {
    n: "02",
    title: "El paciente completo, *en una pantalla.*",
    text: "Datos generales, antecedentes, alergias y hábitos de salud. Capturados una vez y siempre a la mano. Búscalo por CURP al instante.",
    points: ["Antecedentes patológicos", "Búsqueda por CURP"],
    Screen: ExpedienteScreen,
  },
  {
    n: "03",
    title: "Cada consulta, *en su lugar.*",
    text: "El historial se ordena solo. Llega a la consulta sabiendo exactamente qué pasó la vez anterior, sea una limpieza o un control de presión.",
    points: ["Orden cronológico", "Tratamientos de varias sesiones"],
    Screen: HistorialScreen,
  },
  {
    n: "04",
    title: "Cada quien ve *lo que le toca.*",
    text: "Recepción agenda, el doctor consulta y tú administras. Los permisos por rol protegen la información clínica de tus pacientes.",
    points: ["Roles por usuario", "Acceso con contraseña"],
    Screen: PermisosScreen,
  },
];

function Step({ s, i, onActive, active }: { s: (typeof steps)[number]; i: number; onActive: (i: number) => void; active: boolean }) {
  const ref = useRef<HTMLDivElement>(null);
  const inView = useInView(ref, { margin: "-45% 0px -45% 0px" });
  useEffect(() => { if (inView) onActive(i); }, [inView, i, onActive]);
  const Screen = s.Screen;
  return (
    <div ref={ref} className="flex flex-col justify-center py-14 lg:min-h-[85vh] lg:py-0">
      <motion.div animate={{ opacity: active ? 1 : 0.25 }} transition={{ duration: 0.5 }} className="lg:pr-8">
        <p className="font-mono text-sm text-signal">{s.n} <span className="text-ink-faint">/ 04</span></p>
        <Split as="h3" text={s.title} className="mt-4 font-display text-[clamp(2.25rem,4.5vw,3.75rem)] leading-[1] tracking-[-0.02em]" accent="italic" />
        <p className="mt-5 max-w-md text-lg leading-relaxed text-ink-soft [text-wrap:pretty]">{s.text}</p>
        <ul className="mt-6 flex flex-wrap gap-2">
          {s.points.map((p) => (
            <li key={p} className="flex items-center gap-1.5 rounded-full border border-ink/15 px-3 py-1.5 text-[13px]">
              <Icon className="h-3.5 w-3.5 text-mint" strokeWidth={2.4}>{check}</Icon>{p}
            </li>
          ))}
        </ul>
      </motion.div>
      {/* small screens: the screen follows its text */}
      <div className="mt-8 rounded-[22px] bg-white p-5 shadow-[0_30px_60px_-30px_rgba(20,33,29,0.35)] ring-1 ring-ink/10 lg:hidden" aria-hidden="true">
        <Screen />
      </div>
    </div>
  );
}

export default function Features() {
  const [active, setActive] = useState(0);
  const reduce = useReducedMotion();
  const Screen = steps[active].Screen;
  return (
    <section id="funciones" className="relative">
      <div className="mx-auto max-w-6xl px-5 pt-28 sm:px-8 lg:pt-40">
        <div className="grid gap-6 border-b border-ink/15 pb-10 md:grid-cols-[1.7fr_1fr] md:items-end">
          <Split text={"Todo tu consultorio.\n*Nada que estorbe.*"} className="font-display text-[clamp(2.75rem,6.5vw,5.25rem)] leading-[0.95] tracking-[-0.03em]" accent="italic text-signal" />
          <p className="max-w-sm text-lg leading-relaxed text-ink-soft md:justify-self-end">Cuatro herramientas que reemplazan la libreta, el archivero y las hojas de cálculo.</p>
        </div>

        <div className="grid lg:grid-cols-2 lg:gap-16">
          <div>
            {steps.map((s, i) => <Step key={s.n} s={s} i={i} onActive={setActive} active={active === i} />)}
          </div>
          <div className="hidden lg:block" aria-hidden="true">
            <div className="sticky top-0 flex h-screen items-center">
              <div className="relative w-full">
                <div className="absolute -inset-6 -z-10 rounded-[40px] bg-gradient-to-br from-mint-soft via-transparent to-signal-soft opacity-70 blur-2xl" />
                <div className="relative h-[29rem] overflow-hidden rounded-[26px] bg-white p-7 shadow-[0_40px_80px_-30px_rgba(20,33,29,0.4)] ring-1 ring-ink/10">
                  <AnimatePresence mode="wait">
                    <motion.div
                      key={active}
                      initial={reduce ? { opacity: 0 } : { opacity: 0, y: 40, filter: "blur(8px)" }}
                      animate={{ opacity: 1, y: 0, filter: "blur(0px)" }}
                      exit={reduce ? { opacity: 0 } : { opacity: 0, y: -40, filter: "blur(8px)" }}
                      transition={{ duration: 0.5, ease }}
                    >
                      <Screen />
                    </motion.div>
                  </AnimatePresence>
                </div>
                <div className="mt-6 flex gap-2">
                  {steps.map((s, i) => (
                    <span key={s.n} className="h-[3px] flex-1 overflow-hidden rounded-full bg-ink/10">
                      <motion.span className="block h-full origin-left bg-ink" animate={{ scaleX: i <= active ? 1 : 0 }} transition={{ duration: 0.6, ease }} />
                    </span>
                  ))}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
