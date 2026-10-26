<script lang="ts">
  // BORRADOR: ver el comentario de routes/privacidad/+page.svelte. El cuerpo del aviso vive aquí para mostrarse
  // tanto en la página /privacidad como dentro de una ventana (reserva en línea).
  import Fill from './Fill.svelte';
  import { cookieConsent } from '$lib/cookies.svelte';
  import { OPERATOR } from './operator';
</script>

<div class="legal-doc space-y-8 text-[15px] leading-relaxed">
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
    <p><strong>De quien visita el sitio público o nos escribe:</strong> datos técnicos de navegación (dirección IP, tipo de navegador y de dispositivo, páginas visitadas y desde dónde llegaste) mediante las cookies y tecnologías descritas en la sección 11; y, si solicitas una demostración o nos contactas, tu nombre, correo, teléfono, el nombre de tu consultorio y el contenido de tu mensaje.</p>
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
      <li>Atender tu solicitud de demostración o de información y darle seguimiento.</li>
    </ul>
    <p><strong>Finalidades secundarias (opcionales):</strong></p>
    <ul>
      <li>Enviarte novedades del producto, mejoras y recomendaciones de uso.</li>
      <li>Medir las visitas al sitio público y el resultado de nuestras campañas (Google Analytics), y mejorar la plataforma con estadísticas de uso agregadas.</li>
    </ul>
    <p>Si no quieres que tus datos se usen para las finalidades secundarias, escríbenos a <Fill v={OPERATOR.correoPrivacidad} />; negarte no afecta el servicio. Los datos de pacientes no se usan por Caresia para mercadotecnia.</p>
    <p><strong>Tu consentimiento:</strong> al crear tu cuenta y marcar la casilla de aceptación, consientes el tratamiento de tus datos conforme a este aviso. El consentimiento expreso para los datos sensibles de cada paciente lo recaba su consultorio, que es el responsable de esos datos.</p>
  </section>

  <section aria-labelledby="transferencias">
    <h2 id="transferencias" class="display text-2xl">6. Transferencias y proveedores</h2>
    <p>Caresia no vende datos personales. Para operar el servicio se apoya en proveedores que tratan datos por nuestra cuenta (encargados) bajo obligaciones de confidencialidad y seguridad:</p>
    <ul>
      <li><strong>Infraestructura y alojamiento:</strong> Vercel Inc. (Estados Unidos), que entrega el sitio web, y OVHcloud (Canadá), donde operan el servidor de la plataforma y la base de datos con sus respaldos.</li>
      <li><strong>Mercado Pago</strong>, para cobrar la suscripción y, si el consultorio lo activa, los pagos con terminal o enlace de pago.</li>
      <li><strong>Namecheap, Inc. (Private Email)</strong>, proveedor de correo electrónico, para enviar los mensajes de la plataforma (recuperación de contraseña, avisos y confirmaciones de citas) y responder a quien nos escribe.</li>
      <li><strong>Meta Platforms (WhatsApp Business)</strong>, solo si el consultorio activa los mensajes por WhatsApp, por ejemplo para confirmaciones o recordatorios de citas.</li>
      <li><strong>Facturama</strong> u otro proveedor autorizado de timbrado de CFDI, solo si el consultorio activa la facturación.</li>
      <li><strong>Google (Gemini)</strong>, solo si el consultorio usa las funciones de inteligencia artificial. En el inventario se envían las listas o fotos de productos. En las ayudas clínicas (resúmenes, borradores de notas, indicaciones, lecturas de estudios) se envían datos del expediente <strong>sin el nombre ni los datos de contacto del paciente</strong>: edad, sexo, alergias, medicación, notas y mediciones; con el dictado por voz, el audio que dicta el profesional, y con la comparación de fotos, las dos fotos elegidas. Esta información se envía solo en el momento de la solicitud; Caresia no guarda el audio. Lo que genera la IA es siempre un borrador o una sugerencia que el profesional revisa y decide.</li>
      <li><strong>Google LLC (Google Tag Manager y Google Analytics)</strong>, solo en las páginas públicas del sitio (portada, registro, inicio de sesión, aviso y términos), para medir visitas y campañas. Nunca se carga dentro de la plataforma ni recibe datos de pacientes.</li>
      <li><strong>Estadísticas de visitas propias y sin cookies (Umami)</strong>, alojadas en nuestro propio servidor.</li>
      <li><strong>Faro, de Hyperion Software (faro.hyperionmkt.com)</strong>, que escribe y aloja los artículos del blog en <code>/blog</code>. Al abrir el blog, tu navegador descarga su contenido desde sus servidores, que reciben tu dirección IP y datos técnicos del navegador. No se carga dentro de la plataforma ni recibe datos de pacientes.</li>
    </ul>
    <p>Algunos de estos proveedores tienen servidores fuera de México, principalmente en Estados Unidos y Canadá. En esos casos los datos se les envían solo para prestar el servicio, por nuestra cuenta, y con obligaciones de confidencialidad y seguridad.</p>
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
    <h2 id="cookies" class="display text-2xl">11. Cookies y tecnologías de rastreo</h2>
    <ul>
      <li><strong>Cookie de sesión (necesaria):</strong> mantiene abierta tu sesión al entrar a Caresia. No se usa para publicidad ni seguimiento.</li>
      <li><strong>Almacenamiento local del navegador:</strong> guarda preferencias de tu equipo, como el tema claro u oscuro o un borrador de venta.</li>
      <li><strong>Estadísticas propias sin cookies (Umami):</strong> contamos visitas al sitio público de forma agregada, sin cookies, sin perfiles personales y sin conservar los parámetros de la dirección.</li>
      <li><strong>Google Analytics, mediante Google Tag Manager (solo con tu consentimiento):</strong> solo en las páginas públicas (portada, registro, inicio de sesión, aviso, términos, directorio y blog), y solo si las aceptas en el aviso de cookies que aparece en tu primera visita. Mientras no las aceptes, Google Tag Manager no se carga. Usa cookies de Google Analytics (por ejemplo <code>_ga</code> y <code>_ga_…</code>) que asignan un identificador aleatorio a tu navegador para contar visitas, saber desde qué sitio o campaña llegaste y cuánto tiempo pasaste en la página. Google puede recibir tu dirección IP y datos técnicos de tu navegador, y tratarlos en servidores fuera de México. No se usan para identificarte por tu nombre y no se cargan dentro de la plataforma.</li>
    </ul>
    <p><strong>Cómo cambiar tu decisión:</strong> en cualquier momento desde <button type="button" class="underline underline-offset-4" onclick={() => cookieConsent.reopen()}>Preferencias de cookies</button> (también al pie de la portada). Si las rechazas después de haberlas aceptado, borramos las cookies de Google Analytics de este navegador. Además puedes bloquear o borrar las cookies desde la configuración de tu navegador, navegar en modo privado o instalar el complemento de inhabilitación de Google Analytics (<a href="https://tools.google.com/dlpage/gaoptout" target="_blank" rel="noopener">tools.google.com/dlpage/gaoptout</a>). Rechazarlas no te impide usar el sitio ni la plataforma; la cookie de sesión es la única necesaria para entrar.</p>
    <p>Hoy no usamos cookies publicitarias. Si agregamos alguna herramienta de publicidad o de medición de anuncios, actualizaremos este aviso antes de activarla.</p>
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
    <p>Si consideras que tu derecho a la protección de datos personales ha sido vulnerado, escríbenos primero a <Fill v={OPERATOR.correoPrivacidad} /> para resolverlo. Si no quedas conforme, puedes presentar tu queja o denuncia ante la <strong>Secretaría Anticorrupción y Buen Gobierno</strong>, autoridad en materia de protección de datos personales en posesión de los particulares conforme a la ley vigente.</p>
  </section>
</div>

<style>
  .legal-doc :global(h2) {
    margin-bottom: 0.6rem;
  }
  .legal-doc :global(p + p),
  .legal-doc :global(p + ul),
  .legal-doc :global(ul + p) {
    margin-top: 0.75rem;
  }
  .legal-doc :global(ul) {
    list-style: disc;
    padding-left: 1.25rem;
  }
  .legal-doc :global(li + li) {
    margin-top: 0.35rem;
  }
  .legal-doc :global(a) {
    text-decoration: underline;
  }
</style>
