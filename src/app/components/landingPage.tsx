import React from 'react';

const loginPath = "/api/auth/signin";
const contactEmail = "contacto@caresia.com";

// Stagger for the .rise entrance (globals.css): 70ms between items
const delay = (i: number) => ({ animationDelay: `${i * 70}ms` });

// Pressable: feedback on press (not release), no double-tap delay, label not selectable on long-press
const press = "inline-flex select-none touch-manipulation items-center justify-center gap-2 rounded-full font-medium transition-[transform,background-color] duration-150 ease-out-strong active:scale-[0.97] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-teal-500 focus-visible:ring-offset-2";
const dark = "bg-slate-900 text-white hover:bg-slate-800";
const light = "border border-slate-900/10 bg-white text-slate-900 hover:bg-slate-50";

const eyebrow = "text-sm font-medium text-teal-700";
const h2 = "mt-3 text-[clamp(1.875rem,4vw,2.75rem)] font-semibold leading-[1.1] tracking-[-0.025em] [text-wrap:balance]";
const lead = "mt-4 text-lg leading-relaxed text-slate-600 [text-wrap:pretty]";
const card = "rounded-3xl bg-white p-2 shadow-[0_1px_2px_rgba(15,23,42,0.04),0_24px_48px_-12px_rgba(15,23,42,0.18)] ring-1 ring-slate-900/5";

const Icon = ({ children, className = "h-5 w-5" }: { children: React.ReactNode; className?: string }) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className={className}>
    {children}
  </svg>
);

const check = <path d="M20 6 9 17l-5-5" />;

const Wordmark = () => (
  <span className="inline-flex items-center gap-2 text-[17px] font-semibold tracking-[-0.01em] text-slate-900">
    <span className="grid h-7 w-7 place-items-center rounded-lg bg-gradient-to-br from-teal-400 to-teal-600 shadow-sm shadow-teal-600/30">
      <svg viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth={2.5} strokeLinecap="round" aria-hidden="true" className="h-4 w-4">
        <path d="M12 6v12M6 12h12" />
      </svg>
    </span>
    Caresia
  </span>
);

const features = [
  {
    title: "Expedientes de pacientes",
    text: "Registra a cada paciente una sola vez y encuéntralo al instante por su CURP.",
    icon: <><path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2" /><circle cx="12" cy="7" r="4" /></>,
  },
  {
    title: "Agenda de citas",
    text: "Crea, cambia o cancela citas en segundos. Todo tu equipo trabaja sobre la misma agenda.",
    icon: <><rect x="3" y="4" width="18" height="18" rx="2" /><path d="M16 2v4M8 2v4M3 10h18" /></>,
  },
  {
    title: "Historial clínico",
    text: "Cada consulta queda en el expediente, lista para la siguiente visita. Sin carpetas ni papeles.",
    icon: <><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><path d="M14 2v6h6M16 13H8M16 17H8M10 9H8" /></>,
  },
  {
    title: "Permisos por rol",
    text: "Recepción agenda, el doctor consulta y tú administras. Cada quien ve solo lo que necesita.",
    icon: <><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" /><path d="m9 12 2 2 4-4" /></>,
  },
];

const agenda = [
  { time: "09:00", name: "Mariana López", reason: "Limpieza dental" },
  { time: "10:30", name: "Diego Hernández", reason: "Ajuste de ortodoncia" },
  { time: "12:00", name: "Sofía Ramírez", reason: "Consulta general" },
  { time: "13:30", name: "Andrés Torres", reason: "Endodoncia · 2.ª sesión" },
];

const dentalPoints = [
  "Tratamientos de varias sesiones —ortodoncia, endodoncia, implantes— con todo su historial en un solo expediente.",
  "Una agenda hecha para citas cortas y seguidas: limpiezas, revisiones y urgencias del día.",
  "Asistentes y recepción con acceso a la agenda, sin ver lo que no les corresponde.",
];

const timeline = [
  { date: "12 feb", title: "Valoración inicial", note: "Radiografía panorámica" },
  { date: "04 mar", title: "Colocación de brackets", note: "Arcada superior e inferior" },
  { date: "08 abr", title: "Primer ajuste", note: "Sin molestias reportadas" },
  { date: "Próxima", title: "Segundo ajuste", note: "Agendada · 10:30" },
];

