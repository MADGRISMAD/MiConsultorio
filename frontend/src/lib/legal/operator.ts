// Datos legales del OPERADOR de Caresia (quien presta el servicio), usados en el aviso de privacidad y los términos.
//
// BORRADOR: no hay datos reales de la empresa en el repositorio y no se inventan. Cada valor entre corchetes
// («[RAZÓN SOCIAL]») es un pendiente: se muestra resaltado en las páginas públicas hasta que se complete aquí.
// Ver docs/AVISOS-REVISION-LEGAL.md.
export const OPERATOR = {
  /** Denominación o razón social de quien opera Caresia. */
  razonSocial: '[RAZÓN SOCIAL]',
  /** Domicilio completo para oír y recibir notificaciones. */
  domicilio: '[DOMICILIO COMPLETO DEL RESPONSABLE]',
  /** Correo del área o persona que atiende privacidad y derechos ARCO de la plataforma. */
  correoPrivacidad: '[CORREO DE PRIVACIDAD]',
  /** Teléfono de contacto (opcional). */
  telefono: '[TELÉFONO DE CONTACTO]',
  /** Ciudad cuyos tribunales serán competentes (términos). */
  jurisdiccion: '[CIUDAD Y ESTADO DE LA JURISDICCIÓN]',
  /** Fecha de la última actualización de los textos. */
  actualizado: '[FECHA DE ÚLTIMA ACTUALIZACIÓN]'
} as const;

export const isPending = (v: string) => v.startsWith('[') && v.endsWith(']');
