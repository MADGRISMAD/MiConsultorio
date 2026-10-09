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
    blurb: 'Para el profesional independiente que atiende solo o con un asistente.',
    features: [
      '1 profesional de la salud y hasta 3 usuarios',
      'Agenda ilimitada, reserva en línea y recordatorios',
      'Expedientes con formularios de tu especialidad',
      'Recetas, odontograma, plan nutricional y más herramientas por giro',
      'Portal del paciente y consentimientos firmados',
      'Reportes clínicos y descarga de tus datos',
      'Soporte por correo'
    ],
    cta: 'Solicitar demo',
    featured: false
  },
  {
    name: 'Crecimiento',
    price: '$1,199',
    period: 'MXN / mes',
    blurb: 'Para clínicas con varios doctores que además quieren cobrar desde el sistema.',
    features: [
      'Todo lo del Básico',
      'Hasta 5 profesionales y recepción ilimitada',
      'Cobros: punto de venta, caja, servicios, inventario y reportes',
      'Terminal y ligas de pago de Mercado Pago, devoluciones y comisiones',
      'Facturación electrónica (CFDI)',
      '150 usos de IA al mes: plan nutricional, inventario y precios',
      'Hasta 3 sucursales y permisos por persona',
      'Alta y migración asistida · Soporte prioritario'
    ],
    cta: 'Solicitar demo',
    featured: true
  },
  {
    name: 'Pro',
    price: 'A medida',
    period: '',
    blurb: 'Para grupos médicos, redes de clínicas y varias sucursales.',
    features: ['Todo lo de Crecimiento', 'Profesionales ilimitados', 'Hasta 10 sucursales con reportes del grupo', '500 usos de IA al mes', 'Capacitación al equipo', 'Acompañamiento dedicado'],
    cta: 'Hablar con ventas',
    featured: false
  }
];

export const faqs = [
  { q: '¿Necesito instalar algo?', a: 'No. Caresia funciona desde el navegador en computadora, tableta o celular, y si quieres puedes instalarla como una app en tu pantalla de inicio. Solo necesitas conexión a internet.' },
  { q: '¿Para qué especialidades sirve?', a: 'Para odontología, medicina general e interna, pediatría, nutrición, psicología, fisioterapia, quiropráctica, ortopedia, dermatología, ginecología y veterinaria. Cada una trae sus formularios y herramientas, y si atiendes más de una conviven en el mismo consultorio.' },
  { q: '¿Qué hace la inteligencia artificial?', a: 'Arma un borrador del plan nutricional de 7 días a partir del objetivo, los antecedentes y los alimentos que no le gustan al paciente, convierte una lista o una foto en tu inventario y sugiere precios. Las calorías se calculan con fórmulas clínicas y tú siempre revisas y ajustas el resultado antes de guardarlo. No se envía el nombre del paciente.' },
  { q: '¿Puedo atender varias personas a la misma hora?', a: 'Sí. La agenda es por profesional y por sala: varios doctores pueden tener consulta en el mismo horario. Las citas que recomienda un profesional quedan en la agenda como «por confirmar» para que recepción las confirme.' },
  { q: '¿Mis pacientes pueden agendar por su cuenta?', a: 'Sí. Cada consultorio tiene su liga de reserva en línea y un portal donde el paciente ve sus citas, recetas y vacunas. Los recordatorios salen automáticos por correo y WhatsApp.' },
  { q: '¿Mi recepcionista puede agendar sin ver los expedientes?', a: 'Sí. Cada usuario tiene permisos por rol, y además puedes darle o quitarle permisos a una persona en particular: recepción administra la agenda sin acceso a la información clínica.' },
  { q: '¿Puedo cobrar desde Caresia?', a: 'Sí, en los planes Crecimiento y Pro. Incluyen punto de venta, caja con corte, inventario, cobro con terminal o liga de Mercado Pago, devoluciones, comisiones por profesional y facturación electrónica. Al terminar una consulta, la cuenta pasa sola a caja.' },
  { q: '¿Cómo se protege la información de mis pacientes?', a: 'El acceso requiere usuario y contraseña, con verificación en dos pasos opcional. Cada clínica solo ve a sus propios pacientes, los datos sensibles se guardan cifrados, queda registro de quién abre cada expediente y puedes descargar tus datos cuando quieras.' },
  { q: '¿Cumple con la normatividad mexicana?', a: 'Está pensada para ella: notas clínicas que no se alteran después de guardarse (con adendas, como pide la NOM-004), aviso de privacidad y consentimientos firmados en pantalla, y manejo de solicitudes ARCO. Aun así, tu contador y tu responsable sanitario son quienes validan lo que aplica a tu consultorio.' },
  { q: '¿Tengo más de una sucursal?', a: 'Los planes Crecimiento y Pro permiten varias sucursales bajo la misma cuenta, con reportes del grupo.' },
  { q: '¿Me ayudan a empezar?', a: 'Sí. En los planes Crecimiento y Pro te acompañamos en el alta, la migración de tus pacientes, la configuración de usuarios y la capacitación de tu equipo.' },
  { q: '¿Hay plazo forzoso?', a: 'No. Los planes son mensuales y puedes cancelar cuando quieras.' }
];

export const specialties = ['Odontología', 'Medicina general', 'Pediatría', 'Nutrición', 'Veterinaria', 'Medicina interna', 'Psicología', 'Fisioterapia', 'Quiropráctica', 'Dermatología', 'Ginecología', 'Ortopedia'];
