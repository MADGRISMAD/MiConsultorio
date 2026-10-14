# Historias de usuario de Caresia

Formato: **Como** [rol] **quiero** [acción] **para** [beneficio]. Cada historia lleva criterios de aceptación (CA) y, cuando aplica, la prioridad (MoSCoW: **M**ust, **S**hould, **C**ould) y el plan que la incluye. Están escritas a partir de lo que el sistema hace hoy; lo que sigue pendiente está en la última sección.

## Roles

| Rol | Quién es |
|---|---|
| **Visitante** | Persona que llega a caresia.mx o a la página pública de un consultorio. |
| **Paciente / propietario** | Quien se atiende (o el dueño de la mascota). Entra por enlaces y por el portal, sin cuenta con contraseña. |
| **Administrador** (admin) | Dueño o responsable del consultorio. Configura, ve todo y administra al equipo. |
| **Especialista** (doctor) | Médico, dentista, psicólogo, nutriólogo, fisioterapeuta, veterinario, etc. Atiende y firma. |
| **Recepción** | Maneja la agenda; no ve información clínica. |
| **Cajero** | Cobra y maneja la caja; no ve información clínica. |
| **Dueño de organización** | Primer administrador de la clínica matriz; administra sucursales (Plan Pro). |
| **Admin de plataforma** | Equipo de Caresia que administra negocios, suscripciones y pagos. |
| **Soporte de plataforma** | Equipo de Caresia con acceso de solo lectura. |

---

## 1. Registro, acceso y seguridad

**HU-1.1 Registrar mi consultorio** (M)
Como **visitante** quiero crear mi consultorio con mi giro, correo y contraseña para empezar a usar Caresia sin llamar a nadie.
- CA: el alta pide nombre del negocio, giro (Medicina general, Dental, Pediatría, Nutrición, Psicología, Veterinaria, etc.), nombre, correo y contraseña.
- CA: la contraseña respeta el largo mínimo y máximo compartido con el servidor.
- CA: se crea una prueba gratuita y el primer usuario queda como administrador.
- CA: un correo ya registrado se rechaza con un mensaje claro.

**HU-1.2 Iniciar y cerrar sesión** (M)
Como **usuario del consultorio** quiero entrar con mi usuario o correo y salir cuando termine para proteger la información.
- CA: credenciales incorrectas muestran un error genérico, sin revelar cuál falló.
- CA: las pantallas internas redirigen al acceso si no hay sesión.
- CA: la sesión se invalida si cambian mi contraseña o mi rol.

**HU-1.3 Recuperar mi contraseña** (M)
Como **usuario** quiero restablecer mi contraseña por correo para recuperar el acceso si la olvido.
- CA: el enlace de restablecimiento caduca y solo se usa una vez.
- CA: la respuesta es la misma exista o no el correo.

**HU-1.4 Verificación en dos pasos** (S)
Como **administrador** quiero activar un segundo factor (app autenticadora) para proteger cuentas con datos clínicos.
- CA: se activa escaneando un código QR y confirmando un código.
- CA: al iniciar sesión se pide el código después de la contraseña.

**HU-1.5 Equipo y roles** (M)
Como **administrador** quiero agregar personas con rol (administrador, especialista, recepción, cajero) para que cada una vea solo lo que le toca.
- CA: recepción y cajero no ven expedientes ni notas clínicas.
- CA: no se pueden agregar más personas que los lugares del plan.
- CA: puedo desactivar o reactivar a alguien y restablecer su contraseña.

**HU-1.6 Bitácora de accesos al expediente** (M)
Como **administrador** quiero saber quién abrió o modificó un expediente para cumplir con la NOM-004 y la LFPDPPP.
- CA: cada lectura y cambio de un expediente queda registrado con persona, fecha y acción.

---

## 2. Pacientes y propietarios

