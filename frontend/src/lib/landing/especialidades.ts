// Páginas por especialidad (/software-dental, /software-medico…). Todo el texto sale de lo que la portada ya afirma
// (src/lib/landing/Specialties.svelte, data.ts); aquí solo se ordena por giro. No agregar funciones que el producto no tenga.

export interface Especialidad {
  /** ruta pública */
  path: string;
  /** nombre corto del giro, para migas y enlaces */
  tab: string;
  title: string;
  description: string;
  h1: string;
  /** quién es el lector, en una frase */
  lead: string;
  /** frase de la portada para este giro */
  text: string;
  /** funciones distintivas del giro (las mismas fichas de la portada) */
  chips: string[];
  /** artículos del blog relacionados */
  articles: { slug: string; title: string }[];
}

export const especialidades: Especialidad[] = [
  {
    path: '/software-dental',
    tab: 'Dental',
    title: 'Software para clínicas dentales y consultorios · Caresia',
    description: 'Agenda, expediente con odontograma, planes de tratamiento por fases y cobros para tu clínica dental en México. Desde $499 al mes.',
    h1: 'Software para clínicas dentales',
    lead: 'Para odontólogos y clínicas dentales que atienden citas cortas y seguidas, y tratamientos de varias sesiones.',
    text: 'Citas cortas y seguidas, y tratamientos de varias sesiones.',
    chips: ['Odontograma con historial de versiones', 'Planes por fases, con presupuesto y firma'],
    articles: [
      { slug: 'software-para-clinicas-dentales-control', title: "Software para clínicas dentales que da control" },
      { slug: 'como-controlar-cobros-consultorio', title: "Cómo controlar cobros en consultorio sin perder tiempo" }
    ]
  },
  {
    path: '/software-medico',
    tab: 'Médico',
    title: 'Software para consultorio médico · Caresia',
    description: 'Agenda, expediente con diagnósticos CIE-10 y signos vitales, y recetas con alertas de alergias y dosis para tu consultorio médico en México. Desde $499 al mes.',
    h1: 'Software para consultorio médico',
    lead: 'Para médicos generales, internistas y consultorios que quieren conocer a su paciente antes de que entre y darle seguimiento consulta tras consulta.',
    text: 'Conoce a tu paciente antes de que entre y dale seguimiento consulta tras consulta.',
    chips: ['Diagnósticos CIE-10 y signos vitales', 'Recetas con alertas de alergias y dosis'],
    articles: [
      { slug: 'requisitos-expediente-clinico-digital-mexico', title: "Requisitos del expediente clínico digital en México" },
      { slug: 'software-gestion-clinica', title: "Software de gestión clínica para operar mejor" }
    ]
  },
  {
    path: '/software-nutricion',
    tab: 'Nutrición',
    title: 'Software para nutriólogos y consulta nutricional · Caresia',
    description: 'Plan de 7 días con macronutrientes, seguimiento de peso y citas, agenda y cobros para tu consulta de nutrición. Desde $499 al mes.',
    h1: 'Software para nutriólogos',
    lead: 'Para nutriólogos que van de las medidas al menú de la semana, con el cálculo hecho por ellos o con ayuda de IA.',
    text: 'De las medidas al menú de la semana, con el cálculo hecho por ti o con ayuda de IA.',
    chips: ['Plan de 7 días con macronutrientes', 'Seguimiento de peso y citas'],
    articles: [
      { slug: 'programa-para-consulta-nutricional', title: "Programa para consulta nutricional: qué debe incluir" },
      { slug: 'como-reducir-inasistencias-de-pacientes-en-clinica', title: "Cómo reducir inasistencias de pacientes en clínica" }
    ]
  },
  {
    path: '/software-veterinaria',
    tab: 'Veterinaria',
    title: 'Software para clínicas veterinarias · Caresia',
    description: 'Varias mascotas por propietario, carnet de vacunación y desparasitación, agenda y cobros para tu veterinaria en México. Desde $499 al mes.',
    h1: 'Software para clínicas veterinarias',
    lead: 'Para veterinarias y clínicas de mascotas que ponen primero al propietario y a todas sus mascotas juntas, aunque se llamen igual.',
    text: 'El propietario primero y todas sus mascotas juntas, aunque se llamen igual.',
    chips: ['Varias mascotas por propietario', 'Carnet de vacunación y desparasitación'],
    articles: [
      { slug: 'sistema-para-veterinaria-pequena', title: "Sistema para veterinaria pequeña que sí ayuda" },
      { slug: 'coordinar-agenda-varios-especialistas', title: "Cómo coordinar agenda de varios especialistas" }
    ]
  }
];
