"use client"
import React, { useRef, useState } from 'react';
import { AnimatePresence, motion, useReducedMotion, useScroll, useTransform } from 'framer-motion';
import { ButtonLabel, Ecg, Icon, Magnetic, Reveal, Split, Wordmark, arrow, btn, check, ease } from './motion';
import { contactEmail, demoHref, faqs, loginPath, plans } from './data';

const steps = [
  { title: "Agenda una demo", text: "Te mostramos Caresia en 20 minutos con ejemplos de tu especialidad." },
  { title: "Damos de alta tu clínica", text: "Configuramos tu cuenta, tus usuarios y sus permisos." },
  { title: "Empieza a atender", text: "Registra pacientes y citas desde el primer día. Tu equipo aprende en una tarde." },
];

export function Steps() {
  const ref = useRef<HTMLDivElement>(null);
  const { scrollYProgress } = useScroll({ target: ref, offset: ["start 80%", "end 60%"] });
  const line = useTransform(scrollYProgress, [0, 1], [0, 1]);
  return (
    <section className="relative overflow-hidden rounded-t-[36px] bg-ink px-5 py-28 text-paper sm:px-8 lg:py-40">
      <div className="mx-auto max-w-6xl">
        <div className="grid gap-6 md:grid-cols-[1.7fr_1fr] md:items-end">
          <Split text={"Listo en días,\n*no en meses.*"} className="font-display text-[clamp(2.75rem,6.5vw,5.25rem)] leading-[0.95] tracking-[-0.03em]" accent="italic text-signal" />
          <p className="max-w-sm text-lg leading-relaxed text-paper/60 md:justify-self-end">Sin instalaciones, sin servidores y sin capacitaciones eternas.</p>
        </div>
        <div ref={ref} className="relative mt-20">
          <div className="absolute left-0 right-0 top-[27px] hidden h-px bg-paper/15 md:block">
            <motion.div style={{ scaleX: line }} className="h-px origin-left bg-signal" />
          </div>
          <ol className="grid gap-12 md:grid-cols-3 md:gap-8">
            {steps.map((s, i) => (
              <li key={s.title} className="relative">
                <Reveal delay={i * 0.12}>
                  <span className="relative grid h-14 w-14 place-items-center rounded-full bg-ink font-display text-2xl italic text-signal ring-1 ring-paper/20">{i + 1}</span>
                  <h3 className="mt-8 font-display text-4xl leading-none">{s.title}</h3>
                  <p className="mt-4 max-w-xs text-[16px] leading-relaxed text-paper/60">{s.text}</p>
                </Reveal>
              </li>
            ))}
          </ol>
        </div>
      </div>
    </section>
  );
}

function PlanCard({ p, i }: { p: (typeof plans)[number]; i: number }) {
  const ref = useRef<HTMLLIElement>(null);
  const dark = p.featured;
  return (
    <motion.li
      ref={ref}
      initial={{ opacity: 0, y: 60 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, margin: "-10% 0px" }}
      transition={{ duration: 0.9, delay: i * 0.1, ease }}
      onPointerMove={(e) => {
        const el = ref.current;
        if (!el) return;
        const r = el.getBoundingClientRect();
        el.style.setProperty("--x", `${e.clientX - r.left}px`);
        el.style.setProperty("--y", `${e.clientY - r.top}px`);
      }}
      className={`group relative flex flex-col overflow-hidden rounded-[28px] p-8 sm:p-9 ${dark ? "bg-ink text-paper lg:-my-6 lg:py-14" : "bg-white ring-1 ring-ink/10"}`}
    >
      <span
        aria-hidden="true"
        className="pointer-events-none absolute inset-0 opacity-0 transition-opacity duration-300 group-hover:opacity-100"
        style={{ background: `radial-gradient(320px circle at var(--x) var(--y), ${dark ? "rgba(22,115,209,0.22)" : "rgba(22,115,209,0.10)"}, transparent 70%)` }}
      />
      <div className="relative flex items-center justify-between">
        <h3 className="font-display text-4xl">{p.name}</h3>
        {p.featured && <span className="rounded-full bg-signal px-3 py-1 font-mono text-[10px] uppercase tracking-[0.14em] text-white">Más elegido</span>}
      </div>
      <p className={`relative mt-3 text-[15px] leading-relaxed ${dark ? "text-paper/60" : "text-ink-soft"}`}>{p.blurb}</p>
      <p className={`relative mt-8 flex items-baseline gap-2 border-t pt-6 ${dark ? "border-paper/10" : "border-ink/10"}`}>
        <span className="font-display text-6xl leading-none tracking-[-0.02em] tabular-nums">{p.price}</span>
        {p.period && <span className={`font-mono text-[11px] uppercase tracking-[0.1em] ${dark ? "text-paper/50" : "text-ink-faint"}`}>{p.period}</span>}
      </p>
      <ul className="relative mt-8 flex-1 space-y-3 text-[15px]">
        {p.features.map((f) => (
          <li key={f} className="flex gap-3">
            <Icon className={`mt-0.5 h-4 w-4 flex-none ${dark ? "text-signal" : "text-mint"}`} strokeWidth={2.4}>{check}</Icon>
            <span className={dark ? "text-paper/85" : "text-ink"}>{f}</span>
          </li>
        ))}
      </ul>
      <a href={demoHref} className={`${btn} relative mt-10 h-12 px-6 text-[15px] ${dark ? "bg-signal text-white hover:bg-paper hover:text-ink focus-visible:ring-offset-ink" : "bg-ink text-paper hover:bg-signal"}`}>
        <ButtonLabel>{p.cta}</ButtonLabel>
      </a>
    </motion.li>
  );
}