**HU-2.1 Registrar un paciente** (M)
Como **recepción o especialista** quiero dar de alta a una persona o a un animal con sus datos básicos para tener su expediente.
- CA: para animales se capturan especie, raza y propietario; para personas, fecha de nacimiento, contacto y datos de emergencia.
- CA: los datos sensibles se guardan cifrados.
- CA: no se permite el duplicado evidente (mismo nombre y fecha de nacimiento) sin avisar.

**HU-2.2 Un propietario con varias mascotas** (M)
Como **veterinario** quiero ligar varias mascotas a un propietario, incluso con el mismo nombre, para no confundirlas.
- CA: el buscador de propietarios se maneja con teclado y anuncia la opción activa.
- CA: dos mascotas con el mismo nombre se distinguen por especie y propietario.

**HU-2.3 Buscar pacientes** (M)
Como **cualquier usuario del consultorio** quiero buscar un paciente por nombre, teléfono o folio para abrir su expediente rápido.

**HU-2.4 Fusionar duplicados** (S)
Como **administrador** quiero fusionar dos expedientes duplicados para tener un solo historial.
- CA: se conservan citas, consultas, recetas, certificados, paquetes y archivos del expediente que se fusiona.

**HU-2.5 Exportar y archivar** (M)
Como **administrador** quiero exportar el expediente de un paciente y archivarlo para atender una solicitud ARCO o cerrar el caso.
- CA: un expediente archivado es de solo lectura: no acepta consultas, recetas, certificados ni cargos nuevos (409 `PATIENT_ARCHIVED`).

---

## 3. Agenda y citas

**HU-3.1 Ver y crear citas** (M)
Como **recepción** quiero ver la agenda por día y por especialista y crear citas para organizar la atención.
- CA: una cita que choca con otra avisa del conflicto.
- CA: una cita de otro día no se puede marcar como llegada ni terminada.

**HU-3.2 Cambiar el estado de una cita** (M)
Como **recepción o especialista** quiero marcar una cita como llegó, en consulta, completada, cancelada o no asistió para llevar el control del día.
- CA: los estados disponibles dependen del rol y del estado actual.

**HU-3.3 Iniciar la consulta desde la cita** (M)
Como **especialista** quiero iniciar la consulta desde la cita para abrir la nota ya ligada al paciente.
- CA: al guardar la nota, la cita queda completada.
- CA: una cita cancelada, no asistida o pendiente de pago no se cierra por error.

**HU-3.4 Lista de espera** (S)
Como **recepción** quiero una lista de espera para ofrecer un horario que se libera.
- CA: al liberarse el horario, se avisa por correo al primero de la lista con un enlace para tomarlo.

**HU-3.5 Recordatorios de cita** (S)
Como **paciente** quiero recibir un recordatorio por correo para no olvidar mi cita.
- CA: puedo confirmar o cancelar desde el enlace del correo.
- CA: la baja de recordatorios se hace con un clic.

**HU-3.6 Paquetes de sesiones** (S) — plan Crecimiento y Pro
Como **especialista** quiero vender paquetes de sesiones para que el paciente use una por cita.
- CA: cada cita descuenta una sesión; si se elimina la cita, la sesión se libera.

**HU-3.7 Agenda en mi Google Calendar** (C)
Como **especialista** quiero suscribirme a un feed privado para ver mis citas en mi calendario.

---

## 4. Reserva en línea y página pública

**HU-4.1 Página pública del consultorio** (M)
Como **administrador** quiero una página pública con mis servicios, horarios, seguros, preguntas frecuentes y redes para que los pacientes me encuentren.
- CA: puedo cambiar la dirección web (slug) y el sistema avisa si está ocupada y ofrece variantes.
- CA: puedo pedir a la IA un borrador de los textos vacíos y editarlo a mano.

**HU-4.2 Reservar en línea** (M)
Como **paciente** quiero elegir servicio, especialista y horario libre desde la página del consultorio para agendar sin llamar.
- CA: solo se ofrecen horarios realmente libres.
- CA: dos personas no pueden reservar el mismo horario al mismo tiempo.
- CA: recibo un correo con el enlace para ver, cambiar o cancelar mi cita.

