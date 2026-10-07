<script lang="ts">
  // BORRADOR: este aviso de privacidad debe ser revisado por un abogado antes de publicarse como versión definitiva.
  // Los datos del operador (razón social, domicilio, correo de privacidad) viven en $lib/legal/operator.ts y están
  // marcados con corchetes mientras falten. No se inventan datos legales. Ver docs/AVISOS-REVISION-LEGAL.md.
  import Fill from '$lib/legal/Fill.svelte';
  import LegalPage from '$lib/legal/LegalPage.svelte';
  import { OPERATOR } from '$lib/legal/operator';

  const toc = [
    { id: 'responsable', label: 'Quién es el responsable' },
    { id: 'roles', label: 'Caresia y los consultorios: dos papeles distintos' },
    { id: 'datos', label: 'Datos que tratamos' },
    { id: 'sensibles', label: 'Datos sensibles (de salud)' },
    { id: 'finalidades', label: 'Finalidades primarias y secundarias' },
    { id: 'transferencias', label: 'Transferencias y proveedores' },
    { id: 'arco', label: 'Derechos ARCO y cómo ejercerlos' },
    { id: 'revocacion', label: 'Revocación del consentimiento y limitación del uso' },
    { id: 'seguridad', label: 'Medidas de seguridad' },
    { id: 'conservacion', label: 'Conservación de los datos' },
    { id: 'cookies', label: 'Cookies y analítica' },
    { id: 'menores', label: 'Menores de edad' },
    { id: 'cambios', label: 'Cambios a este aviso' },
    { id: 'autoridad', label: 'Autoridad y quejas' }
  ];
</script>