export function Pricing() {
  return (
    <section id="precios" className="mx-auto max-w-6xl px-5 py-28 sm:px-8 lg:py-40">
      <div className="grid gap-6 border-b border-ink/15 pb-10 md:grid-cols-[1.7fr_1fr] md:items-end">
        <Split text={"Planes claros,\n*sin letras chiquitas.*"} className="font-display text-[clamp(2.75rem,6.5vw,5.25rem)] leading-[0.95] tracking-[-0.03em]" accent="italic text-signal" />
        <p className="max-w-sm text-lg leading-relaxed text-ink-soft md:justify-self-end">Pago mensual, sin plazo forzoso. Cancela cuando quieras.</p>
      </div>
      <ul className="mt-14 grid gap-4 lg:mt-20 lg:grid-cols-3 lg:items-stretch">
        {plans.map((p, i) => <PlanCard key={p.name} p={p} i={i} />)}
      </ul>
      <p className="mt-10 text-center font-mono text-[11px] uppercase tracking-[0.14em] text-ink-faint">Precios en pesos mexicanos · IVA no incluido</p>
    </section>
  );
}

export function Faq() {
  const [open, setOpen] = useState<number | null>(0);
  return (
    <section id="preguntas" className="mx-auto grid max-w-6xl gap-12 px-5 pb-28 sm:px-8 lg:grid-cols-[1fr_1.4fr] lg:pb-40">
      <div>
        <Split text={"¿Tienes\n*dudas?*"} className="font-display text-[clamp(2.75rem,6.5vw,5.25rem)] leading-[0.95] tracking-[-0.03em]" accent="italic text-signal" />
        <p className="mt-6 max-w-sm text-lg leading-relaxed text-ink-soft">
          Escríbenos a <a href={`mailto:${contactEmail}`} className="text-ink underline decoration-ink/30 underline-offset-4 hover:decoration-signal">{contactEmail}</a> y te respondemos.
        </p>
      </div>
      <ul className="border-t border-ink/15">
        {faqs.map((f, i) => {
          const isOpen = open === i;
          return (
            <li key={f.q} className="border-b border-ink/15">
              <button
                type="button"
                aria-expanded={isOpen}
                onClick={() => setOpen(isOpen ? null : i)}
                className="group flex w-full items-center justify-between gap-6 py-6 text-left"
              >
                <span className="text-lg font-medium transition-colors group-hover:text-signal sm:text-xl">{f.q}</span>
                <span className={`relative grid h-9 w-9 flex-none place-items-center rounded-full transition-colors duration-300 ${isOpen ? "bg-ink text-paper" : "ring-1 ring-ink/20"}`}>
                  <span className="absolute h-[1.5px] w-3.5 bg-current" />
                  <motion.span className="absolute h-3.5 w-[1.5px] bg-current" animate={{ rotate: isOpen ? 90 : 0 }} transition={{ duration: 0.4, ease }} />
                </span>
              </button>
              <AnimatePresence initial={false}>
                {isOpen && (
                  <motion.div
                    initial={{ height: 0, opacity: 0 }}
                    animate={{ height: "auto", opacity: 1 }}
                    exit={{ height: 0, opacity: 0 }}
                    transition={{ duration: 0.5, ease }}
                    className="overflow-hidden"
                  >
                    <p className="max-w-xl pb-6 pr-12 text-[16px] leading-relaxed text-ink-soft">{f.a}</p>
                  </motion.div>
                )}
              </AnimatePresence>
            </li>
          );
        })}
      </ul>
    </section>
  );
}