**HU-4.3 Pagar anticipo al reservar** (S) — plan con cobros
Como **especialista** quiero pedir un anticipo al reservar para reducir las ausencias.
- CA: si el servicio pide anticipo, el horario se aparta solo 15 minutos.
- CA: si no se confirma el pago en ese lapso, el horario vuelve a quedar libre.
- CA: si el servicio no pide anticipo, la cita es válida de inmediato.
- CA: al cancelar con anticipo se reembolsa o se aplica según la política del consultorio.

**HU-4.4 Directorio y blog** (C)
Como **visitante** quiero encontrar consultorios por especialidad y ciudad y leer artículos para elegir con información.

**HU-4.5 Reseñas y encuesta de satisfacción** (S)
Como **paciente** quiero contestar una encuesta breve después de la consulta para dar mi opinión.
- CA: las calificaciones altas me invitan a calificar en Google Maps.
- CA: el administrador ve las respuestas en indicadores.

---

## 5. Expediente clínico

**HU-5.1 Registrar una consulta** (M)
Como **especialista** quiero escribir la nota de consulta (motivo, exploración, diagnóstico, plan) para dejar constancia clínica.
- CA: puedo agregar adendas sin alterar la nota original.
- CA: los diagnósticos usan catálogo (CIE-10 y, en veterinaria, el catálogo propio).

**HU-5.2 Receta con alertas** (M)
Como **especialista** quiero emitir una receta con alertas de alergias e interacciones para prescribir con seguridad.
- CA: la receta pide cédula profesional y la imprime con mis credenciales.
- CA: la receta tiene verificación pública por código.
- CA: hay catálogo de medicamentos para humanos y para veterinaria.

**HU-5.3 Medicación crónica y alergias** (M)
Como **especialista** quiero registrar los padecimientos, alergias y medicamentos crónicos del paciente para verlos en cada consulta.

**HU-5.4 Certificados médicos e incapacidades** (S)
Como **especialista** quiero emitir certificados imprimibles con verificación para entregar al paciente.

**HU-5.5 Consentimientos informados** (M)
Como **especialista** quiero registrar el consentimiento firmado de un procedimiento para cumplir la norma.

**HU-5.6 Archivos y estudios** (S)
Como **especialista** quiero adjuntar imágenes, PDF y resultados de laboratorio al expediente para tenerlos en un solo lugar.

**HU-5.7 Vacunación y seguimiento automático** (S)
Como **especialista** quiero llevar el esquema de vacunas y recibir avisos de seguimiento (vacunas, limpieza dental, Papanicolaou, anticonceptivo, siguiente consulta) para no perder pacientes.

### Por especialidad

**HU-5.8 Crecimiento y desarrollo** (S) — pediatría
Como **pediatra** quiero ver curvas de peso, talla e IMC con hitos del desarrollo para valorar el crecimiento.

**HU-5.9 Nutrición** (S)
Como **nutriólogo** quiero planes con equivalentes (catálogo SMAE), recordatorio de 24 horas y lista del súper para dar un plan accionable.
- CA: la lista del súper con IA solo está disponible en el giro de nutrición.

**HU-5.10 Psicología y terapia** (S)
Como **psicólogo** quiero aplicar escalas (PHQ-9, GAD-7, PSS-10), registrar objetivos terapéuticos y tareas, y dar videollamada para dar seguimiento.

**HU-5.11 Fisioterapia** (S)
Como **fisioterapeuta** quiero hojas de ejercicios (con ayuda de IA) que edito a mano para dar al paciente.

**HU-5.12 Odontología** (S)
Como **dentista** quiero un periodontograma y un presupuesto con plan de pagos para presentar el tratamiento.

**HU-5.13 Ginecología y dermatología** (C)
Como **ginecólogo** quiero control prenatal (FPP, semanas) y como **dermatólogo** quiero comparar fotos antes/después para dar seguimiento.

**HU-5.14 Veterinaria** (S)
Como **veterinario** quiero registrar microchip, esterilización, peso y vacunas para el historial de la mascota.

---

