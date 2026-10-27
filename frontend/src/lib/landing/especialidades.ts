// Páginas por especialidad (/software-dental, /software-medico…). Todo el texto sale de lo que la portada, los planes y los
// términos ya afirman, o de funciones que existen en la aplicación (cada una con su pantalla). No agregar nada que el producto no tenga.

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
  /** otras funciones propias del giro que existen en la aplicación */
  extras: { titulo: string; texto: string }[];
  /** preguntas propias del giro; las respuestas salen de los planes, los términos y las fichas de la portada */
  faqs: { q: string; a: string }[];
  /** artículos del blog relacionados */
  articles: { slug: string; title: string }[];
}

/** Aviso que va en todas las páginas: es lo que dicen los Términos (src/routes/terminos/+page.svelte). */
export const avisoNormas =
  'Caresia no está certificada bajo la NOM-024-SSA3-2012 y no firma electrónicamente notas ni recetas. Cada nota queda guardada sin poder alterarse y los cambios se agregan como adendas.';

const precio = { q: '¿Cuánto cuesta?', a: 'Desde $499 al mes (más IVA) en el plan Básico. Crecimiento cuesta $999 y Pro $1,899.' };
const cobros = { q: '¿Puedo cobrar desde el mismo sistema?', a: 'Sí, desde el plan Crecimiento: punto de venta, caja con corte y facturación CFDI. La cuenta pasa sola a caja al terminar la consulta.' };