<LegalPage
  title="Aviso de privacidad"
  intro="Este aviso explica cómo Caresia, plataforma para consultorios y clínicas, trata datos personales, y cómo puedes ejercer tus derechos conforme a la Ley Federal de Protección de Datos Personales en Posesión de los Particulares (LFPDPPP) y su normatividad aplicable."
  {toc}>
  <section aria-labelledby="responsable">
    <h2 id="responsable" class="display text-2xl">1. Quién es el responsable</h2>
    <p><strong><Fill v={OPERATOR.razonSocial} /></strong> (en adelante, «Caresia») opera la plataforma Caresia y es responsable del tratamiento de los datos personales de las personas que crean una cuenta o usan la plataforma, con domicilio para oír y recibir notificaciones en <Fill v={OPERATOR.domicilio} />.</p>
    <p>Para cualquier asunto de privacidad puedes escribir a <Fill v={OPERATOR.correoPrivacidad} />.</p>
  </section>

  <section aria-labelledby="roles">
    <h2 id="roles" class="display text-2xl">2. Caresia y los consultorios: dos papeles distintos</h2>
    <p>Hay dos tipos de datos y cada uno tiene un responsable diferente:</p>
    <ul>
      <li><strong>Datos de cuentas y uso de la plataforma</strong> (las personas del consultorio que entran a Caresia): el responsable es Caresia, y este aviso te aplica directamente.</li>
      <li><strong>Datos de pacientes, propietarios de animales y sus expedientes</strong>: el responsable es <strong>cada consultorio o clínica</strong> que los captura. Caresia actúa como <strong>encargado</strong>: los almacena y procesa por cuenta y siguiendo las instrucciones del consultorio, sin usarlos para fines propios. Si eres paciente, el aviso de privacidad que importa para tu atención es el que te entrega tu consultorio.</li>
    </ul>
  </section>

  <section aria-labelledby="datos">
    <h2 id="datos" class="display text-2xl">3. Datos que tratamos</h2>
    <p><strong>De las personas que usan la plataforma:</strong> nombre, correo, usuario, contraseña (guardada de forma cifrada irreversible), rol, cédula profesional e institución cuando el profesional las registra, datos del consultorio (nombre, domicilio, teléfono, datos legales y fiscales), datos de suscripción y cobro, y registros técnicos de uso y seguridad (por ejemplo fecha y hora de acceso y dirección IP).</p>
    <p><strong>Por cuenta de los consultorios:</strong> datos de identificación y contacto de pacientes (nombre, fecha de nacimiento, sexo, CURP, teléfono, correo, domicilio, datos de su tutor o representante), citas, expediente clínico, recetas, archivos adjuntos, ventas y cobros, y, tratándose de animales, datos del propietario y del animal.</p>
    <p><strong>De quien envía una solicitud ARCO</strong> desde el formulario público de un consultorio: nombre, correo, teléfono opcional y el contenido de la solicitud, que se entregan al consultorio correspondiente para que la atienda.</p>
  </section>

  <section aria-labelledby="sensibles">
    <h2 id="sensibles" class="display text-2xl">4. Datos sensibles (de salud)</h2>
    <p>Los expedientes clínicos contienen <strong>datos personales sensibles</strong> (antecedentes, alergias, motivo de consulta, exploración, diagnósticos, tratamientos, recetas, resultados y archivos). Su tratamiento requiere el <strong>consentimiento expreso</strong> del titular, que recaba el consultorio al entregar su propio aviso de privacidad. Caresia solo accede a estos datos cuando es necesario para operar, mantener, proteger y dar soporte al servicio, y limita ese acceso al personal autorizado.</p>
  </section>

  <section aria-labelledby="finalidades">
    <h2 id="finalidades" class="display text-2xl">5. Finalidades primarias y secundarias</h2>
    <p><strong>Finalidades primarias (necesarias para el servicio):</strong></p>
    <ul>
      <li>Crear y administrar tu cuenta y la de tu consultorio, y autenticarte.</li>
      <li>Prestar la plataforma: agenda, expedientes, recetas, cobros, reportes, recordatorios y demás funciones contratadas.</li>
      <li>Gestionar suscripciones, pagos y facturación.</li>
      <li>Garantizar la seguridad, prevenir fraudes y abusos, y mantener bitácoras de actividad y de acceso.</li>
      <li>Dar soporte, atender solicitudes y cumplir obligaciones legales.</li>
    </ul>
    <p><strong>Finalidades secundarias (opcionales):</strong></p>
    <ul>
      <li>Enviarte novedades del producto, mejoras y recomendaciones de uso.</li>
      <li>Medir el uso del sitio de forma agregada para mejorar la plataforma.</li>
    </ul>
    <p>Si no quieres que tus datos se usen para las finalidades secundarias, escríbenos a <Fill v={OPERATOR.correoPrivacidad} />; negarte no afecta el servicio. Los datos de pacientes no se usan por Caresia para mercadotecnia.</p>
  </section>

  <section aria-labelledby="transferencias">
    <h2 id="transferencias" class="display text-2xl">6. Transferencias y proveedores</h2>
    <p>Caresia no vende datos personales. Para operar el servicio se apoya en proveedores que tratan datos por nuestra cuenta (encargados) bajo obligaciones de confidencialidad y seguridad:</p>
    <ul>
      <li><strong>Infraestructura y alojamiento</strong> del sitio y de la base de datos: <Fill v="[PROVEEDORES DE HOSPEDAJE A CONFIRMAR]" />.</li>
      <li><strong>Mercado Pago</strong>, para cobrar la suscripción y, si el consultorio lo activa, los pagos con terminal o enlace de pago.</li>
      <li><strong>Proveedor de correo electrónico (SMTP)</strong>, para enviar mensajes de la plataforma (recuperación de contraseña, avisos y confirmaciones de citas).</li>
      <li><strong>WhatsApp (Meta)</strong>, solo si el consultorio activa los recordatorios por ese medio.</li>
      <li><strong>Facturama</strong> u otro proveedor autorizado de timbrado de CFDI, solo si el consultorio activa la facturación.</li>
      <li><strong>Google (Gemini)</strong>, solo si el consultorio usa las funciones de «magia» del inventario: se envían las listas o fotos de productos que el consultorio sube, no datos de pacientes.</li>
      <li><strong>Herramienta de estadísticas de visitas sin cookies</strong> y <strong>Google Fonts</strong> (tipografías) para el sitio público.</li>
    </ul>
    <p>Además, los datos pueden comunicarse a autoridades competentes cuando una ley o una orden fundada y motivada lo exija. Estas transferencias no requieren tu consentimiento adicional. Si se prevé otra transferencia, se solicitará tu consentimiento.</p>
  </section>

  <section aria-labelledby="arco">
    <h2 id="arco" class="display text-2xl">7. Derechos ARCO y cómo ejercerlos</h2>
    <p>Tienes derecho a <strong>Acceder</strong> a tus datos, <strong>Rectificarlos</strong>, solicitar su <strong>Cancelación</strong> y <strong>Oponerte</strong> a su uso (derechos ARCO).</p>
    <ul>
      <li><strong>Datos de tu atención como paciente:</strong> preséntala a tu consultorio. Cada consultorio que usa Caresia cuenta con un <strong>formulario público de solicitudes ARCO</strong> (la dirección termina en <code>/arco/…</code> y te la proporciona el consultorio o aparece en su aviso de privacidad). Con el formulario recibes un folio y un enlace para consultar el estado.</li>
      <li><strong>Datos de tu cuenta en la plataforma:</strong> escribe a <Fill v={OPERATOR.correoPrivacidad} /> indicando tu nombre, un medio para responderte, la descripción clara de lo que solicitas y una copia de una identificación.</li>
    </ul>
    <p>La respuesta sobre si la solicitud procede se comunica en un plazo máximo de 20 días hábiles contados desde que se recibe, y, si procede, se hace efectiva en los 15 días hábiles siguientes, salvo que la ley aplicable establezca otra cosa. Es posible que se te pida acreditar tu identidad.</p>
    <p><strong>Cancelación de expedientes clínicos:</strong> la NOM-004-SSA3-2012 obliga a conservar el expediente clínico al menos 5 años desde el último acto médico. Durante ese periodo los datos se <strong>bloquean</strong> (no se usan para ningún otro fin) y se suprimen al concluirlo.</p>
  </section>

  <section aria-labelledby="revocacion">
    <h2 id="revocacion" class="display text-2xl">8. Revocación del consentimiento y limitación del uso</h2>
    <p>Puedes revocar el consentimiento que nos diste o pedir que limitemos el uso o la divulgación de tus datos escribiendo a <Fill v={OPERATOR.correoPrivacidad} /> (o, si eres paciente, a tu consultorio con el mismo formulario de la sección anterior). La revocación no tiene efectos retroactivos y no puede aplicarse cuando los datos son necesarios para cumplir obligaciones legales, como la conservación del expediente. Algunas revocaciones pueden implicar que no podamos seguir prestándote el servicio.</p>
  </section>

  <section aria-labelledby="seguridad">
    <h2 id="seguridad" class="display text-2xl">9. Medidas de seguridad</h2>
    <p>Aplicamos medidas administrativas, técnicas y físicas razonables para proteger los datos, entre ellas:</p>
    <ul>
      <li>Conexiones cifradas (HTTPS) y contraseñas almacenadas con funciones de resumen criptográfico (<em>hash</em>).</li>
      <li>Permisos por rol: cada persona del consultorio ve solo lo que su función requiere (por ejemplo, la recepción no ve datos clínicos).</li>
      <li>Separación de los datos de cada consultorio y sesiones que se cierran al cambiar la contraseña o desactivar una cuenta; verificación en dos pasos opcional.</li>
      <li>Cifrado en reposo (AES-256-GCM) de campos clínicos sensibles, archivos adjuntos y credenciales de pago.</li>
      <li>Bitácoras de actividad y de acceso a expedientes, y expedientes que no se pueden borrar ni alterar: las correcciones se hacen con adendas.</li>
    </ul>
    <p>Ningún sistema es infalible. Si ocurre una vulneración que afecte de forma significativa tus derechos, te la informaremos con la oportunidad y por los medios que la ley establezca.</p>
  </section>

  <section aria-labelledby="conservacion">
    <h2 id="conservacion" class="display text-2xl">10. Conservación de los datos</h2>
    <ul>
      <li><strong>Expedientes clínicos:</strong> al menos 5 años desde el último acto médico (NOM-004-SSA3-2012), responsabilidad del consultorio; Caresia los conserva mientras el consultorio mantenga su cuenta y durante el periodo posterior que el contrato prevea para que pueda exportarlos.</li>
      <li><strong>Datos de cuenta y facturación:</strong> mientras dure la relación y el tiempo que exijan las obligaciones fiscales y legales.</li>
      <li><strong>Bitácoras de seguridad:</strong> el tiempo necesario para proteger el servicio y atender requerimientos legales.</li>
    </ul>
  </section>

  <section aria-labelledby="cookies">
    <h2 id="cookies" class="display text-2xl">11. Cookies y analítica</h2>
    <ul>
      <li><strong>Cookie de sesión (necesaria):</strong> mantiene abierta tu sesión al entrar a Caresia. No se usa para publicidad ni seguimiento.</li>
      <li><strong>Almacenamiento local del navegador:</strong> guarda preferencias de tu equipo, como el tema claro u oscuro o un borrador de venta.</li>
      <li><strong>Estadísticas de visitas sin cookies:</strong> contamos visitas al sitio público de forma agregada, sin cookies, sin perfiles personales y sin conservar los parámetros de la dirección.</li>
      <li><strong>Tipografías de Google Fonts:</strong> al cargar la página, tu navegador las solicita a Google, que puede ver tu dirección IP.</li>
    </ul>
    <p>Caresia no usa cookies publicitarias ni de terceros para perfilarte.</p>
  </section>

  <section aria-labelledby="menores">
    <h2 id="menores" class="display text-2xl">12. Menores de edad</h2>
    <p>La plataforma está dirigida a profesionales de la salud y personal de consultorios. Los datos de pacientes menores de edad o que no pueden decidir por sí mismos los captura el consultorio con la intervención de su madre, padre, tutor o representante legal.</p>
  </section>

  <section aria-labelledby="cambios">
    <h2 id="cambios" class="display text-2xl">13. Cambios a este aviso</h2>
    <p>Podemos actualizar este aviso por cambios legales, del servicio o de nuestras prácticas. La versión vigente estará siempre en esta página, con su fecha de actualización, y los cambios importantes se avisarán por correo o dentro de la plataforma.</p>
  </section>

  <section aria-labelledby="autoridad">
    <h2 id="autoridad" class="display text-2xl">14. Autoridad y quejas</h2>
    <p>Si consideras que tu derecho a la protección de datos personales ha sido vulnerado, puedes acudir primero con nosotros y, si no quedas conforme, presentar tu queja o denuncia ante la autoridad de protección de datos personales competente conforme a la ley vigente <Fill v="[DENOMINACIÓN VIGENTE DE LA AUTORIDAD, A CONFIRMAR POR EL ABOGADO]" />.</p>
  </section>
</LegalPage>