## 6. Inteligencia artificial (usos de magia)

**HU-6.1 Ordenar mis notas con IA** (S)
Como **especialista** quiero que la IA ordene mis notas sueltas en los campos vacíos de la consulta para ahorrar captura.
- CA: la IA solo rellena campos vacíos y deja un aviso de que es borrador.
- CA: nunca sustituye mi criterio ni diagnostica.

**HU-6.2 Dictar la consulta** (S)
Como **especialista** quiero dictar la nota por voz para no teclear.
- CA: el audio no se guarda; solo queda el texto.

**HU-6.3 Tareas de IA por paciente** (S)
Como **especialista** quiero tareas de IA por área (resumen de consulta, mensaje al paciente, etc.) que llegan como borrador editable.

**HU-6.4 IA para la clínica** (C)
Como **administrador** quiero un resumen del periodo, pacientes en riesgo de abandono, respuestas a reseñas y textos de la página pública para tomar decisiones.

**HU-6.5 Límite y consumo de usos** (M)
Como **administrador** quiero ver cuántos usos de IA quedan este mes (150/250/500 según el plan) para no quedarme sin ellos.
- CA: si no quedan usos, la IA responde con un aviso claro y no cobra.
- CA: un uso fallido se devuelve.

---

## 7. Punto de venta, caja y facturación (planes con cobros)

**HU-7.1 Cobrar una consulta o venta** (M)
Como **cajero** quiero cobrar servicios y productos con distintos métodos de pago para registrar el ingreso.
- CA: la pre-cuenta de la consulta se cobra sin descontar dos veces el insumo.

**HU-7.2 Caja** (M)
Como **cajero** quiero abrir y cerrar caja con arqueo para cuadrar el efectivo.

**HU-7.3 Devoluciones** (S)
Como **cajero** quiero devolver un producto parcial o totalmente para regresarlo al inventario y sacarlo de la caja.

**HU-7.4 Inventario** (S)
Como **administrador** quiero controlar existencias, lotes y caducidades para surtir a tiempo.

**HU-7.5 Cuentas por cobrar** (S)
Como **cajero** quiero registrar crédito y abonos de un paciente para llevar su saldo.

**HU-7.6 Comisiones** (C)
Como **administrador** quiero comisiones por especialista para pagar con claridad.

**HU-7.7 Factura (CFDI)** (S)
Como **paciente** quiero solicitar mi factura de una venta y como **administrador** quiero timbrarla para cumplir fiscalmente.

**HU-7.8 Reportes del POS** (S)
Como **administrador** quiero ver ventas por periodo, método y producto, y exportarlas.

---

## 8. Indicadores y reportes

**HU-8.1 Panel de indicadores** (S)
Como **administrador** quiero ver citas, ausentismo, pacientes nuevos, ingresos y satisfacción para entender cómo va el negocio.

**HU-8.2 Reportes exportables** (S)
Como **administrador** quiero exportar reportes a CSV para llevarlos a mi contador.

---

## 9. Portal del paciente y privacidad

**HU-9.1 Portal del paciente** (S)
Como **paciente** quiero entrar con un enlace seguro para ver mis citas, recetas y documentos.

**HU-9.2 Derechos ARCO** (M)
Como **paciente** quiero pedir acceso, rectificación, cancelación u oposición de mis datos y seguir el estado de mi solicitud.
- CA: el consultorio ve las solicitudes, sus plazos y puede responderlas con el paquete de datos.

**HU-9.3 Aviso de privacidad y términos** (M)
Como **visitante** quiero leer el aviso de privacidad y los términos antes de dar mis datos.

---

## 10. Suscripción y planes

**HU-10.1 Elegir y pagar un plan** (M)
Como **administrador** quiero elegir Básico, Crecimiento o Pro y pagar con Mercado Pago para seguir usando Caresia.

