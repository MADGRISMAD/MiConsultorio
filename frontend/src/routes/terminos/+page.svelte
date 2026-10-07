<script lang="ts">
  // BORRADOR: estos términos deben ser revisados por un abogado antes de publicarse como versión definitiva.
  // Los datos del operador (razón social, domicilio, jurisdicción) viven en $lib/legal/operator.ts y están
  // marcados con corchetes mientras falten. No se inventan datos legales. Ver docs/AVISOS-REVISION-LEGAL.md.
  import Fill from '$lib/legal/Fill.svelte';
  import LegalPage from '$lib/legal/LegalPage.svelte';
  import { OPERATOR } from '$lib/legal/operator';

  const toc = [
    { id: 'aceptacion', label: 'Quién ofrece el servicio y aceptación' },
    { id: 'servicio', label: 'Qué es Caresia y qué no es' },
    { id: 'cuentas', label: 'Cuentas, roles y seguridad de acceso' },
    { id: 'profesional', label: 'Responsabilidad profesional del consultorio' },
    { id: 'datos', label: 'Datos personales y de pacientes' },
    { id: 'planes', label: 'Planes, prueba y pagos' },
    { id: 'uso', label: 'Uso aceptable' },
    { id: 'disponibilidad', label: 'Disponibilidad, respaldos y cambios del servicio' },
    { id: 'propiedad', label: 'Propiedad intelectual' },
    { id: 'terminacion', label: 'Suspensión, terminación y exportación de datos' },
    { id: 'responsabilidad', label: 'Limitación de responsabilidad' },
    { id: 'cambios', label: 'Cambios a estos términos' },
    { id: 'ley', label: 'Ley aplicable y jurisdicción' },
    { id: 'contacto', label: 'Contacto' }
  ];
</script>

