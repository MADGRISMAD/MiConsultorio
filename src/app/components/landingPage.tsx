"use client"
import React, { useState } from 'react';

// ---------------------------------------------------------------------------
// Datos comerciales: edita aquí precios, contacto y textos de venta.
// ---------------------------------------------------------------------------
const loginPath = "/api/auth/signin";
const contactEmail = "contacto@caresia.com";
const demoHref = `mailto:${contactEmail}?subject=${encodeURIComponent("Quiero una demo de Caresia")}`;

const plans = [
  {
    name: "Consultorio",
    price: "$499",
    period: "MXN / mes",
    blurb: "Para el profesional independiente que atiende solo o con un asistente.",
    features: ["1 profesional de la salud", "1 usuario de recepción", "Agenda de citas ilimitada", "Expedientes e historial clínico", "Soporte por correo"],
    cta: "Solicitar demo",
    featured: false,
  },
  {
    name: "Clínica",
    price: "$1,199",
    period: "MXN / mes",
    blurb: "Para clínicas con varios doctores, dentistas o especialidades.",
    features: ["Hasta 5 profesionales", "Usuarios de recepción ilimitados", "Permisos por rol", "Agenda compartida del equipo", "Alta y migración asistida", "Soporte prioritario"],
    cta: "Solicitar demo",
    featured: true,
  },
  {
    name: "Empresarial",
    price: "A medida",
    period: "",
    blurb: "Para grupos médicos, redes de clínicas y varias sucursales.",
    features: ["Profesionales ilimitados", "Varias sucursales", "Capacitación al equipo", "Acompañamiento dedicado"],
    cta: "Hablar con ventas",
    featured: false,
  },
];

// ---------------------------------------------------------------------------

const delay = (i: number) => ({ animationDelay: `${i * 70}ms` });

const press = "inline-flex select-none touch-manipulation items-center justify-center gap-2 rounded-full font-medium transition-[transform,background-color,box-shadow] duration-150 ease-out-strong active:scale-[0.97] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2";
const primary = "bg-blue-600 text-white shadow-sm shadow-blue-600/30 hover:bg-blue-700";
const secondary = "border border-slate-900/10 bg-white text-slate-900 hover:bg-slate-50";

const container = "mx-auto max-w-6xl px-4 sm:px-6";
const eyebrow = "text-sm font-semibold uppercase tracking-[0.08em] text-blue-600";
const h2 = "mt-3 text-[clamp(1.875rem,4vw,2.75rem)] font-semibold leading-[1.1] tracking-[-0.025em] text-slate-900 [text-wrap:balance]";
const lead = "mt-4 text-lg leading-relaxed text-slate-600 [text-wrap:pretty]";
const card = "rounded-3xl bg-white p-2 shadow-[0_1px_2px_rgba(15,23,42,0.04),0_24px_48px_-12px_rgba(15,23,42,0.18)] ring-1 ring-slate-900/5";

const Icon = ({ children, className = "h-5 w-5" }: { children: React.ReactNode; className?: string }) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className={className}>
    {children}
  </svg>
);

const check = <path d="M20 6 9 17l-5-5" />;
const arrow = <path d="M5 12h14M13 6l6 6-6 6" />;
const tooth = <path d="M12 5.5C10.5 4 8.5 3.5 7 4c-2.5.8-3.3 3.6-2.5 6.3.6 2 1.3 3.2 1.6 5.4.3 2 .6 4.3 1.9 4.3 1.6 0 1.4-5 4-5s2.4 5 4 5c1.3 0 1.6-2.3 1.9-4.3.3-2.2 1-3.4 1.6-5.4.8-2.7 0-5.5-2.5-6.3-1.5-.5-3.5 0-5 1.5z" />;
const stethoscope = <><path d="M5 3v5a5 5 0 0 0 10 0V3" /><path d="M10 13v2a5 5 0 0 0 10 0v-1" /><circle cx="20" cy="12" r="2" /></>;

const Wordmark = ({ light = false }: { light?: boolean }) => (
  <span className={`inline-flex items-center gap-2 text-[17px] font-semibold tracking-[-0.01em] ${light ? "text-white" : "text-slate-900"}`}>
    <span className="grid h-8 w-8 place-items-center rounded-lg bg-gradient-to-br from-blue-500 to-teal-500 shadow-sm shadow-blue-600/30">
      <svg viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth={2.5} strokeLinecap="round" aria-hidden="true" className="h-4 w-4">
        <path d="M12 6v12M6 12h12" />
      </svg>
    </span>
    Caresia
  </span>
);