**HU-10.2 Suscripción vencida** (M)
Como **administrador** quiero que, si mi suscripción vence, se me lleve a la ventana de planes para pagar y recuperar todo.
- CA: con la suscripción vencida las funciones se limitan, la página pública y los correos automáticos se apagan, y no se pierde ningún dato.
- CA: al pagar, todo vuelve a funcionar sin pasos extra.

**HU-10.3 Ver mi plan, lugares y usos** (S)
Como **administrador** quiero ver mi plan, los lugares del equipo y los usos de IA para decidir si cambio de plan.

---

## 11. Sucursales (plan Pro)

**HU-11.1 Crear una sucursal** (S)
Como **dueño de organización** quiero crear sucursales (hasta 10 con Pro) para operar varios consultorios con mi cuenta.
- CA: cada sucursal es independiente: pacientes, agenda, caja y equipo propios.
- CA: la suscripción es una sola, la de la matriz.

**HU-11.2 Cambiar de sucursal** (S)
Como **dueño de organización** quiero cambiar entre sucursales sin cerrar sesión.

**HU-11.3 Suspender y reactivar una sucursal** (S)
Como **dueño de organización** quiero dar de baja una sucursal sin perder sus datos y reactivarla después.

**HU-11.4 Reportes consolidados** (S)
Como **dueño de organización** quiero ver ventas, citas y pacientes de todas las sucursales juntas.

**HU-11.5 Administrador de sucursal** (S)
Como **dueño de organización** quiero poner un administrador en una sucursal sin que pueda crear otras sucursales ni ver las demás.
- CA: solo el dueño de la organización crea sucursales, cambia entre ellas, las suspende y ve reportes consolidados; los demás reciben `NOT_ORG_OWNER`.

---

## 12. Plataforma (equipo de Caresia)

**HU-12.1 Ver negocios** (M)
Como **admin de plataforma o soporte** quiero ver la lista de negocios con su plan, estado y último acceso para dar seguimiento.
- CA: puedo filtrar por estado (en prueba, activo, pago atrasado, prueba vencida, suspendido, sucursal suspendida) y buscar por negocio, dueño o correo.

**HU-12.2 Ver sucursales dentro de su matriz** (S)
Como **admin de plataforma** quiero ver las sucursales anidadas bajo su matriz, con su estado, para llevar mejor control.
- CA: una sucursal suspendida por su dueño aparece como «Sucursal suspendida».
- CA: el perfil de la matriz tiene una pestaña Sucursales.

**HU-12.3 Administrar la licencia** (M)
Como **admin de plataforma** quiero cambiar el plan, suspender, reactivar y registrar pagos manuales de un negocio.
- CA: soporte solo consulta; no cambia nada.

**HU-12.4 Actividad y equipo de plataforma** (S)
Como **admin de plataforma** quiero ver la historia de lo que se ha hecho con una cuenta y administrar a mi equipo.

**HU-12.5 Blog** (C)
Como **admin de plataforma** quiero publicar artículos para atraer pacientes y consultorios.

---

## 13. Calidad transversal (historias técnicas)

- **HU-13.1** Como **usuario** quiero que la interfaz funcione con teclado y lector de pantalla (controles con ARIA, foco visible).
- **HU-13.2** Como **usuario** quiero instalar Caresia como aplicación (PWA) y ver una página de «sin conexión» útil.
- **HU-13.3** Como **visitante** quiero ver las páginas públicas en español o inglés según mi navegador; la aplicación del consultorio es en español.
- **HU-13.4** Como **consultorio** quiero que los datos de un negocio nunca sean visibles para otro (aislamiento por `clinic_id` en cada consulta).
- **HU-13.5** Como **administrador** quiero respaldos y recuperación de datos (ver `docs/RESPALDOS.md`).

---

## Pendiente o fuera de alcance

- **Sincronización en ambos sentidos con Google Calendar** (hoy es solo un feed de lectura). Pendiente de decisiones.
- **Asistente de WhatsApp y avisos por WhatsApp**: fuera de alcance por ahora.
- **Certificación NOM-024 formal**: hay una auditoría de brechas en `docs/CUMPLIMIENTO-MX.md`; falta la validación de un tercero.
