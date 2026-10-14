package api

// blogTopic is one entry of the editorial calendar: what people search, and the angle Caresia can answer well.
type blogTopic struct {
	Key, Keyword, Angle, Category string
}

// Categorías del blog; la IA debe usar una de estas.
var blogCategories = []string{"Odontología", "Medicina", "Nutrición", "Psicología", "Fisioterapia", "Veterinaria", "Gestión", "Marketing", "Finanzas", "Legal"}

// blogTopics se escribe en este orden, uno por corrida. Cuando se acaban, la IA propone temas nuevos sin repetir.
// Para agregar temas basta con sumar líneas aquí (la clave no debe cambiar una vez publicada).
var blogTopics = []blogTopic{
	{"recordatorios-citas", "recordatorios de citas para consultorio", "Cómo reducir las citas perdidas con recordatorios automáticos por correo y confirmación del paciente; qué decir, cuándo enviarlos y qué medir.", "Gestión"},
	{"software-dental", "software para consultorio dental", "Qué debe tener un sistema para consultorio dental en México y cómo elegirlo sin equivocarse: agenda, expediente, planes de tratamiento, cobros y permisos.", "Odontología"},
	{"expediente-nom-004", "expediente clínico electrónico NOM-004", "Qué pide la NOM-004-SSA3-2012 del expediente clínico, en lenguaje sencillo, y cómo llevarlo en digital sin incumplir (notas que no se alteran, adendas, resguardo).", "Legal"},
	{"administrar-consultorio-medico", "cómo administrar un consultorio médico", "Guía práctica para el médico general que atiende y además administra: agenda, recepción, cobros, expedientes y tiempo propio.", "Medicina"},
	{"anticipos-citas", "cobrar anticipo para apartar cita", "Cuándo conviene pedir anticipo para apartar una cita, cómo comunicarlo sin incomodar al paciente y cómo manejar cancelaciones y devoluciones.", "Finanzas"},
	{"aviso-privacidad-consultorio", "aviso de privacidad para consultorio", "Para qué sirve el aviso de privacidad en un consultorio, qué debe incluir según la LFPDPPP y cómo obtener el consentimiento del paciente para datos de salud.", "Legal"},
	{"agenda-consultorio-dental", "cómo organizar la agenda de un consultorio dental", "Bloques por tipo de tratamiento, tiempos entre pacientes, urgencias del día y varios sillones o doctores en la misma agenda.", "Odontología"},
	{"software-nutriologos", "software para nutriólogos", "Qué necesita un nutriólogo para llevar a sus pacientes: mediciones, planes de alimentación, seguimiento y recordatorios; y cómo ahorrar tiempo con ayuda de IA revisada por el profesional.", "Nutrición"},
	{"agenda-en-linea", "agenda en línea para consultorio", "Ventajas de que el paciente reserve solo desde una liga, cómo evitar empalmes y citas falsas, y cómo compartir la liga en WhatsApp, Instagram y Google.", "Marketing"},
	{"facturacion-cfdi-medicos", "facturación electrónica para médicos CFDI 4.0", "Qué datos pedir al paciente para facturar, errores comunes y cómo emitir la factura desde el mismo sistema donde cobras. Sin consejos fiscales específicos: remite al contador.", "Finanzas"},
	{"notas-sesion-psicologia", "notas de sesión en psicología", "Cómo llevar notas de sesión útiles, qué separar entre notas clínicas y notas privadas, confidencialidad y resguardo de la información.", "Psicología"},
	{"software-veterinaria", "software para clínica veterinaria", "Qué debe tener un sistema para veterinaria: expediente por mascota y propietario, cartilla de vacunación y desparasitación, recordatorios y cobros.", "Veterinaria"},
	{"resenas-pacientes", "cómo conseguir reseñas de pacientes", "Cómo pedir opiniones a los pacientes satisfechos en el momento correcto, responder reseñas negativas con profesionalismo y usar encuestas de satisfacción.", "Marketing"},
	{"plan-tratamiento-dental", "plan de tratamiento dental y presupuesto", "Cómo presentar un plan de tratamiento por etapas con presupuesto claro para que el paciente acepte, y cómo darle seguimiento.", "Odontología"},
	{"seguimiento-pacientes-cronicos", "seguimiento de pacientes crónicos", "Cómo organizar el seguimiento de pacientes con diabetes, hipertensión u otras enfermedades crónicas desde la parte administrativa: citas de control, recordatorios, medicación crónica registrada.", "Medicina"},
	{"proteger-datos-pacientes", "cómo proteger los datos de los pacientes", "Medidas prácticas de seguridad en el consultorio: contraseñas, verificación en dos pasos, permisos por rol, registro de accesos y respaldos.", "Legal"},
	{"recepcionista-consultorio", "funciones de la recepcionista de un consultorio", "Qué hace bien una recepción de consultorio, cómo capacitarla y cómo darle acceso a la agenda sin exponer información clínica.", "Gestión"},
	{"pacientes-que-no-regresan", "cómo recuperar pacientes que dejaron de venir", "Por qué los pacientes dejan de regresar, cómo detectarlos a tiempo y cómo invitarlos de vuelta sin ser invasivo.", "Marketing"},
	{"fisioterapia-evolucion", "expediente de fisioterapia y evolución del paciente", "Cómo registrar valoración, sesiones y evolución en fisioterapia para mostrar avances al paciente y planear el alta.", "Fisioterapia"},
	{"precio-consulta", "cómo fijar el precio de la consulta", "Cómo calcular el precio de la consulta con base en costos, tiempo y mercado local, y cómo comunicar aumentos a los pacientes.", "Finanzas"},
	{"consentimiento-informado", "consentimiento informado en el consultorio", "Qué es el consentimiento informado, cuándo pedirlo por escrito, qué debe contener y cómo firmarlo en pantalla y guardarlo en el expediente.", "Legal"},
	{"conseguir-pacientes-nutriologo", "cómo conseguir pacientes como nutriólogo", "Estrategias realistas para un nutriólogo independiente: perfil en Google, reserva en línea, contenido en redes y seguimiento que genera recomendaciones.", "Nutrición"},
	{"inventario-consultorio", "control de inventario en consultorio", "Cómo controlar materiales e insumos en un consultorio dental o médico: existencias mínimas, caducidades, lotes y consumo por tratamiento.", "Gestión"},
	{"agenda-psicologo", "cómo organizar la agenda de un psicólogo", "Sesiones recurrentes, cancelaciones, sesiones en línea y cobro por paquete de sesiones, cuidando la confidencialidad.", "Psicología"},
	{"google-maps-consultorio", "cómo aparecer en Google Maps con tu consultorio", "Cómo crear y optimizar el perfil de Google del consultorio, fotos, horarios, reseñas y liga de reserva.", "Marketing"},
	{"cartilla-vacunacion-mascotas", "cartilla de vacunación digital para mascotas", "Ventajas de llevar la cartilla de vacunación y desparasitación en digital y de enviar recordatorios a los propietarios.", "Veterinaria"},
	{"excel-a-software", "dejar la agenda de papel y Excel en el consultorio", "Señales de que el consultorio ya no cabe en papel o Excel, cómo migrar pacientes sin perder información y cómo lograr que el equipo lo adopte.", "Gestión"},
	{"derechos-arco", "derechos ARCO en un consultorio", "Qué son los derechos de acceso, rectificación, cancelación y oposición, cómo atender una solicitud de un paciente y en qué plazos.", "Legal"},
	{"indicadores-consultorio", "indicadores para medir un consultorio", "Qué números revisar cada mes: ocupación de agenda, inasistencias, ticket promedio, pacientes nuevos y recurrentes; y qué decisiones tomar con ellos.", "Gestión"},
	{"teleconsulta", "teleconsulta en el consultorio", "Cuándo tiene sentido la consulta en línea, cómo agendarla y cobrarla, y sus límites clínicos y de privacidad.", "Medicina"},
	{"corte-de-caja", "corte de caja en el consultorio", "Cómo hacer un corte de caja diario sin errores: efectivo, tarjeta, transferencias, devoluciones y quién es responsable.", "Finanzas"},
	{"comisiones-especialistas", "cómo pagar comisiones a especialistas en una clínica", "Esquemas comunes de pago a especialistas (porcentaje, renta de consultorio, mixto) y cómo calcularlos sin discusiones.", "Finanzas"},
	{"portal-paciente", "portal del paciente", "Qué gana el consultorio cuando el paciente puede ver sus citas, recetas y vacunas en línea, y cómo invitarlo a usarlo.", "Gestión"},
	{"varias-sucursales", "cómo administrar varias sucursales de una clínica", "Agenda, inventario, caja y reportes cuando la clínica crece a dos o más sucursales; qué centralizar y qué dejar a cada sucursal.", "Gestión"},
	{"primera-consulta", "experiencia del paciente en la primera consulta", "Cómo cuidar la primera visita desde que agenda hasta que sale: confirmación, llegada, papeleo, explicación del plan y seguimiento.", "Gestión"},
	{"receta-medica-requisitos", "requisitos de una receta médica en México", "Qué datos debe llevar una receta médica en México y ventajas de la receta electrónica con verificación. Sin recomendar medicamentos.", "Legal"},
}
