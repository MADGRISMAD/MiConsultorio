/**
 * Ejemplos para empezar la página pública: preguntas frecuentes y motivos para elegir el consultorio, por giro.
 * Se cargan como borrador en el editor; la clínica los revisa, los cambia y decide qué publicar.
 */
export interface Faq {
  q: string;
  a: string;
}

const COMMON: Faq[] = [
  { q: '¿Qué pasa si necesito cambiar mi cita?', a: 'Desde el enlace de tu cita puedes cancelarla y reservar otro horario. Si falta poco tiempo para tu cita, comunícate con el consultorio.' },
  { q: '¿Cómo agendo mi cita?', a: 'Elige al especialista y el horario que más te convenga con el botón «Agendar cita». Te llega una confirmación y un recordatorio.' }
];

const BY_GIRO: Record<string, { faqs: Faq[]; highlights: string[] }> = {
  DENTAL: {
    faqs: [
      { q: '¿Aceptan seguros dentales?', a: 'Consulta en la sección de seguros qué aseguradoras aceptamos. Si tienes dudas sobre tu cobertura, escríbenos y te orientamos.' },
      { q: '¿Atienden niños?', a: 'Sí, atendemos a pacientes de todas las edades. Cuéntanos la edad del niño al agendar para prepararle la consulta.' },
      { q: 'Llevo años sin ir al dentista, ¿por dónde empiezo?', a: 'Empieza por una valoración: revisamos tu boca, te explicamos qué encontramos y te damos un presupuesto claro antes de hacer cualquier tratamiento.' },
      { q: '¿Ofrecen planes de pago?', a: 'Para tratamientos largos podemos dividir el pago en partes. Pregunta por las opciones al recibir tu presupuesto.' }
    ],
    highlights: ['Valoración con diagnóstico y presupuesto por escrito', 'Atención para toda la familia', 'Materiales y esterilización con protocolos de higiene', 'Planes de pago en tratamientos largos']
  },
  PSYCHOLOGY: {
    faqs: [
      { q: '¿Lo que platico es confidencial?', a: 'Sí. Lo que hablas en terapia es confidencial y solo se comparte con tu autorización, salvo las excepciones que marca la ley, como un riesgo para tu vida o la de alguien más.' },
      { q: '¿Atienden en línea?', a: 'Sí, ofrecemos sesiones por videollamada. Al agendar eliges si prefieres presencial o en línea.' },
      { q: '¿Cómo es la primera sesión?', a: 'Es un primer encuentro para conocerte, entender qué te trae y acordar cómo trabajaremos. No necesitas preparar nada.' },
      { q: '¿Cada cuánto son las sesiones?', a: 'Lo habitual es una sesión por semana al inicio; se ajusta a tu proceso. También hay paquetes de sesiones.' }
    ],
    highlights: ['Espacio confidencial y sin juicios', 'Sesiones presenciales o por videollamada', 'Seguimiento con objetivos claros', 'Horarios flexibles']
  },
  NUTRITION: {
    faqs: [
      { q: '¿Necesito llevar estudios a la primera consulta?', a: 'Si tienes análisis recientes, tráelos; ayudan mucho. Si no, no te preocupes: en la valoración revisamos tus hábitos y medidas.' },
      { q: '¿Cómo es el plan de alimentación?', a: 'Es personalizado: considera tus gustos, tu rutina y tu objetivo. Lo recibes por escrito y con una lista del súper.' },
      { q: '¿Cada cuánto son las consultas de seguimiento?', a: 'Normalmente cada 2 a 4 semanas, según tu objetivo. Allí revisamos tu avance y ajustamos el plan.' },
      { q: '¿Atienden en línea?', a: 'Sí, puedes tener consultas de seguimiento por videollamada.' }
    ],
    highlights: ['Plan de alimentación hecho para ti', 'Cálculo de calorías y seguimiento de medidas', 'Lista del súper incluida', 'Consultas presenciales o en línea']
  },
  PHYSIOTHERAPY: {
    faqs: [
      { q: '¿Necesito referencia médica?', a: 'No es obligatoria para empezar. En la valoración revisamos tu caso y, si hace falta, te sugerimos estudios o consultar a otro especialista.' },
      { q: '¿Cuántas sesiones necesitaré?', a: 'Depende de tu lesión y de cómo respondas. Después de la valoración te damos un estimado y lo vamos ajustando.' },
      { q: '¿Qué debo llevar a mi primera sesión?', a: 'Ropa cómoda y, si tienes, estudios o indicaciones de tu médico.' },
      { q: '¿Ofrecen paquetes de sesiones?', a: 'Sí, hay paquetes con un mejor precio por sesión. Pregunta por las opciones.' }
    ],
    highlights: ['Valoración inicial con plan de tratamiento', 'Ejercicios para hacer en casa', 'Seguimiento de tu evolución', 'Paquetes de sesiones']
  },
  GENERAL_MEDICAL: {
    faqs: [
      { q: '¿Aceptan seguros médicos?', a: 'Consulta en la sección de seguros qué aseguradoras aceptamos. Si tienes dudas, escríbenos antes de tu cita.' },
      { q: '¿Qué debo llevar a mi primera consulta?', a: 'Una identificación y, si los tienes, tus estudios y la lista de medicamentos que tomas.' },
      { q: '¿Dan receta y facturan?', a: 'Sí, entregamos receta cuando el tratamiento la requiere y podemos facturar si nos pides tus datos fiscales.' }
    ],
    highlights: ['Consulta sin prisas', 'Expediente digital de tu historial', 'Receta y seguimiento en un solo lugar', 'Citas en línea las 24 horas']
  },
  PEDIATRICS: {
    faqs: [
      { q: '¿Desde qué edad atienden?', a: 'Atendemos desde recién nacidos hasta la adolescencia.' },
      { q: '¿Llevan el control de vacunas?', a: 'Sí, llevamos el carnet de vacunación y te avisamos cuando toca la siguiente dosis.' },
      { q: '¿Qué hago si mi hijo se enferma fuera de horario?', a: 'Escríbenos por WhatsApp y te orientamos. Si es una urgencia, acude al servicio de urgencias más cercano.' }
    ],
    highlights: ['Control del crecimiento y vacunación', 'Trato amable con los niños', 'Recordatorios de vacunas y citas', 'Orientación por WhatsApp']
  },
  VETERINARY: {
    faqs: [
      { q: '¿Atienden urgencias?', a: 'Escríbenos o llámanos y te decimos cómo atender a tu mascota. Si es una emergencia grave, acude de inmediato al servicio de urgencias veterinario más cercano.' },
      { q: '¿Llevan el control de vacunas y desparasitación?', a: 'Sí, registramos cada aplicación y te recordamos cuándo toca la siguiente.' },
      { q: '¿Qué debo llevar a la primera consulta?', a: 'Su cartilla de vacunación, si la tiene, y cualquier estudio o medicamento que ya esté tomando.' }
    ],
    highlights: ['Historial clínico de tu mascota', 'Recordatorios de vacunas y desparasitación', 'Atención para perros, gatos y más', 'Venta de alimento y productos']
  },
  DERMATOLOGY: {
    faqs: [
      { q: '¿Atienden en línea?', a: 'Para algunos seguimientos sí, con fotos y videollamada. La primera valoración suele ser presencial.' },
      { q: '¿Qué debo llevar a la consulta?', a: 'Sin maquillaje en la zona a revisar, tus productos actuales y, si los tienes, estudios previos.' }
    ],
    highlights: ['Valoración de piel, cabello y uñas', 'Plan de tratamiento claro', 'Seguimiento de tu evolución']
  },
  GYNECOLOGY: {
    faqs: [
      { q: '¿Cada cuánto debo hacerme una revisión?', a: 'Se recomienda una revisión al año, o antes si tienes molestias. Tu médica te indicará la frecuencia ideal.' },
      { q: '¿Qué debo llevar a mi primera cita?', a: 'La fecha de tu última menstruación y, si los tienes, estudios previos.' }
    ],
    highlights: ['Atención respetuosa y confidencial', 'Seguimiento de embarazo y revisiones anuales', 'Recordatorios de tus estudios']
  },
  INTERNAL_MEDICINE: {
    faqs: [
      { q: '¿Atienden padecimientos crónicos?', a: 'Sí, damos seguimiento a diabetes, hipertensión y otros padecimientos crónicos, con control de tus medicamentos.' },
      { q: '¿Aceptan seguros médicos?', a: 'Consulta en la sección de seguros qué aseguradoras aceptamos.' }
    ],
    highlights: ['Seguimiento de padecimientos crónicos', 'Control de medicamentos y estudios', 'Expediente digital']
  },
  ORTHOPEDICS: {
    faqs: [{ q: '¿Qué debo llevar a mi consulta?', a: 'Tus estudios de imagen, si los tienes, y ropa cómoda para la exploración.' }],
    highlights: ['Valoración de huesos y articulaciones', 'Plan de rehabilitación', 'Seguimiento de tu recuperación']
  },
  CHIROPRACTIC: {
    faqs: [{ q: '¿Cuántas sesiones necesitaré?', a: 'Depende de tu caso. Después de la valoración te damos un estimado y lo ajustamos según tu respuesta.' }],
    highlights: ['Valoración postural', 'Plan de tratamiento personalizado', 'Paquetes de sesiones']
  }
};

/** Ejemplos para los giros de la clínica: primero los de cada giro y al final los generales; sin repetir. */
export function defaultsFor(kinds: string[]): { faqs: Faq[]; highlights: string[] } {
  const faqs: Faq[] = [];
  const highlights: string[] = [];
  for (const k of kinds) {
    for (const f of BY_GIRO[k]?.faqs ?? []) if (!faqs.some((x) => x.q === f.q)) faqs.push(f);
    for (const h of BY_GIRO[k]?.highlights ?? []) if (!highlights.includes(h)) highlights.push(h);
  }
  for (const f of COMMON) if (!faqs.some((x) => x.q === f.q)) faqs.push(f);
  return { faqs: faqs.slice(0, 12), highlights: highlights.slice(0, 6) };
}
