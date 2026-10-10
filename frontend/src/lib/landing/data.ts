// ---------------------------------------------------------------------------
// Datos comerciales de la landing: edita aquí precios, contacto y textos.
// ---------------------------------------------------------------------------

export const loginPath = '/login';
export const contactEmail = 'contacto@caresia.com';
export const demoHref = `mailto:${contactEmail}?subject=${encodeURIComponent('Quiero una demo de Caresia')}`;

export const plans = [
  {
    name: 'Básico',
    price: '$499',
    period: 'MXN / mes',
    tagline: 'Para el profesional independiente',
    magic: 0,
    blurb: 'Ordena tu consultorio: agenda, expedientes y recordatorios en un solo lugar, para ti y tu pequeño equipo.',
    includes: '',
    features: [
      '2 especialistas, 1 recepción y 1 caja (más el administrador)',
      '1 giro y 1 sucursal',
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
    name: 'Crecimiento',
    price: '$1,199',
    period: 'MXN / mes',
    tagline: 'Para clínicas que crecen',
    magic: 250,
    blurb: 'Suma a tu equipo y empieza a cobrar desde el sistema: de la consulta a la caja, sin capturar dos veces.',
    includes: 'Todo lo del plan Básico, más:',
    features: [
      'Hasta 5 especialistas, 2 recepciones y 2 cajas',
      'Hasta 3 giros y 3 sucursales',
      'Cobros: punto de venta, caja, inventario y reportes',
      'Terminal y ligas de Mercado Pago, devoluciones y comisiones',
      'Facturación electrónica (CFDI)',
      'Recordatorios por WhatsApp',
      'Permisos por persona',
      'Asistente de IA con 250 usos al mes',
      '20 GB de archivos · Soporte prioritario y alta asistida'
    ],
    cta: 'Solicitar demo',
    featured: true
  },
  {
    name: 'Pro',
    price: 'Desde $2,499',
    period: 'MXN / mes',
    tagline: 'Para grupos médicos y redes',
    magic: 500,
    blurb: 'Para varias sucursales y equipos grandes: sin límites de personas ni de giros, con atención dedicada.',
    includes: 'Todo lo del plan Crecimiento, más:',
    features: ['Cuentas sin límite (por ahora) y todos los giros', 'Hasta 10 sucursales con reportes del grupo', 'Asistente de IA con 500 usos al mes', '100 GB de archivos', 'Capacitación al equipo', 'Acompañamiento y soporte dedicado'],
    cta: 'Hablar con ventas',
    featured: false
  }
];

export const faqs = [
  { q: '¿Necesito instalar algo?', a: 'No. Caresia funciona desde el navegador en computadora, tableta o celular, y si quieres puedes instalarla como una app en tu pantalla de inicio. Solo necesitas conexión a internet.' },
  { q: '¿Para qué especialidades sirve?', a: 'Para odontología, medicina general e interna, pediatría, nutrición, psicología, fisioterapia, quiropráctica, ortopedia, dermatología, ginecología y veterinaria. Cada una trae sus formularios y herramientas, y si atiendes más de una conviven en el mismo consultorio.' },
  { q: '¿Qué hace la inteligencia artificial?', a: 'Arma un borrador del plan nutricional de 7 días (y cambia una comida si al paciente no le gusta), prepara un resumen del expediente antes de la consulta, convierte una lista o una foto en tu inventario y sugiere precios. Cada acción es un «uso de magia»: el plan Crecimiento incluye 250 al mes y el Pro 500; el Básico no incluye asistente de IA. Las calorías se calculan con fórmulas clínicas y tú siempre revisas y ajustas el resultado antes de guardarlo. No se envía el nombre del paciente.' },
  { q: '¿Puedo atender varias personas a la misma hora?', a: 'Sí. La agenda es por profesional y por sala: varios doctores pueden tener consulta en el mismo horario. Las citas que recomienda un profesional quedan en la agenda como «por confirmar» para que recepción las confirme.' },
  { q: '¿Mis pacientes pueden agendar por su cuenta?', a: 'Sí. Cada consultorio tiene su liga de reserva en línea y un portal donde el paciente ve sus citas, recetas y vacunas. Los recordatorios salen automáticos por correo, y también por WhatsApp desde el plan Crecimiento.' },
  { q: '¿Mi recepcionista puede agendar sin ver los expedientes?', a: 'Sí. Cada usuario tiene permisos por rol, y además puedes darle o quitarle permisos a una persona en particular: recepción administra la agenda sin acceso a la información clínica.' },
  { q: '¿Puedo cobrar desde Caresia?', a: 'Sí, en los planes Crecimiento y Pro. Incluyen punto de venta, caja con corte, inventario, cobro con terminal o liga de Mercado Pago, devoluciones, comisiones por profesional y facturación electrónica. Al terminar una consulta, la cuenta pasa sola a caja.' },
  { q: '¿Cómo se protege la información de mis pacientes?', a: 'El acceso requiere usuario y contraseña, con verificación en dos pasos opcional. Cada clínica solo ve a sus propios pacientes, los datos sensibles se guardan cifrados, queda registro de quién abre cada expediente y puedes descargar tus datos cuando quieras.' },
  { q: '¿Cumple con la normatividad mexicana?', a: 'Está pensada para ella: notas clínicas que no se alteran después de guardarse (con adendas, como pide la NOM-004), aviso de privacidad y consentimientos firmados en pantalla, y manejo de solicitudes ARCO. Aun así, tu contador y tu responsable sanitario son quienes validan lo que aplica a tu consultorio.' },
  { q: '¿Tengo más de una sucursal?', a: 'Los planes Crecimiento y Pro permiten varias sucursales bajo la misma cuenta, con reportes del grupo.' },
  { q: '¿Me ayudan a empezar?', a: 'Sí. En los planes Crecimiento y Pro te acompañamos en el alta, la migración de tus pacientes, la configuración de usuarios y la capacitación de tu equipo.' },
  { q: '¿Hay plazo forzoso?', a: 'No. Los planes son mensuales y puedes cancelar cuando quieras.' }
];

export const specialties = ['Odontología', 'Medicina general', 'Pediatría', 'Nutrición', 'Veterinaria', 'Medicina interna', 'Psicología', 'Fisioterapia', 'Quiropráctica', 'Dermatología', 'Ginecología', 'Ortopedia'];