export function Cta() {
  const ref = useRef<HTMLElement>(null);
  const reduce = useReducedMotion();
  const { scrollYProgress } = useScroll({ target: ref, offset: ["start end", "end end"] });
  const scale = useTransform(scrollYProgress, [0, 1], reduce ? [1, 1] : [0.92, 1]);
  const radius = useTransform(scrollYProgress, [0, 1], reduce ? [36, 36] : [80, 36]);
  return (
    <section id="contacto" ref={ref} className="px-3 sm:px-5">
      <motion.div style={{ scale, borderRadius: radius }} className="relative isolate overflow-hidden bg-signal px-6 py-24 text-center text-white sm:py-32">
        <Ecg className="absolute inset-x-0 top-1/2 -z-10 h-40 w-full -translate-y-1/2" base="stroke-white/15" sweep="stroke-white/70" beats={5} />
        <p className="font-mono text-[11px] uppercase tracking-[0.16em] text-white/75">Demo gratuita · 20 minutos</p>
        <Split
          text={"¿Listo para *ordenar*\ntu consulta?"}
          className="mx-auto mt-6 max-w-5xl font-display text-[clamp(3rem,9vw,8.5rem)] leading-[0.9] tracking-[-0.035em]"
          accent="italic"
        />
        <div className="mt-12 flex flex-wrap items-center justify-center gap-4">
          <Magnetic>
            <a href={demoHref} className={`${btn} h-16 bg-ink pl-8 pr-2 text-[17px] text-paper hover:bg-paper hover:text-ink focus-visible:ring-offset-signal`}>
              <ButtonLabel>Solicitar demo gratis</ButtonLabel>
              <span className="ml-2 grid h-12 w-12 place-items-center rounded-full bg-signal text-white transition-transform duration-500 ease-out-strong group-hover:rotate-[-45deg]">
                <Icon className="h-5 w-5">{arrow}</Icon>
              </span>
            </a>
          </Magnetic>
          <a href={loginPath} className="px-4 py-2 text-[16px] font-medium text-white/90 underline decoration-white/40 underline-offset-4 hover:decoration-white">Ya soy cliente</a>
        </div>
      </motion.div>
    </section>
  );
}

export function Footer() {
  const ref = useRef<HTMLElement>(null);
  const reduce = useReducedMotion();
  const { scrollYProgress } = useScroll({ target: ref, offset: ["start end", "end end"] });
  const y = useTransform(scrollYProgress, [0, 1], reduce ? ["0%", "0%"] : ["55%", "0%"]);
  const cols = [
    ["Producto", [["#funciones", "Funciones"], ["#especialidades", "Especialidades"], ["#precios", "Precios"]]],
    ["Soporte", [["#preguntas", "Preguntas frecuentes"], [`mailto:${contactEmail}`, "Contacto"]]],
    ["Cuenta", [[loginPath, "Iniciar sesión"], [demoHref, "Solicitar demo"]]],
  ] as const;
  return (
    <footer ref={ref} className="relative mt-3 overflow-hidden bg-ink text-paper">
      <div className="mx-auto grid max-w-6xl gap-12 px-5 pb-10 pt-20 sm:px-8 md:grid-cols-[1.6fr_1fr_1fr_1fr]">
        <div>
          <Wordmark light />
          <p className="mt-5 max-w-xs text-[15px] leading-relaxed text-paper/55">Software de gestión para clínicas dentales y consultorios médicos.</p>
        </div>
        {cols.map(([title, links]) => (
          <div key={title}>
            <p className="font-mono text-[11px] uppercase tracking-[0.16em] text-paper/45">{title}</p>
            <ul className="mt-5 space-y-3 text-[15px]">
              {links.map(([href, label]) => (
                <li key={label}><a href={href} className="text-paper/80 transition-colors hover:text-signal">{label}</a></li>
              ))}
            </ul>
          </div>
        ))}
      </div>
      <div className="mx-auto max-w-6xl px-5 sm:px-8">
      <div className="flex justify-between border-t border-paper/10 py-6 font-mono text-[11px] uppercase tracking-[0.12em] text-paper/45">
        <span>© {new Date().getFullYear()} Caresia</span>
        <span>Todos los derechos reservados</span>
      </div>
      </div>
      <div aria-hidden="true" className="overflow-hidden">
        <motion.p style={{ y }} className="select-none text-center font-display text-[27vw] leading-[0.78] tracking-[-0.04em] text-paper/[0.07]">
          Caresia
        </motion.p>
      </div>
    </footer>
  );
}