const navLinks = [["#funciones", "Funciones"], ["#especialidades", "Especialidades"], ["#precios", "Precios"], ["#preguntas", "Preguntas"]];

const highlights = ["Sin instalación", "Desde cualquier dispositivo", "Soporte en español"];

const agenda = [
  { time: "09:00", name: "Mariana López", reason: "Limpieza dental", kind: "dental" },
  { time: "09:30", name: "Jorge Medina", reason: "Consulta general", kind: "medica" },
  { time: "10:30", name: "Diego Hernández", reason: "Ajuste de ortodoncia", kind: "dental" },
  { time: "11:15", name: "Sofía Ramírez", reason: "Control de presión arterial", kind: "medica" },
  { time: "12:00", name: "Andrés Torres", reason: "Endodoncia · 2.ª sesión", kind: "dental" },
];

const stats = [
  { label: "Citas hoy", value: "18" },
  { label: "Pacientes", value: "1,240" },
  { label: "Doctores", value: "4" },
];

const features = [
  {
    title: "Agenda de citas",
    text: "Crea, reprograma o cancela citas en segundos. Todo el equipo trabaja sobre la misma agenda, sin empalmes.",
    icon: <><rect x="3" y="4" width="18" height="18" rx="2" /><path d="M16 2v4M8 2v4M3 10h18" /></>,
  },
  {
    title: "Expediente clínico",
    text: "Datos generales, antecedentes, alergias y hábitos del paciente, capturados una vez y siempre a la mano.",
    icon: <><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><path d="M14 2v6h6M16 13H8M16 17H8M10 9H8" /></>,
  },
  {
    title: "Historial de consultas",
    text: "Cada visita queda registrada en orden cronológico. Llega a la consulta sabiendo exactamente qué pasó la vez anterior.",
    icon: <><path d="M3 12a9 9 0 1 0 3-6.7L3 8" /><path d="M3 3v5h5M12 7v5l3 2" /></>,
  },
  {
    title: "Búsqueda por CURP",
    text: "Encuentra a cualquier paciente al instante. Se acabaron las carpetas perdidas y los expedientes duplicados.",
    icon: <><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></>,
  },
  {
    title: "Usuarios y permisos",
    text: "Recepción agenda, el doctor consulta y tú administras. Cada quien ve solo lo que le corresponde.",
    icon: <><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" /><path d="m9 12 2 2 4-4" /></>,
  },
  {
    title: "En la nube",
    text: "Funciona en el navegador de tu computadora, tableta o celular. Sin servidores, sin instalaciones, sin respaldos manuales.",
    icon: <><path d="M17.5 19a4.5 4.5 0 1 0-1.4-8.8A6 6 0 0 0 4.5 12 3.5 3.5 0 0 0 6 19z" /></>,
  },
];

const specialtyTabs = {
  dental: {
    label: "Clínica dental",
    icon: tooth,
    title: "Hecho para el ritmo de un consultorio dental.",
    text: "Tratamientos de varias sesiones, citas cortas y seguidas, y un equipo que se mueve entre el sillón y la recepción.",
    points: [
      "Ortodoncia, endodoncia o implantes: todas las sesiones en un solo expediente.",
      "Agenda pensada para limpiezas, revisiones y urgencias del día.",
      "Asistentes y recepción con acceso a la agenda, sin ver información clínica.",
    ],
  },
  medica: {
    label: "Medicina general",
    icon: stethoscope,
    title: "Todo lo que necesitas para tu consulta médica.",
    text: "Un expediente completo para conocer a tu paciente antes de que entre al consultorio.",
    points: [
      "Antecedentes patológicos: diabetes, cardiopatías, alergias, cirugías y más.",
      "Peso, estatura y hábitos de salud registrados en cada expediente.",
      "Seguimiento de pacientes crónicos con su historial de consultas a la mano.",
    ],
  },
} as const;

type SpecialtyKey = keyof typeof specialtyTabs;

const timeline = [
  { date: "12 feb", title: "Valoración inicial", note: "Radiografía panorámica" },
  { date: "04 mar", title: "Colocación de brackets", note: "Arcada superior e inferior" },
  { date: "08 abr", title: "Primer ajuste", note: "Sin molestias reportadas" },
  { date: "Próxima", title: "Segundo ajuste", note: "Agendada · 10:30" },
];