<LegalPage
  title="Términos de uso"
  intro="Estas condiciones regulan el uso de Caresia, el software para consultorios y clínicas de salud. Léelas con atención: al crear una cuenta o usar la plataforma las aceptas."
  {toc}>
  <section aria-labelledby="aceptacion">
    <h2 id="aceptacion" class="display text-2xl">1. Quién ofrece el servicio y aceptación</h2>
    <p>Caresia es operada por <strong><Fill v={OPERATOR.razonSocial} /></strong>, con domicilio en <Fill v={OPERATOR.domicilio} /> (el «Operador»). «Tú» o «el consultorio» es la persona física o moral que contrata el servicio y las personas a quienes da acceso. Si contratas en nombre de una empresa o clínica, declaras tener facultades para obligarla.</p>
  </section>

  <section aria-labelledby="servicio">
    <h2 id="servicio" class="display text-2xl">2. Qué es Caresia y qué no es</h2>
    <p>Caresia es una herramienta de gestión: agenda, expedientes, recetas, cobros, reportes y comunicación con pacientes. <strong>No presta servicios médicos, odontológicos, veterinarios ni de otra profesión de la salud</strong>, no emite diagnósticos ni sustituye el criterio profesional.</p>
    <p>La plataforma incluye catálogos de referencia (medicamentos, dosis, códigos CIE-10, esquemas de vacunación, textos de consentimientos y de avisos). Son <strong>material de apoyo</strong>, pueden contener errores o quedar desactualizados y <strong>deben ser validados por un profesional con cédula</strong> antes de usarse con un paciente.</p>
    <p>Caresia no está certificado bajo la NOM-024-SSA3-2012 y no firma electrónicamente notas ni recetas, salvo que se indique expresamente lo contrario en la plataforma.</p>
  </section>

  <section aria-labelledby="cuentas">
    <h2 id="cuentas" class="display text-2xl">3. Cuentas, roles y seguridad de acceso</h2>
    <ul>
      <li>La información que registres (tuya, de tu consultorio y de tu equipo) debe ser veraz y estar actualizada.</li>
      <li>El administrador del consultorio decide quién tiene acceso y con qué rol, y debe desactivar de inmediato las cuentas de quien deje de colaborar.</li>
      <li>Cada persona debe usar su propia cuenta, con una contraseña robusta y, de preferencia, la verificación en dos pasos. No compartas credenciales.</li>
      <li>Eres responsable de la actividad realizada con las cuentas de tu consultorio. Avísanos pronto si sospechas un acceso no autorizado.</li>
    </ul>
  </section>

  <section aria-labelledby="profesional">
    <h2 id="profesional" class="display text-2xl">4. Responsabilidad profesional del consultorio</h2>
    <p>El consultorio es el único responsable de la atención que brinda, del contenido de los expedientes, recetas e indicaciones, de contar con título, cédula profesional y los permisos o avisos de funcionamiento que su actividad exige, de recabar los consentimientos informados necesarios y de cumplir las normas aplicables a su giro (entre otras, la Ley General de Salud, la NOM-004-SSA3-2012 y, en veterinaria, la normatividad zoosanitaria). Caresia ofrece funciones para facilitarlo, pero no garantiza que el uso del software, por sí solo, cumpla con esas normas.</p>
  </section>

  <section aria-labelledby="datos">
    <h2 id="datos" class="display text-2xl">5. Datos personales y de pacientes</h2>
    <p>Respecto de los datos de pacientes y propietarios, <strong>el consultorio es el responsable</strong> del tratamiento (LFPDPPP) y el Operador actúa como <strong>encargado</strong>: los trata únicamente para prestar el servicio y siguiendo tus instrucciones. Eres responsable de contar con un aviso de privacidad, de obtener el consentimiento expreso para datos sensibles y de atender las solicitudes ARCO de tus pacientes (Caresia ofrece un formulario público y un tablero de gestión de solicitudes para ayudarte). Estos términos funcionan como el contrato de encargo entre tú y el Operador; si tu asesor requiere un instrumento adicional, solicítalo a <Fill v={OPERATOR.correoPrivacidad} />.</p>
    <p>El tratamiento de datos de las personas que usan la plataforma se describe en el <a href="/privacidad">Aviso de privacidad</a>.</p>
  </section>

  <section aria-labelledby="planes">
    <h2 id="planes" class="display text-2xl">6. Planes, prueba y pagos</h2>
    <ul>
      <li>Caresia ofrece planes de suscripción mensual o anual y un periodo de prueba, con las funciones y precios vigentes al momento de contratar, que se muestran en la plataforma.</li>
      <li>Los pagos se procesan mediante Mercado Pago u otros medios que se indiquen. Los precios pueden incluir o no impuestos según se señale.</li>
      <li>La suscripción se renueva al terminar cada periodo mientras no la canceles. Los cambios de precio se avisarán con anticipación razonable.</li>
      <li>Si la suscripción no está activa, el acceso a los datos puede limitarse hasta regularizar el pago; no se borran datos por ese motivo durante el plazo previsto en la sección 10.</li>
      <li>Reembolsos y facturación: <Fill v="[POLÍTICA DE REEMBOLSOS A DEFINIR POR EL OPERADOR]" />.</li>
    </ul>
  </section>

  <section aria-labelledby="uso">
    <h2 id="uso" class="display text-2xl">7. Uso aceptable</h2>
    <p>No puedes: usar Caresia para fines ilícitos; capturar datos de personas sin tener derecho a hacerlo; intentar acceder a datos de otros consultorios; vulnerar, sobrecargar o hacer ingeniería inversa de la plataforma; ni enviar comunicaciones masivas no solicitadas (spam) con las herramientas de correo o mensajería. El Operador puede suspender cuentas que incumplan esta sección.</p>
  </section>

  <section aria-labelledby="disponibilidad">
    <h2 id="disponibilidad" class="display text-2xl">8. Disponibilidad, respaldos y cambios del servicio</h2>
    <p>Procuramos que el servicio esté disponible de forma continua, pero puede interrumpirse por mantenimiento, fallas de terceros o causas de fuerza mayor. Realizamos respaldos operativos, pero te recomendamos conservar tus propias exportaciones periódicas de la información que debas resguardar por ley. Podemos mejorar, modificar o retirar funciones; si un cambio afecta de forma sustancial lo contratado, te avisaremos.</p>
  </section>

  <section aria-labelledby="propiedad">
    <h2 id="propiedad" class="display text-2xl">9. Propiedad intelectual</h2>
    <p>El software, su diseño, marcas y contenidos propios son del Operador o de sus licenciantes; te concedemos una licencia limitada, revocable, no exclusiva e intransferible para usarlo mientras dure tu suscripción. Los datos y documentos que capturas siguen siendo tuyos (o de tus pacientes, según corresponda).</p>
  </section>

  <section aria-labelledby="terminacion">
    <h2 id="terminacion" class="display text-2xl">10. Suspensión, terminación y exportación de datos</h2>
    <p>Puedes dejar de usar Caresia cuando quieras. Antes de terminar, exporta la información que necesites (por ejemplo, los expedientes en formato de exportación y los reportes). El Operador puede suspender o terminar el servicio por incumplimiento de estos términos o falta de pago, con aviso previo cuando sea posible. Tras la terminación conservaremos los datos durante <Fill v="[PLAZO DE CONSERVACIÓN POSTERIOR, A DEFINIR]" /> para que puedas recuperarlos, y después los suprimiremos o bloquearemos conforme a la ley; recuerda que los expedientes clínicos deben conservarse al menos 5 años desde el último acto médico (NOM-004-SSA3-2012) y esa obligación es del consultorio.</p>
  </section>

  <section aria-labelledby="responsabilidad">
    <h2 id="responsabilidad" class="display text-2xl">11. Limitación de responsabilidad</h2>
    <p>En la medida que la ley lo permita, el Operador no responde por daños derivados de decisiones clínicas, de la atención a pacientes, del uso indebido de la plataforma, de información capturada incorrectamente ni de interrupciones atribuibles a terceros o a caso fortuito. Su responsabilidad total frente a ti por el servicio se limitará, salvo dolo o lo que la ley no permita limitar, a <Fill v="[LÍMITE DE RESPONSABILIDAD A DEFINIR, POR EJEMPLO LO PAGADO EN LOS ÚLTIMOS 12 MESES]" />. Nada de lo anterior limita derechos irrenunciables que la ley te otorgue como consumidor, si te aplican.</p>
  </section>

  <section aria-labelledby="cambios">
    <h2 id="cambios" class="display text-2xl">12. Cambios a estos términos</h2>
    <p>Podemos actualizar estos términos. Publicaremos la versión vigente en esta página con su fecha y avisaremos los cambios importantes por correo o dentro de la plataforma. Si continúas usando Caresia después del aviso, aceptas la nueva versión; si no estás de acuerdo, puedes cancelar tu suscripción.</p>
  </section>

  <section aria-labelledby="ley">
    <h2 id="ley" class="display text-2xl">13. Ley aplicable y jurisdicción</h2>
    <p>Estos términos se rigen por las leyes de los Estados Unidos Mexicanos. Para su interpretación y cumplimiento, las partes se someten a los tribunales competentes de <Fill v={OPERATOR.jurisdiccion} />, renunciando a cualquier otro fuero que pudiera corresponderles, salvo que la ley disponga otra cosa.</p>
  </section>

  <section aria-labelledby="contacto">
    <h2 id="contacto" class="display text-2xl">14. Contacto</h2>
    <p>Dudas sobre estos términos o el servicio: <Fill v={OPERATOR.correoPrivacidad} /> · <Fill v={OPERATOR.telefono} />.</p>
  </section>
</LegalPage>