const specialties = ["Odontología", "Medicina general", "Pediatría", "Fisioterapia", "Nutrición", "Psicología", "Dermatología", "Ginecología"];

const landingPage: React.FC = () => (
  <div className="bg-white text-slate-900 antialiased selection:bg-teal-100 [-webkit-tap-highlight-color:transparent]">
    <header className="sticky top-0 z-40 border-b border-slate-900/5 bg-white/75 backdrop-blur-xl backdrop-saturate-150">
      <nav className="mx-auto flex h-16 max-w-6xl items-center justify-between px-4 sm:px-6">
        <a href="#" aria-label="Caresia, inicio" className="rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-teal-500">
          <Wordmark />
        </a>
        <div className="flex items-center gap-1">
          {[["#funciones", "Funciones"], ["#dentistas", "Dentistas"], ["#contacto", "Contacto"]].map(([href, label]) => (
            <a key={href} href={href} className="hidden h-10 items-center rounded-full px-3 text-sm text-slate-600 transition-colors duration-150 hover:text-slate-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-teal-500 md:inline-flex">
              {label}
            </a>
          ))}
          <a href={loginPath} className={`${press} ${dark} ml-2 h-10 px-4 text-sm`}>Iniciar sesión</a>
        </div>
      </nav>
    </header>

    <section className="relative isolate overflow-hidden">
      <div aria-hidden="true" className="pointer-events-none absolute inset-x-0 -top-40 -z-10 h-[40rem] bg-[radial-gradient(55%_50%_at_50%_0%,#ccfbf1,transparent)]" />
      <div className="mx-auto grid max-w-6xl items-center gap-16 px-4 pb-24 pt-14 sm:px-6 md:pt-20 lg:grid-cols-[1.05fr_1fr] lg:gap-12 lg:pb-32 lg:pt-24">
        <div>
          <p className="rise inline-flex items-center gap-2 rounded-full bg-white/70 px-3 py-1 text-sm font-medium text-teal-800 ring-1 ring-inset ring-teal-600/20" style={delay(0)}>
            <span className="h-1.5 w-1.5 rounded-full bg-teal-500" />
            Para dentistas y consultorios médicos
          </p>
          <h1 className="rise mt-6 text-[clamp(2.5rem,6vw,4.25rem)] font-semibold leading-[1.04] tracking-[-0.03em] [text-wrap:balance]" style={delay(1)}>
            Tu consultorio, por fin en orden.
          </h1>
          <p className="rise mt-6 max-w-xl text-lg leading-relaxed text-slate-600 [text-wrap:pretty] sm:text-xl" style={delay(2)}>
            Caresia reúne pacientes, citas e historiales clínicos en un solo lugar. Pensado para clínicas dentales y para cualquier consultorio que atiende con cita.
          </p>
          <div className="rise mt-10 flex flex-col gap-3 sm:flex-row" style={delay(3)}>
            <a href={loginPath} className={`${press} ${dark} group h-12 px-6 text-[15px]`}>
              Iniciar sesión
              <Icon className="h-4 w-4 transition-transform duration-150 ease-out-strong group-hover:translate-x-0.5"><path d="M5 12h14M13 6l6 6-6 6" /></Icon>
            </a>
            <a href="#funciones" className={`${press} ${light} h-12 px-6 text-[15px]`}>Ver funciones</a>
          </div>
        </div>

        {/* Product preview: illustrative, so hidden from assistive tech */}
        <div aria-hidden="true" className="relative">
          <div className={`rise ${card}`} style={delay(2)}>
            <div className="rounded-[18px] bg-slate-50 p-5 sm:p-6">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-xs font-medium uppercase tracking-[0.08em] text-slate-500">Agenda</p>
                  <p className="mt-1 text-lg font-semibold tracking-[-0.01em]">Hoy</p>
                </div>
                <span className="rounded-full bg-teal-50 px-2.5 py-1 text-xs font-medium text-teal-700 ring-1 ring-inset ring-teal-600/20">4 citas</span>
              </div>
              <ul className="mt-5 space-y-2">
                {agenda.map((a, i) => (
                  <li key={a.time} className="rise flex items-center gap-4 rounded-xl bg-white px-4 py-3 ring-1 ring-slate-900/5" style={delay(4 + i)}>
                    <span className="w-11 flex-none text-sm font-medium tabular-nums text-slate-500">{a.time}</span>
                    <span className="h-8 w-px flex-none bg-slate-200" />
                    <span className="min-w-0">
                      <span className="block truncate text-sm font-medium">{a.name}</span>
                      <span className="block truncate text-[13px] text-slate-500">{a.reason}</span>
                    </span>
                  </li>
                ))}
              </ul>
            </div>
          </div>
          <div className="rise absolute -bottom-7 -left-3 hidden items-center gap-3 rounded-2xl bg-white/90 py-3 pl-3 pr-5 shadow-lg shadow-slate-900/10 ring-1 ring-slate-900/5 backdrop-blur sm:flex lg:-left-8" style={delay(8)}>
            <span className="grid h-9 w-9 place-items-center rounded-full bg-teal-500 text-white"><Icon className="h-4 w-4">{check}</Icon></span>
            <span>
              <span className="block text-sm font-medium">Historial actualizado</span>
              <span className="block text-[13px] text-slate-500">Mariana López · 6 consultas</span>
            </span>
          </div>
        </div>
      </div>
    </section>

    <section id="funciones" className="scroll-mt-16 border-y border-slate-900/5 bg-slate-50/70">
      <div className="mx-auto max-w-6xl px-4 py-20 sm:px-6 lg:py-28">
        <div className="max-w-2xl">
          <p className={eyebrow}>Funciones</p>
          <h2 className={h2}>Todo tu consultorio, en un solo lugar.</h2>
          <p className={lead}>Lo esencial para atender mejor, sin funciones que estorben.</p>
        </div>
        <ul className="mt-12 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {features.map((f) => (
            <li key={f.title} className="rounded-2xl bg-white p-6 ring-1 ring-slate-900/5">
              <span className="grid h-10 w-10 place-items-center rounded-xl bg-teal-50 text-teal-700 ring-1 ring-inset ring-teal-600/10"><Icon>{f.icon}</Icon></span>
              <h3 className="mt-5 font-semibold tracking-[-0.01em]">{f.title}</h3>
              <p className="mt-2 text-[15px] leading-relaxed text-slate-600 [text-wrap:pretty]">{f.text}</p>
            </li>
          ))}
        </ul>
      </div>
    </section>

    <section id="dentistas" className="scroll-mt-16">
      <div className="mx-auto grid max-w-6xl items-center gap-14 px-4 py-20 sm:px-6 lg:grid-cols-2 lg:gap-20 lg:py-28">
        <div>
          <p className={eyebrow}>Odontología</p>
          <h2 className={h2}>Hecho para el ritmo de un consultorio dental.</h2>
          <p className={lead}>Del sillón a la recepción: cada tratamiento queda documentado y cada cita, en su lugar.</p>
          <ul className="mt-8 space-y-4">
            {dentalPoints.map((p) => (
              <li key={p} className="flex gap-3 text-[15px] leading-relaxed text-slate-700">
                <span className="mt-0.5 grid h-5 w-5 flex-none place-items-center rounded-full bg-teal-500 text-white"><Icon className="h-3 w-3">{check}</Icon></span>
                {p}
              </li>
            ))}
          </ul>
        </div>

        <div aria-hidden="true" className={`${card} lg:order-first`}>
          <div className="rounded-[18px] bg-slate-50 p-5 sm:p-6">
            <div className="flex items-center gap-3">
              <span className="grid h-10 w-10 flex-none place-items-center rounded-full bg-teal-100 text-sm font-semibold text-teal-800">DH</span>
              <span className="min-w-0 flex-1">
                <span className="block truncate font-semibold tracking-[-0.01em]">Diego Hernández</span>
                <span className="block text-[13px] text-slate-500">Ortodoncia · Expediente</span>
              </span>
              <span className="rounded-full bg-white px-2.5 py-1 text-xs font-medium text-slate-600 ring-1 ring-inset ring-slate-900/10">En tratamiento</span>
            </div>
            <ol className="relative mt-6 space-y-5 before:absolute before:bottom-2 before:left-[5px] before:top-2 before:w-px before:bg-slate-200">
              {timeline.map((t, i) => {
                const next = i === timeline.length - 1;
                return (
                  <li key={t.title} className="relative flex gap-4 pl-7">
                    <span className={`absolute left-0 top-1.5 h-[11px] w-[11px] rounded-full ring-4 ring-slate-50 ${next ? "bg-teal-500" : "bg-slate-300"}`} />
                    <span className="min-w-0 flex-1 rounded-xl bg-white px-4 py-3 ring-1 ring-slate-900/5">
                      <span className="flex items-baseline justify-between gap-3">
                        <span className="truncate text-sm font-medium">{t.title}</span>
                        <span className={`flex-none text-xs tabular-nums ${next ? "font-medium text-teal-700" : "text-slate-500"}`}>{t.date}</span>
                      </span>
                      <span className="mt-0.5 block text-[13px] text-slate-500">{t.note}</span>
                    </span>
                  </li>
                );
              })}
            </ol>
          </div>
        </div>
      </div>
    </section>

    <section className="border-y border-slate-900/5 bg-slate-50/70">
      <div className="mx-auto max-w-6xl px-4 py-20 text-center sm:px-6 lg:py-24">
        <p className={eyebrow}>Para cualquier consulta</p>
        <h2 className={`${h2} mx-auto max-w-2xl`}>Si atiendes con cita, Caresia es para ti.</h2>
        <p className={`${lead} mx-auto max-w-xl`}>Expedientes, agenda e historial funcionan igual en un consultorio de medicina general que en una clínica con varias especialidades.</p>
        <ul className="mx-auto mt-10 flex max-w-3xl flex-wrap justify-center gap-2">
          {specialties.map((s) => (
            <li key={s} className="rounded-full bg-white px-4 py-2 text-sm text-slate-700 ring-1 ring-inset ring-slate-900/10">{s}</li>
          ))}
        </ul>
      </div>
    </section>

    <section id="contacto" className="scroll-mt-16 px-4 py-20 sm:px-6 lg:py-28">
      <div className="relative isolate mx-auto max-w-6xl overflow-hidden rounded-[28px] bg-slate-900 px-6 py-16 text-center sm:px-12 lg:py-20">
        <div aria-hidden="true" className="absolute inset-0 -z-10 bg-[radial-gradient(50%_80%_at_50%_0%,rgba(45,212,191,0.22),transparent)]" />
        <h2 className="mx-auto max-w-2xl text-[clamp(1.875rem,4vw,2.75rem)] font-semibold leading-[1.1] tracking-[-0.025em] text-white [text-wrap:balance]">
          Pon en orden tu consultorio esta semana.
        </h2>
        <p className="mx-auto mt-4 max-w-xl text-lg leading-relaxed text-slate-300 [text-wrap:pretty]">
          Escríbenos y te ayudamos a dar de alta tu clínica y a tu equipo.
        </p>
        <div className="mt-10 flex flex-col justify-center gap-3 sm:flex-row">
          <a href={`mailto:${contactEmail}`} className={`${press} h-12 bg-white px-6 text-[15px] text-slate-900 hover:bg-slate-100 focus-visible:ring-offset-slate-900`}>
            <Icon className="h-4 w-4"><rect x="2" y="4" width="20" height="16" rx="2" /><path d="m22 7-10 5L2 7" /></Icon>
            Escríbenos
          </a>
          <a href={loginPath} className={`${press} h-12 bg-white/10 px-6 text-[15px] text-white hover:bg-white/20 focus-visible:ring-offset-slate-900`}>Iniciar sesión</a>
        </div>
        <p className="mt-6 text-sm text-slate-400">{contactEmail}</p>
      </div>
    </section>

    <footer className="border-t border-slate-900/5">
      <div className="mx-auto flex max-w-6xl flex-col items-center justify-between gap-4 px-4 py-10 text-sm text-slate-500 sm:flex-row sm:px-6">
        <Wordmark />
        <p>© 2026 Caresia. Todos los derechos reservados.</p>
      </div>
    </footer>
  </div>
);

export default landingPage;
