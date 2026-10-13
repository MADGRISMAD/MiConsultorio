// Datos legales del OPERADOR de Caresia (quien presta el servicio), usados en el aviso de privacidad y los términos.
//
// Cada valor entre corchetes («[RAZÓN SOCIAL]») es un pendiente: no se inventa y se muestra resaltado en las páginas
// públicas (con un aviso arriba) hasta que se complete aquí. Domicilio, correo y jurisdicción los dio el operador.
// Ver docs/AVISOS-REVISION-LEGAL.md.
export const OPERATOR = {
  /** Denominación o razón social de quien opera Caresia. */
  razonSocial: '[RAZÓN SOCIAL]',
  /** Domicilio completo para oír y recibir notificaciones. */
  domicilio: 'Calle 20, Privada Fresnos, interior 12, Casas Beta, C.P. 22245, Tijuana, Baja California, México',
  /** Correo del área o persona que atiende privacidad y derechos ARCO de la plataforma. */
  correoPrivacidad: 'caresia-info@caresia.mx',
  /** Teléfono de contacto (opcional). */
  telefono: '[TELÉFONO DE CONTACTO]',
  /** Ciudad cuyos tribunales serán competentes (términos). */
  jurisdiccion: 'Tijuana, Baja California',
  /** Fecha de la última actualización de los textos. */
  actualizado: '13 de octubre de 2026'
} as const;

export const isPending = (v: string) => v.startsWith('[') && v.endsWith(']');
