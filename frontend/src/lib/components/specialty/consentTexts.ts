import type { ConsentKind } from '$lib/types/specialty';

export const CONSENT_KINDS: { v: ConsentKind; label: string }[] = [
  { v: 'procedimiento', label: 'Procedimiento' },
  { v: 'privacidad', label: 'Aviso de privacidad (acuse)' },
  { v: 'telemedicina', label: 'Telemedicina' },
  { v: 'psicologia', label: 'Atención psicológica' },
  { v: 'animal', label: 'Autorización del propietario (animal)' },
  { v: 'plan_tratamiento', label: 'Plan de tratamiento' }
];

/** Suggested texts: editable before signing; the exact text shown is what gets stored. */
export function consentText(kind: ConsentKind, ctx: { patient: string; clinic: string; animal: boolean }): string {
  const who = ctx.animal ? `del animal ${ctx.patient}` : `de ${ctx.patient}`;
  switch (kind) {
    case 'procedimiento':
      return `Yo, quien firma, declaro que el profesional me explicó en términos claros el procedimiento que se realizará ${who}: en qué consiste, para qué sirve, sus beneficios esperados, sus riesgos y posibles complicaciones, y las alternativas disponibles, incluida la de no realizarlo. Tuve oportunidad de hacer preguntas y fueron respondidas. Entiendo que puedo retirar mi consentimiento antes del procedimiento. Autorizo su realización en ${ctx.clinic}.\n\nProcedimiento: `;
    case 'privacidad':
      return `Recibí el aviso de privacidad de ${ctx.clinic}. Entiendo que mis datos personales, incluidos los de salud (datos sensibles), serán tratados para brindar atención médica, integrar el expediente clínico conforme a la NOM-004-SSA3-2012 y cumplir obligaciones legales, y que puedo ejercer mis derechos de acceso, rectificación, cancelación y oposición (ARCO) con el responsable indicado en el aviso.`;
    case 'telemedicina':
      return `Acepto recibir atención a distancia por medios electrónicos de ${ctx.clinic} para ${ctx.patient}. Entiendo sus limitaciones (no sustituye la exploración física cuando esta es necesaria), que puede suspenderse y requerir una consulta presencial, y que la información se maneja con confidencialidad conforme a la ley.`;
    case 'psicologia':
      return `Acepto recibir atención psicológica en ${ctx.clinic}. Me explicaron el encuadre del servicio (duración, frecuencia, honorarios y cancelaciones), la confidencialidad de lo que se trate y sus límites (riesgo grave para mí o para terceros, o requerimiento de autoridad competente), y que puedo suspender el proceso cuando lo decida.`;
    case 'animal':
      return `Como propietario o responsable del animal ${ctx.patient}, autorizo al Médico Veterinario de ${ctx.clinic} a realizar los estudios, tratamientos y procedimientos que considere necesarios, habiendo sido informado de sus riesgos, costos aproximados y alternativas, incluida la anestesia o sedación cuando aplique. Me hago responsable de los honorarios y de las indicaciones posteriores.\n\nProcedimiento: `;
    case 'plan_tratamiento':
      return `Acepto el plan de tratamiento propuesto para ${ctx.patient}, sus fases y costos, y entiendo que puede modificarse con mi autorización.`;
  }
}
