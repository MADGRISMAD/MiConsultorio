// ---------------------------------------------------------------------------
// Datos comerciales de la landing: edita aquí precios, contacto y textos.
// ---------------------------------------------------------------------------

export const loginPath = "/api/auth/signin";
export const contactEmail = "contacto@caresia.com";
export const demoHref = `mailto:${contactEmail}?subject=${encodeURIComponent("Quiero una demo de Caresia")}`;

export const plans = [
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

export const faqs = [
  { q: "¿Necesito instalar algo?", a: "No. Caresia funciona desde el navegador en computadora, tableta o celular. Solo necesitas conexión a internet." },
  { q: "¿Sirve igual para una clínica dental que para medicina general?", a: "Sí. La agenda, los expedientes y el historial clínico funcionan igual de bien en odontología, medicina general y cualquier especialidad que atiende con cita." },
  { q: "¿Mi recepcionista puede agendar sin ver los expedientes?", a: "Sí. Cada usuario tiene permisos por rol: recepción puede administrar la agenda sin acceso a la información clínica." },
  { q: "¿Cómo se protege la información de mis pacientes?", a: "El acceso requiere usuario y contraseña, cada clínica solo ve a sus propios pacientes y los permisos por rol limitan quién consulta la información clínica." },
  { q: "¿Me ayudan a empezar?", a: "Sí. En los planes Clínica y Empresarial te acompañamos en el alta, la configuración de usuarios y la capacitación de tu equipo." },
  { q: "¿Hay plazo forzoso?", a: "No. Los planes son mensuales y puedes cancelar cuando quieras." },
];

export const specialties = ["Odontología", "Medicina general", "Pediatría", "Medicina interna", "Fisioterapia", "Nutrición", "Psicología", "Dermatología", "Ginecología", "Ortopedia"];