const medicalFields = [
  { label: "Peso", value: "78 kg" },
  { label: "Estatura", value: "172 cm" },
  { label: "Actividad física", value: "3 veces / semana" },
  { label: "Tabaquismo", value: "No" },
];

const antecedentes = [
  { label: "Diabetes", on: true },
  { label: "Alergias", on: true },
  { label: "Cardiopatías", on: false },
  { label: "Cirugías", on: false },
];

const moreSpecialties = ["Odontología", "Medicina general", "Pediatría", "Medicina interna", "Fisioterapia", "Nutrición", "Psicología", "Dermatología", "Ginecología", "Ortopedia"];

const steps = [
  { title: "Agenda una demo", text: "Te mostramos Caresia en 20 minutos con ejemplos de tu especialidad." },
  { title: "Damos de alta tu clínica", text: "Configuramos tu cuenta, tus usuarios y sus permisos." },
  { title: "Empieza a atender", text: "Registra pacientes y citas desde el primer día. Tu equipo aprende en una tarde." },
];

const faqs = [
  { q: "¿Necesito instalar algo?", a: "No. Caresia funciona desde el navegador en computadora, tableta o celular. Solo necesitas conexión a internet." },
  { q: "¿Sirve para una clínica dental y para un consultorio de medicina general?", a: "Sí. La agenda, los expedientes y el historial clínico funcionan igual de bien en odontología, medicina general y otras especialidades que atienden con cita." },
  { q: "¿Puedo dar acceso a mi recepcionista sin que vea los expedientes?", a: "Sí. Cada usuario tiene permisos por rol: puedes permitir que recepción administre la agenda sin acceso a la información clínica." },
  { q: "¿Cómo se protege la información de mis pacientes?", a: "El acceso requiere usuario y contraseña, cada clínica solo ve a sus propios pacientes y los permisos por rol limitan quién consulta la información clínica." },
  { q: "¿Me ayudan a empezar?", a: "Sí. En los planes Clínica y Empresarial te acompañamos en el alta, la configuración de usuarios y la capacitación de tu equipo." },
  { q: "¿Hay plazo forzoso?", a: "No. Los planes son mensuales y puedes cancelar cuando quieras." },
];

function SpecialtyPreview({ tab }: { tab: SpecialtyKey }) {
  if (tab === "dental") {
    return (
      <div className="rounded-[18px] bg-slate-50 p-5 sm:p-6">
        <div className="flex items-center gap-3">
          <span className="grid h-10 w-10 flex-none place-items-center rounded-full bg-teal-100 text-sm font-semibold text-teal-800">DH</span>
          <span className="min-w-0 flex-1">
            <span className="block truncate font-semibold tracking-[-0.01em]">Diego Hernández</span>
            <span className="block text-[13px] text-slate-500">Ortodoncia · Expediente</span>
          </span>
          <span className="rounded-full bg-white px-2.5 py-1 text-xs font-medium text-slate-600 ring-1 ring-inset ring-slate-900/10">En tratamiento</span>
        </div>
        <ol className="relative mt-6 space-y-4 before:absolute before:bottom-2 before:left-[5px] before:top-2 before:w-px before:bg-slate-200">
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
    );
  }
  return (
    <div className="rounded-[18px] bg-slate-50 p-5 sm:p-6">
      <div className="flex items-center gap-3">
        <span className="grid h-10 w-10 flex-none place-items-center rounded-full bg-blue-100 text-sm font-semibold text-blue-800">JM</span>
        <span className="min-w-0 flex-1">
          <span className="block truncate font-semibold tracking-[-0.01em]">Jorge Medina</span>
          <span className="block text-[13px] text-slate-500">Medicina general · 54 años</span>
        </span>
        <span className="rounded-full bg-white px-2.5 py-1 text-xs font-medium text-slate-600 ring-1 ring-inset ring-slate-900/10">Seguimiento</span>
      </div>
      <dl className="mt-6 grid grid-cols-2 gap-2">
        {medicalFields.map((f) => (
          <div key={f.label} className="rounded-xl bg-white px-4 py-3 ring-1 ring-slate-900/5">
            <dt className="text-xs text-slate-500">{f.label}</dt>
            <dd className="mt-0.5 text-sm font-medium tabular-nums">{f.value}</dd>
          </div>
        ))}
      </dl>
      <div className="mt-2 rounded-xl bg-white px-4 py-3 ring-1 ring-slate-900/5">
        <p className="text-xs text-slate-500">Antecedentes patológicos</p>
        <ul className="mt-2 flex flex-wrap gap-1.5">
          {antecedentes.map((a) => (
            <li key={a.label} className={`rounded-full px-2.5 py-1 text-xs font-medium ring-1 ring-inset ${a.on ? "bg-blue-50 text-blue-700 ring-blue-600/20" : "bg-slate-50 text-slate-400 ring-slate-900/5"}`}>
              {a.label}
            </li>
          ))}
        </ul>
      </div>
      <div className="mt-2 flex items-center justify-between rounded-xl bg-white px-4 py-3 ring-1 ring-slate-900/5">
        <span className="text-sm font-medium">Próxima consulta</span>
        <span className="text-xs font-medium tabular-nums text-blue-700">15 nov · 09:30</span>
      </div>
    </div>
  );
}