export const especialidades: Especialidad[] = [
  {
    path: '/software-dental',
    tab: 'Dental',
    title: 'Software dental con agenda para clínicas en México · Caresia',
    description: 'Agenda y expediente dental con odontograma, periodontograma y planes por fases con presupuesto, para clínicas dentales en México. Planes en pesos.',
    h1: 'Software para clínicas dentales en México',
    lead: 'Para odontólogos y clínicas dentales que atienden citas cortas y seguidas, y tratamientos de varias sesiones.',
    text: 'Citas cortas y seguidas, y tratamientos de varias sesiones.',
    chips: ['Odontograma con historial de versiones', 'Planes por fases, con presupuesto firmado por el paciente en pantalla'],
    extras: [
      { titulo: 'Periodontograma', texto: 'Registra por diente la profundidad de sondaje, el sangrado y la placa, por vestibular y lingual, y guárdalo en el expediente.' },
      { titulo: 'Agenda para citas cortas y seguidas', texto: 'Reserva en línea y recordatorios por correo para que los pacientes lleguen a su cita; los tratamientos de varias sesiones viven en un mismo plan.' }
    ],
    faqs: [
      { q: '¿Incluye odontograma?', a: 'Sí. El odontograma guarda el historial de versiones, así que puedes ver cómo cambió la boca del paciente de una visita a otra.' },
      { q: '¿Puedo presentar un presupuesto por fases?', a: 'Sí. Armas planes por fases con presupuesto, y el paciente lo firma en pantalla.' },
      cobros,
      precio
    ],
    articles: [
      { slug: 'software-para-clinicas-dentales-control', title: 'Software para clínicas dentales que da control' },
      { slug: 'como-controlar-cobros-consultorio', title: 'Cómo controlar cobros en consultorio sin perder tiempo' }
    ]
  },
  {
    path: '/software-medico',
    tab: 'Médico',
    title: 'Software para consultorio médico en México · Caresia',
    description: 'Agenda, expediente con diagnósticos CIE-10 y signos vitales, y recetas con alertas de alergias y dosis, para consultorios médicos en México. Planes en pesos.',
    h1: 'Software para consultorio médico en México',
    lead: 'Para médicos generales, internistas y consultorios que quieren conocer a su paciente antes de que entre y darle seguimiento consulta tras consulta.',
    text: 'Conoce a tu paciente antes de que entre y dale seguimiento consulta tras consulta.',
    chips: ['Diagnósticos CIE-10 y signos vitales', 'Recetas con alertas de alergias y dosis'],
    extras: [
      { titulo: 'Conoce al paciente antes de que entre', texto: 'El expediente reúne antecedentes, medicación crónica y alertas de alergias e interacciones para que llegues a la consulta sabiendo lo importante.' },
      { titulo: 'Notas que no se alteran', texto: 'Cada nota queda guardada tal cual y los cambios se agregan como adendas, como pide la NOM-004.' }
    ],
    faqs: [
      { q: '¿Qué diagnósticos puedo registrar?', a: 'Diagnósticos con la clasificación CIE-10, junto con los signos vitales de cada consulta.' },
      { q: '¿Las recetas avisan de alergias y dosis?', a: 'Sí. Las recetas muestran alertas de alergias y de dosis. La receta se arma en el sistema; Caresia no firma electrónicamente notas ni recetas.' },
      { q: '¿Cumple con la NOM-004?', a: 'Está pensada para ella: las notas no se alteran después de guardarse y los cambios se agregan como adendas. Caresia no está certificada bajo la NOM-024-SSA3-2012, así que tu responsable sanitario valida lo que aplica a tu consultorio.' },
      precio
    ],
    articles: [
      { slug: 'requisitos-expediente-clinico-digital-mexico', title: 'Requisitos del expediente clínico digital en México' },
      { slug: 'software-gestion-clinica', title: 'Software de gestión clínica para operar mejor' }
    ]
  },
  {
    path: '/software-nutricion',
    tab: 'Nutrición',
    title: 'Software para nutriólogos en México · Caresia',
    description: 'Agenda y expediente para nutrición: plan de 7 días con macronutrientes, catálogo de equivalentes SMAE y seguimiento de peso y citas. Planes en pesos.',
    h1: 'Software para nutriólogos en México',
    lead: 'Para nutriólogos que van de las medidas al menú de la semana, con el cálculo hecho por ellos (y apoyo de IA en los planes Crecimiento y Pro).',
    text: 'De las medidas al menú de la semana, con el cálculo hecho por ti o, en Crecimiento y Pro, con ayuda de IA.',
    chips: ['Plan de 7 días con macronutrientes', 'Seguimiento de peso y citas'],
    extras: [
      { titulo: 'Equivalentes SMAE', texto: 'Consulta cada alimento con la porción que equivale a un equivalente de su grupo y agrégalo al plan.' },
      { titulo: 'Seguimiento de peso y citas', texto: 'Mide la evolución del paciente consulta tras consulta y agenda el siguiente control sin salir del expediente.' }
    ],
    faqs: [
      { q: '¿Cómo armo el plan de alimentación?', a: 'Con un plan de 7 días con macronutrientes. El cálculo lo haces tú; si quieres apoyo, la IA ayuda a armar un borrador y tú siempre revisas y ajustas el resultado.' },
      { q: '¿Qué plan incluye la ayuda de IA?', a: 'Crecimiento (500 usos al mes) y Pro (1,500). El plan Básico no incluye asistente de IA; el plan de 7 días y el seguimiento sí están desde el Básico.' },
      { q: '¿Puedo dar seguimiento al peso?', a: 'Sí. El expediente guarda el seguimiento de peso y de citas del paciente.' },
      precio
    ],
    articles: [
      { slug: 'programa-para-consulta-nutricional', title: 'Programa para consulta nutricional: qué debe incluir' },
      { slug: 'como-reducir-inasistencias-de-pacientes-en-clinica', title: 'Cómo reducir inasistencias de pacientes en clínica' }
    ]
  },
  {
    path: '/software-veterinaria',
    tab: 'Veterinaria',
    title: 'Software para clínicas veterinarias en México · Caresia',
    description: 'Agenda y expediente veterinario con varias mascotas por propietario, carnet de vacunación y desparasitación, para clínicas veterinarias en México. Planes en pesos.',
    h1: 'Software para clínicas veterinarias en México',
    lead: 'Para veterinarias y clínicas de mascotas que ponen primero al propietario y a todas sus mascotas juntas, aunque se llamen igual.',
    text: 'El propietario primero y todas sus mascotas juntas, aunque se llamen igual.',
    chips: ['Varias mascotas por propietario', 'Carnet de vacunación y desparasitación'],
    extras: [
      { titulo: 'El propietario primero', texto: 'Todas las mascotas de una persona juntas en su ficha, aunque se llamen igual, con su historial y sus citas.' },
      { titulo: 'Vacunas y desparasitación a la vista', texto: 'El carnet de vacunación y desparasitación de cada mascota queda en su expediente para saber cuándo toca la siguiente.' }
    ],
    faqs: [
      { q: '¿Puedo registrar varias mascotas de un mismo propietario?', a: 'Sí. El propietario va primero y todas sus mascotas quedan juntas, aunque se llamen igual.' },
      { q: '¿Hay carnet de vacunación?', a: 'Sí. Cada mascota tiene su carnet de vacunación y desparasitación en el expediente.' },
      { q: '¿Puedo cobrar desde el mismo sistema?', a: 'Sí, desde el plan Crecimiento: punto de venta, caja con corte, inventario y facturación CFDI.' },
      precio
    ],
    articles: [
      { slug: 'sistema-para-veterinaria-pequena', title: 'Sistema para veterinaria pequeña que sí ayuda' },
      { slug: 'coordinar-agenda-varios-especialistas', title: 'Cómo coordinar agenda de varios especialistas' }
    ]
  }
];
