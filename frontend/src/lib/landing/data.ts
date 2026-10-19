// ---------------------------------------------------------------------------
// Datos comerciales de la landing: edita aquí precios, contacto y textos.
// ---------------------------------------------------------------------------

export const loginPath = '/login';
export const contactEmail = 'caresia-info@caresia.mx';
/** Registro: todos los botones de demo y de prueba llevan aquí. La cuenta empieza siempre en el plan Básico. */
export const registerHref = '/register';
/** Registro con el plan de la tarjeta que se pulsó: llega al equipo por correo para activárselo. */
export const registerFor = (plan: string) => `/register?plan=${plan}`;

export const plans = [
  {
    id: 'basico',
    name: 'Básico',
    highlights: ['2 especialistas y 1 recepcionista', '2 giros y 1 sucursal', 'Agenda, expedientes y recetas', 'Sin asistente de IA', '2 GB de archivos · Soporte por correo'],
    price: '$499',
    period: 'MXN / mes',
    tagline: 'Para el profesional independiente',
    magic: 0,
    blurb: 'Ordena tu consultorio: agenda, expedientes y recordatorios en un solo lugar, para ti y tu pequeño equipo.',
    includes: '',
    features: [
      '2 especialistas y 1 recepcionista (más el administrador)',
      '2 giros y 1 sucursal',
      'Agenda ilimitada, reserva en línea y recordatorios por correo',
      'Expedientes, recetas y herramientas de tu especialidad',
      'Alertas de alergias e interacciones y medicación crónica',
      'Portal del paciente y consentimientos firmados',
      'Página pública del consultorio y encuesta de satisfacción',
      'Panel de indicadores y tus citas en Google Calendar',
      '2 GB de archivos · Soporte por correo'
    ],
    cta: 'Solicitar demo',
    featured: false
  },
  {
    id: 'crecimiento',
    name: 'Crecimiento',
    highlights: ['Hasta 5 especialistas, 2 recepcionistas y 2 cajeros', 'Hasta 3 giros y 1 sucursal', 'Cobros: punto de venta, caja y CFDI', 'Asistente de IA: 500 usos al mes', '20 GB · Soporte prioritario y alta asistida'],
    price: '$999',
    period: 'MXN / mes',
    tagline: 'Para clínicas que crecen',
    magic: 500,
    blurb: 'Suma a tu equipo y empieza a cobrar desde el sistema: de la consulta a la caja, sin capturar dos veces.',
    includes: 'Todo lo del plan Básico, más:',
    features: [
      'Hasta 5 especialistas, 2 recepcionistas y 2 cajeros',
      'Hasta 3 giros y 1 sucursal',
      'Cobros: punto de venta, caja, inventario y reportes',
      'Terminal y ligas de Mercado Pago, devoluciones y comisiones',
      'Facturación electrónica (CFDI)',
      'Permisos por persona',
      'Asistente de IA con 500 usos al mes',
      '20 GB de archivos · Soporte prioritario y alta asistida'
    ],
    cta: 'Solicitar demo',
    featured: true
  },
  {
    id: 'pro',
    name: 'Pro',
    highlights: ['Cuentas sin límite y todos los giros', 'Hasta 10 sucursales con reportes del grupo', 'Todo lo de Crecimiento', 'Asistente de IA: 1,500 usos al mes', '100 GB · Capacitación y soporte dedicado'],
    price: '$1,899',
    period: 'MXN / mes',
    tagline: 'Para grupos médicos y redes',
    magic: 1500,
    blurb: 'Para varias sucursales y equipos grandes: sin límites de personas ni de giros, con atención dedicada.',
    includes: 'Todo lo del plan Crecimiento, más:',
    features: ['Cuentas sin límite y todos los giros', 'Hasta 10 sucursales con reportes del grupo', 'Asistente de IA con 1,500 usos al mes', '100 GB de archivos', 'Capacitación al equipo', 'Acompañamiento y soporte dedicado'],
    cta: 'Solicitar demo',
    featured: false
  }
];

export const faqs = [
  { q: '¿Necesito instalar algo?', a: 'No. Caresia funciona desde el navegador en computadora, tableta o celular, y puedes instalarla como app en tu pantalla de inicio. Solo necesitas internet.' },
  { q: '¿Cómo se protege la información de mis pacientes?', a: 'Cada usuario tiene permisos por rol: recepción puede llevar la agenda sin ver la información clínica. Cada clínica ve solo a sus pacientes, los datos sensibles se guardan cifrados y queda registro de quién abre cada expediente. Puedes activar la verificación en dos pasos y descargar tus datos cuando quieras.' },
  { q: '¿Cumple con la normatividad mexicana?', a: 'Está pensada para ella: notas clínicas que no se alteran después de guardarse (con adendas, como pide la NOM-004), aviso de privacidad y consentimientos firmados en pantalla, y manejo de solicitudes ARCO. Aun así, tu contador y tu responsable sanitario validan lo que aplica a tu consultorio.' },
  { q: '¿Qué hace la inteligencia artificial?', a: 'Ayuda a armar un borrador del plan nutricional, resume el expediente antes de la consulta, convierte una lista o foto en inventario y sugiere precios. Cada acción cuenta como un uso de IA: Crecimiento incluye 500 al mes y Pro 1,500; Básico no incluye asistente. Tú siempre revisas y ajustas el resultado, y no se envía el nombre del paciente.' },
  { q: '¿Me ayudan a empezar?', a: 'Sí. En los planes Crecimiento y Pro te acompañamos en el alta, la migración de tus pacientes, la configuración de usuarios y la capacitación de tu equipo.' },
  { q: '¿Hay plazo forzoso?', a: 'No. Los planes son mensuales y puedes cancelar cuando quieras.' }
];

export const specialties = ['Odontología', 'Medicina general', 'Pediatría', 'Nutrición', 'Veterinaria', 'Medicina interna', 'Psicología y Terapia', 'Fisioterapia', 'Quiropráctica', 'Dermatología', 'Ginecología', 'Ortopedia'];