const LandingPage: React.FC = () => {
  const [tab, setTab] = useState<SpecialtyKey>("dental");
  const current = specialtyTabs[tab];
  const accent = tab === "dental" ? "bg-teal-500" : "bg-blue-600";

  return (
    <div className="bg-white text-slate-900 antialiased selection:bg-blue-100 [-webkit-tap-highlight-color:transparent]">
      {/* Navegación */}
      <header className="sticky top-0 z-40 border-b border-slate-900/5 bg-white/80 backdrop-blur-xl backdrop-saturate-150">
        <nav className={`${container} flex h-16 items-center justify-between`}>
          <a href="#" aria-label="Caresia, inicio" className="rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500">
            <Wordmark />
          </a>
          <div className="hidden items-center gap-1 md:flex">
            {navLinks.map(([href, label]) => (
              <a key={href} href={href} className="inline-flex h-10 items-center rounded-full px-3 text-sm text-slate-600 transition-colors duration-150 hover:text-slate-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500">
                {label}
              </a>
            ))}
          </div>
          <div className="flex items-center gap-2">
            <a href={loginPath} className="hidden h-10 items-center rounded-full px-3 text-sm font-medium text-slate-700 hover:text-slate-900 sm:inline-flex">Iniciar sesión</a>
            <a href={demoHref} className={`${press} ${primary} h-10 px-4 text-sm`}>Solicitar demo</a>
          </div>
        </nav>
      </header>

      {/* Hero */}
      <section className="relative isolate overflow-hidden">
        <div aria-hidden="true" className="pointer-events-none absolute inset-x-0 -top-40 -z-10 h-[44rem] bg-[radial-gradient(60%_50%_at_50%_0%,#dbeafe,transparent)]" />
        <div aria-hidden="true" className="pointer-events-none absolute -right-40 top-40 -z-10 h-[28rem] w-[28rem] rounded-full bg-teal-100/60 blur-3xl" />
        <div className={`${container} grid items-center gap-16 pb-24 pt-14 md:pt-20 lg:grid-cols-[1fr_1.1fr] lg:gap-12 lg:pb-28 lg:pt-24`}>
          <div>
            <p className="rise inline-flex items-center gap-2 rounded-full bg-white/80 px-3 py-1 text-sm font-medium text-blue-800 ring-1 ring-inset ring-blue-600/20" style={delay(0)}>
              <span className="h-1.5 w-1.5 rounded-full bg-blue-500" />
              Software para clínicas dentales y consultorios médicos
            </p>
            <h1 className="rise mt-6 text-[clamp(2.25rem,5vw,3.5rem)] font-semibold leading-[1.04] tracking-[-0.03em] [text-wrap:balance]" style={delay(1)}>
              Tu consultorio, organizado de principio a fin.
            </h1>
            <p className="rise mt-6 max-w-xl text-lg leading-relaxed text-slate-600 [text-wrap:pretty] sm:text-xl" style={delay(2)}>
              Agenda, expedientes e historial clínico en una sola plataforma. Para dentistas, médicos generales y clínicas con varias especialidades.
            </p>
            <div className="rise mt-10 flex flex-col gap-3 sm:flex-row" style={delay(3)}>
              <a href={demoHref} className={`${press} ${primary} group h-12 px-6 text-[15px]`}>
                Solicitar demo gratis
                <Icon className="h-4 w-4 transition-transform duration-150 ease-out-strong group-hover:translate-x-0.5">{arrow}</Icon>
              </a>
              <a href="#precios" className={`${press} ${secondary} h-12 px-6 text-[15px]`}>Ver planes y precios</a>
            </div>
            <ul className="rise mt-8 flex flex-wrap gap-x-6 gap-y-2 text-sm text-slate-600" style={delay(4)}>
              {highlights.map((h) => (
                <li key={h} className="flex items-center gap-2">
                  <Icon className="h-4 w-4 text-teal-600">{check}</Icon>
                  {h}
                </li>
              ))}
            </ul>
          </div>

          {/* Vista previa del producto (ilustrativa) */}
          <div aria-hidden="true" className="relative">
            <div className={`rise ${card}`} style={delay(2)}>
              <div className="overflow-hidden rounded-[18px] bg-slate-50 ring-1 ring-slate-900/5">
                <div className="flex items-center gap-1.5 border-b border-slate-900/5 bg-white px-4 py-3">
                  <span className="h-2.5 w-2.5 rounded-full bg-slate-200" />
                  <span className="h-2.5 w-2.5 rounded-full bg-slate-200" />
                  <span className="h-2.5 w-2.5 rounded-full bg-slate-200" />
                  <span className="ml-3 rounded-md bg-slate-100 px-3 py-1 text-[11px] text-slate-500">app.caresia.com/agenda</span>
                </div>
                <div className="p-4 sm:p-5">
                  <div className="grid grid-cols-3 gap-2">
                    {stats.map((s) => (
                      <div key={s.label} className="rounded-xl bg-white px-3 py-2.5 ring-1 ring-slate-900/5">
                        <p className="text-[11px] text-slate-500">{s.label}</p>
                        <p className="mt-0.5 text-lg font-semibold tabular-nums tracking-[-0.01em]">{s.value}</p>
                      </div>
                    ))}
                  </div>
                  <div className="mt-4 flex items-center justify-between">
                    <p className="text-sm font-semibold">Agenda de hoy</p>
                    <span className="flex gap-3 text-[11px] text-slate-500">
                      <span className="flex items-center gap-1"><span className="h-2 w-2 rounded-full bg-teal-500" />Dental</span>
                      <span className="flex items-center gap-1"><span className="h-2 w-2 rounded-full bg-blue-600" />Médica</span>
                    </span>
                  </div>
                  <ul className="mt-3 space-y-2">
                    {agenda.map((a, i) => (
                      <li key={a.time} className="rise flex items-center gap-3 rounded-xl bg-white px-3 py-2.5 ring-1 ring-slate-900/5 sm:gap-4 sm:px-4" style={delay(4 + i)}>
                        <span className="w-11 flex-none text-sm font-medium tabular-nums text-slate-500">{a.time}</span>
                        <span className={`h-8 w-1 flex-none rounded-full ${a.kind === "dental" ? "bg-teal-500" : "bg-blue-600"}`} />
                        <span className="min-w-0 flex-1">
                          <span className="block truncate text-sm font-medium">{a.name}</span>
                          <span className="block truncate text-[13px] text-slate-500">{a.reason}</span>
                        </span>
                      </li>
                    ))}
                  </ul>
                </div>
              </div>
            </div>
            <div className="rise absolute -bottom-7 -left-3 hidden items-center gap-3 rounded-2xl bg-white/90 py-3 pl-3 pr-5 shadow-lg shadow-slate-900/10 ring-1 ring-slate-900/5 backdrop-blur sm:flex lg:-left-8" style={delay(9)}>
              <span className="grid h-9 w-9 place-items-center rounded-full bg-teal-500 text-white"><Icon className="h-4 w-4">{check}</Icon></span>
              <span>
                <span className="block text-sm font-medium">Historial actualizado</span>
                <span className="block text-[13px] text-slate-500">Sofía Ramírez · 6 consultas</span>
              </span>
            </div>
          </div>
        </div>
      </section>

      {/* Franja de especialidades */}
      <section aria-label="Especialidades compatibles" className="border-y border-slate-900/5 bg-slate-50/70">
        <div className={`${container} py-8`}>
          <p className="text-center text-sm text-slate-500">Diseñado para consultorios de</p>
          <ul className="mt-4 flex flex-wrap justify-center gap-x-8 gap-y-3 text-[15px] font-medium text-slate-700">
            {moreSpecialties.slice(0, 7).map((s) => <li key={s}>{s}</li>)}
          </ul>
        </div>
      </section>

      {/* Funciones */}
      <section id="funciones" className="scroll-mt-16">
        <div className={`${container} py-20 lg:py-28`}>
          <div className="mx-auto max-w-2xl text-center">
            <p className={eyebrow}>Funciones</p>
            <h2 className={h2}>Todo lo que tu consultorio necesita. Nada que estorbe.</h2>
            <p className={lead}>Deja atrás las agendas de papel, las hojas de cálculo y los expedientes en carpetas.</p>
          </div>
          <ul className="mt-14 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {features.map((f) => (
              <li key={f.title} className="rounded-2xl bg-white p-6 ring-1 ring-slate-900/[0.07] transition-shadow duration-200 hover:shadow-lg hover:shadow-slate-900/5">
                <span className="grid h-11 w-11 place-items-center rounded-xl bg-gradient-to-br from-blue-50 to-teal-50 text-blue-700 ring-1 ring-inset ring-blue-600/10"><Icon>{f.icon}</Icon></span>
                <h3 className="mt-5 font-semibold tracking-[-0.01em]">{f.title}</h3>
                <p className="mt-2 text-[15px] leading-relaxed text-slate-600 [text-wrap:pretty]">{f.text}</p>
              </li>
            ))}
          </ul>
        </div>
      </section>

      {/* Especialidades: dental / medicina general */}
      <section id="especialidades" className="scroll-mt-16 border-y border-slate-900/5 bg-slate-50/70">
        <div className={`${container} py-20 lg:py-28`}>
          <div className="mx-auto max-w-2xl text-center">
            <p className={eyebrow}>Especialidades</p>
            <h2 className={h2}>Una plataforma para dentistas y médicos.</h2>
            <p className={lead}>Caresia se adapta a la forma en que trabaja tu especialidad.</p>
          </div>

          <div role="tablist" aria-label="Especialidad" className="mx-auto mt-10 flex w-fit gap-1 rounded-full bg-white p-1 ring-1 ring-slate-900/10">
            {(Object.keys(specialtyTabs) as SpecialtyKey[]).map((key) => {
              const active = key === tab;
              return (
                <button
                  key={key}
                  role="tab"
                  type="button"
                  aria-selected={active}
                  aria-controls="panel-especialidad"
                  onClick={() => setTab(key)}
                  className={`${press} h-10 px-4 text-sm sm:px-5 ${active ? "bg-slate-900 text-white" : "text-slate-600 hover:text-slate-900"}`}
                >
                  <Icon className="h-4 w-4">{specialtyTabs[key].icon}</Icon>
                  {specialtyTabs[key].label}
                </button>
              );
            })}
          </div>

          <div id="panel-especialidad" role="tabpanel" key={tab} className="mt-12 grid items-center gap-12 lg:grid-cols-2 lg:gap-20">
            <div className="rise">
              <h3 className="text-[clamp(1.5rem,3vw,2rem)] font-semibold leading-tight tracking-[-0.02em] [text-wrap:balance]">{current.title}</h3>
              <p className={lead}>{current.text}</p>
              <ul className="mt-8 space-y-4">
                {current.points.map((p) => (
                  <li key={p} className="flex gap-3 text-[15px] leading-relaxed text-slate-700">
                    <span className={`mt-0.5 grid h-5 w-5 flex-none place-items-center rounded-full text-white ${accent}`}><Icon className="h-3 w-3">{check}</Icon></span>
                    {p}
                  </li>
                ))}
              </ul>
            </div>
            <div aria-hidden="true" className={`rise ${card}`} style={delay(1)}>
              <SpecialtyPreview tab={tab} />
            </div>
          </div>

          <div className="mt-16 text-center">
            <p className="text-sm text-slate-500">También funciona para</p>
            <ul className="mx-auto mt-4 flex max-w-3xl flex-wrap justify-center gap-2">
              {moreSpecialties.slice(2).map((s) => (
                <li key={s} className="rounded-full bg-white px-4 py-2 text-sm text-slate-700 ring-1 ring-inset ring-slate-900/10">{s}</li>
              ))}
            </ul>
          </div>
        </div>
      </section>

      {/* Cómo empezar */}
      <section className="scroll-mt-16">
        <div className={`${container} py-20 lg:py-28`}>
          <div className="mx-auto max-w-2xl text-center">
            <p className={eyebrow}>Cómo empezar</p>
            <h2 className={h2}>Listo para usar en días, no en meses.</h2>
          </div>
          <ol className="mt-14 grid gap-4 md:grid-cols-3">
            {steps.map((s, i) => (
              <li key={s.title} className="relative rounded-2xl bg-white p-6 ring-1 ring-slate-900/[0.07]">
                <span className="grid h-10 w-10 place-items-center rounded-full bg-blue-600 text-sm font-semibold text-white tabular-nums">{i + 1}</span>
                <h3 className="mt-5 font-semibold tracking-[-0.01em]">{s.title}</h3>
                <p className="mt-2 text-[15px] leading-relaxed text-slate-600 [text-wrap:pretty]">{s.text}</p>
              </li>
            ))}
          </ol>
        </div>
      </section>

      {/* Precios */}
      <section id="precios" className="scroll-mt-16 border-y border-slate-900/5 bg-slate-50/70">
        <div className={`${container} py-20 lg:py-28`}>
          <div className="mx-auto max-w-2xl text-center">
            <p className={eyebrow}>Precios</p>
            <h2 className={h2}>Planes claros, sin letras chiquitas.</h2>
            <p className={lead}>Pago mensual, sin plazo forzoso. Cancela cuando quieras.</p>
          </div>
          <ul className="mt-14 grid items-start gap-4 lg:grid-cols-3">
            {plans.map((p) => (
              <li
                key={p.name}
                className={`relative flex h-full flex-col rounded-3xl p-8 ${p.featured ? "bg-slate-900 text-white shadow-2xl shadow-blue-900/20 ring-1 ring-slate-900" : "bg-white ring-1 ring-slate-900/[0.07]"}`}
              >
                {p.featured && (
                  <span className="absolute -top-3 left-8 rounded-full bg-gradient-to-r from-blue-500 to-teal-500 px-3 py-1 text-xs font-semibold text-white">Más elegido</span>
                )}
                <h3 className="text-lg font-semibold">{p.name}</h3>
                <p className={`mt-2 text-[15px] leading-relaxed ${p.featured ? "text-slate-300" : "text-slate-600"}`}>{p.blurb}</p>
                <p className="mt-6 flex items-baseline gap-2">
                  <span className="text-4xl font-semibold tracking-[-0.02em] tabular-nums">{p.price}</span>
                  {p.period && <span className={`text-sm ${p.featured ? "text-slate-400" : "text-slate-500"}`}>{p.period}</span>}
                </p>
                <a
                  href={demoHref}
                  className={`${press} mt-8 h-11 px-5 text-[15px] ${p.featured ? "bg-white text-slate-900 hover:bg-slate-100 focus-visible:ring-offset-slate-900" : primary}`}
                >
                  {p.cta}
                </a>
                <ul className="mt-8 space-y-3 text-[15px]">
                  {p.features.map((f) => (
                    <li key={f} className="flex gap-3">
                      <Icon className={`mt-0.5 h-5 w-5 flex-none ${p.featured ? "text-teal-400" : "text-blue-600"}`}>{check}</Icon>
                      <span className={p.featured ? "text-slate-200" : "text-slate-700"}>{f}</span>
                    </li>
                  ))}
                </ul>
              </li>
            ))}
          </ul>
          <p className="mt-8 text-center text-sm text-slate-500">Precios en pesos mexicanos. IVA no incluido.</p>
        </div>
      </section>

      {/* Preguntas frecuentes */}
      <section id="preguntas" className="scroll-mt-16">
        <div className={`${container} grid gap-12 py-20 lg:grid-cols-[1fr_1.6fr] lg:py-28`}>
          <div>
            <p className={eyebrow}>Preguntas frecuentes</p>
            <h2 className={h2}>¿Tienes dudas?</h2>
            <p className={lead}>
              Si no encuentras tu respuesta, escríbenos a{" "}
              <a href={`mailto:${contactEmail}`} className="font-medium text-blue-700 underline-offset-4 hover:underline">{contactEmail}</a>.
            </p>
          </div>
          <div className="divide-y divide-slate-900/10 border-y border-slate-900/10">
            {faqs.map((f) => (
              <details key={f.q} className="group py-5">
                <summary className="flex cursor-pointer list-none items-center justify-between gap-6 font-medium [&::-webkit-details-marker]:hidden">
                  {f.q}
                  <span className="grid h-7 w-7 flex-none place-items-center rounded-full ring-1 ring-slate-900/10 transition-transform duration-200 group-open:rotate-45">
                    <Icon className="h-3.5 w-3.5"><path d="M12 5v14M5 12h14" /></Icon>
                  </span>
                </summary>
                <p className="mt-3 pr-12 text-[15px] leading-relaxed text-slate-600 [text-wrap:pretty]">{f.a}</p>
              </details>
            ))}
          </div>
        </div>
      </section>

      {/* CTA final */}
      <section id="contacto" className="scroll-mt-16 px-4 pb-20 sm:px-6 lg:pb-28">
        <div className="relative isolate mx-auto max-w-6xl overflow-hidden rounded-[28px] bg-slate-900 px-6 py-16 text-center sm:px-12 lg:py-20">
          <div aria-hidden="true" className="absolute inset-0 -z-10 bg-[radial-gradient(50%_80%_at_30%_0%,rgba(59,130,246,0.35),transparent),radial-gradient(40%_70%_at_80%_100%,rgba(45,212,191,0.25),transparent)]" />
          <h2 className="mx-auto max-w-2xl text-[clamp(1.875rem,4vw,2.75rem)] font-semibold leading-[1.1] tracking-[-0.025em] text-white [text-wrap:balance]">
            Dedica tu tiempo a tus pacientes, no al papeleo.
          </h2>
          <p className="mx-auto mt-4 max-w-xl text-lg leading-relaxed text-slate-300 [text-wrap:pretty]">
            Agenda una demostración gratuita y descubre cómo Caresia se adapta a tu consultorio dental o médico.
          </p>
          <div className="mt-10 flex flex-col justify-center gap-3 sm:flex-row">
            <a href={demoHref} className={`${press} h-12 bg-white px-6 text-[15px] text-slate-900 hover:bg-slate-100 focus-visible:ring-offset-slate-900`}>
              Solicitar demo gratis
              <Icon className="h-4 w-4">{arrow}</Icon>
            </a>
            <a href={loginPath} className={`${press} h-12 bg-white/10 px-6 text-[15px] text-white hover:bg-white/20 focus-visible:ring-offset-slate-900`}>Ya soy cliente</a>
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-slate-900/5 bg-slate-50/70">
        <div className={`${container} grid gap-10 py-14 sm:grid-cols-2 lg:grid-cols-[2fr_1fr_1fr_1fr]`}>
          <div>
            <Wordmark />
            <p className="mt-4 max-w-xs text-sm leading-relaxed text-slate-600">
              Software de gestión para clínicas dentales y consultorios médicos.
            </p>
          </div>
          <div>
            <p className="text-sm font-semibold">Producto</p>
            <ul className="mt-4 space-y-3 text-sm text-slate-600">
              <li><a href="#funciones" className="hover:text-slate-900">Funciones</a></li>
              <li><a href="#especialidades" className="hover:text-slate-900">Especialidades</a></li>
              <li><a href="#precios" className="hover:text-slate-900">Precios</a></li>
            </ul>
          </div>
          <div>
            <p className="text-sm font-semibold">Soporte</p>
            <ul className="mt-4 space-y-3 text-sm text-slate-600">
              <li><a href="#preguntas" className="hover:text-slate-900">Preguntas frecuentes</a></li>
              <li><a href={`mailto:${contactEmail}`} className="hover:text-slate-900">Contacto</a></li>
            </ul>
          </div>
          <div>
            <p className="text-sm font-semibold">Cuenta</p>
            <ul className="mt-4 space-y-3 text-sm text-slate-600">
              <li><a href={loginPath} className="hover:text-slate-900">Iniciar sesión</a></li>
              <li><a href={demoHref} className="hover:text-slate-900">Solicitar demo</a></li>
            </ul>
          </div>
        </div>
        <div className={`${container} border-t border-slate-900/5 py-6 text-sm text-slate-500`}>
          © {new Date().getFullYear()} Caresia. Todos los derechos reservados.
        </div>
      </footer>
    </div>
  );
};

export default LandingPage;
